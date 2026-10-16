//go:build linux

package icmp

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/net/icmp"
)

// CheckCapabilities tests the host Linux environment for ICMP sending support.
func CheckCapabilities() Capabilities {
	// 1. Test unprivileged datagram ICMP socket (requires ping_group_range)
	dgramConn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err == nil {
		_ = dgramConn.Close()
		return Capabilities{
			CanSendICMP: true,
			Reason:      "unprivileged datagram ICMP socket allowed by kernel ping_group_range",
		}
	}

	// 2. Test raw socket (requires cap_net_raw or root)
	rawConn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err == nil {
		_ = rawConn.Close()
		return Capabilities{
			CanSendICMP: true,
			Reason:      "raw ICMP socket allowed via cap_net_raw capability or root",
		}
	}

	// Read ping_group_range details for friendly diagnostics
	pingGroupRange := "unknown"
	if b, err := os.ReadFile("/proc/sys/net/ipv4/ping_group_range"); err == nil {
		pingGroupRange = strings.TrimSpace(string(b))
	}

	return Capabilities{
		CanSendICMP: false,
		Reason: fmt.Sprintf(
			"insufficient privileges: ping_group_range=%q (unprivileged ICMP disabled) and executable lacks cap_net_raw",
			pingGroupRange,
		),
	}
}
