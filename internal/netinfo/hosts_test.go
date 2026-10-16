package netinfo_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"netprobe-wifi/internal/netinfo"
)

func TestTarget_Hosts(t *testing.T) {
	t.Parallel()

	// /30 has 4 addresses: .0 (net), .1, .2, .3 (bcast).
	// Usable hosts: .1, .2.
	target, err := netinfo.ParseTarget("192.168.1.0/30", netip.Addr{}, netinfo.Policy{})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	hosts, err := target.Hosts(100)
	if err != nil {
		t.Fatalf("unexpected Hosts error: %v", err)
	}

	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
	if hosts[0].String() != "192.168.1.1" || hosts[1].String() != "192.168.1.2" {
		t.Errorf("hosts = %v; want [192.168.1.1, 192.168.1.2]", hosts)
	}
}

func TestTarget_Hosts_SelfExclusion(t *testing.T) {
	t.Parallel()

	self := netip.MustParseAddr("192.168.1.1")
	target, err := netinfo.ParseTarget("192.168.1.0/30", self, netinfo.Policy{})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	hosts, err := target.Hosts(100)
	if err != nil {
		t.Fatalf("unexpected Hosts error: %v", err)
	}

	if len(hosts) != 1 {
		t.Fatalf("expected 1 host with self excluded, got %d", len(hosts))
	}
	if hosts[0].String() != "192.168.1.2" {
		t.Errorf("host = %s; want 192.168.1.2", hosts[0])
	}
}

func TestTarget_IterateHosts_EarlyStop(t *testing.T) {
	t.Parallel()

	target, err := netinfo.ParseTarget("10.0.0.0/24", netip.Addr{}, netinfo.Policy{})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	var count int
	err = target.IterateHosts(context.Background(), func(addr netip.Addr) bool {
		count++
		return count < 5 // stop after 5
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 5 {
		t.Errorf("count = %d; want 5", count)
	}
}

func TestTarget_IterateHosts_ContextCancel(t *testing.T) {
	t.Parallel()

	target, err := netinfo.ParseTarget("10.0.0.0/24", netip.Addr{}, netinfo.Policy{})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var count int
	err = target.IterateHosts(ctx, func(addr netip.Addr) bool {
		count++
		if count == 3 {
			cancel()
		}
		return true
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestTarget_Hosts_ExceedsLimit(t *testing.T) {
	t.Parallel()

	target, err := netinfo.ParseTarget("10.0.0.0/24", netip.Addr{}, netinfo.Policy{})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	// Request slice with max limit of 10 for a /24 (254 hosts)
	_, err = target.Hosts(10)
	if !errors.Is(err, netinfo.ErrSliceLimitExceeded) {
		t.Fatalf("expected ErrSliceLimitExceeded, got: %v", err)
	}
}

func TestSuggestPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		iface      netinfo.Interface
		wantPrefix string
	}{
		{
			name: "subnet /24 kept as-is",
			iface: netinfo.Interface{
				Addr:   netip.MustParseAddr("192.168.1.15"),
				Prefix: netip.MustParsePrefix("192.168.1.0/24"),
			},
			wantPrefix: "192.168.1.0/24",
		},
		{
			name: "subnet /28 kept as-is",
			iface: netinfo.Interface{
				Addr:   netip.MustParseAddr("192.168.1.33"),
				Prefix: netip.MustParsePrefix("192.168.1.32/28"),
			},
			wantPrefix: "192.168.1.32/28",
		},
		{
			name: "wide /16 subnet narrowed to /24 surrounding host IP",
			iface: netinfo.Interface{
				Addr:   netip.MustParseAddr("172.16.5.42"),
				Prefix: netip.MustParsePrefix("172.16.0.0/16"),
			},
			wantPrefix: "172.16.5.0/24",
		},
		{
			name: "wide /8 subnet narrowed to /24 surrounding host IP",
			iface: netinfo.Interface{
				Addr:   netip.MustParseAddr("10.20.30.40"),
				Prefix: netip.MustParsePrefix("10.0.0.0/8"),
			},
			wantPrefix: "10.20.30.0/24",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := netinfo.SuggestPrefix(tc.iface)
			if got.String() != tc.wantPrefix {
				t.Errorf("SuggestPrefix = %s; want %s", got, tc.wantPrefix)
			}
		})
	}
}
