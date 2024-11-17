package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/DrC0ns0le/net-bird/bird"
	"github.com/DrC0ns0le/net-bird/cost"
	"github.com/DrC0ns0le/net-bird/logging"
	"github.com/DrC0ns0le/net-bird/utils"
	"github.com/vishvananda/netlink"
)

var (
	config bird.Config

	updateRoutes = flag.Bool("u", false, "Update routes")
	showInfo     = flag.Bool("i", false, "Show all routes info")
	showRoutes   = flag.Bool("s", false, "Show managed routes")
	daemonMode   = flag.Bool("d", false, "Run in daemon mode")

	debugLevel = flag.Bool("debug", false, "Debug level")
)

func main() {

	flag.Parse()

	if *debugLevel {
		logging.SetLevel(slog.LevelDebug)
	}

	var err error
	config, err = bird.ParseBirdConfig("/etc/bird/bird.conf")
	if err != nil {
		logging.Errorf("Error parsing BIRD config: %v\n", err)
	}

	if *daemonMode {
		utils.RemoveAllManagedRoutes()
		for {
			logging.Infof("Running in daemon mode...")
			run()
			// sleep for 2 minutes
			time.Sleep(2 * time.Minute)
		}
	} else {
		run()
	}

}

func run() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, mode := range []string{"v4", "v6"} {
		routes, err := bird.GetRoutes(mode)
		if err != nil {
			logging.Errorf("Error getting routes: %v\n", err)
		}

		if *updateRoutes {
			UpdateRoutes(ctx, routes, mode)
		}

		if *showInfo {
			ShowAllInfoTable(ctx, routes)
		}
	}

	if *showRoutes {
		err := ShowManagedRoutes()
		if err != nil {
			logging.Errorf("failed to get managed routes: %v", err)
		}
	}
}

func ShowAllInfoTable(ctx context.Context, routes []bird.Route) {
	// Define table formatting constants
	var (
		headerFormat = "| %-20s | %-10s | %-8s | %-40s | %-15s | %-12s | %-6s | %-10s | %-12s | %-10s |\n"
		rowFormat    = "| %-20s | %-10d | %-8d | %-40s | %-15s | %-12s | %-6d | %-10d | %-12s | %-10.2f |\n"
		separator    = strings.Repeat("-", 174)
	)

	// Print header
	fmt.Println(separator)
	fmt.Printf("Local AS Number: %d\n", config.ASNumber)
	fmt.Println(separator)

	// Print table header
	fmt.Printf(headerFormat,
		"Network",
		"Origin AS",
		"Via AS",
		"AS Path",
		"Next Hop",
		"Interface",
		"MED",
		"Local Pref",
		"Origin Type",
		"Total Cost",
	)
	fmt.Println(separator)

	// Helper function to convert AS path to string
	asPathToString := func(asPath []int) string {
		strPath := make([]string, len(asPath))
		for i, as := range asPath {
			strPath[i] = fmt.Sprintf("%d", as)
		}
		return strings.Join(strPath, " -> ")
	}

	// Process and print each route
	for i, route := range routes {
		// If not the first route, print a route separator
		if i > 0 {
			fmt.Println(separator)
		}

		// If route has no paths, print empty row
		if len(route.Paths) == 0 {
			fmt.Printf(rowFormat,
				route.Network.String(),
				route.OriginAS,
				0,
				"",
				"",
				"",
				0,
				0,
				"",
				0.0,
			)
			continue
		}

		// Create slice of paths with their costs for sorting
		type pathWithCost struct {
			path bird.BGPPath
			cost float64
		}
		pathsWithCosts := make([]pathWithCost, len(route.Paths))

		// Calculate costs for all paths
		for i, path := range route.Paths {
			totalCost := CalculateTotalCost(ctx, path.ASPath, config.ASNumber)
			pathsWithCosts[i] = pathWithCost{
				path: path,
				cost: totalCost,
			}
		}

		// Sort paths by cost
		sort.Slice(pathsWithCosts, func(i, j int) bool {
			// Handle infinity cases
			if math.IsInf(pathsWithCosts[i].cost, 1) {
				return false
			}
			if math.IsInf(pathsWithCosts[j].cost, 1) {
				return true
			}
			return pathsWithCosts[i].cost < pathsWithCosts[j].cost
		})

		// Print sorted paths
		for _, pwc := range pathsWithCosts {
			fmt.Printf(rowFormat,
				route.Network.String(),
				route.OriginAS,
				pwc.path.AS,
				asPathToString(pwc.path.ASPath),
				pwc.path.Next.String(),
				pwc.path.Interface,
				pwc.path.MED,
				pwc.path.LocalPreference,
				pwc.path.OriginType,
				pwc.cost,
			)
		}
	}
	fmt.Println(separator)
}

