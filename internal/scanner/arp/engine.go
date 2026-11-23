package arp

import (
	"bytes"
	"context"
	"fmt"
	"net/netip"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"netprobe-wifi/internal/netinfo"
	"netprobe-wifi/internal/oui"
)

// EngineOption configures Engine behavior.
type EngineOption func(*Engine)

// WithTimeout sets the maximum time to wait for kernel neighbor resolution.
func WithTimeout(d time.Duration) EngineOption {
	return func(e *Engine) {
		if d > 0 {
			e.timeout = d
		}
	}
}

// WithPollInterval sets the interval between ARP table checks.
func WithPollInterval(d time.Duration) EngineOption {
	return func(e *Engine) {
		if d > 0 {
			e.pollInterval = d
		}
	}
}

// WithConcurrency sets the maximum concurrent trigger emissions.
func WithConcurrency(c int) EngineOption {
	return func(e *Engine) {
		if c > 0 {
			e.concurrency = c
		}
	}
}

// WithOnHostDiscovered sets an optional callback invoked immediately when a host is resolved.
func WithOnHostDiscovered(fn func(HostInfo)) EngineOption {
	return func(e *Engine) {
		e.onHostDiscovered = fn
	}
}

// WithFilterSnapshot controls whether pre-scan snapshot filtering is enabled.
func WithFilterSnapshot(filter bool) EngineOption {
	return func(e *Engine) {
		e.filterSnapshot = filter
	}
}

// WithRateLimit sets the maximum trigger packets per second during ARP resolution.
func WithRateLimit(pps int) EngineOption {
	return func(e *Engine) {
		if pps > 0 {
			e.rateLimitPPS = pps
		}
	}
}

// WithOnProgress sets an optional callback invoked as silent targets are probed.
func WithOnProgress(fn func(probed, total int)) EngineOption {
	return func(e *Engine) {
		e.onProgress = fn
	}
}

// Engine coordinates kernel-provoked ARP discovery and IP-to-MAC resolution.
type Engine struct {
	reader           TableReader
	trigger          Trigger
	ouiDB            *oui.DB
	iface            netinfo.Interface
	timeout          time.Duration
	pollInterval     time.Duration
	concurrency      int
	rateLimitPPS     int
	filterSnapshot   bool
	onHostDiscovered func(HostInfo)
	onProgress       func(probed, total int)

	mu       sync.Mutex
	snapshot map[netip.Addr]TableEntry
}

