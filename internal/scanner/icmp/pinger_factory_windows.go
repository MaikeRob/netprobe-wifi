//go:build windows

package icmp

// NewPinger instantiates the default Pinger for Windows.
func NewPinger() (Pinger, error) {
	return NewWindowsPinger()
}
