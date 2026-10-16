package icmp

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"netprobe-wifi/internal/netinfo"
)

// SweepConfig contains tuning parameters for the ICMP sweep.
type SweepConfig struct {
	Timeout          time.Duration
	RateLimitPPS     int
	ConcurrencyLimit int
}

// SweepEngine coordinates the distributed probing of a target subnet.
type SweepEngine struct {
	pinger Pinger
	cfg    SweepConfig
}

// NewSweepEngine creates an engine with the given pinger and configuration.
func NewSweepEngine(pinger Pinger, cfg SweepConfig) (*SweepEngine, error) {
	if pinger == nil {
		return nil, fmt.Errorf("pinger cannot be nil")
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	} else if cfg.Timeout < MinTimeout || cfg.Timeout > MaxTimeout {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTimeout, cfg.Timeout)
	}

	if cfg.RateLimitPPS == 0 {
		cfg.RateLimitPPS = DefaultRateLimitPPS
	} else if cfg.RateLimitPPS < MinRateLimitPPS || cfg.RateLimitPPS > MaxRateLimitPPS {
		return nil, fmt.Errorf("%w: %d", ErrInvalidRateLimit, cfg.RateLimitPPS)
	}

	if cfg.ConcurrencyLimit <= 0 {
		cfg.ConcurrencyLimit = DefaultConcurrencyLimit
	}

	return &SweepEngine{
		pinger: pinger,
		cfg:    cfg,
	}, nil
}

// Sweep probes all hosts in the target network sequentially according to the rate limiter,
// dispatching pings to a bounded pool of concurrent workers.
// The callback onResult is invoked for each probed host as soon as its ping completes.
// Cancelling ctx immediately aborts new dispatches and waits gracefully for in-flight workers.
func (s *SweepEngine) Sweep(ctx context.Context, target netinfo.Target, onResult func(Result)) error {
	interval := time.Second / time.Duration(s.cfg.RateLimitPPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sem := make(chan struct{}, s.cfg.ConcurrencyLimit)
	var wg sync.WaitGroup

	defer wg.Wait() // Ensure zero leaked worker goroutines on exit

	err := target.IterateHosts(ctx, func(addr netip.Addr) bool {
		// 1. Check context before waiting
		if ctx.Err() != nil {
			return false
		}

		// 2. Rate limiter wait
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}

		// 3. Acquire worker semaphore
		select {
		case <-ctx.Done():
			return false
		case sem <- struct{}{}:
		}

		// 4. Launch ping worker
		wg.Add(1)
		go func(ip netip.Addr) {
			defer wg.Done()
			defer func() { <-sem }()

			// Create a per-host ping context bounded by timeout and parent ctx
			pingCtx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
			defer cancel()

			res, err := s.pinger.Ping(pingCtx, ip, s.cfg.Timeout)
			if err != nil && res.Err == nil {
				res.Err = err
			}

			// Don't deliver results if parent context was cancelled
			if ctx.Err() == nil && onResult != nil {
				onResult(res)
			}
		}(addr)

		return true
	})

	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	return nil
}
