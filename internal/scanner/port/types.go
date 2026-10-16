package port

import (
	"context"
	"errors"
	"net/netip"
	"time"
)

// PortState classifies the operational status of a scanned TCP port.
type PortState string

const (
	// StateOpen indicates a full TCP 3-way handshake was established.
	StateOpen PortState = "open"
	// StateClosed indicates the target actively rejected the connection (TCP RST).
	StateClosed PortState = "closed"
	// StateFiltered indicates no response was received before timeout (SYN dropped by firewall).
	StateFiltered PortState = "filtered"
)

const (
	// DefaultTimeout is the default duration to wait for a TCP connection handshake.
	DefaultTimeout = 1000 * time.Millisecond
	// MinTimeout is the minimum allowed timeout to prevent false filtered classifications.
	MinTimeout = 200 * time.Millisecond
	// MaxTimeout is the maximum allowed timeout per port.
	MaxTimeout = 5000 * time.Millisecond

	// DefaultConcurrency is the default number of concurrent TCP connections.
	DefaultConcurrency = 200
	// MaxConcurrency is the strict backend ceiling to avoid hitting system file descriptor limits (ulimit -n).
	MaxConcurrency = 500
)

var (
	// ErrInvalidPortRange is returned when port numbers are outside 1..65535 or start > end.
	ErrInvalidPortRange = errors.New("start port must be >= 1 and <= end port <= 65535")
	// ErrInvalidTimeout is returned when timeout is outside 200ms..5000ms.
	ErrInvalidTimeout = errors.New("timeout must be between 200ms and 5000ms")
	// ErrInvalidTarget is returned when target IP is invalid, loopback, multicast, or broadcast.
	ErrInvalidTarget = errors.New("target IP must be a valid, non-loopback, non-multicast IPv4 address")
	// ErrTargetOutsideSubnet is returned when target does not belong to the allowed local interface prefix.
	ErrTargetOutsideSubnet = errors.New("target IP is outside the allowed local subnet")
	// ErrConcurrencyLimit is returned when concurrency exceeds MaxConcurrency.
	ErrConcurrencyLimit = errors.New("concurrency exceeds backend limit")
)

// PortResult represents the outcome of probing a single TCP port.
type PortResult struct {
	Port    uint16        `json:"port"`
	State   PortState     `json:"state"`
	Service string        `json:"service"`
	Latency time.Duration `json:"latency"`
}

// ScanConfig defines the parameters for a TCP port sweep.
type ScanConfig struct {
	Target      netip.Addr    `json:"target"`
	Subnet      netip.Prefix  `json:"subnet,omitempty"`
	StartPort   uint16        `json:"startPort"`
	EndPort     uint16        `json:"endPort"`
	Timeout     time.Duration `json:"timeout"`
	Concurrency int           `json:"concurrency"`
}

// PortDialer abstracts initiating a TCP connect scan against a target port.
type PortDialer interface {
	DialPort(ctx context.Context, target netip.Addr, port uint16, timeout time.Duration) PortResult
}
