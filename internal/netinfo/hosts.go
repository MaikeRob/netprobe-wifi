package netinfo

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/netip"
)

// IterateHosts calls yield on each scannable host in the target range.
// Scanning stops early if yield returns false or if ctx is cancelled.
func (t Target) IterateHosts(ctx context.Context, yield func(netip.Addr) bool) error {
	bits := t.Prefix.Bits()
	startIP := binary.BigEndian.Uint32(t.Prefix.Addr().AsSlice())

	var firstHost, lastHost uint32

	switch {
	case bits == 32:
		firstHost = startIP
		lastHost = startIP
	case bits == 31:
		firstHost = startIP
		lastHost = startIP + 1
	default: // <= 30
		hostBits := 32 - bits
		total := uint32(1 << hostBits)
		firstHost = startIP + 1
		lastHost = startIP + total - 2
	}

	for current := firstHost; current <= lastHost; current++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var b [4]byte
		binary.BigEndian.PutUint32(b[:], current)
		addr := netip.AddrFrom4(b)

		if t.Self.IsValid() && addr == t.Self {
			continue
		}

		if !yield(addr) {
			break
		}

		// Prevent uint32 overflow on 255.255.255.255
		if current == lastHost {
			break
		}
	}

	return nil
}

// Hosts returns all scannable addresses as a slice, up to maxLimit.
// If TotalHosts exceeds maxLimit, ErrSliceLimitExceeded is returned.
func (t Target) Hosts(maxLimit int) ([]netip.Addr, error) {
	if maxLimit <= 0 {
		maxLimit = MaxSliceMaterializeLimit
	}
	if t.TotalHosts > uint64(maxLimit) {
		return nil, fmt.Errorf("%w: total %d exceeds limit %d", ErrSliceLimitExceeded, t.TotalHosts, maxLimit)
	}

	hosts := make([]netip.Addr, 0, t.TotalHosts)
	err := t.IterateHosts(context.Background(), func(addr netip.Addr) bool {
		hosts = append(hosts, addr)
		return true
	})
	if err != nil {
		return nil, err
	}
	return hosts, nil
}

// broadcastAddress returns the directed broadcast address for an IPv4 prefix.
func broadcastAddress(p netip.Prefix) netip.Addr {
	bits := p.Bits()
	if bits >= 31 {
		return p.Addr()
	}
	ipUint := binary.BigEndian.Uint32(p.Addr().AsSlice())
	mask := ^uint32(0) >> bits
	bcastUint := ipUint | mask

	var b [4]byte
	binary.BigEndian.PutUint32(b[:], bcastUint)
	return netip.AddrFrom4(b)
}

// SuggestPrefix determines a safe and practical default scan range for the given interface.
// If the interface network is /24 or narrower, it suggests the full interface subnet.
// If the interface network is wider (e.g. /16 or /8), it suggests a /24 subnet surrounding the interface IP.
func SuggestPrefix(iface Interface) netip.Prefix {
	if !iface.Prefix.IsValid() || !iface.Addr.IsValid() {
		return netip.Prefix{}
	}

	if iface.Prefix.Bits() >= 24 {
		return iface.Prefix
	}

	// For wider subnets (e.g. 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16),
	// default to a /24 containing the host IP to avoid accidental massive sweeps.
	return netip.PrefixFrom(iface.Addr, 24).Masked()
}
