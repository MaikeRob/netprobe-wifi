package arp

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"netprobe-wifi/internal/netinfo"
	"netprobe-wifi/internal/oui"
)

type mockTableReader struct {
	mu     sync.Mutex
	tables []map[netip.Addr]TableEntry
	call   int
}

func (m *mockTableReader) ReadTable() (map[netip.Addr]TableEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.tables) == 0 {
		return make(map[netip.Addr]TableEntry), nil
	}

	idx := m.call
	if idx >= len(m.tables) {
		idx = len(m.tables) - 1
	}
	m.call++
	return m.tables[idx], nil
}

type mockTrigger struct {
	mu       sync.Mutex
	provoked []netip.Addr
}

func (m *mockTrigger) Provoke(ctx context.Context, target netip.Addr, iface netinfo.Interface) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.provoked = append(m.provoked, target)
	return nil
}

func (m *mockTrigger) ProvokedList() []netip.Addr {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]netip.Addr, len(m.provoked))
	copy(copied, m.provoked)
	return copied
}

func TestEngine_ResolveSilentHosts(t *testing.T) {
	db, err := oui.Load()
	if err != nil {
		t.Fatalf("failed to load embedded OUI database: %v", err)
	}

	iface := netinfo.Interface{
		Name:   "wlan0",
		Addr:   netip.MustParseAddr("192.168.1.50"),
		Prefix: netip.MustParsePrefix("192.168.1.0/24"),
	}

	ipPreExisting := netip.MustParseAddr("192.168.1.1")
	ipIncompleteBefore := netip.MustParseAddr("192.168.1.10")
	ipNewTarget := netip.MustParseAddr("192.168.1.20")

	// Snapshot 0: before scan
	snapTable := map[netip.Addr]TableEntry{
		ipPreExisting: {
			IP:      ipPreExisting,
			MAC:     net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
			IsValid: true,
		},
		ipIncompleteBefore: {
			IP:      ipIncompleteBefore,
			MAC:     nil,
			IsValid: false, // incomplete
		},
	}

	// Table 1: after trigger provoked resolution
	afterTable := map[netip.Addr]TableEntry{
		ipPreExisting: {
			IP:      ipPreExisting,
			MAC:     net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
			IsValid: true,
		},
		ipIncompleteBefore: {
			IP:      ipIncompleteBefore,
			MAC:     net.HardwareAddr{0x00, 0x50, 0x56, 0xC0, 0x00, 0x08}, // VMware OUI
			IsValid: true,
		},
		ipNewTarget: {
			IP:      ipNewTarget,
			MAC:     net.HardwareAddr{0xDA, 0xA1, 0x19, 0x12, 0x34, 0x56}, // Randomized MAC (bit 1 of DA is set)
			IsValid: true,
		},
	}

	mockReader := &mockTableReader{
		tables: []map[netip.Addr]TableEntry{snapTable, afterTable},
	}
	mockTrig := &mockTrigger{}

	engine := NewEngine(mockReader, mockTrig, db, iface,
		WithTimeout(200*time.Millisecond),
		WithPollInterval(20*time.Millisecond),
	)

	// Take pre-scan snapshot
	if err := engine.TakeSnapshot(); err != nil {
		t.Fatalf("TakeSnapshot failed: %v", err)
	}

	silentIPs := []netip.Addr{ipPreExisting, ipIncompleteBefore, ipNewTarget}
	var streamReceived []HostInfo
	engine.onHostDiscovered = func(h HostInfo) {
		streamReceived = append(streamReceived, h)
	}

	ctx := context.Background()
	results, err := engine.ResolveSilentHosts(ctx, silentIPs)
	if err != nil {
		t.Fatalf("ResolveSilentHosts returned error: %v", err)
	}

	// 192.168.1.1 was already valid in snapshot -> MUST BE DISCARDED (rules/redes.md)
	// 192.168.1.10 and 192.168.1.20 were resolved -> MUST BE RETURNED
	if len(results) != 2 {
		t.Fatalf("expected 2 resolved hosts, got %d: %+v", len(results), results)
	}

	for _, h := range results {
		if h.IP == ipPreExisting {
			t.Errorf("host %s was valid in snapshot and should not have been labeled as discovered by ARP", ipPreExisting)
		}
		if h.DiscoveredBy != StatusDiscoveredARP {
			t.Errorf("expected DiscoveredBy %q, got %q", StatusDiscoveredARP, h.DiscoveredBy)
		}
		if h.Responded {
			t.Errorf("expected Responded false for silent ARP host, got true")
		}

		if h.IP == ipIncompleteBefore {
			if h.Kind != oui.KindVendor {
				t.Errorf("expected KindVendor for VMware MAC, got %v", h.Kind)
			}
		}

		if h.IP == ipNewTarget {
			if h.Kind != oui.KindPrivate {
				t.Errorf("expected KindPrivate for randomized MAC, got %v", h.Kind)
			}
			if h.Vendor != "Private / Randomized MAC" {
				t.Errorf("expected 'Private / Randomized MAC', got %q", h.Vendor)
			}
		}
	}

	if len(streamReceived) != 2 {
		t.Errorf("expected 2 stream callback invocations, got %d", len(streamReceived))
	}
}

