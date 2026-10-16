package icmp

import (
	"context"
	"errors"
	"net/netip"
	"time"
)

const (
	// DefaultRateLimitPPS is the default rate limit in packets per second.
	DefaultRateLimitPPS = 100
	// MinRateLimitPPS is the minimum allowed rate limit.
	MinRateLimitPPS = 10
	// MaxRateLimitPPS is the maximum allowed rate limit enforced by backend.
	MaxRateLimitPPS = 500

	// DefaultTimeout is the default ICMP wait time per target.
	DefaultTimeout = 1000 * time.Millisecond
	// MinTimeout is the minimum allowed timeout.
	MinTimeout = 200 * time.Millisecond
	// MaxTimeout is the maximum allowed timeout.
	MaxTimeout = 5000 * time.Millisecond

	// DefaultConcurrencyLimit is the default number of concurrent ping workers.
	DefaultConcurrencyLimit = 50
)

var (
	// ErrPermissionDenied is returned when the process lacks permissions to send ICMP echo requests.
	ErrPermissionDenied = errors.New("insufficient privileges for ICMP sockets (requires cap_net_raw or ping_group_range)")
	// ErrSweepCancelled is returned when the sweep is aborted via context.
	ErrSweepCancelled = errors.New("sweep cancelled by context")
	// ErrPingerClosed is returned when an operation is performed on a closed Pinger.
	ErrPingerClosed = errors.New("pinger is closed")
	// ErrInvalidRateLimit is returned when requested PPS exceeds allowed bounds.
	ErrInvalidRateLimit = errors.New("rate limit out of allowed range (10 to 500 pps)")
	// ErrInvalidTimeout is returned when requested timeout exceeds allowed bounds.
	ErrInvalidTimeout = errors.New("timeout out of allowed range (200 to 5000 ms)")
)

// Result represents the outcome of probing an individual host with ICMP.
type Result struct {
	IP        netip.Addr
	Responded bool
	RTT       time.Duration
	Err       error
}

// Pinger defines the interface for sending ICMP echo requests to a target address.
type Pinger interface {
	Ping(ctx context.Context, target netip.Addr, timeout time.Duration) (Result, error)
	Close() error
}
