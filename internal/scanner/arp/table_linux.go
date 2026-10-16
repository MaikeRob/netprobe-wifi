//go:build linux

package arp

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"

	"netprobe-wifi/internal/oui"
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

// ReadTable reads the system /proc/net/arp file and returns the parsed entries.
func (r *LinuxTableReader) ReadTable() (map[netip.Addr]TableEntry, error) {
	path := r.path
	if path == "" {
		path = DefaultProcARPPath
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	return ParseARPTable(f)
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
		if flags != 0 && macStr != "00:00:00:00:00:00" {
			parsedMAC, err := oui.ParseMAC(macStr)
			if err == nil && !isAllZeros(parsedMAC) {
				mac = parsedMAC
				// Linux ATF_COM flag (0x02) indicates a completed entry
				if flags&0x02 != 0 {
					isValid = true
				}
			}
		}

		entries[ip] = TableEntry{
			IP:      ip,
			MAC:     mac,
			Flags:   flags,
			Device:  dev,
			IsValid: isValid,
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading ARP table: %w", err)
	}

	return entries, nil
}
