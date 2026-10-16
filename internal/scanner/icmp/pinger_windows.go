//go:build windows

package icmp

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/netip"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modIphlpapi         = windows.NewLazySystemDLL("iphlpapi.dll")
	procIcmpCreateFile  = modIphlpapi.NewProc("IcmpCreateFile")
	procIcmpCloseHandle = modIphlpapi.NewProc("IcmpCloseHandle")
	procIcmpSendEcho    = modIphlpapi.NewProc("IcmpSendEcho")
)

type ipOptionInformation struct {
	Ttl         byte
	Tos         byte
	Flags       byte
	OptionsSize byte
	OptionsData uintptr
}

type icmpEchoReply struct {
	Address       uint32
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          uintptr
	Options       ipOptionInformation
}

// WindowsPinger implements Pinger on Windows using the native iphlpapi.dll IcmpSendEcho API.
// It requires NO administrator privileges and does not require raw sockets.
type WindowsPinger struct {
	mu     sync.Mutex
	handle windows.Handle
	closed bool
}

// NewWindowsPinger opens an ICMP handle via iphlpapi.dll.
func NewWindowsPinger() (*WindowsPinger, error) {
	h, _, err := procIcmpCreateFile.Call()
	if windows.Handle(h) == windows.InvalidHandle {
		return nil, fmt.Errorf("IcmpCreateFile failed: %w", err)
	}

	return &WindowsPinger{
		handle: windows.Handle(h),
	}, nil
}

// Ping sends a single ICMP echo probe using IcmpSendEcho.
func (p *WindowsPinger) Ping(ctx context.Context, target netip.Addr, timeout time.Duration) (Result, error) {
	if ctx.Err() != nil {
		return Result{IP: target, Err: ctx.Err()}, ctx.Err()
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return Result{IP: target, Err: ErrPingerClosed}, ErrPingerClosed
	}
	h := p.handle
	p.mu.Unlock()

	// Convert IPv4 to in_addr uint32 (little endian representation of bytes)
	ip4 := target.As4()
	destIP := binary.LittleEndian.Uint32(ip4[:])

	sendData := []byte("netprobe-wifi-ping")
	replySize := uint32(unsafe.Sizeof(icmpEchoReply{}) + uintptr(len(sendData)) + 32)
	replyBuf := make([]byte, replySize)

	timeoutMs := uint32(timeout.Milliseconds())
	if timeoutMs == 0 {
		timeoutMs = 1000
	}

	start := time.Now()
	ret, _, _ := procIcmpSendEcho.Call(
		uintptr(h),
		uintptr(destIP),
		uintptr(unsafe.Pointer(&sendData[0])),
		uintptr(len(sendData)),
		0,
		uintptr(unsafe.Pointer(&replyBuf[0])),
		uintptr(replySize),
		uintptr(timeoutMs),
	)

	// If context was cancelled during the blocking call, abort
	if ctx.Err() != nil {
		return Result{IP: target, Err: ctx.Err()}, ctx.Err()
	}

	if ret == 0 {
		// No reply received (timeout or host unreachable)
		return Result{
			IP:        target,
			Responded: false,
			RTT:       timeout,
		}, nil
	}

	reply := (*icmpEchoReply)(unsafe.Pointer(&replyBuf[0]))
	if reply.Status != 0 {
		// Non-zero status indicates destination unreachable or TTL expired
		return Result{
			IP:        target,
			Responded: false,
			RTT:       timeout,
		}, nil
	}

	rtt := time.Duration(reply.RoundTripTime) * time.Millisecond
	if rtt <= 0 {
		rtt = time.Since(start)
	}

	return Result{
		IP:        target,
		Responded: true,
		RTT:       rtt,
	}, nil
}

// Close closes the ICMP handle.
func (p *WindowsPinger) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true

	ret, _, err := procIcmpCloseHandle.Call(uintptr(p.handle))
	if ret == 0 {
		return fmt.Errorf("IcmpCloseHandle failed: %w", err)
	}
	return nil
}
