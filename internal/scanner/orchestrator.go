package scanner

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"netprobe-wifi/internal/netinfo"
	"netprobe-wifi/internal/oui"
	"netprobe-wifi/internal/scanner/arp"
	"netprobe-wifi/internal/scanner/icmp"
)

// OrchestratorConfig specifies configuration parameters for a unified network sweep.
type OrchestratorConfig struct {
	CIDR        string
	Interface   netinfo.Interface
	RateLimit   int
	ICMPTimeout time.Duration
	ARPTimeout  time.Duration
	AllowPublic bool
}

// SweepHandlers defines callback functions invoked during sweep lifecycle transitions.
type SweepHandlers struct {
	OnProgress func(SweepProgressEvent)
	OnBatch    func(HostBatchEvent)
	OnSummary  func(SweepSummaryEvent)
}

// Orchestrator coordinates the end-to-end multi-stage network discovery workflow:
// Stage 1: ICMP Echo Sweep (active ping sweep with rate limiting)
// Stage 2: Kernel-Provoked ARP (discovering silent/firewalled hosts in local L2 subnet)
// Stage 3: Neighbor cache resolution and IEEE OUI manufacturer enrichment
type Orchestrator struct {
	pinger    icmp.Pinger
	arpEngine *arp.Engine
	ouiDB     *oui.DB
}

// NewOrchestrator creates an Orchestrator with the required scanner dependencies.
func NewOrchestrator(pinger icmp.Pinger, arpEngine *arp.Engine, db *oui.DB) *Orchestrator {
	return &Orchestrator{
		pinger:    pinger,
		arpEngine: arpEngine,
		ouiDB:     db,
	}
}

