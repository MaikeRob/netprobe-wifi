package oui_test

import (
	"net"
	"strings"
	"testing"

	"netprobe-wifi/internal/oui"
)

func TestDB_LoadAndLookup(t *testing.T) {
	t.Parallel()

	db, err := oui.Load()
	if err != nil {
		t.Fatalf("failed to load embedded OUI database: %v", err)
	}

	meta := db.Metadata()
	if meta.EntryCount < 40000 {
		t.Errorf("entry count = %d; expected > 40000", meta.EntryCount)
	}
	if meta.Date == "" {
		t.Errorf("expected non-empty snapshot date in metadata")
	}

	tests := []struct {
		name       string
		mac        string
		wantKind   oui.Kind
		wantVendor string
	}{
		{
			name:       "Cisco Systems",
			mac:        "00:00:0C:4A:2B:1C",
			wantKind:   oui.KindVendor,
			wantVendor: "Cisco Systems",
		},
		{
			name:       "Xerox Corporation",
			mac:        "00:00:00:11:22:33",
			wantKind:   oui.KindVendor,
			wantVendor: "XEROX",
		},
		{
			name:       "Nokia Shanghai Bell",
			mac:        "28:6F:B9:AA:BB:CC",
			wantKind:   oui.KindVendor,
			wantVendor: "Nokia Shanghai Bell",
		},
		{
			name:       "Locally Administered / Private MAC",
			mac:        "02:00:0C:11:22:33", // prefix 00:00:0C would match Cisco, but bit 1 is set
			wantKind:   oui.KindPrivate,
			wantVendor: "Private / Randomized MAC",
		},
		{
			name:       "Another Randomized MAC (second hex digit is 6)",
			mac:        "F6:A1:B2:C3:D4:E5",
			wantKind:   oui.KindPrivate,
			wantVendor: "Private / Randomized MAC",
		},
		{
			name:       "Unknown OUI",
			mac:        "00:55:00:01:02:03",
			wantKind:   oui.KindUnknown,
			wantVendor: "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			hw, err := oui.ParseMAC(tc.mac)
			if err != nil {
				t.Fatalf("unexpected ParseMAC error: %v", err)
			}

			res := db.Lookup(hw)
			if res.Kind != tc.wantKind {
				t.Errorf("Kind = %v (%s); want %v (%s)", res.Kind, res.Kind, tc.wantKind, tc.wantKind)
			}
			if tc.wantVendor != "" && !strings.Contains(res.Vendor, tc.wantVendor) {
				t.Errorf("Vendor = %q; want to contain %q", res.Vendor, tc.wantVendor)
			}
		})
	}
}

func TestDB_Lookup_InvalidSlice(t *testing.T) {
	t.Parallel()

	db, err := oui.Load()
	if err != nil {
		t.Fatalf("failed to load embedded OUI database: %v", err)
	}

	res := db.Lookup(net.HardwareAddr{0x00, 0x11})
	if res.Kind != oui.KindUnknown {
		t.Errorf("Lookup short MAC kind = %v; want KindUnknown", res.Kind)
	}

	res = db.Lookup(nil)
	if res.Kind != oui.KindUnknown {
		t.Errorf("Lookup nil MAC kind = %v; want KindUnknown", res.Kind)
	}
}
