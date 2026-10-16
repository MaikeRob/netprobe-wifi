package netinfo

import (
	"errors"
	"net"
	"net/netip"
	"testing"
)

type mockRouteSource struct {
	addr netip.Addr
	err  error
}

func (m *mockRouteSource) DefaultSourceAddr() (netip.Addr, error) {
	return m.addr, m.err
}

func TestFilterInterfaces(t *testing.T) {
	t.Parallel()

	mac := net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}

	raws := []rawInterface{
		{
			Name:         "lo",
			Index:        1,
			Flags:        net.FlagUp | net.FlagLoopback,
			HardwareAddr: nil,
			Addrs: []net.Addr{
				&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
			},
		},
		{
			Name:         "eth0_down",
			Index:        2,
			Flags:        0, // Down
			HardwareAddr: mac,
			Addrs: []net.Addr{
				&net.IPNet{IP: net.ParseIP("192.168.1.10"), Mask: net.CIDRMask(24, 32)},
			},
		},
		{
			Name:         "tun0_p2p",
			Index:        3,
			Flags:        net.FlagUp | net.FlagPointToPoint,
			HardwareAddr: nil,
			Addrs: []net.Addr{
				&net.IPNet{IP: net.ParseIP("10.8.0.2"), Mask: net.CIDRMask(24, 32)},
			},
		},
		{
			Name:         "eth1_ipv6_only",
			Index:        4,
			Flags:        net.FlagUp | net.FlagBroadcast,
			HardwareAddr: mac,
			Addrs: []net.Addr{
				&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
			},
		},
		{
			Name:         "wlan0_active",
			Index:        5,
			Flags:        net.FlagUp | net.FlagBroadcast,
			HardwareAddr: mac,
			Addrs: []net.Addr{
				&net.IPNet{IP: net.ParseIP("192.168.1.55"), Mask: net.CIDRMask(24, 32)},
				&net.IPNet{IP: net.ParseIP("10.0.0.55"), Mask: net.CIDRMask(16, 32)},
			},
		},
	}

	filtered := filterInterfaces(raws)

	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered candidates from wlan0, got %d", len(filtered))
	}

	first := filtered[0]
	if first.Name != "wlan0_active" || first.Addr.String() != "192.168.1.55" || first.Prefix.String() != "192.168.1.0/24" {
		t.Errorf("unexpected first candidate: %+v", first)
	}

	second := filtered[1]
	if second.Name != "wlan0_active" || second.Addr.String() != "10.0.0.55" || second.Prefix.String() != "10.0.0.0/16" {
		t.Errorf("unexpected second candidate: %+v", second)
	}
}

func TestDefaultInterface(t *testing.T) {
	t.Parallel()

	candidates := []Interface{
		{
			Name:   "wlan0",
			Index:  2,
			Addr:   netip.MustParseAddr("192.168.1.50"),
			Prefix: netip.MustParsePrefix("192.168.1.0/24"),
		},
		{
			Name:   "docker0",
			Index:  3,
			Addr:   netip.MustParseAddr("172.17.0.1"),
			Prefix: netip.MustParsePrefix("172.17.0.0/16"),
		},
	}

	t.Run("exact match", func(t *testing.T) {
		t.Parallel()
		rs := &mockRouteSource{addr: netip.MustParseAddr("192.168.1.50")}
		iface, err := DefaultInterface(candidates, rs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if iface.Name != "wlan0" {
			t.Errorf("expected wlan0, got %s", iface.Name)
		}
	})

	t.Run("subnet containment match", func(t *testing.T) {
		t.Parallel()
		rs := &mockRouteSource{addr: netip.MustParseAddr("192.168.1.99")}
		iface, err := DefaultInterface(candidates, rs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if iface.Name != "wlan0" {
			t.Errorf("expected wlan0 via subnet fallback, got %s", iface.Name)
		}
	})

	t.Run("route error", func(t *testing.T) {
		t.Parallel()
		rs := &mockRouteSource{err: errors.New("network down")}
		_, err := DefaultInterface(candidates, rs)
		if !errors.Is(err, ErrNoDefaultInterface) {
			t.Fatalf("expected ErrNoDefaultInterface, got: %v", err)
		}
	})

	t.Run("no match", func(t *testing.T) {
		t.Parallel()
		rs := &mockRouteSource{addr: netip.MustParseAddr("10.254.254.1")}
		_, err := DefaultInterface(candidates, rs)
		if !errors.Is(err, ErrNoDefaultInterface) {
			t.Fatalf("expected ErrNoDefaultInterface when route IP does not match, got: %v", err)
		}
	})

	t.Run("nil route source", func(t *testing.T) {
		t.Parallel()
		_, err := DefaultInterface(candidates, nil)
		if err == nil {
			t.Fatal("expected error for nil route source")
		}
	})
}