// Run executes the complete multi-stage sweep, reporting batched hosts and progress events.
func (o *Orchestrator) Run(ctx context.Context, cfg OrchestratorConfig, handlers SweepHandlers) error {
	startTime := time.Now()

	// 1. Validate Target Range according to network security policy
	policy := netinfo.Policy{
		LocalNetworks: []netip.Prefix{cfg.Interface.Prefix},
		AllowPublic:   cfg.AllowPublic,
	}

	target, err := netinfo.ParseTarget(cfg.CIDR, cfg.Interface.Addr, policy)
	if err != nil {
		if handlers.OnProgress != nil {
			handlers.OnProgress(SweepProgressEvent{
				Stage:        StageError,
				ErrorMessage: formatTargetError(cfg.CIDR, err),
			})
		}
		return fmt.Errorf("validating target %q: %w", cfg.CIDR, err)
	}

	totalHosts := int(target.TotalHosts)

	// Helper to safely emit progress events
	emitProgress := func(p SweepProgressEvent) {
		if handlers.OnProgress != nil {
			handlers.OnProgress(p)
		}
	}

	emitProgress(SweepProgressEvent{
		Stage:      StageStarting,
		TotalHosts: totalHosts,
		Percent:    0,
	})

	// 2. Pre-scan Neighbor Table Snapshot (rules/redes.md temporal diff)
	// When ICMP pinger is active, snapshot ensures Stage 2 ARP only credits newly provoked entries.
	// When ICMP is unavailable, disable snapshot filtering so pre-existing active hosts (e.g. gateway) are discovered.
	if o.arpEngine != nil {
		if o.pinger != nil {
			o.arpEngine.SetFilterSnapshot(true)
			_ = o.arpEngine.TakeSnapshot()
		} else {
			o.arpEngine.SetFilterSnapshot(false)
		}
	}

	// 3. Start Event Aggregator (~100 ms periodic batch flush)
	aggregator := NewEventAggregator(DefaultBatchInterval, handlers.OnBatch)
	aggregator.Start()
	defer aggregator.Stop()

	// 4. Stage 1: ICMP Ping Sweep (if pinger is available)
	var scannedCount int64
	var icmpCount int64
	var silentIPs []netip.Addr
	var silentMu sync.Mutex

	if o.pinger != nil {
		emitProgress(SweepProgressEvent{
			Stage:      StageICMP,
			TotalHosts: totalHosts,
			Percent:    0,
		})

		icmpRate := cfg.RateLimit
		if icmpRate <= 0 {
			icmpRate = icmp.DefaultRateLimitPPS
		}
		icmpTimeout := cfg.ICMPTimeout
		if icmpTimeout <= 0 {
			icmpTimeout = icmp.DefaultTimeout
		}

		sweepEngine, err := icmp.NewSweepEngine(o.pinger, icmp.SweepConfig{
			RateLimitPPS: icmpRate,
			Timeout:      icmpTimeout,
		})
		if err != nil {
			emitProgress(SweepProgressEvent{
				Stage:        StageError,
				ErrorMessage: formatSweepConfigError(err),
			})
			return fmt.Errorf("initializing icmp sweep engine: %w", err)
		}

		sweepErr := sweepEngine.Sweep(ctx, target, func(res icmp.Result) {
			scanned := atomic.AddInt64(&scannedCount, 1)

			if res.Responded {
				atomic.AddInt64(&icmpCount, 1)

				macStr := ""
				vendor := ""
				kind := "unknown"

				if o.arpEngine != nil {
					hostInfo, found, _ := o.arpEngine.EnrichHost(res.IP)
					if found {
						macStr = hostInfo.MACString
						vendor = hostInfo.Vendor
						kind = hostInfo.Kind.String()
					}
				}

				aggregator.AddHost(DiscoveredHost{
					IP:           res.IP.String(),
					MAC:          macStr,
					Vendor:       vendor,
					Kind:         kind,
					RTT:          res.RTT,
					DiscoveredBy: arp.StatusDiscoveredICMP,
					Responded:    true,
				})
			} else {
				// Silent host: save for Stage 2 if inside local L2 subnet
				if cfg.Interface.Prefix.IsValid() && cfg.Interface.Prefix.Contains(res.IP) && res.IP != cfg.Interface.Addr {
					silentMu.Lock()
					silentIPs = append(silentIPs, res.IP)
					silentMu.Unlock()
				}
			}

			// Emit ICMP progress: represents the first 80% of total progress
			curICMP := atomic.LoadInt64(&icmpCount)
			pct := 0.0
			if totalHosts > 0 {
				pct = (float64(scanned) / float64(totalHosts)) * 80.0
			}

			emitProgress(SweepProgressEvent{
				Stage:          StageICMP,
				TotalHosts:     totalHosts,
				ScannedHosts:   int(scanned),
				ActiveHosts:    int(curICMP),
				ICMPDiscovered: int(curICMP),
				Percent:        pct,
			})
		})

		if sweepErr != nil && ctx.Err() == nil {
			emitProgress(SweepProgressEvent{
				Stage:        StageError,
				ErrorMessage: "Não foi possível concluir a verificação da rede. Verifique sua conexão e tente novamente.",
			})
			return fmt.Errorf("icmp sweep failed: %w", sweepErr)
		}
	} else {
		// When ICMP pinger is unavailable (e.g. unprivileged Linux without ping_group_range),
		// all candidate hosts in local subnet are collected for Stage 2 ARP discovery.
		_ = target.IterateHosts(ctx, func(ip netip.Addr) bool {
			if cfg.Interface.Prefix.IsValid() && cfg.Interface.Prefix.Contains(ip) && ip != cfg.Interface.Addr {
				silentIPs = append(silentIPs, ip)
			}
			return true
		})
	}

	// 5. Stage 2: Kernel-Provoked ARP for Silent Hosts
	var arpCount int64
	if ctx.Err() == nil && len(silentIPs) > 0 && o.arpEngine != nil {
		curICMP := int(atomic.LoadInt64(&icmpCount))
		emitProgress(SweepProgressEvent{
			Stage:          StageARP,
			TotalHosts:     totalHosts,
			ScannedHosts:   totalHosts,
			ActiveHosts:    curICMP,
			ICMPDiscovered: curICMP,
			Percent:        80.0,
		})

		if cfg.RateLimit > 0 {
			o.arpEngine.SetRateLimit(cfg.RateLimit)
		}

		o.arpEngine.SetOnHostDiscovered(func(h arp.HostInfo) {
			newARP := int(atomic.AddInt64(&arpCount, 1))
			aggregator.AddHost(DiscoveredHost{
				IP:           h.IP.String(),
				MAC:          h.MACString,
				Vendor:       h.Vendor,
				Kind:         h.Kind.String(),
				RTT:          0,
				DiscoveredBy: arp.StatusDiscoveredARP,
				Responded:    false,
			})
			emitProgress(SweepProgressEvent{
				Stage:          StageARP,
				TotalHosts:     totalHosts,
				ScannedHosts:   totalHosts,
				ActiveHosts:    curICMP + newARP,
				ICMPDiscovered: curICMP,
				ARPDiscovered:  newARP,
				Percent:        95.0,
			})
		})

		o.arpEngine.SetOnProgress(func(probed, totalSilent int) {
			pct := 80.0
			if totalSilent > 0 {
				pct = 80.0 + (float64(probed)/float64(totalSilent))*15.0
			}
			curARP := int(atomic.LoadInt64(&arpCount))
			emitProgress(SweepProgressEvent{
				Stage:          StageARP,
				TotalHosts:     totalHosts,
				ScannedHosts:   totalHosts,
				ActiveHosts:    curICMP + curARP,
				ICMPDiscovered: curICMP,
				ARPDiscovered:  curARP,
				Percent:        pct,
			})
		})

		_, _ = o.arpEngine.ResolveSilentHosts(ctx, silentIPs)
	}

	// 6. Finalization and Flush
	aggregator.Flush()

	finalICMP := int(atomic.LoadInt64(&icmpCount))
	finalARP := int(atomic.LoadInt64(&arpCount))
	finalActive := finalICMP + finalARP

	finalStage := StageFinished
	if ctx.Err() != nil {
		finalStage = StageCanceled
	}

	emitProgress(SweepProgressEvent{
		Stage:          finalStage,
		TotalHosts:     totalHosts,
		ScannedHosts:   totalHosts,
		ActiveHosts:    finalActive,
		ICMPDiscovered: finalICMP,
		ARPDiscovered:  finalARP,
		Percent:        100.0,
	})

	if handlers.OnSummary != nil {
		reason := ""
		if ctx.Err() != nil {
			reason = ctx.Err().Error()
		}
		handlers.OnSummary(SweepSummaryEvent{
			TotalHosts:     totalHosts,
			ActiveHosts:    finalActive,
			ICMPDiscovered: finalICMP,
			ARPDiscovered:  finalARP,
			Duration:       time.Since(startTime),
			Reason:         reason,
		})
	}

	return ctx.Err()
}

