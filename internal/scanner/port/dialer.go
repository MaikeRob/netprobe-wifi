package port

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// TCPDialer implements PortDialer using standard library TCP connect scan.
// OBSERVABILITY:
// This performs a standard full TCP 3-way handshake:
// 1. Client sends [SYN]
// 2. Open port replies [SYN, ACK] -> client sends [ACK] -> client immediately closes connection with [FIN] / [RST]
// 3. Closed port replies [RST, ACK]
// 4. Filtered port silently drops [SYN] -> client retransmits until timeout
type TCPDialer struct{}

// NewTCPDialer creates a standard TCP connect scanner.
func NewTCPDialer() *TCPDialer {
	return &TCPDialer{}
}

// DialPort probes a single port and classifies its state as Open, Closed, or Filtered.
func (d *TCPDialer) DialPort(ctx context.Context, target netip.Addr, port uint16, timeout time.Duration) PortResult {
	if ctx.Err() != nil {
		return PortResult{
			Port:    port,
			State:   StateFiltered,
			Service: LookupService(port),
		}
	}

	addr := net.JoinHostPort(target.String(), strconv.Itoa(int(port)))
	dialer := net.Dialer{
		Timeout: timeout,
	}

	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp4", addr)
	latency := time.Since(start)

	if err == nil {
		_ = conn.Close()
		return PortResult{
			Port:    port,
			State:   StateOpen,
			Service: LookupService(port),
			Latency: latency,
		}
	}

	state := classifyError(err, ctx)
	return PortResult{
		Port:    port,
		State:   state,
		Service: LookupService(port),
		Latency: latency,
	}
}

func classifyError(err error, ctx context.Context) PortState {
	// If context was canceled or expired, report as filtered
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return StateFiltered
	}

	// Timeout indicates dropped packets (SYN ignored by firewall)
	if os.IsTimeout(err) {
		return StateFiltered
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return StateFiltered
	}

	// Connection refused indicates target actively responded with TCP RST
	if errors.Is(err, syscall.ECONNREFUSED) {
		return StateClosed
	}

	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "refused") {
		return StateClosed
	}

	// Unreachable or other network drops are treated as filtered
	return StateFiltered
}
