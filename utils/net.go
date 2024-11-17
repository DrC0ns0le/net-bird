package utils

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// GetOutboundIPs determines the preferred outgoing private IPv4 and IPv6 addresses.
// IPv4 is obtained through UDP connection, while IPv6 is obtained from network interfaces
// whose names start with "e".
func GetOutboundIPs() (net.IP, net.IP, error) {
	// Context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a dialer with timeout
	dialer := net.Dialer{
		Timeout: 5 * time.Second,
	}

	// Get IPv4 address
	conn4, err := dialer.DialContext(ctx, "udp4", "8.8.8.8:80")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get IPv4 address: %v", err)
	}
	defer conn4.Close()

	localAddr4, ok := conn4.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, nil, fmt.Errorf("failed to get local IPv4 address from connection")
	}

	// Get IPv6 address from interface starting with "e"
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get network interfaces: %v", err)
	}

	var ipv6Addr net.IP
	for _, iface := range interfaces {
		// Check if interface name starts with "e"
		if !strings.HasPrefix(iface.Name, "e") && iface.Name != "lo" {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			// Check if this is an IP network address
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			// Check if this is an IPv6 address
			if ip := ipNet.IP.To16(); ip != nil && ipNet.IP.To4() == nil {
				if ip.IsPrivate() && strings.HasPrefix(ip.String(), "fdac:c9:") && strings.HasSuffix(ip.String(), "::2") {
					ipv6Addr = ip
					break
				}

			}
		}

		if ipv6Addr != nil {
			break
		}
	}

	if ipv6Addr == nil {
		return nil, nil, fmt.Errorf("no IPv6 address found on interfaces starting with 'e'")
	}

	return localAddr4.IP, ipv6Addr, nil
}
