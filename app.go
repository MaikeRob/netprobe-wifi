package main

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"netprobe-wifi/internal/netinfo"
	"netprobe-wifi/internal/oui"
	"netprobe-wifi/internal/scanner"
	"netprobe-wifi/internal/scanner/arp"
	"netprobe-wifi/internal/scanner/icmp"
	"netprobe-wifi/internal/scanner/port"
)

// App is the thin backend layer exposed to the Wails frontend.
type App struct {
	ctx context.Context

	mu                sync.Mutex
	activeSweepCancel context.CancelFunc
	activePortCancel  context.CancelFunc
	isSweeping        bool
	isScanningPorts   bool

	ouiDB            *oui.DB
	pingerFactory    func() (icmp.Pinger, error)
	arpEngineFactory func(iface netinfo.Interface) (*arp.Engine, error)
	portEngine       *port.Engine
}

// NewApp creates a new App application struct.
func NewApp() *App {
	db, _ := oui.Load()
	return &App{
		ouiDB:      db,
		portEngine: port.NewEngine(nil),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// NetworkAdapter represents a network interface with strings formatted for the UI.
type NetworkAdapter struct {
	Name  string `json:"name"`
	IP    string `json:"ip"`
	CIDR  string `json:"cidr"`
	MAC   string `json:"mac"`
	Index int    `json:"index"`
}

func toNetworkAdapter(iface netinfo.Interface) NetworkAdapter {
	ipStr := ""
	if iface.Addr.IsValid() {
		ipStr = iface.Addr.String()
	}
	cidrStr := ""
	if iface.Prefix.IsValid() {
		cidrStr = iface.Prefix.String()
	}
	return NetworkAdapter{
		Name:  iface.Name,
		IP:    ipStr,
		CIDR:  cidrStr,
		MAC:   iface.MAC.String(),
		Index: iface.Index,
	}
}

// GetNetworkInterfaces returns the list of active IPv4 network interfaces.
func (a *App) GetNetworkInterfaces() ([]NetworkAdapter, error) {
	ifaces, err := netinfo.ListInterfaces()
	if err != nil {
		return nil, err
	}
	adapters := make([]NetworkAdapter, 0, len(ifaces))
	for _, iface := range ifaces {
		adapters = append(adapters, toNetworkAdapter(iface))
	}
	return adapters, nil
}

// GetDefaultInterface resolves the primary network interface via zero-packet routing query.
func (a *App) GetDefaultInterface() (NetworkAdapter, error) {
	ifaces, err := netinfo.ListInterfaces()
	if err != nil {
		return NetworkAdapter{}, err
	}
	defIface, err := netinfo.DefaultInterface(ifaces, netinfo.NewUDPRouteSource())
	if err != nil {
		return NetworkAdapter{}, err
	}
	return toNetworkAdapter(defIface), nil
}

// CheckCapabilities returns diagnostic information about ICMP socket capabilities.
func (a *App) CheckCapabilities() icmp.Capabilities {
	return icmp.CheckCapabilities()
}

// StartSweep initiates a multi-stage network sweep in a background goroutine.
func (a *App) StartSweep(cidr string, ifaceName string, rateLimit int, timeoutMs int) error {
	a.mu.Lock()
	if a.isSweeping {
		a.mu.Unlock()
		return errors.New("varredura já está em andamento")
	}

	// 1. Resolve interface
	var iface netinfo.Interface
	var err error
	if ifaceName != "" {
		ifaces, listErr := netinfo.ListInterfaces()
		if listErr != nil {
			a.mu.Unlock()
			return fmt.Errorf("listando interfaces: %w", listErr)
		}
		found := false
		for _, item := range ifaces {
			if item.Name == ifaceName {
				iface = item
				found = true
				break
			}
		}
		if !found {
			a.mu.Unlock()
			return fmt.Errorf("interface %q não encontrada", ifaceName)
		}
	} else {
		ifaces, listErr := netinfo.ListInterfaces()
		if listErr != nil {
			a.mu.Unlock()
			return fmt.Errorf("listando interfaces: %w", listErr)
		}
		iface, err = netinfo.DefaultInterface(ifaces, netinfo.NewUDPRouteSource())
		if err != nil {
			a.mu.Unlock()
			return fmt.Errorf("detectando interface padrão: %w", err)
		}
	}

	baseCtx := a.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(baseCtx)
	a.activeSweepCancel = cancel
	a.isSweeping = true
	a.mu.Unlock()

	// Launch background sweep
	go a.runSweep(ctx, cidr, iface, rateLimit, timeoutMs)

	return nil
}

func (a *App) runSweep(ctx context.Context, cidr string, iface netinfo.Interface, rateLimit int, timeoutMs int) {
	defer func() {
		a.mu.Lock()
		a.isSweeping = false
		a.activeSweepCancel = nil
		a.mu.Unlock()
	}()

	var pinger icmp.Pinger
	var err error
	if a.pingerFactory != nil {
		pinger, err = a.pingerFactory()
	} else {
		pinger, err = icmp.NewPinger()
	}
	if err != nil {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "sweep:progress", scanner.SweepProgressEvent{
				Stage: scanner.StageError,
			})
		}
		return
	}
	defer pinger.Close()

	var arpEngine *arp.Engine
	if a.arpEngineFactory != nil {
		arpEngine, _ = a.arpEngineFactory(iface)
	} else {
		arpEngine = arp.NewEngine(nil, nil, a.ouiDB, iface)
	}

	orch := scanner.NewOrchestrator(pinger, arpEngine, a.ouiDB)

	handlers := scanner.SweepHandlers{
		OnProgress: func(p scanner.SweepProgressEvent) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "sweep:progress", p)
			}
		},
		OnBatch: func(b scanner.HostBatchEvent) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "sweep:batch", b)
			}
		},
		OnSummary: func(s scanner.SweepSummaryEvent) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "sweep:summary", s)
			}
		},
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = icmp.DefaultTimeout
	}

	cfg := scanner.OrchestratorConfig{
		CIDR:        cidr,
		Interface:   iface,
		RateLimit:   rateLimit,
		ICMPTimeout: timeout,
		ARPTimeout:  arp.DefaultARPTimeout,
	}

	_ = orch.Run(ctx, cfg, handlers)
}

