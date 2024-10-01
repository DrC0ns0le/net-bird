package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/DrC0ns0le/net-bird/bird"
)

func main() {

	fmt.Println(strings.Repeat("-", 40))
	routes, err := bird.GetRoutes()
	if err != nil {
		fmt.Printf("Error getting routes: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(strings.Repeat("-", 40))
	for _, route := range routes {
		fmt.Printf("Network: %s\n", route.Network)
		fmt.Printf("Origin AS: %d\n", route.OriginAS)
		fmt.Printf("Paths:\n")
		for i, path := range route.Paths {
			fmt.Printf("  Path %d:\n", i+1)
			fmt.Printf("    Via: %d\n", path.AS)
			fmt.Printf("    Path: -> %s\n", func(asPath []int) string {
				strPath := make([]string, len(asPath))
				for i, as := range asPath {
					strPath[i] = fmt.Sprintf("%d", as)
				}
				return strings.Join(strPath, " -> ")
			}(path.ASPath))
			fmt.Printf("    Next Hop: %s\n", path.Next)
			fmt.Printf("    Interface: %s\n", path.Interface)
			fmt.Printf("    MED: %d\n", path.MED)
			fmt.Printf("    Local Preference: %d\n", path.LocalPreference)
			fmt.Printf("    Origin Type: %s\n", path.OriginType)
		}
		fmt.Println(strings.Repeat("-", 40))
	}
}
