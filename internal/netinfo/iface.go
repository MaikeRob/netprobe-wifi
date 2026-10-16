package netinfo

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
)

// ErrNoDefaultInterface is returned when no active interface matches the system default route.
var ErrNoDefaultInterface = errors.New("no default interface found matching active route")

// Interface represents an active IPv4 network interface candidate.
type Interface struct {
	Name   string
	Index  int
	MAC    net.HardwareAddr
	Addr   netip.Addr   // interface IPv4 address
	Prefix netip.Prefix // masked network prefix (e.g. 192.168.1.0/24)
}

// RouteSource defines an abstraction for determining the default outbound source IP.
type RouteSource interface {
	DefaultSourceAddr() (netip.Addr, error)
}

type rawInterface struct {
	Name         string
	Index        int
	Flags        net.Flags
	HardwareAddr net.HardwareAddr
	Addrs        []net.Addr
}

// ListInterfaces discovers all active IPv4 interfaces suitable for network scanning.
func ListInterfaces() ([]Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("listing network interfaces: %w", err)
	}

	raws := make([]rawInterface, 0, len(ifaces))
	for _, ifi := range ifaces {
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		raws = append(raws, rawInterface{
			Name:         ifi.Name,
			Index:        ifi.Index,
			Flags:        ifi.Flags,
			HardwareAddr: ifi.HardwareAddr,
			Addrs:        addrs,
		})
	}

	return filterInterfaces(raws), nil
}

// filterInterfaces filters out loopback, inactive, point-to-point, and non-IPv4 addresses.
func filterInterfaces(raws []rawInterface) []Interface {
	var results []Interface

	for _, raw := range raws {
		// Must be Up, non-loopback, and non-point-to-point
		if raw.Flags&net.FlagUp == 0 {
			continue
		}
		if raw.Flags&net.FlagLoopback != 0 {
			continue
		}
		if raw.Flags&net.FlagPointToPoint != 0 {
			continue
		}

		for _, a := range raw.Addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}

			ipv4 := ipNet.IP.To4()
			if ipv4 == nil {
				continue
			}

			addr, ok := netip.AddrFromSlice(ipv4)
			if !ok {
				continue
			}

			ones, _ := ipNet.Mask.Size()
			prefix := netip.PrefixFrom(addr, ones).Masked()

			results = append(results, Interface{
				Name:   raw.Name,
				Index:  raw.Index,
				MAC:    raw.HardwareAddr,
				Addr:   addr,
				Prefix: prefix,
			})
		}
	}

	return results
}

// DefaultInterface selects the default scanning interface from candidates using RouteSource.
func DefaultInterface(ifs []Interface, rs RouteSource) (Interface, error) {
	if rs == nil {
		return Interface{}, errors.New("route source cannot be nil")
	}

	srcAddr, err := rs.DefaultSourceAddr()
	if err != nil {
		return Interface{}, fmt.Errorf("%w: %v", ErrNoDefaultInterface, err)
	}

	// 1. Exact IP match
	for _, ifi := range ifs {
		if ifi.Addr == srcAddr {
			return ifi, nil
		}
	}

	// 2. Subnet containment match (fallback)
	for _, ifi := range ifs {
		if ifi.Prefix.Contains(srcAddr) {
			return ifi, nil
		}
	}

	return Interface{}, fmt.Errorf("%w: outbound IP %s does not match any candidate", ErrNoDefaultInterface, srcAddr)
}
