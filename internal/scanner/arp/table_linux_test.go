//go:build linux

package arp

import (
	"encoding/binary"
	"net"
	"net/netip"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestParseARPTable_Fixture(t *testing.T) {
	f, err := os.Open("testdata/proc_net_arp_sample.txt")
	if err != nil {
		t.Fatalf("failed to open test fixture: %v", err)
	}
	defer f.Close()

	entries, err := ParseARPTable(f)
	if err != nil {
		t.Fatalf("ParseARPTable failed: %v", err)
	}

	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	// 1. 192.168.1.1 (Valid, Complete ATF_COM 0x2)
	ip1 := netip.MustParseAddr("192.168.1.1")
	entry1, ok := entries[ip1]
	if !ok {
		t.Fatalf("missing entry for %s", ip1)
	}
	if !entry1.IsValid {
		t.Errorf("expected %s to be valid", ip1)
	}
	if entry1.MAC.String() != "00:1a:2b:3c:4d:5e" {
		t.Errorf("expected MAC 00:1a:2b:3c:4d:5e, got %s", entry1.MAC)
	}
	if entry1.Device != "wlan0" {
		t.Errorf("expected device wlan0, got %s", entry1.Device)
	}

	// 2. 192.168.1.10 (Incomplete 0x0, 00:00:00:00:00:00)
	ip2 := netip.MustParseAddr("192.168.1.10")
	entry2, ok := entries[ip2]
	if !ok {
		t.Fatalf("missing entry for %s", ip2)
	}
	if entry2.IsValid {
		t.Errorf("expected %s (incomplete) to be invalid", ip2)
	}
	if entry2.MAC != nil {
		t.Errorf("expected nil MAC for incomplete entry, got %v", entry2.MAC)
	}

	// 3. 192.168.1.20 (Valid, randomized MAC da:a1:19:12:34:56)
	ip3 := netip.MustParseAddr("192.168.1.20")
	entry3, ok := entries[ip3]
	if !ok {
		t.Fatalf("missing entry for %s", ip3)
	}
	if !entry3.IsValid {
		t.Errorf("expected %s to be valid", ip3)
	}
	if entry3.MAC.String() != "da:a1:19:12:34:56" {
		t.Errorf("expected MAC da:a1:19:12:34:56, got %s", entry3.MAC)
	}

	// 4. 192.168.1.30 (Valid, Flags 0x6: ATF_COM | ATF_PERM)
	ip4 := netip.MustParseAddr("192.168.1.30")
	entry4, ok := entries[ip4]
	if !ok {
		t.Fatalf("missing entry for %s", ip4)
	}
	if !entry4.IsValid {
		t.Errorf("expected %s to be valid with flags 0x6", ip4)
	}
}

func TestParseARPTable_EdgeCases(t *testing.T) {
	data := `
# some junk comment
invalid_ip       0x1         0x2         00:11:22:33:44:55     *        eth0
192.168.1.50     0x1         not_hex     00:11:22:33:44:55     *        eth0
short_line
192.168.1.60     0x1         0x2         bad_mac               *        eth0
`
	entries, err := ParseARPTable(strings.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error parsing edge cases: %v", err)
	}

	// Bad lines should be silently skipped without crashing
	ipBadMac := netip.MustParseAddr("192.168.1.60")
	if entry, ok := entries[ipBadMac]; ok {
		if entry.IsValid {
			t.Errorf("entry with bad MAC should not be marked valid")
		}
	}
}

func buildTestNetlinkNeighMsg(state uint16, ip [4]byte, mac net.HardwareAddr) []byte {
	// rtattr 1: NDA_DST (len=8, type=1, 4-byte IPv4)
	// rtattr 2: NDA_LLADDR (len=10, type=2, 6-byte MAC, padded to 12 bytes)
	const nlmsgHdrLen = 16
	const ndmsgLen = 12
	const dstAttrLen = 8
	const llAttrLen = 10
	const llAttrAligned = 12
	totalLen := nlmsgHdrLen + ndmsgLen + dstAttrLen + llAttrAligned

	buf := make([]byte, totalLen)
	// nlmsghdr
	binary.NativeEndian.PutUint32(buf[0:4], uint32(totalLen))
	binary.NativeEndian.PutUint16(buf[4:6], syscall.RTM_NEWNEIGH)
	binary.NativeEndian.PutUint16(buf[6:8], 0)
	// ndmsg
	buf[nlmsgHdrLen] = syscall.AF_INET
	binary.NativeEndian.PutUint16(buf[nlmsgHdrLen+8:nlmsgHdrLen+10], state)

	// NDA_DST
	off := nlmsgHdrLen + ndmsgLen
	binary.NativeEndian.PutUint16(buf[off:off+2], uint16(dstAttrLen))
	binary.NativeEndian.PutUint16(buf[off+2:off+4], ndaDst)
	copy(buf[off+4:off+8], ip[:])

	// NDA_LLADDR
	off += dstAttrLen
	binary.NativeEndian.PutUint16(buf[off:off+2], uint16(llAttrLen))
	binary.NativeEndian.PutUint16(buf[off+2:off+4], ndaLLAddr)
	copy(buf[off+4:off+10], mac)

	return buf
}

func TestApplyNetlinkNUDMessages(t *testing.T) {
	ipReachable := netip.MustParseAddr("192.168.1.10")
	ipDelay := netip.MustParseAddr("192.168.1.20")
	ipProbe := netip.MustParseAddr("192.168.1.30")
	ipStale := netip.MustParseAddr("192.168.1.40")
	mac := net.HardwareAddr{0x00, 0x50, 0x56, 0x11, 0x22, 0x33}

	var rib []byte
	rib = append(rib, buildTestNetlinkNeighMsg(nudReachable, ipReachable.As4(), mac)...)
	rib = append(rib, buildTestNetlinkNeighMsg(nudDelay, ipDelay.As4(), mac)...)
	rib = append(rib, buildTestNetlinkNeighMsg(nudProbe, ipProbe.As4(), mac)...)
	rib = append(rib, buildTestNetlinkNeighMsg(nudStale, ipStale.As4(), mac)...)

	entries := make(map[netip.Addr]TableEntry)
	applyNetlinkNUDMessages(entries, rib)

	if !entries[ipReachable].ConfirmedActive || !entries[ipReachable].IsValid {
		t.Errorf("expected NUD_REACHABLE entry to be IsValid=true and ConfirmedActive=true, got %+v", entries[ipReachable])
	}
	if entries[ipDelay].ConfirmedActive || !entries[ipDelay].IsValid {
		t.Errorf("expected NUD_DELAY entry to have ConfirmedActive=false, got %+v", entries[ipDelay])
	}
	if entries[ipProbe].ConfirmedActive || !entries[ipProbe].IsValid {
		t.Errorf("expected NUD_PROBE entry to have ConfirmedActive=false, got %+v", entries[ipProbe])
	}
	if entries[ipStale].ConfirmedActive || !entries[ipStale].IsValid {
		t.Errorf("expected NUD_STALE entry to have ConfirmedActive=false, got %+v", entries[ipStale])
	}
}