// NewEngine creates an ARP discovery engine.
func NewEngine(reader TableReader, trigger Trigger, db *oui.DB, iface netinfo.Interface, opts ...EngineOption) *Engine {
	if reader == nil {
		reader = NewDefaultTableReader()
	}
	if trigger == nil {
		trigger = NewDefaultTrigger()
	}

	e := &Engine{
		reader:         reader,
		trigger:        trigger,
		ouiDB:          db,
		iface:          iface,
		timeout:        DefaultARPTimeout,
		pollInterval:   DefaultPollInterval,
		concurrency:    50,
		rateLimitPPS:   DefaultRateLimitPPS,
		filterSnapshot: true,
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// SetOnHostDiscovered configures the callback invoked immediately when a silent host is resolved.
func (e *Engine) SetOnHostDiscovered(fn func(HostInfo)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onHostDiscovered = fn
}

// SetOnProgress configures the callback invoked as silent hosts are probed.
func (e *Engine) SetOnProgress(fn func(probed, total int)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onProgress = fn
}

// SetRateLimit configures the pacing rate for ARP triggers in packets per second.
func (e *Engine) SetRateLimit(pps int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if pps > 0 {
		e.rateLimitPPS = pps
	}
}

// SetFilterSnapshot dynamically toggles snapshot filtering.
func (e *Engine) SetFilterSnapshot(filter bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.filterSnapshot = filter
}

// TakeSnapshot records the current state of the neighbor table.
// Entries that are already valid in this snapshot will not be labeled as "Descoberto via ARP",
// ensuring only newly provoked entries are credited to the scan.
func (e *Engine) TakeSnapshot() error {
	entries, err := e.reader.ReadTable()
	if err != nil {
		return fmt.Errorf("reading neighbor table for snapshot: %w", err)
	}

	e.mu.Lock()
	e.snapshot = entries
	e.mu.Unlock()
	return nil
}

// ResolveSilentHosts targets hosts that did not respond to ICMP, provoking kernel ARP resolution.
// Only IPs within the local interface subnet (L2 domain) are processed.
func (e *Engine) ResolveSilentHosts(ctx context.Context, silentIPs []netip.Addr) ([]HostInfo, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// 1. Filter targets: must be in local L2 subnet and not our own IP
	var targets []netip.Addr
	for _, ip := range silentIPs {
		if !ip.Is4() || !ip.IsValid() {
			continue
		}
		// If interface prefix is defined, target must be inside it
		if e.iface.Prefix.IsValid() && !e.iface.Prefix.Contains(ip) {
			continue
		}
		if e.iface.Addr.IsValid() && ip == e.iface.Addr {
			continue
		}
		targets = append(targets, ip)
	}

	if len(targets) == 0 {
		return nil, nil
	}

	// 2. Ensure snapshot exists
	e.mu.Lock()
	snapshot := e.snapshot
	e.mu.Unlock()

	if e.filterSnapshot && snapshot == nil {
		// Take snapshot if caller did not explicitly invoke TakeSnapshot()
		_ = e.TakeSnapshot()
		e.mu.Lock()
		snapshot = e.snapshot
		e.mu.Unlock()
	}

	// 3. Provoke kernel ARP requests with bounded concurrency and rate limiting
	e.mu.Lock()
	ratePPS := e.rateLimitPPS
	onProg := e.onProgress
	onDisc := e.onHostDiscovered
	filterSnap := e.filterSnapshot
	e.mu.Unlock()

	if ratePPS <= 0 {
		ratePPS = DefaultRateLimitPPS
	}
	// Scale rate limit for ARP triggers so a /24 completes smoothly within ~1.5s while preventing kernel queue drops
	effectivePPS := ratePPS * 2
	if effectivePPS < 100 {
		effectivePPS = 100
	}
	if effectivePPS > 500 {
		effectivePPS = 500
	}
	triggerInterval := time.Second / time.Duration(effectivePPS)
	triggerTicker := time.NewTicker(triggerInterval)
	defer triggerTicker.Stop()

	sem := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup
	var probedCount int64
	totalTargets := len(targets)

provokeLoop:
	for _, ip := range targets {
		select {
		case <-ctx.Done():
			break provokeLoop
		case <-triggerTicker.C:
		}

		select {
		case <-ctx.Done():
			break provokeLoop
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(target netip.Addr) {
			defer wg.Done()
			defer func() { <-sem }()

			// Provoke kernel resolution (e.g. UDP probe on Linux, SendARP on Windows)
			_ = e.trigger.Provoke(ctx, target, e.iface)

			probed := int(atomic.AddInt64(&probedCount, 1))
			if onProg != nil && ctx.Err() == nil {
				onProg(probed, totalTargets)
			}
		}(ip)
	}

	wg.Wait()

	// 4. Poll neighbor table for resolution results
	unresolved := make(map[netip.Addr]struct{}, len(targets))
	for _, ip := range targets {
		unresolved[ip] = struct{}{}
	}

	directResolver, hasDirectResolver := e.trigger.(DirectResolver)

	resolvedHosts := make(map[netip.Addr]HostInfo)
	deadline := time.Now().Add(e.timeout)
	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	checkTable := func() {
		table, err := e.reader.ReadTable()
		if err != nil && !hasDirectResolver {
			return
		}
		if table == nil {
			table = make(map[netip.Addr]TableEntry)
		}

		for ip := range unresolved {
			entry, exists := table[ip]
			if hasDirectResolver {
				if directMAC, ok := directResolver.ResolvedMAC(ip); ok && len(directMAC) == 6 {
					entry = TableEntry{
						IP:              ip,
						MAC:             directMAC,
						IsValid:         true,
						ConfirmedActive: true,
					}
					exists = true
				}
			}

			if !exists || !entry.IsValid {
				continue
			}

			// Check against pre-scan snapshot:
			// Discard if it was already in the snapshot AND has not been actively confirmed reachable
			// during the current scan (e.g., an old unconfirmed STALE cache entry).
			// If the kernel or SendARP confirms the entry is actively reachable (ConfirmedActive == true)
			// or its MAC changed, accept it so firewalled hosts never vanish on consecutive scans.
			if filterSnap && snapshot != nil {
				if oldEntry, wasInSnapshot := snapshot[ip]; wasInSnapshot && oldEntry.IsValid {
					macChanged := len(oldEntry.MAC) > 0 && len(entry.MAC) > 0 && !bytes.Equal(oldEntry.MAC, entry.MAC)
					if !entry.ConfirmedActive && !macChanged {
						delete(unresolved, ip)
						continue
					}
				}
			}

			// Resolved host in neighbor table (valid hardware address)
			host := e.buildHostInfo(ip, entry, StatusDiscoveredARP, false)
			resolvedHosts[ip] = host
			delete(unresolved, ip)

			if onDisc != nil {
				onDisc(host)
			}
		}
	}

	// Immediate check after all triggers have been dispatched
	checkTable()

	for len(unresolved) > 0 {
		select {
		case <-ctx.Done():
			return e.collectResults(resolvedHosts), ctx.Err()
		case now := <-ticker.C:
			checkTable()
			if len(unresolved) == 0 || now.After(deadline) {
				return e.collectResults(resolvedHosts), nil
			}
		}
	}

	return e.collectResults(resolvedHosts), nil
}

// EnrichHost looks up MAC and vendor information for any active host (e.g. from ICMP responses)
// by consulting the operating system neighbor table without emitting extra network packets.
func (e *Engine) EnrichHost(ip netip.Addr) (HostInfo, bool, error) {
	table, err := e.reader.ReadTable()
	if err != nil {
		return HostInfo{}, false, fmt.Errorf("reading neighbor table: %w", err)
	}

	entry, exists := table[ip]
	if !exists || !entry.IsValid {
		return HostInfo{IP: ip, Responded: true, DiscoveredBy: StatusDiscoveredICMP}, false, nil
	}

	host := e.buildHostInfo(ip, entry, StatusDiscoveredICMP, true)
	return host, true, nil
}

func (e *Engine) buildHostInfo(ip netip.Addr, entry TableEntry, status string, responded bool) HostInfo {
	var vendor string
	kind := oui.KindUnknown

	if e.ouiDB != nil && len(entry.MAC) == 6 {
		res := e.ouiDB.Lookup(entry.MAC)
		vendor = res.Vendor
		kind = res.Kind
	}

	return HostInfo{
		IP:           ip,
		MAC:          entry.MAC,
		MACString:    oui.FormatMAC(entry.MAC),
		Vendor:       vendor,
		Kind:         kind,
		DiscoveredBy: status,
		Responded:    responded,
	}
}

func (e *Engine) collectResults(hosts map[netip.Addr]HostInfo) []HostInfo {
	result := make([]HostInfo, 0, len(hosts))
	for _, h := range hosts {
		result = append(result, h)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].IP.Compare(result[j].IP) < 0
	})

	return result
}
