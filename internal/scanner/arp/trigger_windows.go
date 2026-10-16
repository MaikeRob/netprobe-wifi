//go:build windows

package arp

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
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
// It triggers an ARP request at the NDIS layer and updates the Windows neighbor cache.
type WindowsTrigger struct {
	mu sync.Mutex
}

// NewWindowsTrigger creates a Trigger for Windows.
func NewWindowsTrigger() *WindowsTrigger {
	return &WindowsTrigger{}
}

// Provoke invokes SendARP to resolve the MAC address of target.
func (t *WindowsTrigger) Provoke(ctx context.Context, target netip.Addr, iface netinfo.Interface) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	destIPBytes := target.As4()
	destIP := binary.LittleEndian.Uint32(destIPBytes[:])

	var srcIP uint32
	if iface.Addr.IsValid() {
		srcIPBytes := iface.Addr.As4()
		srcIP = binary.LittleEndian.Uint32(srcIPBytes[:])
	}

	var macAddr [2]uint32
	var phyAddrLen uint32 = 6

	ret, _, err := procSendARP.Call(
		uintptr(destIP),
		uintptr(srcIP),
		uintptr(unsafe.Pointer(&macAddr[0])),
		uintptr(unsafe.Pointer(&phyAddrLen)),
	)

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if ret != 0 {
		return fmt.Errorf("SendARP returned code %d: %w", ret, err)
	}

	return nil
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
