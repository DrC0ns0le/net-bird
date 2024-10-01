package bird

import (
	"fmt"
	"net"
	"os"
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

func GetRoutes() ([]Route, error) {
	conn, reader, writer, err := Begin()
	if err != nil {
		return nil, err
	}

	defer conn.Close()

	// Send command to show routes
	_, err = writer.WriteString("show route all\n")
	if err != nil {
		fmt.Printf("Error writing to BIRD socket: %v\n", err)
		os.Exit(1)
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
				if len(splits) == 10 {
					isBGP = true
					_, ipnet, _ := net.ParseCIDR(rt)
					route.Network = ipnet

					if route.OriginAS == 0 {
						originAS := splits[9]
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

		if strings.HasPrefix(line, "1012-") && isBGP {
			// if EBGP or IBGP
			split := strings.Split(line, ":")
			bgpPath.OriginType = strings.TrimSpace(split[1])

			// read AS path
			line, err = reader.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimSpace(line)
			split = strings.Split(line, ":")
			if len(split) == 2 {
				asPathString := strings.TrimSpace(split[1])

				if asPathString != "" {
					split = strings.Split(asPathString, " ")

					for i, as := range split {
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
				}

			}

			// read next hop
			line, err = reader.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimSpace(line)
			split = strings.Split(line, ":")
			if len(split) == 2 {
				nextHop := strings.TrimSpace(split[1])
				if nextHop != "" {
					bgpPath.Next = net.ParseIP(nextHop)
				}
			}

			// read MED
			if bgpPath.OriginType == "BGP" {
				line, err = reader.ReadString('\n')
				if err != nil {
					break
				}
				line = strings.TrimSpace(line)
				split = strings.Split(line, ":")
				if len(split) == 2 {
					med := strings.TrimSpace(split[1])
					if med != "" {
						bgpPath.MED, err = strconv.Atoi(med)
						if err != nil {
							fmt.Println("Error parsing MED number:", err)
							return nil, err
						}
					}
				}
			}

			// read local preference
			line, err = reader.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimSpace(line)
			split = strings.Split(line, ":")
			if len(split) == 2 {
				localPreference := strings.TrimSpace(split[1])
				if localPreference != "" {
					bgpPath.LocalPreference, err = strconv.Atoi(localPreference)
					if err != nil {
						fmt.Println("Error parsing local preference number:", err)
						return nil, err
					}
				}
			}

			route.Paths = append(route.Paths, bgpPath)
			bgpPath = BGPPath{}
		}
	}

	return routes, nil
}
