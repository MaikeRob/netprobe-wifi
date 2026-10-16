//go:build windows

package icmp

// CheckCapabilities tests the host Windows environment for ICMP sending support.
func CheckCapabilities() Capabilities {
	return Capabilities{
		CanSendICMP: true,
		Reason:      "Windows iphlpapi.dll (IcmpSendEcho) available without administrator privileges",
	}
}
