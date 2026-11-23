package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	lastActiveSubnet  netip.Prefix
	discoveredIPs     map[netip.Addr]struct{}

	ouiDB            *oui.DB
	listInterfaces   func() ([]netinfo.Interface, error)
	routeSource      netinfo.RouteSource
	pingerFactory    func() (icmp.Pinger, error)
	arpEngineFactory func(iface netinfo.Interface) (*arp.Engine, error)
	portEngine       *port.Engine
}

// NewApp creates a new App application struct.
func NewApp() *App {
	db, _ := oui.Load()
	return &App{
		ouiDB:         db,
		discoveredIPs: make(map[netip.Addr]struct{}),
		portEngine:    port.NewEngine(nil),
	}
}

func (a *App) getInterfaces() ([]netinfo.Interface, error) {
	if a.listInterfaces != nil {
		return a.listInterfaces()
	}
	return netinfo.ListInterfaces()
}

func (a *App) getRouteSource() netinfo.RouteSource {
	if a.routeSource != nil {
		return a.routeSource
	}
	return netinfo.NewUDPRouteSource()
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
	suggested := netinfo.SuggestPrefix(iface)
	if suggested.IsValid() {
		cidrStr = suggested.String()
	} else if iface.Prefix.IsValid() {
		cidrStr = iface.Prefix.String()
	}
	macStr := ""
	if len(iface.MAC) == 6 {
		macStr = oui.FormatMAC(iface.MAC)
	} else if len(iface.MAC) > 0 {
		macStr = iface.MAC.String()
	}
	return NetworkAdapter{
		Name:  iface.Name,
		IP:    ipStr,
		CIDR:  cidrStr,
		MAC:   macStr,
		Index: iface.Index,
	}
}

// GetNetworkInterfaces returns the list of active IPv4 network interfaces.
func (a *App) GetNetworkInterfaces() ([]NetworkAdapter, error) {
	ifaces, err := a.getInterfaces()
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
	ifaces, err := a.getInterfaces()
	if err != nil {
		return NetworkAdapter{}, err
	}
	defIface, err := netinfo.DefaultInterface(ifaces, a.getRouteSource())
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

	ifaces, listErr := a.getInterfaces()
	if listErr != nil {
		a.mu.Unlock()
		return fmt.Errorf("erro ao listar interfaces de rede: %w", listErr)
	}
	if len(ifaces) == 0 {
		a.mu.Unlock()
		return errors.New("nenhuma interface de rede Wi-Fi/IPv4 ativa encontrada — verifique se o notebook está conectado à rede")
	}

	// 1. Resolve interface
	var iface netinfo.Interface
	var err error
	if ifaceName != "" {
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
			return fmt.Errorf("interface de rede %q não está mais ativa — clique em Atualizar Interfaces", ifaceName)
		}
	} else {
		iface, err = netinfo.DefaultInterface(ifaces, a.getRouteSource())
		if err != nil {
			a.mu.Unlock()
			return fmt.Errorf("não foi possível detectar uma interface de rede ativa: %w", err)
		}
	}

	baseCtx := a.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(baseCtx)
	a.activeSweepCancel = cancel
	a.isSweeping = true
	a.lastActiveSubnet = iface.Prefix
	a.discoveredIPs = make(map[netip.Addr]struct{})
	a.mu.Unlock()

	slog.Info("starting network sweep",
		"interface", iface.Name,
		"rate_limit_pps", rateLimit,
		"timeout_ms", timeoutMs,
	)

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
		slog.Warn("icmp socket unavailable; falling back to kernel-provoked ARP discovery", "reason", err)
		pinger = nil
	} else {
		slog.Info("icmp socket initialized")
	}
	if pinger != nil {
		defer func() { _ = pinger.Close() }()
	}

	var arpEngine *arp.Engine
	if a.arpEngineFactory != nil {
		arpEngine, _ = a.arpEngineFactory(iface)
	} else {
		arpEngine = arp.NewEngine(nil, nil, a.ouiDB, iface)
	}

	orch := scanner.NewOrchestrator(pinger, arpEngine, a.ouiDB)

	handlers := scanner.SweepHandlers{
		OnProgress: func(p scanner.SweepProgressEvent) {
			slog.Debug("sweep progress",
				"stage", string(p.Stage),
				"percent", p.Percent,
				"scanned", p.ScannedHosts,
				"total", p.TotalHosts,
				"active", p.ActiveHosts,
				"icmp", p.ICMPDiscovered,
				"arp", p.ARPDiscovered,
			)
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "sweep:progress", p)
			}
		},
		OnBatch: func(b scanner.HostBatchEvent) {
			a.mu.Lock()
			for _, h := range b.Hosts {
				if ip, parseErr := netip.ParseAddr(h.IP); parseErr == nil {
					a.discoveredIPs[ip] = struct{}{}
				}
				slog.Debug("discovered active host",
					"ip", h.IP,
					"mac", h.MAC,
					"vendor", h.Vendor,
					"rtt", h.RTT,
					"method", h.DiscoveredBy,
				)
			}
			a.mu.Unlock()
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "sweep:batch", b)
			}
		},
		OnSummary: func(s scanner.SweepSummaryEvent) {
			slog.Info("sweep completed",
				"duration", s.Duration,
				"total_hosts", s.TotalHosts,
				"active_hosts", s.ActiveHosts,
				"icmp_discovered", s.ICMPDiscovered,
				"arp_discovered", s.ARPDiscovered,
			)
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

	runErr := orch.Run(ctx, cfg, handlers)
	if runErr != nil && ctx.Err() == nil {
		slog.Error("sweep failed", "error", runErr)
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "sweep:progress", scanner.SweepProgressEvent{
				Stage:        scanner.StageError,
				ErrorMessage: runErr.Error(),
			})
		}
	}
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

