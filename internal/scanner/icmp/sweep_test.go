package icmp_test

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"netprobe-wifi/internal/netinfo"
	"netprobe-wifi/internal/scanner/icmp"
)

func TestSweepEngine_NormalSweep(t *testing.T) {
	t.Parallel()

	fake := icmp.NewFakePinger()
	defer fake.Close()

	// 192.168.1.0/29 has usable hosts .1 through .6
	ip1 := netip.MustParseAddr("192.168.1.1")
	ip3 := netip.MustParseAddr("192.168.1.3")
	ip5 := netip.MustParseAddr("192.168.1.5")

	fake.SetResponse(ip1, true, 12*time.Millisecond, nil)
	fake.SetResponse(ip3, true, 25*time.Millisecond, nil)
	fake.SetResponse(ip5, true, 5*time.Millisecond, nil)

	target, err := netinfo.ParseTarget("192.168.1.0/29", netip.Addr{}, netinfo.Policy{})
	if err != nil {
		t.Fatalf("unexpected target parse error: %v", err)
	}

	engine, err := icmp.NewSweepEngine(fake, icmp.SweepConfig{
		Timeout:          500 * time.Millisecond,
		RateLimitPPS:     200,
		ConcurrencyLimit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected NewSweepEngine error: %v", err)
	}

	var mu sync.Mutex
	results := make(map[netip.Addr]icmp.Result)

	err = engine.Sweep(context.Background(), target, func(res icmp.Result) {
		mu.Lock()
		defer mu.Unlock()
		results[res.IP] = res
	})

	if err != nil {
		t.Fatalf("unexpected Sweep error: %v", err)
	}

	if len(results) != 6 {
		t.Fatalf("expected 6 results, got %d", len(results))
	}

	// Verify responding hosts
	for _, ip := range []netip.Addr{ip1, ip3, ip5} {
		res, ok := results[ip]
		if !ok {
			t.Errorf("missing result for %s", ip)
			continue
		}
		if !res.Responded {
			t.Errorf("host %s should have responded", ip)
		}
		if res.RTT <= 0 {
			t.Errorf("host %s expected positive RTT, got %v", ip, res.RTT)
		}
	}

	// Verify silent hosts
	ip2 := netip.MustParseAddr("192.168.1.2")
	if res, ok := results[ip2]; !ok || res.Responded {
		t.Errorf("host %s should not have responded", ip2)
	}
}

func TestSweepEngine_RateLimiter(t *testing.T) {
	t.Parallel()

	fake := icmp.NewFakePinger()
	defer fake.Close()

	// /29 = 6 hosts. At 20 PPS, interval is 50ms per host.
	// 6 hosts * 50ms = at least ~250ms elapsed time.
	target, err := netinfo.ParseTarget("192.168.1.0/29", netip.Addr{}, netinfo.Policy{})
	if err != nil {
		t.Fatalf("unexpected target parse error: %v", err)
	}

	engine, err := icmp.NewSweepEngine(fake, icmp.SweepConfig{
		Timeout:          200 * time.Millisecond,
		RateLimitPPS:     20,
		ConcurrencyLimit: 5,
	})
	if err != nil {
		t.Fatalf("unexpected engine error: %v", err)
	}

	start := time.Now()
	err = engine.Sweep(context.Background(), target, func(res icmp.Result) {})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected Sweep error: %v", err)
	}

	// At 20 PPS, 6 hosts require at least ~200ms
	if elapsed < 180*time.Millisecond {
		t.Errorf("sweep finished too fast (%v), rate limiter may not be pacing dispatches", elapsed)
	}
}

func TestSweepEngine_ContextCancellation(t *testing.T) {
	t.Parallel()

	fake := icmp.NewFakePinger()
	defer fake.Close()
	fake.SetDelay(50 * time.Millisecond) // Artificial delay on each probe

	// Large subnet /24 = 254 hosts
	target, err := netinfo.ParseTarget("10.0.0.0/24", netip.Addr{}, netinfo.Policy{})
	if err != nil {
		t.Fatalf("unexpected target parse error: %v", err)
	}

	engine, err := icmp.NewSweepEngine(fake, icmp.SweepConfig{
		Timeout:          500 * time.Millisecond,
		RateLimitPPS:     100,
		ConcurrencyLimit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected engine error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	count := 0

	start := time.Now()
	err = engine.Sweep(ctx, target, func(res icmp.Result) {
		mu.Lock()
		defer mu.Unlock()
		count++
		if count >= 3 {
			cancel() // Cancel sweep after 3 results
		}
	})
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}

	// Must stop promptly, not finish all 254 hosts
	if count > 20 {
		t.Errorf("sweep processed %d hosts after cancellation; expected prompt stop", count)
	}

	// Sweep should cancel within 1 second, not run for 2.5s
	if elapsed > 1500*time.Millisecond {
		t.Errorf("sweep took %v to abort; expected prompt cancellation", elapsed)
	}
}

func TestSweepEngine_Validation(t *testing.T) {
	t.Parallel()

	fake := icmp.NewFakePinger()
	defer fake.Close()

	tests := []struct {
		name    string
		pinger  icmp.Pinger
		cfg     icmp.SweepConfig
		wantErr error
	}{
		{
			name:    "nil pinger",
			pinger:  nil,
			cfg:     icmp.SweepConfig{},
			wantErr: errors.New("pinger cannot be nil"),
		},
		{
			name:    "rate limit too low (< 10)",
			pinger:  fake,
			cfg:     icmp.SweepConfig{RateLimitPPS: 5},
			wantErr: icmp.ErrInvalidRateLimit,
		},
		{
			name:    "rate limit too high (> 500)",
			pinger:  fake,
			cfg:     icmp.SweepConfig{RateLimitPPS: 600},
			wantErr: icmp.ErrInvalidRateLimit,
		},
		{
			name:    "timeout too low (< 200ms)",
			pinger:  fake,
			cfg:     icmp.SweepConfig{Timeout: 100 * time.Millisecond},
			wantErr: icmp.ErrInvalidTimeout,
		},
		{
			name:    "timeout too high (> 5000ms)",
			pinger:  fake,
			cfg:     icmp.SweepConfig{Timeout: 6000 * time.Millisecond},
			wantErr: icmp.ErrInvalidTimeout,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := icmp.NewSweepEngine(tc.pinger, tc.cfg)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) && err.Error() != tc.wantErr.Error() {
				t.Errorf("got error %v; want %v", err, tc.wantErr)
			}
		})
	}
}

func TestFakePinger_Close(t *testing.T) {
	t.Parallel()

	fake := icmp.NewFakePinger()
	if err := fake.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}

	ip := netip.MustParseAddr("192.168.1.1")
	_, err := fake.Ping(context.Background(), ip, 100*time.Millisecond)
	if !errors.Is(err, icmp.ErrPingerClosed) {
		t.Errorf("got %v; want ErrPingerClosed", err)
	}
}
