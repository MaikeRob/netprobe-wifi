package netinfo

import (
	"errors"
	"fmt"
	"net/netip"
)

var (
	// ErrInvalidCIDR is returned when the CIDR string cannot be parsed.
	ErrInvalidCIDR = errors.New("invalid CIDR")
	// ErrNotIPv4 is returned when the provided network is not an IPv4 network.
	ErrNotIPv4 = errors.New("only IPv4 is supported")
	// ErrTargetNotAllowed is returned when a target does not comply with the allowed network policy.
	ErrTargetNotAllowed = errors.New("target range not allowed by policy")
	// ErrSliceLimitExceeded is returned when requesting a slice of hosts for an overly large range.
	ErrSliceLimitExceeded = errors.New("range is too large to materialize in a single slice")
)

// DefaultWarningPrefixLen is the threshold under which a network is flagged as large (prefix bits < 20, i.e. > 4094 hosts).
const DefaultWarningPrefixLen = 20

// MaxSliceMaterializeLimit is the safe upper bound of hosts to return as an in-memory slice ([]netip.Addr).
// Ranges larger than this should be processed using IterateHosts.
const MaxSliceMaterializeLimit = 65536

// Policy defines which IP ranges are permitted for scanning.
// Enforced in backend to prevent accidental or unauthorized sweeps.
type Policy struct {
	// LocalNetworks lists prefixes directly attached to active host interfaces.
	LocalNetworks []netip.Prefix
	// AllowPublic allows scanning public IP ranges when explicitly confirmed by the user.
	AllowPublic bool
	// WarningPrefixLen is the threshold to flag IsLargeRange. Defaults to 20 if zero or negative.
	WarningPrefixLen int
}

// Target represents a validated IPv4 scan range.
type Target struct {
	Prefix       netip.Prefix
	Self         netip.Addr
	TotalHosts   uint64
	IsLargeRange bool
}

// ParseTarget validates the given CIDR against policy and normalizes it.
// The self address is excluded from generated scan hosts if present inside the prefix.
func ParseTarget(cidr string, self netip.Addr, p Policy) (Target, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return Target{}, fmt.Errorf("%w: %q", ErrInvalidCIDR, cidr)
	}

	// Normalize prefix (IPv4 only, unmap IPv6-mapped IPv4)
	addr := prefix.Addr().Unmap()
	if !addr.Is4() {
		return Target{}, fmt.Errorf("%w: %q", ErrNotIPv4, cidr)
	}
	prefix = netip.PrefixFrom(addr, prefix.Bits()).Masked()

	if self.IsValid() {
		self = self.Unmap()
	}

	if err := checkPolicy(prefix, p); err != nil {
		return Target{}, fmt.Errorf("%w: %s", ErrTargetNotAllowed, err.Error())
	}

	warnLen := p.WarningPrefixLen
	if warnLen <= 0 {
		warnLen = DefaultWarningPrefixLen
	}

	count := calculateHostCount(prefix, self)
	isLarge := prefix.Bits() < warnLen

	return Target{
		Prefix:       prefix,
		Self:         self,
		TotalHosts:   count,
		IsLargeRange: isLarge,
	}, nil
}

// checkPolicy enforces network boundary rules.
func checkPolicy(p netip.Prefix, policy Policy) error {
	addr := p.Addr()

	// Special non-routable / prohibited ranges
	if addr.IsLoopback() {
		return fmt.Errorf("loopback addresses (127.0.0.0/8) are not scannable")
	}
	if addr.IsMulticast() {
		return fmt.Errorf("multicast ranges (224.0.0.0/4) are not scannable")
	}
	if addr.IsUnspecified() || addr.As4() == [4]byte{255, 255, 255, 255} {
		return fmt.Errorf("unspecified or limited broadcast addresses are not scannable")
	}

	// 240.0.0.0/4 (Reserved)
	reservedPrefix := netip.MustParsePrefix("240.0.0.0/4")
	if reservedPrefix.Overlaps(p) {
		return fmt.Errorf("reserved range (240.0.0.0/4) is not scannable")
	}

	// Private ranges (RFC 1918)
	if isRFC1918(p) {
		return nil
	}

	// Link-Local (RFC 3927) - 169.254.0.0/16
	linkLocalPrefix := netip.MustParsePrefix("169.254.0.0/16")
	if linkLocalPrefix.Contains(p.Addr()) && p.Bits() >= 16 {
		return nil
	}

	// CGNAT (RFC 6598) - 100.64.0.0/10: only permitted if it matches a local network interface
	cgnatPrefix := netip.MustParsePrefix("100.64.0.0/10")
	if cgnatPrefix.Contains(p.Addr()) {
		for _, local := range policy.LocalNetworks {
			if local.Contains(p.Addr()) {
				return nil
			}
		}
		return fmt.Errorf("CGNAT range (100.64.0.0/10) is only permitted when matching a local interface network")
	}

	// Public range
	if policy.AllowPublic {
		return nil
	}

	return fmt.Errorf("public IP range %s requires explicit user confirmation", p.String())
}

var (
	rfc1918_10  = netip.MustParsePrefix("10.0.0.0/8")
	rfc1918_172 = netip.MustParsePrefix("172.16.0.0/12")
	rfc1918_192 = netip.MustParsePrefix("192.168.0.0/16")
)

func isRFC1918(p netip.Prefix) bool {
	addr := p.Addr()
	if rfc1918_10.Contains(addr) && p.Bits() >= 8 {
		return true
	}
	if rfc1918_172.Contains(addr) && p.Bits() >= 12 {
		return true
	}
	if rfc1918_192.Contains(addr) && p.Bits() >= 16 {
		return true
	}
	return false
}

// calculateHostCount returns the number of scannable addresses in prefix, excluding self.
func calculateHostCount(prefix netip.Prefix, self netip.Addr) uint64 {
	bits := prefix.Bits()
	var rawCount uint64

	switch {
	case bits == 32:
		rawCount = 1
	case bits == 31:
		rawCount = 2 // RFC 3021
	case bits <= 30:
		hostBits := 32 - bits
		rawCount = (1 << hostBits) - 2 // exclude network and broadcast
	}

	if self.IsValid() && prefix.Contains(self) {
		// If self is in the scannable set
		if bits == 32 {
			return 0
		}
		if bits == 31 {
			return rawCount - 1
		}
		// In <= 30, self is excluded if it's not the network or broadcast address
		netAddr := prefix.Addr()
		bcastAddr := broadcastAddress(prefix)
		if self != netAddr && self != bcastAddr && rawCount > 0 {
			return rawCount - 1
		}
	}

	return rawCount
}
