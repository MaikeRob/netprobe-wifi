package oui

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
)

var (
	// ErrInvalidMAC is returned when a MAC address cannot be parsed or is not EUI-48.
	ErrInvalidMAC = errors.New("invalid MAC address: must be a 6-byte EUI-48 address")
)

// ParseMAC parses an EUI-48 MAC address supporting colon, hyphen, or dot notation.
// Returns normalized net.HardwareAddr of exactly 6 bytes.
func ParseMAC(s string) (net.HardwareAddr, error) {
	clean := strings.TrimSpace(s)
	if clean == "" {
		return nil, ErrInvalidMAC
	}

	// Remove common separators
	stripped := strings.ReplaceAll(clean, ":", "")
	stripped = strings.ReplaceAll(stripped, "-", "")
	stripped = strings.ReplaceAll(stripped, ".", "")

	if len(stripped) != 12 {
		return nil, fmt.Errorf("%w: input %q has invalid length", ErrInvalidMAC, s)
	}

	bytes, err := hex.DecodeString(stripped)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMAC, err)
	}

	return net.HardwareAddr(bytes), nil
}

// FormatMAC returns the normalized upper-case, colon-separated string representation of hw.
// Example: "00:1A:2B:3C:4D:5E".
func FormatMAC(hw net.HardwareAddr) string {
	if len(hw) != 6 {
		return hw.String()
	}
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
		hw[0], hw[1], hw[2], hw[3], hw[4], hw[5])
}

// IsLocallyAdministered returns true if the MAC address has the IEEE 802 Locally Administered bit set.
// In IEEE 802, bit 1 (0x02) of the most significant octet indicates a locally administered / randomized address.
// Devices such as modern smartphones (iOS, Android) and Windows 10/11 randomize their MACs using this bit,
// causing the second hexadecimal character of the first octet to be 2, 6, A, or E (e.g., x2:xx:xx..., x6:xx:xx...).
func IsLocallyAdministered(hw net.HardwareAddr) bool {
	if len(hw) == 0 {
		return false
	}
	return (hw[0] & 0x02) != 0
}
