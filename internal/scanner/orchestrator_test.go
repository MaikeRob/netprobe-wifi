package scanner

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"netprobe-wifi/internal/netinfo"
	"netprobe-wifi/internal/oui"
	"netprobe-wifi/internal/scanner/arp"
	"netprobe-wifi/internal/scanner/icmp"
)

type mockTableReader struct {
	table map[netip.Addr]arp.TableEntry
}

func (m *mockTableReader) ReadTable() (map[netip.Addr]arp.TableEntry, error) {
	return m.table, nil
}

type mockTrigger struct{}

func (m *mockTrigger) Provoke(ctx context.Context, target netip.Addr, iface netinfo.Interface) error {
	return nil
}

func TestOrchestrator_FullWorkflow(t *testing.T) {
	db, err := oui.Load()
	if err != nil {
		t.Fatalf("failed to load OUI database: %v", err)
	}

	iface := netinfo.Interface{
		Name:   "wlan0",
		Addr:   netip.MustParseAddr("192.168.1.1"),
		Prefix: netip.MustParsePrefix("192.168.1.0/29"), // Hosts .1 to .6 (.1 is self, so targets .2 to .6 = 5 hosts)
	}

	ipICMPActive := netip.MustParseAddr("192.168.1.2")
	ipARPActive := netip.MustParseAddr("192.168.1.3")

	// Pinger that only responds to 192.168.1.2
	fakePinger := icmp.NewFakePinger()
	fakePinger.SetResponse(ipICMPActive, true, 5*time.Millisecond, nil)
	defer fakePinger.Close()

	// Initial table with empty/incomplete
	initialTable := map[netip.Addr]arp.TableEntry{
		ipICMPActive: {
			IP:      ipICMPActive,
			MAC:     net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
			IsValid: true,
		},
		ipARPActive: {
			IP:      ipARPActive,
			MAC:     nil,
			IsValid: false, // incomplete before
		},
	}

	// After ARP trigger, ipARPActive completes with a VMware MAC
	afterTriggerTable := map[netip.Addr]arp.TableEntry{
		ipICMPActive: {
			IP:      ipICMPActive,
			MAC:     net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
			IsValid: true,
		},
		ipARPActive: {
			IP:      ipARPActive,
			MAC:     net.HardwareAddr{0x00, 0x50, 0x56, 0xC0, 0x00, 0x08},
			IsValid: true,
		},
	}

	reader := &mockTableReader{table: initialTable}
	arpEngine := arp.NewEngine(reader, &mockTrigger{}, db, iface,
		arp.WithTimeout(100*time.Millisecond),
		arp.WithPollInterval(20*time.Millisecond),
	)

	orch := NewOrchestrator(fakePinger, arpEngine, db)

	var mu sync.Mutex
	var stages []SweepStage
	var receivedHosts []DiscoveredHost
	var summary SweepSummaryEvent

	// We switch the table when StageARP starts to simulate neighbor resolution
	handlers := SweepHandlers{
		OnProgress: func(p SweepProgressEvent) {
			mu.Lock()
			stages = append(stages, p.Stage)
			if p.Stage == StageARP {
				reader.table = afterTriggerTable
			}
			mu.Unlock()
		},
		OnBatch: func(b HostBatchEvent) {
			mu.Lock()
			receivedHosts = append(receivedHosts, b.Hosts...)
			mu.Unlock()
		},
		OnSummary: func(s SweepSummaryEvent) {
			mu.Lock()
			summary = s
			mu.Unlock()
		},
	}

	cfg := OrchestratorConfig{
		CIDR:        "192.168.1.0/29",
		Interface:   iface,
		RateLimit:   200,
		ICMPTimeout: 500 * time.Millisecond,
		ARPTimeout:  200 * time.Millisecond,
	}

	err = orch.Run(context.Background(), cfg, handlers)
	if err != nil {
		t.Fatalf("Orchestrator.Run failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Verify stages
	hasICMP := false
	hasARP := false
	hasFinished := false
	for _, s := range stages {
		if s == StageICMP {
			hasICMP = true
		}
		if s == StageARP {
			hasARP = true
		}
		if s == StageFinished {
			hasFinished = true
		}
	}

	if !hasICMP || !hasARP || !hasFinished {
		t.Errorf("missing expected stages in %v", stages)
	}

	// Verify hosts received
	if len(receivedHosts) != 2 {
		t.Fatalf("expected 2 discovered hosts (1 ICMP, 1 ARP), got %d: %+v", len(receivedHosts), receivedHosts)
	}

	var foundICMP, foundARP bool
	for _, h := range receivedHosts {
		if h.IP == ipICMPActive.String() && h.DiscoveredBy == arp.StatusDiscoveredICMP {
			foundICMP = true
		}
		if h.IP == ipARPActive.String() && h.DiscoveredBy == arp.StatusDiscoveredARP {
			foundARP = true
		}
	}

	if !foundICMP {
		t.Errorf("expected to find ICMP host %s", ipICMPActive)
	}
	if !foundARP {
		t.Errorf("expected to find ARP host %s", ipARPActive)
	}

	// Verify summary
	if summary.TotalHosts != 5 {
		t.Errorf("expected 5 total hosts, got %d", summary.TotalHosts)
	}
	if summary.ActiveHosts != 2 {
		t.Errorf("expected 2 active hosts, got %d", summary.ActiveHosts)
	}
	if summary.ICMPDiscovered != 1 || summary.ARPDiscovered != 1 {
		t.Errorf("expected 1 ICMP and 1 ARP, got ICMP=%d ARP=%d", summary.ICMPDiscovered, summary.ARPDiscovered)
	}
}

func TestOrchestrator_InvalidCIDR(t *testing.T) {
	orch := NewOrchestrator(icmp.NewFakePinger(), nil, nil)
	cfg := OrchestratorConfig{
		CIDR: "invalid_cidr",
	}

	err := orch.Run(context.Background(), cfg, SweepHandlers{})
	if err == nil {
		t.Fatal("expected error for invalid CIDR, got nil")
	}
}
