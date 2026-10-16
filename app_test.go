package main

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"netprobe-wifi/internal/netinfo"
	"netprobe-wifi/internal/scanner/arp"
	"netprobe-wifi/internal/scanner/icmp"
	"netprobe-wifi/internal/scanner/port"
)

type mockTrigger struct{}

func (m *mockTrigger) Provoke(ctx context.Context, target netip.Addr, iface netinfo.Interface) error {
	return nil
}

type mockReader struct{}

func (m *mockReader) ReadTable() (map[netip.Addr]arp.TableEntry, error) {
	return make(map[netip.Addr]arp.TableEntry), nil
}

func TestApp_GetNetworkInterfaces(t *testing.T) {
	app := NewApp()
	ifaces, err := app.GetNetworkInterfaces()
	if err != nil {
		t.Fatalf("GetNetworkInterfaces failed: %v", err)
	}
	// On real system, ifaces should be a non-nil slice
	if ifaces == nil {
		t.Errorf("expected non-nil ifaces slice")
	}
}

func TestApp_CheckCapabilities(t *testing.T) {
	app := NewApp()
	caps := app.CheckCapabilities()
	// Should return a valid capabilities struct without panicking
	_ = caps.CanSendICMP
}

func TestApp_SweepLifecycle(t *testing.T) {
	app := NewApp()

	// Inject fake pinger and fake arpEngine
	app.pingerFactory = func() (icmp.Pinger, error) {
		return icmp.NewFakePinger(), nil
	}
	app.arpEngineFactory = func(iface netinfo.Interface) (*arp.Engine, error) {
		return arp.NewEngine(&mockReader{}, &mockTrigger{}, app.ouiDB, iface), nil
	}

	err := app.StartSweep("192.168.1.0/29", "", 100, 500)
	if err != nil {
		t.Fatalf("StartSweep failed: %v", err)
	}

	// Double start should return error
	errDouble := app.StartSweep("192.168.1.0/29", "", 100, 500)
	if errDouble == nil {
		t.Errorf("expected error on concurrent StartSweep, got nil")
	}

	// Cancel sweep
	errCancel := app.CancelSweep()
	if errCancel != nil {
		t.Errorf("CancelSweep returned unexpected error: %v", errCancel)
	}

	// Give a moment for background goroutine to clean up
	time.Sleep(50 * time.Millisecond)

	app.mu.Lock()
	sweeping := app.isSweeping
	app.mu.Unlock()

	if sweeping {
		t.Errorf("expected isSweeping false after cancel")
	}
}

func TestApp_ScanPortsValidation(t *testing.T) {
	app := NewApp()

	// Invalid IP address
	_, err := app.ScanPorts("not_an_ip", 1, 100, 500)
	if err == nil {
		t.Errorf("expected error for invalid IP, got nil")
	}

	// Valid execution using fake dialer
	app.portEngine = port.NewEngine(nil)
	_, _ = app.ScanPorts("127.0.0.1", 80, 80, 200)
}
