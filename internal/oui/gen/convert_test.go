package main

import (
	"bytes"
	"strings"
	"testing"

	"netprobe-wifi/internal/oui"
)

func TestConvert(t *testing.T) {
	t.Parallel()

	inputCSV := `Registry,Assignment,Organization Name,Organization Address
MA-L,00000C,Cisco Systems, Inc,170 West Tasman Drive San Jose CA 95134
MA-M,001122,Small Vendor Inc,Some Address
MA-L,00000C,"Cisco Systems, Inc.",Different Address (duplicate assignment)
MA-L,001A2B,  "Tabbed	and   Spaced   Vendor  "  ,Address
MA-L,002233,"Super Long Vendor Name That Definitely Exceeds The Maximum Allowed Truncation Limit Of Sixty Four Characters Per IEEE Specifications",Address
`

	meta := oui.Metadata{
		SourceURL: "https://test.ieee.org/oui.csv",
		Date:      "2026-10-16",
		SHA256:    "fake-sha256",
	}

	var buf bytes.Buffer
	err := Convert(strings.NewReader(inputCSV), &buf, meta)
	if err != nil {
		t.Fatalf("unexpected Convert error: %v", err)
	}

	// Decompress and verify using oui.Parse
	db, err := oui.Parse(&buf)
	if err != nil {
		t.Fatalf("unexpected Parse error: %v", err)
	}

	if db.Metadata().SourceURL != meta.SourceURL {
		t.Errorf("SourceURL = %s; want %s", db.Metadata().SourceURL, meta.SourceURL)
	}
	if db.Metadata().Date != meta.Date {
		t.Errorf("Date = %s; want %s", db.Metadata().Date, meta.Date)
	}

	// Expected: 3 MA-L entries (00000C, 001A2B, AABBCC). MA-M must be omitted.
	if db.Metadata().EntryCount != 3 {
		t.Fatalf("expected 3 entries, got %d", db.Metadata().EntryCount)
	}

	// Lookup Cisco
	macCisco, _ := oui.ParseMAC("00:00:0C:11:22:33")
	res := db.Lookup(macCisco)
	if res.Kind != oui.KindVendor || !strings.Contains(res.Vendor, "Cisco") {
		t.Errorf("Lookup Cisco = %+v; want Vendor with Cisco", res)
	}

	// Lookup Tabbed & Spaced
	macSpaced, _ := oui.ParseMAC("00:1A:2B:44:55:66")
	res = db.Lookup(macSpaced)
	if res.Kind != oui.KindVendor || res.Vendor != "Tabbed and Spaced Vendor" {
		t.Errorf("Lookup Spaced = %+v; want 'Tabbed and Spaced Vendor'", res)
	}

	// Lookup Truncated
	macLong, _ := oui.ParseMAC("00:22:33:77:88:99")
	res = db.Lookup(macLong)
	if res.Kind != oui.KindVendor || len([]rune(res.Vendor)) > MaxVendorNameLen {
		t.Errorf("Lookup Long = %+v; length %d exceeded max %d", res, len([]rune(res.Vendor)), MaxVendorNameLen)
	}
}
