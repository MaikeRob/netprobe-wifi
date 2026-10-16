//go:build !linux && !windows

package icmp

// CheckCapabilities returns false on unsupported operating systems.
func CheckCapabilities() Capabilities {
	return Capabilities{
		CanSendICMP: false,
		Reason:      "operating system not supported for ICMP scanner",
	}
}
