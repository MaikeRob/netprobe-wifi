//go:build linux

package arp

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"netprobe-wifi/internal/netinfo"
)

// LinuxTrigger provokes the Linux kernel to resolve a neighbor MAC by sending
// a short UDP datagram to an unlikely high port.
//
// OBSERVABILITY & NETWORK TRACE:
// When sending UDP to an unmapped local IPv4 address within the interface subnet:
// 1. The Linux kernel pauses the UDP packet and broadcasts an ARP Request (Who has <target>? Tell <source>).
// 2. If the target host is present, it responds with an ARP Reply (<target> is at <target_mac>).
// 3. The Linux kernel populates its neighbor cache (/proc/net/arp) with the received MAC.
// 4. The short UDP datagram is then transmitted across the wire.
// 5. The destination host may optionally respond with ICMP Port Unreachable (Type 3, Code 3).
type LinuxTrigger struct {
	port int
}

// NewLinuxTrigger returns a Trigger configured for Linux.
func NewLinuxTrigger() *LinuxTrigger {
	return &LinuxTrigger{port: DefaultTriggerPort}
}

// Provoke emits the trigger datagram to target on the given interface.
func (t *LinuxTrigger) Provoke(ctx context.Context, target netip.Addr, iface netinfo.Interface) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	port := t.port
	if port <= 0 {
		port = DefaultTriggerPort
	}

	var laddr *net.UDPAddr
	if iface.Addr.IsValid() {
		laddr = &net.UDPAddr{IP: iface.Addr.AsSlice(), Port: 0}
	}
	raddr := &net.UDPAddr{IP: target.AsSlice(), Port: port}

	conn, err := net.DialUDP("udp4", laddr, raddr)
	if err != nil {
		return fmt.Errorf("dialing trigger udp to %s: %w", target, err)
	}
	defer conn.Close()

	// Send a 1-byte payload to trigger L2 resolution in the kernel
	if _, err := conn.Write([]byte{0x00}); err != nil {
		return fmt.Errorf("writing trigger payload to %s: %w", target, err)
	}

	return nil
}
