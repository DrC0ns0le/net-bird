package utils

import (
	"fmt"
	"net"

	"github.com/DrC0ns0le/net-bird/logging"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	// CustomRouteProtocol is a unique identifier for routes managed by this package
	// See /etc/iproute2/rt_protos for standard protocol numbers
	CustomRouteProtocol = 201
)

// ConfigureRoute creates or updates a route in the Linux routing table.
// It takes a network (*net.IPNet), gateway address (net.IP), and optional source IP as input.
// The route will be marked with our custom protocol identifier.
// If a route already exists with different parameters, it will be updated.
// Returns an error if the operation fails.
func ConfigureRoute(dst *net.IPNet, gw net.IP, src net.IP) error {
	if dst == nil {
		return fmt.Errorf("network cannot be nil")
	}
	if gw == nil {
		return fmt.Errorf("gateway cannot be nil")
	}

	// Validate IP versions match
	if (dst.IP.To4() != nil) != (gw.To4() != nil) {
		return fmt.Errorf("destination and gateway IP versions must match")
	}
	if src != nil && (gw.To4() != nil) != (src.To4() != nil) {
		return fmt.Errorf("source and gateway IP versions must match")
	}

	route := &netlink.Route{
		Dst:      dst,
		Gw:       gw,
		Protocol: CustomRouteProtocol,
		Src:      src,
	}

	// List all routes matching our destination
	filter := &netlink.Route{
		Dst: dst,
	}
	existing, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, filter, netlink.RT_FILTER_DST)
	if err != nil {
		return fmt.Errorf("failed to list routes: %v", err)
	}

	// Check for existing managed route
	for _, r := range existing {
		if r.Protocol == CustomRouteProtocol {
			// Check if parameters need updating
			if r.Gw.Equal(gw) &&
				((src == nil && r.Src == nil) || (src != nil && r.Src != nil && r.Src.Equal(src))) {
				logging.Debugf("Route to %s via %s already exists with correct parameters", dst, gw)
				return nil
			}
			// Update existing route
			logging.Infof("Updating existing route to %s via %s", dst, gw)
			if err := netlink.RouteReplace(route); err != nil {
				return fmt.Errorf("failed to update route: %v", err)
			}
			return nil
		}
	}

	// Add new route
	logging.Debugf("Adding route to %s via %s", dst, gw)
	err = netlink.RouteAdd(route)
	if err != nil {
		if err == unix.EEXIST {
			logging.Debugf("Route exists but not visible, attempting replace for %s via %s", dst, gw)
			return netlink.RouteReplace(route)
		}
		return fmt.Errorf("failed to add route: %v", err)
	}
	logging.Infof("Added route to %s via %s", dst, gw)

	return nil
}

// RemoveRoute removes a route from the Linux routing table given a network.
// Only removes routes that were created by this package (identified by CustomRouteProtocol).
// Returns an error if the operation fails.
func RemoveRoute(dst *net.IPNet) error {
	if dst == nil {
		return fmt.Errorf("network cannot be nil")
	}

	// Create a route object with the destination and protocol
	route := &netlink.Route{
		Dst:      dst,
		Protocol: CustomRouteProtocol,
	}

	// Try to delete the route directly without checking existence
	err := netlink.RouteDel(route)
	if err != nil {
		// Only return error if it's not "not exists" error
		if err != unix.ESRCH {
			return fmt.Errorf("failed to remove route: %v", err)
		}
		return nil
	}
	logging.Infof("Removed route to %s", dst)

	return nil
}

// RouteExists checks if a route to the specified network exists.
// Only checks for routes managed by this package (identified by CustomRouteProtocol).
// If src is provided, only checks for routes with matching source IP.
// Returns true if the route exists, false otherwise.
func RouteExists(dst *net.IPNet, src net.IP) (bool, error) {
	if dst == nil {
		return false, fmt.Errorf("network cannot be nil")
	}

	// Try to get the route directly
	routes, err := netlink.RouteGet(dst.IP)
	if err != nil {
		return false, fmt.Errorf("failed to get route: %v", err)
	}

	// Check if any of the returned routes match our criteria
	for _, r := range routes {
		if r.Protocol == CustomRouteProtocol {
			// If source IP is specified, check if it matches
			if src != nil {
				if r.Src != nil && r.Src.Equal(src) {
					return true, nil
				}
			} else {
				return true, nil
			}
		}
	}

	return false, nil
}

// ListManagedRoutes returns a list of all routes managed by this package
// (identified by CustomRouteProtocol).
func ListManagedRoutes() ([]netlink.Route, error) {
	// Get all routes but filter by our protocol in the kernel
	filter := &netlink.Route{
		Protocol: CustomRouteProtocol,
	}

	routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, filter, netlink.RT_FILTER_PROTOCOL)
	if err != nil {
		return nil, fmt.Errorf("failed to list routes: %v", err)
	}

	return routes, nil
}

// GetRoute returns a specific route matching the destination and source IP.
// Returns nil and no error if no matching route is found.
func GetRoute(dst *net.IPNet, src net.IP) (*netlink.Route, error) {
	if dst == nil {
		return nil, fmt.Errorf("network cannot be nil")
	}

	routes, err := netlink.RouteGet(dst.IP)
	if err != nil {
		return nil, fmt.Errorf("failed to get route: %v", err)
	}

	for _, r := range routes {
		if r.Protocol == CustomRouteProtocol {
			if src != nil {
				if r.Src != nil && r.Src.Equal(src) {
					return &r, nil
				}
			} else {
				return &r, nil
			}
		}
	}

	return nil, nil
}

// RemoveAllManagedRoutes deletes all routes managed by this package
// (identified by CustomRouteProtocol).
// Returns the number of routes removed and any error encountered.
func RemoveAllManagedRoutes() (int, error) {
	// Get list of all managed routes
	routes, err := ListManagedRoutes()
	if err != nil {
		return 0, fmt.Errorf("failed to list managed routes: %v", err)
	}

	// Keep track of successfully removed routes
	removed := 0

	// Remove each route
	for _, route := range routes {
		err := netlink.RouteDel(&route)
		if err != nil {
			// Log error but continue trying to remove other routes
			logging.Errorf("failed to remove route %v: %v", route, err)
			continue
		}
		removed++
		logging.Debugf("Removed route %v", route)
	}

	if removed > 0 {
		logging.Infof("Removed %d managed routes", removed)
	}

	return removed, nil
}
