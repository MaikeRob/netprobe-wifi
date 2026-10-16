//go:build windows

package arp

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procGetIpNetTable = modIphlpapi.NewProc("GetIpNetTable")
)

type mibIPNetRow struct {
	Index       uint32
	PhysAddrLen uint32
	PhysAddr    [8]byte
	Addr        uint32
	Type        uint32 // 1: Other, 2: Invalid, 3: Dynamic, 4: Static
}

// WindowsTableReader queries the Windows ARP cache using GetIpNetTable.
type WindowsTableReader struct{}

// NewWindowsTableReader returns a TableReader for Windows.
func NewWindowsTableReader() *WindowsTableReader {
	return &WindowsTableReader{}
}

// ReadTable reads the Windows ARP cache via GetIpNetTable.
func (w *WindowsTableReader) ReadTable() (map[netip.Addr]TableEntry, error) {
	var size uint32 = 0
	// 1. Query required buffer size
	ret, _, _ := procGetIpNetTable.Call(0, uintptr(unsafe.Pointer(&size)), 0)
	if ret != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) && ret != 0 {
		return nil, fmt.Errorf("GetIpNetTable initial size query returned %d", ret)
	}

	if size == 0 {
		return make(map[netip.Addr]TableEntry), nil
	}

	buf := make([]byte, size)
	ret, _, err := procGetIpNetTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0)
	if ret != 0 {
		return nil, fmt.Errorf("GetIpNetTable failed with code %d: %w", ret, err)
	}

	numEntries := binary.LittleEndian.Uint32(buf[0:4])
	entries := make(map[netip.Addr]TableEntry, numEntries)

	rowSize := unsafe.Sizeof(mibIPNetRow{})
	data := buf[4:]

	for i := uint32(0); i < numEntries; i++ {
		offset := uintptr(i) * rowSize
		if offset+rowSize > uintptr(len(data)) {
			break
		}

		row := (*mibIPNetRow)(unsafe.Pointer(&data[offset]))

		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], row.Addr)
		ip := netip.AddrFrom4(b)

		var mac net.HardwareAddr
		var isValid bool

		// Type 2 is MIB_IPNET_TYPE_INVALID (incomplete)
		if row.Type != 2 && row.PhysAddrLen == 6 {
			hw := make(net.HardwareAddr, 6)
			copy(hw, row.PhysAddr[:6])
			if !isAllZeros(hw) {
				mac = hw
				isValid = true
			}
		}

		entries[ip] = TableEntry{
			IP:      ip,
			MAC:     mac,
			Flags:   row.Type,
			Device:  fmt.Sprintf("iface_%d", row.Index),
			IsValid: isValid,
		}
	}

	return entries, nil
}
