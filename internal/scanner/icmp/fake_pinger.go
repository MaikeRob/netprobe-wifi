package icmp

import (
	"context"
	"net/netip"
	"sync"
	"time"
)

type fakeResponse struct {
	responded bool
	rtt       time.Duration
	err       error
}

// FakePinger is an in-memory, deterministic Pinger for unit tests.
type FakePinger struct {
	mu        sync.RWMutex
	responses map[netip.Addr]fakeResponse
	pingCount map[netip.Addr]int
	closed    bool
	delay     time.Duration
}

// NewFakePinger creates a new FakePinger.
func NewFakePinger() *FakePinger {
	return &FakePinger{
		responses: make(map[netip.Addr]fakeResponse),
		pingCount: make(map[netip.Addr]int),
	}
}

// SetResponse configures the simulated outcome for a specific IP.
func (f *FakePinger) SetResponse(ip netip.Addr, responded bool, rtt time.Duration, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responses[ip] = fakeResponse{
		responded: responded,
		rtt:       rtt,
		err:       err,
	}
}

// SetDelay configures an artificial latency delay simulated before responding.
func (f *FakePinger) SetDelay(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.delay = d
}

// Ping simulates sending an echo request to target.
func (f *FakePinger) Ping(ctx context.Context, target netip.Addr, timeout time.Duration) (Result, error) {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return Result{IP: target, Err: ErrPingerClosed}, ErrPingerClosed
	}
	f.pingCount[target]++
	resp, exists := f.responses[target]
	delay := f.delay
	f.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return Result{IP: target, Err: ctx.Err()}, ctx.Err()
		}
	}

	if !exists {
		// Default: no response (timeout)
		return Result{
			IP:        target,
			Responded: false,
			RTT:       timeout,
		}, nil
	}

	return Result{
		IP:        target,
		Responded: resp.responded,
		RTT:       resp.rtt,
		Err:       resp.err,
	}, resp.err
}

// PingCount returns the number of times target was probed.
func (f *FakePinger) PingCount(target netip.Addr) int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.pingCount[target]
}

// TotalPings returns the sum of all pings executed.
func (f *FakePinger) TotalPings() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	total := 0
	for _, count := range f.pingCount {
		total += count
	}
	return total
}

// Close marks the fake pinger as closed.
func (f *FakePinger) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}
