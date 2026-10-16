package port

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockDialer struct {
	mu          sync.Mutex
	dialFunc    func(ctx context.Context, target netip.Addr, port uint16, timeout time.Duration) PortResult
	activeDials int64
	maxActive   int64
	probedPorts []uint16
}

func (m *mockDialer) DialPort(ctx context.Context, target netip.Addr, port uint16, timeout time.Duration) PortResult {
	curr := atomic.AddInt64(&m.activeDials, 1)
	defer atomic.AddInt64(&m.activeDials, -1)

	// Record maximum simultaneous dials
	for {
		max := atomic.LoadInt64(&m.maxActive)
		if curr <= max || atomic.CompareAndSwapInt64(&m.maxActive, max, curr) {
			break
		}
	}

	m.mu.Lock()
	m.probedPorts = append(m.probedPorts, port)
	m.mu.Unlock()

	if m.dialFunc != nil {
		return m.dialFunc(ctx, target, port, timeout)
	}

	return PortResult{
		Port:    port,
		State:   StateClosed,
		Service: LookupService(port),
	}
}

func TestEngine_Validation(t *testing.T) {
	t.Parallel()

	engine := NewEngine(&mockDialer{})
	validTarget := netip.MustParseAddr("192.168.1.10")
	validSubnet := netip.MustParsePrefix("192.168.1.0/24")

	tests := []struct {
		name        string
		cfg         ScanConfig
		expectedErr error
	}{
		{
			name: "valid_config",
			cfg: ScanConfig{
				Target:      validTarget,
				Subnet:      validSubnet,
				StartPort:   1,
				EndPort:     1024,
				Timeout:     1000 * time.Millisecond,
				Concurrency: 100,
			},
			expectedErr: nil,
		},
		{
			name: "invalid_target_loopback",
			cfg: ScanConfig{
				Target:    netip.MustParseAddr("127.0.0.1"),
				StartPort: 80,
				EndPort:   80,
			},
			expectedErr: ErrInvalidTarget,
		},
		{
			name: "invalid_target_multicast",
			cfg: ScanConfig{
				Target:    netip.MustParseAddr("224.0.0.1"),
				StartPort: 80,
				EndPort:   80,
			},
			expectedErr: ErrInvalidTarget,
		},
		{
			name: "invalid_target_broadcast",
			cfg: ScanConfig{
				Target:    netip.MustParseAddr("255.255.255.255"),
				StartPort: 80,
				EndPort:   80,
			},
			expectedErr: ErrInvalidTarget,
		},
		{
			name: "target_outside_subnet",
			cfg: ScanConfig{
				Target:    netip.MustParseAddr("10.0.0.5"),
				Subnet:    validSubnet,
				StartPort: 80,
				EndPort:   80,
			},
			expectedErr: ErrTargetOutsideSubnet,
		},
		{
			name: "start_port_zero",
			cfg: ScanConfig{
				Target:    validTarget,
				StartPort: 0,
				EndPort:   80,
			},
			expectedErr: ErrInvalidPortRange,
		},
		{
			name: "start_port_greater_than_end",
			cfg: ScanConfig{
				Target:    validTarget,
				StartPort: 443,
				EndPort:   80,
			},
			expectedErr: ErrInvalidPortRange,
		},
		{
			name: "timeout_too_low",
			cfg: ScanConfig{
				Target:    validTarget,
				StartPort: 80,
				EndPort:   80,
				Timeout:   50 * time.Millisecond,
			},
			expectedErr: ErrInvalidTimeout,
		},
		{
			name: "timeout_too_high",
			cfg: ScanConfig{
				Target:    validTarget,
				StartPort: 80,
				EndPort:   80,
				Timeout:   10000 * time.Millisecond,
			},
			expectedErr: ErrInvalidTimeout,
		},
		{
			name: "concurrency_exceeds_max",
			cfg: ScanConfig{
				Target:      validTarget,
				StartPort:   80,
				EndPort:     80,
				Concurrency: 600,
			},
			expectedErr: ErrConcurrencyLimit,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := engine.ValidateConfig(tt.cfg)
			if tt.expectedErr == nil && err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if tt.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.expectedErr)
				}
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected error wrapping %v, got: %v", tt.expectedErr, err)
				}
			}
		})
	}
}

