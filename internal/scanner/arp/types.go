package arp

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"time"

	"netprobe-wifi/internal/netinfo"
	"netprobe-wifi/internal/oui"
)

const (
	// StatusDiscoveredARP is the exact label required for hosts discovered via kernel-provoked ARP.
	StatusDiscoveredARP = "Descoberto via ARP"
	// StatusDiscoveredICMP is the label for hosts responding directly to ICMP ping.
	StatusDiscoveredICMP = "Ativo via ICMP"

	// DefaultARPTimeout is the maximum duration to wait for the kernel to resolve ARP entries.
	DefaultARPTimeout = 2500 * time.Millisecond
	// DefaultPollInterval is the interval between neighbor table checks.
	DefaultPollInterval = 100 * time.Millisecond
	// DefaultTriggerPort is an unlikely high UDP port used on Linux to trigger kernel neighbor resolution.
	DefaultTriggerPort = 38472
)

var (
	// ErrOutsideSubnet is returned when attempting an ARP probe on an IP outside the local L2 interface subnet.
	ErrOutsideSubnet = errors.New("target is outside local L2 interface subnet")
	// ErrResolutionTimeout is returned when kernel neighbor resolution does not complete within timeout.
	ErrResolutionTimeout = errors.New("arp resolution timed out")
)

// HostInfo contains complete Layer 2 and Layer 3 host identification and manufacturer details.
type HostInfo struct {
	IP           netip.Addr
	MAC          net.HardwareAddr
	MACString    string
	Vendor       string
	Kind         oui.Kind
	DiscoveredBy string
	Responded    bool
}

// TableEntry represents a parsed entry from the operating system's neighbor/ARP cache.
type TableEntry struct {
	IP      netip.Addr
	MAC     net.HardwareAddr
	Flags   uint32
	Device  string
	IsValid bool
}

// TableReader abstracts querying the operating system neighbor/ARP cache.
type TableReader interface {
	ReadTable() (map[netip.Addr]TableEntry, error)
}

// Trigger abstracts provoking the operating system to send an ARP request for target.
type Trigger interface {
	Provoke(ctx context.Context, target netip.Addr, iface netinfo.Interface) error
}

func isAllZeros(hw net.HardwareAddr) bool {
	for _, b := range hw {
		if b != 0 {
			return false
		}
	}
	return true
}
