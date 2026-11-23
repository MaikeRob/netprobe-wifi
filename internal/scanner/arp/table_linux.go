//go:build linux

package arp

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"syscall"

	"netprobe-wifi/internal/oui"
)

// Linux NUD (Neighbor Unreachability Detection) state bitmasks from <linux/neighbour.h>.
const (
	nudIncomplete = 0x01
	nudReachable  = 0x02
	nudStale      = 0x04
	nudDelay      = 0x08
	nudProbe      = 0x10
	nudFailed     = 0x20
	nudPermanent  = 0x80

	ndaDst    = 1
	ndaLLAddr = 2
)

// DefaultProcARPPath is the default Linux neighbor table file path.
const DefaultProcARPPath = "/proc/net/arp"

// LinuxTableReader reads and parses the Linux kernel ARP cache from /proc/net/arp.
type LinuxTableReader struct {
	path string
}

// NewLinuxTableReader creates a TableReader pointing to the system /proc/net/arp file.
func NewLinuxTableReader() *LinuxTableReader {
	return &LinuxTableReader{path: DefaultProcARPPath}
}

// ReadTable reads the system /proc/net/arp file and enriches entries with Linux Netlink NUD states.
func (r *LinuxTableReader) ReadTable() (map[netip.Addr]TableEntry, error) {
	path := r.path
	if path == "" {
		path = DefaultProcARPPath
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	entries, err := ParseARPTable(f)
	if err != nil {
		return nil, err
	}

	if path == DefaultProcARPPath {
		enrichWithNetlinkNUD(entries)
	}

	return entries, nil
}

// ParseARPTable parses entries from any reader emitting /proc/net/arp formatted text.
// Header format:
// IP address       HW type     Flags       HW address            Mask     Device
func ParseARPTable(r io.Reader) (map[netip.Addr]TableEntry, error) {
	scanner := bufio.NewScanner(r)
	entries := make(map[netip.Addr]TableEntry)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Skip header line
		if lineNum == 1 && strings.HasPrefix(line, "IP address") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		ip, err := netip.ParseAddr(fields[0])
		if err != nil || !ip.Is4() {
			continue
		}

		flagsStr := strings.TrimPrefix(fields[2], "0x")
		flags64, err := strconv.ParseUint(flagsStr, 16, 32)
		if err != nil {
			continue
		}
		flags := uint32(flags64)

		macStr := fields[3]
		dev := fields[5]

		var mac net.HardwareAddr
		var isValid bool

		// Incomplete entries in Linux have flags=0x0 and MAC="00:00:00:00:00:00"
		var confirmedActive bool
		if flags != 0 && macStr != "00:00:00:00:00:00" {
			parsedMAC, err := oui.ParseMAC(macStr)
			if err == nil && !isAllZeros(parsedMAC) {
				mac = parsedMAC
				// Linux ATF_COM flag (0x02) indicates a completed entry
				if flags&0x02 != 0 {
					isValid = true
				}
				// Linux ATF_PERM flag (0x04) indicates a permanent neighbor entry
				if flags&0x04 != 0 {
					confirmedActive = true
				}
			}
		}

		// Do not overwrite an already valid entry from another interface with an invalid/incomplete one.
		if existing, exists := entries[ip]; exists && existing.IsValid && !isValid {
			continue
		}

		entries[ip] = TableEntry{
			IP:              ip,
			MAC:             mac,
			Flags:           flags,
			Device:          dev,
			IsValid:         isValid,
			ConfirmedActive: confirmedActive,
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading ARP table: %w", err)
	}

	return entries, nil
}

// enrichWithNetlinkNUD queries the kernel neighbor table via unprivileged Netlink (RTM_GETNEIGH)
// to distinguish actively confirmed entries (REACHABLE, PERMANENT) from unconfirmed STALE/DELAY/PROBE entries.
func enrichWithNetlinkNUD(entries map[netip.Addr]TableEntry) {
	rib, err := syscall.NetlinkRIB(syscall.RTM_GETNEIGH, syscall.AF_INET)
	if err != nil {
		return
	}
	applyNetlinkNUDMessages(entries, rib)
}

func applyNetlinkNUDMessages(entries map[netip.Addr]TableEntry, rib []byte) {
	msgs, err := syscall.ParseNetlinkMessage(rib)
	if err != nil {
		return
	}

	for _, m := range msgs {
		if m.Header.Type != syscall.RTM_NEWNEIGH || len(m.Data) < 12 {
			continue
		}

		// struct ndmsg: family(1), pad1(1), pad2(2), ifindex(4), state(2), flags(1), type(1)
		state := binary.NativeEndian.Uint16(m.Data[8:10])
		attrs := m.Data[12:]

		var ip netip.Addr
		var mac net.HardwareAddr

		for len(attrs) >= 4 {
			rtaLen := int(binary.NativeEndian.Uint16(attrs[0:2]))
			rtaType := binary.NativeEndian.Uint16(attrs[2:4])
			if rtaLen < 4 || rtaLen > len(attrs) {
				break
			}
			payload := attrs[4:rtaLen]

			switch rtaType & 0x3fff {
			case ndaDst:
				if len(payload) == 4 {
					ip = netip.AddrFrom4([4]byte{payload[0], payload[1], payload[2], payload[3]})
				}
			case ndaLLAddr:
				if len(payload) == 6 {
					hw := make(net.HardwareAddr, 6)
					copy(hw, payload)
					if !isAllZeros(hw) {
						mac = hw
					}
				}
			}

			aligned := (rtaLen + 3) & ^3
			if aligned >= len(attrs) {
				break
			}
			attrs = attrs[aligned:]
		}

		if !ip.IsValid() {
			continue
		}

		// Only NUD_REACHABLE and NUD_PERMANENT represent L2 reachability confirmed by the kernel.
		// NUD_DELAY and NUD_PROBE are transient states entered immediately when a packet is sent to a
		// NUD_STALE entry before any ARP Reply is received, so they must NOT set ConfirmedActive.
		isConfirmedReachable := (state & (nudReachable | nudPermanent)) != 0
		isValidState := isConfirmedReachable || (state&(nudStale|nudDelay|nudProbe)) != 0

		existing := entries[ip]
		if isValidState && len(mac) == 6 {
			existing.IP = ip
			existing.MAC = mac
			existing.IsValid = true
			if isConfirmedReachable {
				existing.ConfirmedActive = true
			}
			entries[ip] = existing
		}
	}
}
