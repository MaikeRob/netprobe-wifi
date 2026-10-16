package oui_test

import (
	"errors"
	"net"
	"testing"

	"netprobe-wifi/internal/oui"
)

func TestParseMAC_ValidFormats(t *testing.T) {
	t.Parallel()

	expected := net.HardwareAddr{0x00, 0x1A, 0x2B, 0x3C, 0x4D, 0x5E}

	tests := []struct {
		name  string
		input string
	}{
		{"standard colon uppercase", "00:1A:2B:3C:4D:5E"},
		{"standard colon lowercase", "00:1a:2b:3c:4d:5e"},
		{"windows hyphen", "00-1A-2B-3C-4D-5E"},
		{"cisco dot notation", "001a.2b3c.4d5e"},
		{"raw hex without separators", "001A2B3C4D5E"},
		{"with surrounding whitespace", "   00:1A:2B:3C:4D:5E \n"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := oui.ParseMAC(tc.input)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if got.String() != expected.String() {
				t.Errorf("got %s; want %s", got, expected)
			}
		})
	}
}

func TestParseMAC_InvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"too short", "00:1A:2B:3C:4D"},
		{"too long EUI-64", "00:1A:2B:FF:FE:3C:4D:5E"},
		{"invalid characters", "00:1A:2B:3C:4D:ZZ"},
		{"whitespace only", "   "},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := oui.ParseMAC(tc.input)
			if !errors.Is(err, oui.ErrInvalidMAC) {
				t.Errorf("input %q err = %v; want ErrInvalidMAC", tc.input, err)
			}
		})
	}
}

func TestFormatMAC(t *testing.T) {
	t.Parallel()

	hw := net.HardwareAddr{0x00, 0x0C, 0x29, 0xAB, 0xCD, 0xEF}
	got := oui.FormatMAC(hw)
	want := "00:0C:29:AB:CD:EF"

	if got != want {
		t.Errorf("FormatMAC = %s; want %s", got, want)
	}
}

func TestIsLocallyAdministered(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mac      string
		expected bool
	}{
		{"Cisco globally unique (00:00:0C)", "00:00:0C:11:22:33", false},
		{"Xerox globally unique (00:00:00)", "00:00:00:01:02:03", false},
		{"Apple globally unique (AC:BC:32)", "AC:BC:32:01:02:03", false},
		{"Locally administered 02:xx", "02:00:00:11:22:33", true},
		{"Locally administered 06:xx", "06:11:22:33:44:55", true},
		{"Locally administered 0A:xx", "0A:11:22:33:44:55", true},
		{"Locally administered 0E:xx", "0E:11:22:33:44:55", true},
		{"Locally administered DA:xx (1101 1010 -> bit 1 set)", "DA:A1:19:22:33:44", true},
		{"Locally administered F6:xx", "F6:12:34:56:78:9A", true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			hw, err := oui.ParseMAC(tc.mac)
			if err != nil {
				t.Fatalf("unexpected ParseMAC error: %v", err)
			}

			got := oui.IsLocallyAdministered(hw)
			if got != tc.expected {
				t.Errorf("IsLocallyAdministered(%s) = %v; want %v", tc.mac, got, tc.expected)
			}
		})
	}
}
