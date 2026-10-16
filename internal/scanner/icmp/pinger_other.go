//go:build !linux && !windows

package icmp

import (
	"context"
	"errors"
	"net/netip"
	"time"
)

// ErrUnsupportedOS is returned when running on an operating system other than Windows or Linux.
var ErrUnsupportedOS = errors.New("operating system not supported for ICMP scanner (Windows and Linux only)")

// UnsupportedPinger is a stub for unsupported operating systems.
type UnsupportedPinger struct{}

// NewUnsupportedPinger returns ErrUnsupportedOS.
func NewUnsupportedPinger() (*UnsupportedPinger, error) {
	return nil, ErrUnsupportedOS
}

// Ping returns ErrUnsupportedOS.
func (u *UnsupportedPinger) Ping(ctx context.Context, target netip.Addr, timeout time.Duration) (Result, error) {
	return Result{IP: target, Err: ErrUnsupportedOS}, ErrUnsupportedOS
}

// Close is a no-op.
func (u *UnsupportedPinger) Close() error {
	return nil
}
