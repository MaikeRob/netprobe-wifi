package netinfo

import (
	"fmt"
	"net"
	"net/netip"
)

// DefaultDummyTarget is a non-routable documentation address (RFC 5737 TEST-NET-1)
// used to trigger a kernel routing lookup via UDP connect().
const DefaultDummyTarget = "192.0.2.1:53"

// UDPRouteSource determines the default outbound IPv4 address by asking the kernel
// for the best routing path to a dummy destination.
//
// OBSERVABILITY & SAFETY GUARANTEE:
// Calling net.Dial("udp", target) performs a connect() syscall on a UDP socket.
// On both Linux and Windows, this only queries the OS routing table to bind the local IP.
// Because no write() or sendto() call is made on the socket, ZERO network packets are transmitted.
type UDPRouteSource struct {
	target string
}

// NewUDPRouteSource returns a RouteSource configured with the default dummy target.
func NewUDPRouteSource() *UDPRouteSource {
	return &UDPRouteSource{target: DefaultDummyTarget}
}

// DefaultSourceAddr queries the kernel routing table for the outbound source IP.
func (u *UDPRouteSource) DefaultSourceAddr() (netip.Addr, error) {
	target := u.target
	if target == "" {
		target = DefaultDummyTarget
	}

	conn, err := net.Dial("udp", target)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("resolving default route via udp connect: %w", err)
	}
	defer conn.Close()

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || localAddr == nil {
		return netip.Addr{}, fmt.Errorf("unexpected local address type: %T", conn.LocalAddr())
	}

	ipv4 := localAddr.IP.To4()
	if ipv4 == nil {
		return netip.Addr{}, fmt.Errorf("local address is not an IPv4 address: %s", localAddr.IP.String())
	}

	addr, ok := netip.AddrFromSlice(ipv4)
	if !ok {
		return netip.Addr{}, fmt.Errorf("failed to convert IP to netip.Addr: %s", ipv4.String())
	}

	return addr, nil
}