func TestEngine_SubnetFiltering(t *testing.T) {
	iface := netinfo.Interface{
		Name:   "wlan0",
		Addr:   netip.MustParseAddr("192.168.1.50"),
		Prefix: netip.MustParsePrefix("192.168.1.0/24"),
	}

	mockReader := &mockTableReader{}
	mockTrig := &mockTrigger{}

	engine := NewEngine(mockReader, mockTrig, nil, iface,
		WithTimeout(50*time.Millisecond),
		WithPollInterval(10*time.Millisecond),
	)

	silentIPs := []netip.Addr{
		netip.MustParseAddr("192.168.1.1"),
		netip.MustParseAddr("10.0.0.1"),     // Outside subnet -> must be skipped
		netip.MustParseAddr("192.168.1.50"), // Self IP -> must be skipped
		netip.MustParseAddr("192.168.1.2"),
	}

	ctx := context.Background()
	_, _ = engine.ResolveSilentHosts(ctx, silentIPs)

	provoked := mockTrig.ProvokedList()
	if len(provoked) != 2 {
		t.Fatalf("expected 2 provoked targets, got %d: %v", len(provoked), provoked)
	}

	for _, p := range provoked {
		if p == netip.MustParseAddr("10.0.0.1") {
			t.Errorf("outside subnet target 10.0.0.1 should not have been provoked")
		}
		if p == iface.Addr {
			t.Errorf("self IP %s should not have been provoked", iface.Addr)
		}
	}
}

func TestEngine_ContextCancellation(t *testing.T) {
	iface := netinfo.Interface{
		Name:   "wlan0",
		Addr:   netip.MustParseAddr("192.168.1.50"),
		Prefix: netip.MustParsePrefix("192.168.1.0/24"),
	}

	// Empty table forever
	mockReader := &mockTableReader{}
	mockTrig := &mockTrigger{}

	engine := NewEngine(mockReader, mockTrig, nil, iface,
		WithTimeout(5*time.Second),
		WithPollInterval(50*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := engine.ResolveSilentHosts(ctx, []netip.Addr{netip.MustParseAddr("192.168.1.1")})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context error, got nil")
	}

	if elapsed > 1*time.Second {
		t.Fatalf("cancellation took too long: %v", elapsed)
	}
}

func TestEngine_EnrichHost(t *testing.T) {
	db, err := oui.Load()
	if err != nil {
		t.Fatalf("failed to load embedded OUI database: %v", err)
	}

	ip := netip.MustParseAddr("192.168.1.1")
	table := map[netip.Addr]TableEntry{
		ip: {
			IP:      ip,
			MAC:     net.HardwareAddr{0x00, 0x50, 0x56, 0x12, 0x34, 0x56},
			IsValid: true,
		},
	}

	mockReader := &mockTableReader{tables: []map[netip.Addr]TableEntry{table}}
	engine := NewEngine(mockReader, &mockTrigger{}, db, netinfo.Interface{})

	host, ok, err := engine.EnrichHost(ip)
	if err != nil {
		t.Fatalf("EnrichHost failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected host to be found")
	}
	if host.DiscoveredBy != StatusDiscoveredICMP {
		t.Errorf("expected DiscoveredBy %q, got %q", StatusDiscoveredICMP, host.DiscoveredBy)
	}
	if !host.Responded {
		t.Errorf("expected Responded true")
	}
	if host.MACString != "00:50:56:12:34:56" {
		t.Errorf("expected normalized uppercase MAC string, got %s", host.MACString)
	}
}
