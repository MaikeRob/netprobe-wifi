package scanner

import (
	"context"
	"fmt"
	"net/netip"
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
				Stage: StageError,
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
	if o.arpEngine != nil {
		_ = o.arpEngine.TakeSnapshot()
	}

	// 3. Start Event Aggregator (~100 ms periodic batch flush)
	aggregator := NewEventAggregator(DefaultBatchInterval, handlers.OnBatch)
	aggregator.Start()
	defer aggregator.Stop()

	// 4. Stage 1: ICMP Ping Sweep
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
		return fmt.Errorf("initializing icmp sweep engine: %w", err)
	}

	var scannedCount int64
	var icmpCount int64
	var silentIPs []netip.Addr
	var silentMu sync.Mutex

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
		emitProgress(SweepProgressEvent{Stage: StageError})
		return fmt.Errorf("icmp sweep failed: %w", sweepErr)
	}

	// 5. Stage 2: Kernel-Provoked ARP for Silent Hosts
	var arpCount int64
	if ctx.Err() == nil && len(silentIPs) > 0 && o.arpEngine != nil {
		emitProgress(SweepProgressEvent{
			Stage:          StageARP,
			TotalHosts:     totalHosts,
			ScannedHosts:   totalHosts,
			ActiveHosts:    int(atomic.LoadInt64(&icmpCount)),
			ICMPDiscovered: int(atomic.LoadInt64(&icmpCount)),
			Percent:        80.0,
		})

		resolvedHosts, err := o.arpEngine.ResolveSilentHosts(ctx, silentIPs)
		if err == nil {
			for _, h := range resolvedHosts {
				atomic.AddInt64(&arpCount, 1)

				aggregator.AddHost(DiscoveredHost{
					IP:           h.IP.String(),
					MAC:          h.MACString,
					Vendor:       h.Vendor,
					Kind:         h.Kind.String(),
					RTT:          0,
					DiscoveredBy: arp.StatusDiscoveredARP,
					Responded:    false,
				})
			}
		}
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
