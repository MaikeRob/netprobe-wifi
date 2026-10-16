//go:build linux

package icmp

// NewPinger instantiates the default Pinger for Linux.
func NewPinger() (Pinger, error) {
	return NewLinuxPinger()
}