func UpdateRoutes(ctx context.Context, routes []bird.Route, mode string) {
	outboundV4, outboundV6, err := utils.GetOutboundIPs()
	if err != nil {
		logging.Errorf("Error getting outbound IP: %v\n", err)
		os.Exit(1)
	}
	for _, route := range routes {
		if len(route.Paths) == 0 {
			continue
		}
		var chosenPathIndex int
		minCost := math.Inf(1)
		for i, path := range route.Paths {
			totalCost := CalculateTotalCost(ctx, path.ASPath, config.ASNumber)
			if totalCost < minCost {
				minCost = totalCost
				chosenPathIndex = i
			}
		}

		if len(route.Paths[chosenPathIndex].ASPath) == 0 || route.Paths[chosenPathIndex].ASPath[0] == config.ASNumber {
			if err := utils.RemoveRoute(route.Network); err != nil {
				logging.Errorf("Error removing route for network %s: %v\n", route.Network, err)
				os.Exit(1)
			}
		}

		if len(route.Paths[chosenPathIndex].ASPath) == 0 {
			continue
		} else if err := utils.ConfigureRoute(route.Network, route.Paths[chosenPathIndex].Next, func() net.IP {
			if mode == "v6" {
				return outboundV6
			} else {
				return outboundV4
			}
		}(),
		); err != nil {
			logging.Errorf("Error configuring route for network %s: %v\n", route.Network, err)
			os.Exit(1)
		}
	}
}

func ShowManagedRoutes() error {
	routes, err := utils.ListManagedRoutes()
	if err != nil {
		return fmt.Errorf("failed to get managed routes: %v", err)
	}

	if len(routes) == 0 {
		fmt.Println("No managed routes found")
		return nil
	}

	// Define column headers and format
	format := "%-20s %-15s %-10s\n"
	fmt.Println(strings.Repeat("-", 47))
	fmt.Printf(format, "DESTINATION", "GATEWAY", "INTERFACE")
	fmt.Println(strings.Repeat("-", 47))

	for _, route := range routes {
		// Handle nil destination (default route)
		dst := "default"
		if route.Dst != nil {
			dst = route.Dst.String()
		}

		// Handle nil gateway
		gw := "none"
		if route.Gw != nil {
			gw = route.Gw.String()
		}

		// Get interface name instead of just index
		intf, err := netlink.LinkByIndex(route.LinkIndex)
		if err != nil {
			intf = nil
		}
		intfName := "unknown"
		if intf != nil {
			intfName = intf.Attrs().Name
		}

		fmt.Printf(format, dst, gw, intfName)
	}

	return nil
}

// CalculateTotalCost returns the total cost of a BGP path given its AS path and the local AS number.
// The total cost is calculated as the sum of the costs of each hop in the path, plus 10000 for each hop.
// If any of the intermediate costs are infinite, the total cost is set to infinity and returned.
func CalculateTotalCost(ctx context.Context, asPath []int, localAS int) float64 {
	var totalCost float64
	for i, as := range asPath {
		var c float64
		if i > 0 {
			c = cost.GetPathCost(ctx, asPath[i-1], as)
		} else {
			c = cost.GetPathCost(ctx, localAS, as)
		}
		if c == math.Inf(1) {
			return math.Inf(1)
		}
		totalCost += c + 10000
	}
	return totalCost
}
