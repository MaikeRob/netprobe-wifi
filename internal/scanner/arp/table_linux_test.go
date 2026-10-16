//go:build linux

package arp

import (
	"net/netip"
	"os"
	"strings"
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
