//go:build !linux && !windows

package icmp

// NewPinger returns ErrUnsupportedOS on unsupported platforms.
func NewPinger() (Pinger, error) {
	return NewUnsupportedPinger()
}