func formatTargetError(cidr string, err error) string {
	if errors.Is(err, netinfo.ErrInvalidCIDR) {
		return fmt.Sprintf("O endereço %q não está no formato de rede esperado. Use o formato IP/máscara (exemplo: 192.168.1.0/24).", cidr)
	}
	if errors.Is(err, netinfo.ErrNotIPv4) {
		return "Informe um endereço de rede IPv4 válido (exemplo: 192.168.1.0/24)."
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "loopback"):
		return "O endereço local da própria máquina (127.0.0.0/8) não pode ser escaneado."
	case strings.Contains(msg, "multicast"):
		return "Endereços de transmissão em grupo (multicast) não podem ser escaneados."
	case strings.Contains(msg, "public IP"):
		return "Por segurança, apenas redes locais privadas podem ser escaneadas."
	default:
		return fmt.Sprintf("A faixa de rede %q não é permitida. Informe uma sub-rede local válida.", cidr)
	}
}

func formatSweepConfigError(err error) string {
	if errors.Is(err, icmp.ErrInvalidRateLimit) {
		return "A velocidade de envio deve ficar entre 10 e 500 pacotes por segundo."
	}
	if errors.Is(err, icmp.ErrInvalidTimeout) {
		return "O tempo limite de resposta deve ficar entre 200 ms e 5000 ms."
	}
	return "Não foi possível iniciar a verificação com os parâmetros informados."
}
