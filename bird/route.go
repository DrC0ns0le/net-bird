package bird

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

type Route struct {
	Network  *net.IPNet
	Paths    []BGPPath
	OriginAS int
}
type BGPPath struct {
	AS              int
	ASPath          []int
	Next            net.IP
	Interface       string
	MED             int
	LocalPreference int
	OriginType      string
}

func GetRoutes(mode string) ([]Route, error) {
	conn, reader, writer, err := Begin(mode)
	if err != nil {
		return nil, err
	}

	defer conn.Close()

	// Send command to show routes
	_, err = writer.WriteString("show route all\n")
	if err != nil {
		return nil, err
	}
	writer.Flush()

	var line string
	var route Route

	var routes []Route
	var bgpPath BGPPath

	var isBGP bool
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "0000") {
			break // End of data
		}

		if strings.HasPrefix(line, "1007-") {

			if bgpPath.ASPath != nil {
				route.Paths = append(route.Paths, bgpPath)
				bgpPath = BGPPath{}
			}

			// split to fields separated by space
			splits := strings.Fields(line)
			rt := strings.TrimPrefix(splits[0], "1007-")

			// check if this line is start of a new route
			if rt != "" {
				// append previous completed route
				if route.Network != nil {
					routes = append(routes, route)

					route = Route{}
				}

				// check if this route is a BGP route
				nextLine, err := reader.ReadString('\n')
				if err != nil {
					return nil, err
				}
				var typeFields []string
				if strings.HasPrefix(nextLine, "1008-	Type:") {
					typeFields = strings.Fields(strings.SplitN(nextLine, ":", 2)[1])
				}
				if len(typeFields) > 0 && strings.TrimSpace(typeFields[0]) == "BGP" {
					isBGP = true
					_, ipnet, _ := net.ParseCIDR(rt)
					route.Network = ipnet

					if route.OriginAS == 0 {
						originAS := splits[len(splits)-1]
						if originAS != "" {
							// Use regex to extract only digits
							re := regexp.MustCompile(`\d+`)
							match := re.FindString(originAS)

							// Parse the matched string to an integer
							if match != "" {
								originASint, err := strconv.Atoi(match)
								if err != nil {
									fmt.Println("Error parsing number:", err)
									return nil, err
								}
								route.OriginAS = originASint
							}
						}
					}
					bgpPath.Next = net.ParseIP(splits[2])
					bgpPath.Interface = splits[4]
				} else {
					isBGP = false
				}

			} else if isBGP {
				// This line is part of the previous BGP route
				bgpPath.Next = net.ParseIP(splits[2])
				bgpPath.Interface = splits[4]
			}
		}

		if (strings.HasPrefix(line, "1012-") || strings.HasPrefix(strings.TrimSpace(line), "BGP.")) && isBGP {
			halfSplit := strings.SplitN(line, "BGP.", 2)
			split := strings.SplitN(halfSplit[1], ":", 2)

			switch split[0] {
			case "origin":
				bgpPath.OriginType = strings.TrimSpace(split[1])
			case "next_hop":
				bgpPath.Next = net.ParseIP(strings.TrimSpace(split[1]))
			case "as_path":
				if split[1] != "" {
					sp := strings.Split(strings.TrimSpace(split[1]), " ")

					for i, as := range sp {
						asInt, err := strconv.Atoi(strings.TrimSpace(as))
						if err != nil {
							fmt.Printf("Error parsing path number: %v, got '%s'\n", err, line)
							return nil, err
						}
						bgpPath.ASPath = append(bgpPath.ASPath, asInt)

						if i == 0 {
							bgpPath.AS = asInt
						}
					}
				} else {
					bgpPath.ASPath = make([]int, 0)
				}
			case "local_pref":
				bgpPath.LocalPreference, err = strconv.Atoi(strings.TrimSpace(split[1]))
				if err != nil {
					fmt.Printf("Error parsing path number: %v, got '%s'\n", err, line)
					return nil, err
				}
			case "med":
				bgpPath.MED, err = strconv.Atoi(strings.TrimSpace(split[1]))
				if err != nil {
					fmt.Printf("Error parsing path number: %v, got '%s'\n", err, line)
					return nil, err
				}
			}
		}
	}

	return routes, nil
}