func TestEngine_ConcurrencyLimitEnforced(t *testing.T) {
	mock := &mockDialer{
		dialFunc: func(ctx context.Context, target netip.Addr, port uint16, timeout time.Duration) PortResult {
			// Simulate slight network delay
			time.Sleep(20 * time.Millisecond)
			return PortResult{Port: port, State: StateClosed}
		},
	}

	engine := NewEngine(mock)
	cfg := ScanConfig{
		Target:      netip.MustParseAddr("192.168.1.10"),
		StartPort:   1,
		EndPort:     50,
		Timeout:     500 * time.Millisecond,
		Concurrency: 5, // Strict limit of 5 simultaneous workers
	}

	results, err := engine.Scan(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}

	if len(results) != 50 {
		t.Fatalf("expected 50 results, got %d", len(results))
	}

	maxObserved := atomic.LoadInt64(&mock.maxActive)
	if maxObserved > 5 {
		t.Errorf("concurrency violation: max concurrent dials was %d, expected <= 5", maxObserved)
	}
}

func TestEngine_ContextCancellation(t *testing.T) {
	mock := &mockDialer{
		dialFunc: func(ctx context.Context, target netip.Addr, port uint16, timeout time.Duration) PortResult {
			select {
			case <-ctx.Done():
				return PortResult{Port: port, State: StateFiltered}
			case <-time.After(100 * time.Millisecond):
				return PortResult{Port: port, State: StateClosed}
			}
		},
	}

	engine := NewEngine(mock)
	cfg := ScanConfig{
		Target:      netip.MustParseAddr("192.168.1.10"),
		StartPort:   1,
		EndPort:     100,
		Timeout:     500 * time.Millisecond,
		Concurrency: 5,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := engine.Scan(ctx, cfg, nil)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation/deadline error, got %v", err)
	}

	if elapsed > 500*time.Millisecond {
		t.Fatalf("cancellation took too long to abort: %v", elapsed)
	}
}

func TestEngine_SortingAndStreaming(t *testing.T) {
	mock := &mockDialer{
		dialFunc: func(ctx context.Context, target netip.Addr, port uint16, timeout time.Duration) PortResult {
			state := StateClosed
			if port == 80 || port == 443 {
				state = StateOpen
			}
			return PortResult{
				Port:    port,
				State:   state,
				Service: LookupService(port),
			}
		},
	}

	engine := NewEngine(mock)
	cfg := ScanConfig{
		Target:      netip.MustParseAddr("192.168.1.10"),
		StartPort:   79,
		EndPort:     82,
		Timeout:     500 * time.Millisecond,
		Concurrency: 10,
	}

	var streamMu sync.Mutex
	var streamed []PortResult

	results, err := engine.Scan(context.Background(), cfg, func(res PortResult) {
		streamMu.Lock()
		streamed = append(streamed, res)
		streamMu.Unlock()
	})

	if err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// Verify ascending sort order
	expectedPorts := []uint16{79, 80, 81, 82}
	for i, r := range results {
		if r.Port != expectedPorts[i] {
			t.Errorf("result[%d] port = %d, expected %d", i, r.Port, expectedPorts[i])
		}
		if r.Port == 80 {
			if r.State != StateOpen {
				t.Errorf("expected port 80 to be StateOpen, got %v", r.State)
			}
			if r.Service != "HTTP" {
				t.Errorf("expected port 80 service HTTP, got %q", r.Service)
			}
		}
	}

	if len(streamed) != 4 {
		t.Errorf("expected 4 streamed callbacks, got %d", len(streamed))
	}
}
