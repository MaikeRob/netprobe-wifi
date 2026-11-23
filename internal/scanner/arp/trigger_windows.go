//go:build windows

package arp

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"netprobe-wifi/internal/netinfo"
)

var (
	modIphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")
	procSendARP = modIphlpapi.NewProc("SendARP")
)

// WindowsTrigger implements Trigger using the native SendARP function in iphlpapi.dll.
// It triggers an ARP request at the NDIS layer and records active MAC confirmations.
type WindowsTrigger struct {
	mu       sync.Mutex
	resolved map[netip.Addr]net.HardwareAddr
}

// NewWindowsTrigger creates a Trigger for Windows.
func NewWindowsTrigger() *WindowsTrigger {
	return &WindowsTrigger{
		resolved: make(map[netip.Addr]net.HardwareAddr),
	}
}

// Provoke invokes SendARP to resolve the MAC address of target.
func (t *WindowsTrigger) Provoke(ctx context.Context, target netip.Addr, iface netinfo.Interface) error {
	mac, err := SendAndGetMAC(ctx, target, iface)
	t.mu.Lock()
	if err == nil && len(mac) == 6 && !isAllZeros(mac) {
		if t.resolved == nil {
			t.resolved = make(map[netip.Addr]net.HardwareAddr)
		}
		t.resolved[target] = mac
	} else if t.resolved != nil {
		delete(t.resolved, target)
	}
	t.mu.Unlock()
	return err
}

// ResolvedMAC returns the hardware address confirmed by SendARP during the current scan.
func (t *WindowsTrigger) ResolvedMAC(ip netip.Addr) (net.HardwareAddr, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.resolved == nil {
		return nil, false
	}
	mac, ok := t.resolved[ip]
	return mac, ok
}

// SendAndGetMAC executes SendARP and directly returns the resolved MAC on Windows.
func SendAndGetMAC(ctx context.Context, target netip.Addr, iface netinfo.Interface) (net.HardwareAddr, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	destIPBytes := target.As4()
	destIP := binary.LittleEndian.Uint32(destIPBytes[:])

	var srcIP uint32
	if iface.Addr.IsValid() {
		srcIPBytes := iface.Addr.As4()
		srcIP = binary.LittleEndian.Uint32(srcIPBytes[:])
	}

	var macBuf [8]byte
	var phyAddrLen uint32 = 6

	ret, _, err := procSendARP.Call(
		uintptr(destIP),
		uintptr(srcIP),
		uintptr(unsafe.Pointer(&macBuf[0])),
		uintptr(unsafe.Pointer(&phyAddrLen)),
	)
	runtime.KeepAlive(&macBuf)
	runtime.KeepAlive(&phyAddrLen)

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if ret != 0 {
		return nil, fmt.Errorf("SendARP error code %d: %w", ret, err)
	}

	if phyAddrLen != 6 {
		return nil, fmt.Errorf("unexpected physical address length: %d", phyAddrLen)
	}

	mac := make(net.HardwareAddr, 6)
	copy(mac, macBuf[:6])
	return mac, nil
}
