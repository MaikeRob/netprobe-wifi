package port

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"
)

func TestTCPDialer_LocalIntegration(t *testing.T) {
	// 1. Setup a real local TCP listener (Open Port)
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start local test listener: %v", err)
	}
	defer l.Close()

	// Accept connections in background to allow clean 3-way handshake completion
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	_, portStr, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatalf("failed to parse listener port: %v", err)
	}
	openPort, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to convert port: %v", err)
	}

	dialer := NewTCPDialer()
	target := netip.MustParseAddr("127.0.0.1")
	ctx := context.Background()

	// 2. Test Open Port
	resOpen := dialer.DialPort(ctx, target, uint16(openPort), 500*time.Millisecond)
	if resOpen.State != StateOpen {
		t.Errorf("expected StateOpen for port %d, got %v", openPort, resOpen.State)
	}
	if resOpen.Port != uint16(openPort) {
		t.Errorf("expected Port %d, got %d", openPort, resOpen.Port)
	}

	// 3. Test Closed Port (Find a port that is NOT listening)
	// Pick an unlikely high port and ensure it is closed by briefly binding and releasing it
	probeListener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate unused port: %v", err)
	}
	_, closedPortStr, _ := net.SplitHostPort(probeListener.Addr().String())
	closedPort, _ := strconv.Atoi(closedPortStr)
	probeListener.Close() // closed immediately!

	resClosed := dialer.DialPort(ctx, target, uint16(closedPort), 500*time.Millisecond)
	if resClosed.State != StateClosed {
		t.Errorf("expected StateClosed for closed port %d, got %v", closedPort, resClosed.State)
	}

	// 4. Test Filtered on Canceled Context
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // canceled immediately

	resFiltered := dialer.DialPort(canceledCtx, target, uint16(openPort), 500*time.Millisecond)
	if resFiltered.State != StateFiltered {
		t.Errorf("expected StateFiltered on canceled context, got %v", resFiltered.State)
	}
}
