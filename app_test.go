package main

import (
	"context"
	"errors"
	"net"
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

type fakeRouteSource struct {
	addr netip.Addr
	err  error
}

func (f *fakeRouteSource) DefaultSourceAddr() (netip.Addr, error) {
	return f.addr, f.err
}

type fakePortDialer struct{}

func (f *fakePortDialer) DialPort(ctx context.Context, target netip.Addr, p uint16, timeout time.Duration) port.PortResult {
	state := port.StateClosed
	if p == 80 {
		state = port.StateOpen
	}
	return port.PortResult{
		Port:    p,
		State:   state,
		Service: port.LookupService(p),
		Latency: time.Millisecond,
	}
}

func stubTestInterfaces() []netinfo.Interface {
	return []netinfo.Interface{
		{
			Name:   "wlan0",
			Index:  2,
			MAC:    net.HardwareAddr{0x00, 0x1A, 0x2B, 0x3C, 0x4D, 0x5E},
			Addr:   netip.MustParseAddr("192.168.1.5"),
			Prefix: netip.MustParsePrefix("192.168.1.0/24"),
		},
	}
}

func TestApp_GetNetworkInterfaces(t *testing.T) {
	app := NewApp()
	app.listInterfaces = func() ([]netinfo.Interface, error) {
		return []netinfo.Interface{
			{
				Name:   "wlan0",
				Index:  2,
				MAC:    net.HardwareAddr{0x00, 0x1a, 0x2b, 0x3c, 0x4d, 0x5e},
				Addr:   netip.MustParseAddr("172.16.10.25"),
				Prefix: netip.MustParsePrefix("172.16.0.0/16"),
			},
		}, nil
	}

	ifaces, err := app.GetNetworkInterfaces()
	if err != nil {
		t.Fatalf("GetNetworkInterfaces failed: %v", err)
	}
	if len(ifaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(ifaces))
	}
	// Verify SuggestPrefix narrows a /16 interface to the surrounding /24 by default
	if ifaces[0].CIDR != "172.16.10.0/24" {
		t.Errorf("expected suggested /24 CIDR 172.16.10.0/24, got %s", ifaces[0].CIDR)
	}
	if ifaces[0].MAC != "00:1A:2B:3C:4D:5E" {
		t.Errorf("expected normalized uppercase MAC, got %s", ifaces[0].MAC)
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
	app.listInterfaces = func() ([]netinfo.Interface, error) {
		return stubTestInterfaces(), nil
	}
	app.routeSource = &fakeRouteSource{
		addr: netip.MustParseAddr("192.168.1.5"),
	}

	// Inject fake pinger and fake arpEngine
	app.pingerFactory = func() (icmp.Pinger, error) {
		fake := icmp.NewFakePinger()
		fake.SetDelay(25 * time.Millisecond)
		return fake, nil
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

	// Wait for background goroutine to clean up
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		app.mu.Lock()
		sweeping := app.isSweeping
		app.mu.Unlock()
		if !sweeping {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	app.mu.Lock()
	sweeping := app.isSweeping
	app.mu.Unlock()

	if sweeping {
		t.Errorf("expected isSweeping false after cancel")
	}
}

func TestApp_StartSweep_NoActiveNetwork(t *testing.T) {
	app := NewApp()
	app.listInterfaces = func() ([]netinfo.Interface, error) {
		return nil, nil
	}

	err := app.StartSweep("192.168.1.0/24", "", 100, 500)
	if err == nil {
		t.Fatal("expected clear error when no active network interfaces exist, got nil")
	}
}

func TestApp_ScanPortsValidation(t *testing.T) {
	app := NewApp()
	app.listInterfaces = func() ([]netinfo.Interface, error) {
		return stubTestInterfaces(), nil
	}
	app.portEngine = port.NewEngine(&fakePortDialer{})

	// 1. Invalid IP address
	_, err := app.ScanPorts("not_an_ip", 1, 100, 500)
	if err == nil {
		t.Errorf("expected error for invalid IP, got nil")
	}

	// 2. Arbitrary external IP outside local subnet and not discovered must be rejected
	_, err = app.ScanPorts("8.8.8.8", 80, 80, 200)
	if !errors.Is(err, port.ErrTargetOutsideSubnet) {
		t.Errorf("expected ErrTargetOutsideSubnet for external IP 8.8.8.8, got: %v", err)
	}

	// 3. Valid host inside local active subnet succeeds
	results, err := app.ScanPorts("192.168.1.10", 79, 80, 200)
	if err != nil {
		t.Fatalf("expected valid local target port scan to succeed, got: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 port results, got %d", len(results))
	}
}
