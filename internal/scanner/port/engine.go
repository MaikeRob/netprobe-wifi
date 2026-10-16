package port

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Engine coordinates bounded concurrent TCP port sweeps.
type Engine struct {
	dialer PortDialer
}

// NewEngine creates a new port scanning Engine with the provided dialer.
// If dialer is nil, a standard TCPDialer is used.
func NewEngine(dialer PortDialer) *Engine {
	if dialer == nil {
		dialer = NewTCPDialer()
	}
	return &Engine{
		dialer: dialer,
	}
}

// ValidateConfig verifies and normalizes scan configuration according to backend security policy.
func (e *Engine) ValidateConfig(cfg ScanConfig) (ScanConfig, error) {
	// 1. Target IP validation
	if !cfg.Target.IsValid() || !cfg.Target.Is4() {
		return cfg, ErrInvalidTarget
	}
	if cfg.Target.IsLoopback() || cfg.Target.IsMulticast() || cfg.Target.IsUnspecified() {
		return cfg, ErrInvalidTarget
	}
	if cfg.Target.As4() == [4]byte{255, 255, 255, 255} {
		return cfg, ErrInvalidTarget
	}

	// 2. Subnet boundary enforcement
	if cfg.Subnet.IsValid() && !cfg.Subnet.Contains(cfg.Target) {
		return cfg, ErrTargetOutsideSubnet
	}

	// 3. Port range validation
	if cfg.StartPort < 1 || cfg.StartPort > cfg.EndPort {
		return cfg, ErrInvalidPortRange
	}

	// 4. Timeout validation and defaults
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	} else if cfg.Timeout < MinTimeout || cfg.Timeout > MaxTimeout {
		return cfg, fmt.Errorf("%w: got %v", ErrInvalidTimeout, cfg.Timeout)
	}

	// 5. Concurrency validation and defaults
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = DefaultConcurrency
	} else if cfg.Concurrency > MaxConcurrency {
		return cfg, fmt.Errorf("%w: maximum allowed is %d, got %d", ErrConcurrencyLimit, MaxConcurrency, cfg.Concurrency)
	}

	return cfg, nil
}

// Scan executes a TCP connect sweep across the configured port range with global concurrency limits.
// The optional onPort callback is invoked immediately as each port check concludes.
func (e *Engine) Scan(ctx context.Context, cfg ScanConfig, onPort func(PortResult)) ([]PortResult, error) {
	validCfg, err := e.ValidateConfig(cfg)
	if err != nil {
		return nil, err
	}

	totalPorts := int(validCfg.EndPort) - int(validCfg.StartPort) + 1
	results := make([]PortResult, 0, totalPorts)

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, validCfg.Concurrency)

scanLoop:
	for p := int(validCfg.StartPort); p <= int(validCfg.EndPort); p++ {
		select {
		case <-ctx.Done():
			break scanLoop
		case sem <- struct{}{}:
		}

		portNum := uint16(p)
		wg.Add(1)
		go func(targetPort uint16) {
			defer wg.Done()
			defer func() { <-sem }()

			res := e.dialer.DialPort(ctx, validCfg.Target, targetPort, validCfg.Timeout)

			mu.Lock()
			results = append(results, res)
			mu.Unlock()

			if onPort != nil {
				onPort(res)
			}
		}(portNum)
	}

	wg.Wait()

	// Sort results ascending by port number
	sort.Slice(results, func(i, j int) bool {
		return results[i].Port < results[j].Port
	})

	if ctx.Err() != nil {
		return results, ctx.Err()
	}

	return results, nil
}
