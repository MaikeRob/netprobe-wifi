//go:build !linux && !windows

package arp

import (
	"context"
	"errors"
	"net/netip"

	"netprobe-wifi/internal/netinfo"
)

var ErrUnsupportedOS = errors.New("ARP discovery is only supported on Linux and Windows")

type UnsupportedTableReader struct{}

func NewUnsupportedTableReader() *UnsupportedTableReader {
	return &UnsupportedTableReader{}
}

func (u *UnsupportedTableReader) ReadTable() (map[netip.Addr]TableEntry, error) {
	return nil, ErrUnsupportedOS
}

type UnsupportedTrigger struct{}

func NewUnsupportedTrigger() *UnsupportedTrigger {
	return &UnsupportedTrigger{}
}

func (u *UnsupportedTrigger) Provoke(ctx context.Context, target netip.Addr, iface netinfo.Interface) error {
	return ErrUnsupportedOS
}

// NewDefaultTableReader provides a fallback TableReader for unsupported platforms.
func NewDefaultTableReader() TableReader {
	return NewUnsupportedTableReader()
}

// NewDefaultTrigger provides a fallback Trigger for unsupported platforms.
func NewDefaultTrigger() Trigger {
	return NewUnsupportedTrigger()
}
