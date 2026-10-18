package arp

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"sync"
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

// Engine coordinates kernel-provoked ARP discovery and IP-to-MAC resolution.
type Engine struct {
	reader           TableReader
	trigger          Trigger
	ouiDB            *oui.DB
	iface            netinfo.Interface
	timeout          time.Duration
	pollInterval     time.Duration
	concurrency      int
	filterSnapshot   bool
	onHostDiscovered func(HostInfo)

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
		filterSnapshot: true,
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
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

	// 3. Provoke kernel ARP requests with bounded concurrency
	sem := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup

	for _, ip := range targets {
		if ctx.Err() != nil {
			break
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(target netip.Addr) {
			defer wg.Done()
			defer func() { <-sem }()

			// Provoke kernel resolution (e.g. UDP probe on Linux, SendARP on Windows)
			_ = e.trigger.Provoke(ctx, target, e.iface)
		}(ip)
	}

	wg.Wait()

	// 4. Poll neighbor table for resolution results
	unresolved := make(map[netip.Addr]struct{}, len(targets))
	for _, ip := range targets {
		unresolved[ip] = struct{}{}
	}

	resolvedHosts := make(map[netip.Addr]HostInfo)
	deadline := time.Now().Add(e.timeout)
	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	for len(unresolved) > 0 {
		select {
		case <-ctx.Done():
			return e.collectResults(resolvedHosts), ctx.Err()
		case now := <-ticker.C:
			table, err := e.reader.ReadTable()
			if err != nil {
				// Table read failure is transient or fatal; if deadline reached, stop
				if now.After(deadline) {
					return e.collectResults(resolvedHosts), nil
				}
				continue
			}

			for ip := range unresolved {
				entry, exists := table[ip]
				if !exists || !entry.IsValid {
					continue
				}

				// Check against pre-scan snapshot:
				// Discard if it was already completed before the scan
				if e.filterSnapshot && snapshot != nil {
					if oldEntry, wasInSnapshot := snapshot[ip]; wasInSnapshot && oldEntry.IsValid {
						delete(unresolved, ip)
						continue
					}
				}

				// Resolved host in neighbor table (valid hardware address)
				host := e.buildHostInfo(ip, entry, StatusDiscoveredARP, false)
				resolvedHosts[ip] = host
				delete(unresolved, ip)

				if e.onHostDiscovered != nil {
					e.onHostDiscovered(host)
				}
			}

			if now.After(deadline) {
				return e.collectResults(resolvedHosts), nil
			}
		}
	}

	return e.collectResults(resolvedHosts), nil
}

// EnrichHost looks up MAC and vendor information for any active host (e.g. from ICMP responses).
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