// CancelSweep immediately aborts the active sweep.
func (a *App) CancelSweep() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isSweeping || a.activeSweepCancel == nil {
		return nil
	}

	a.activeSweepCancel()
	return nil
}

// ScanPorts runs a TCP connect scan against the target IP.
func (a *App) ScanPorts(targetIP string, startPort, endPort uint16, timeoutMs int) ([]port.PortResult, error) {
	a.mu.Lock()
	if a.isScanningPorts {
		a.mu.Unlock()
		return nil, errors.New("varredura de portas já está em andamento")
	}

	target, err := netip.ParseAddr(targetIP)
	if err != nil {
		a.mu.Unlock()
		return nil, fmt.Errorf("endereço IP inválido %q: %w", targetIP, err)
	}

	baseCtx := a.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(baseCtx)
	a.activePortCancel = cancel
	a.isScanningPorts = true
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.isScanningPorts = false
		a.activePortCancel = nil
		a.mu.Unlock()
	}()

	timeout := time.Duration(timeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = port.DefaultTimeout
	}

	cfg := port.ScanConfig{
		Target:      target,
		StartPort:   startPort,
		EndPort:     endPort,
		Timeout:     timeout,
		Concurrency: port.DefaultConcurrency,
	}

	engine := a.portEngine
	if engine == nil {
		engine = port.NewEngine(nil)
	}

	results, err := engine.Scan(ctx, cfg, func(res port.PortResult) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "port:result", res)
		}
	})

	return results, err
}

// CancelPortScan immediately aborts an in-progress TCP port scan.
func (a *App) CancelPortScan() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isScanningPorts || a.activePortCancel == nil {
		return nil
	}

	a.activePortCancel()
	return nil
}
