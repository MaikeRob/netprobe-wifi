package netinfo_test

import (
	"errors"
	"net/netip"
	"testing"

	"netprobe-wifi/internal/netinfo"
)

func TestParseTarget_ValidPrivateRanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cidr       string
		self       netip.Addr
		policy     netinfo.Policy
		wantPrefix string
		wantCount  uint64
		wantLarge  bool
	}{
		{
			name:       "192.168.1.0/24 without self",
			cidr:       "192.168.1.0/24",
			self:       netip.Addr{},
			policy:     netinfo.Policy{},
			wantPrefix: "192.168.1.0/24",
			wantCount:  254,
			wantLarge:  false,
		},
		{
			name:       "192.168.1.50/24 normalized with self inside",
			cidr:       "192.168.1.50/24",
			self:       netip.MustParseAddr("192.168.1.50"),
			policy:     netinfo.Policy{},
			wantPrefix: "192.168.1.0/24",
			wantCount:  253, // 254 - 1
			wantLarge:  false,
		},
		{
			name:       "10.0.0.0/22 without self",
			cidr:       "10.0.0.0/22",
			self:       netip.Addr{},
			policy:     netinfo.Policy{},
			wantPrefix: "10.0.0.0/22",
			wantCount:  1022,
			wantLarge:  false, // prefix 22 >= 20
		},
		{
			name:       "172.16.0.0/16 flagged as large range",
			cidr:       "172.16.0.0/16",
			self:       netip.Addr{},
			policy:     netinfo.Policy{},
			wantPrefix: "172.16.0.0/16",
			wantCount:  65534,
			wantLarge:  true, // prefix 16 < 20
		},
		{
			name:       "169.254.1.0/24 link-local allowed",
			cidr:       "169.254.1.0/24",
			self:       netip.Addr{},
			policy:     netinfo.Policy{},
			wantPrefix: "169.254.1.0/24",
			wantCount:  254,
			wantLarge:  false,
		},
		{
			name:       "/32 single host without self",
			cidr:       "192.168.1.10/32",
			self:       netip.Addr{},
			policy:     netinfo.Policy{},
			wantPrefix: "192.168.1.10/32",
			wantCount:  1,
			wantLarge:  false,
		},
		{
			name:       "/32 single host matching self",
			cidr:       "192.168.1.10/32",
			self:       netip.MustParseAddr("192.168.1.10"),
			policy:     netinfo.Policy{},
			wantPrefix: "192.168.1.10/32",
			wantCount:  0,
			wantLarge:  false,
		},
		{
			name:       "/31 point-to-point RFC 3021",
			cidr:       "10.0.0.0/31",
			self:       netip.MustParseAddr("10.0.0.0"),
			policy:     netinfo.Policy{},
			wantPrefix: "10.0.0.0/31",
			wantCount:  1, // 2 - 1
			wantLarge:  false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			target, err := netinfo.ParseTarget(tc.cidr, tc.self, tc.policy)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if target.Prefix.String() != tc.wantPrefix {
				t.Errorf("prefix = %s; want %s", target.Prefix, tc.wantPrefix)
			}
			if target.TotalHosts != tc.wantCount {
				t.Errorf("totalHosts = %d; want %d", target.TotalHosts, tc.wantCount)
			}
			if target.IsLargeRange != tc.wantLarge {
				t.Errorf("isLargeRange = %v; want %v", target.IsLargeRange, tc.wantLarge)
			}
		})
	}
}

func TestParseTarget_CGNAT(t *testing.T) {
	t.Parallel()

	cgnatCIDR := "100.64.1.0/24"

	// Without local network matching: rejected
	_, err := netinfo.ParseTarget(cgnatCIDR, netip.Addr{}, netinfo.Policy{})
	if !errors.Is(err, netinfo.ErrTargetNotAllowed) {
		t.Fatalf("expected ErrTargetNotAllowed for CGNAT without matching local interface, got: %v", err)
	}

	// With matching local interface prefix: accepted
	policy := netinfo.Policy{
		LocalNetworks: []netip.Prefix{netip.MustParsePrefix("100.64.0.0/16")},
	}
	target, err := netinfo.ParseTarget(cgnatCIDR, netip.Addr{}, policy)
	if err != nil {
		t.Fatalf("expected CGNAT with matching local network to succeed, got: %v", err)
	}
	if target.Prefix.String() != cgnatCIDR {
		t.Errorf("prefix = %s; want %s", target.Prefix, cgnatCIDR)
	}
}

func TestParseTarget_PublicRanges(t *testing.T) {
	t.Parallel()

	publicCIDR := "8.8.8.0/24"

	// Denied by default
	_, err := netinfo.ParseTarget(publicCIDR, netip.Addr{}, netinfo.Policy{})
	if !errors.Is(err, netinfo.ErrTargetNotAllowed) {
		t.Fatalf("expected ErrTargetNotAllowed for public IP without AllowPublic, got: %v", err)
	}

	// Allowed with explicit user permission
	target, err := netinfo.ParseTarget(publicCIDR, netip.Addr{}, netinfo.Policy{AllowPublic: true})
	if err != nil {
		t.Fatalf("expected public range to be permitted when AllowPublic is true, got: %v", err)
	}
	if target.Prefix.String() != publicCIDR {
		t.Errorf("prefix = %s; want %s", target.Prefix, publicCIDR)
	}
}

func TestParseTarget_ProhibitedSpecialRanges(t *testing.T) {
	t.Parallel()

	forbidden := []string{
		"127.0.0.0/8",
		"127.0.0.1/32",
		"224.0.0.0/4",
		"239.255.255.0/24",
		"240.0.0.0/4",
		"0.0.0.0/8",
		"255.255.255.255/32",
	}

	for _, cidr := range forbidden {
		cidr := cidr
		t.Run(cidr, func(t *testing.T) {
			t.Parallel()

			// Should fail even if AllowPublic is true
			_, err := netinfo.ParseTarget(cidr, netip.Addr{}, netinfo.Policy{AllowPublic: true})
			if !errors.Is(err, netinfo.ErrTargetNotAllowed) {
				t.Fatalf("expected ErrTargetNotAllowed for %s, got: %v", cidr, err)
			}
		})
	}
}

func TestParseTarget_InvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cidr    string
		wantErr error
	}{
		{"empty string", "", netinfo.ErrInvalidCIDR},
		{"malformed CIDR", "192.168.1.1", netinfo.ErrInvalidCIDR},
		{"invalid IP octet", "192.168.300.0/24", netinfo.ErrInvalidCIDR},
		{"invalid prefix mask", "192.168.1.0/33", netinfo.ErrInvalidCIDR},
		{"IPv6 network", "fe80::/64", netinfo.ErrNotIPv4},
		{"IPv6 localhost", "::1/128", netinfo.ErrNotIPv4},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := netinfo.ParseTarget(tc.cidr, netip.Addr{}, netinfo.Policy{})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ParseTarget(%q) err = %v; want %v", tc.cidr, err, tc.wantErr)
			}
		})
	}
}