// isAllowedPortScanTarget verifies that target belongs to an active local subnet or was discovered in the current sweep.
func (a *App) isAllowedPortScanTarget(target netip.Addr) (netip.Prefix, bool) {
	a.mu.Lock()
	_, wasDiscovered := a.discoveredIPs[target]
	lastSubnet := a.lastActiveSubnet
	a.mu.Unlock()

	if wasDiscovered {
		if lastSubnet.IsValid() && lastSubnet.Contains(target) {
			return lastSubnet, true
		}
		return netip.PrefixFrom(target, 32), true
	}

	if lastSubnet.IsValid() && lastSubnet.Contains(target) {
		return lastSubnet, true
	}

	if ifaces, err := a.getInterfaces(); err == nil {
		for _, iface := range ifaces {
			if iface.Prefix.IsValid() && iface.Prefix.Contains(target) {
				return iface.Prefix, true
			}
		}
	}

	return netip.Prefix{}, false
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
	target = target.Unmap()

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

	allowedSubnet, allowed := a.isAllowedPortScanTarget(target)
	if !allowed {
		return nil, fmt.Errorf("%w: o IP %s não pertence à sub-rede ativa nem foi descoberto na varredura", port.ErrTargetOutsideSubnet, target)
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = port.DefaultTimeout
	}

	totalPorts := int(endPort) - int(startPort) + 1
	slog.Info("starting tcp port scan",
		"start_port", startPort,
		"end_port", endPort,
		"timeout_ms", timeoutMs,
	)

	cfg := port.ScanConfig{
		Target:      target,
		Subnet:      allowedSubnet,
		StartPort:   startPort,
		EndPort:     endPort,
		Timeout:     timeout,
		Concurrency: port.DefaultConcurrency,
	}

	engine := a.portEngine
	if engine == nil {
		engine = port.NewEngine(nil)
	}

	// Stream open ports immediately; for small ranges (<= 256 ports), also stream closed/filtered ports live.
	// Full sorted results are always returned upon completion.
	streamAll := totalPorts > 0 && totalPorts <= 256

	results, err := engine.Scan(ctx, cfg, func(res port.PortResult) {
		if a.ctx != nil && (res.State == port.StateOpen || streamAll) {
			runtime.EventsEmit(a.ctx, "port:result", res)
		}
	})

	slog.Info("tcp port scan completed",
		"evaluated_ports", len(results),
		"error", err,
	)

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
