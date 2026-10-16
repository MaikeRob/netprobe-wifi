package main

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"

	"netprobe-wifi/internal/oui"
)

const MaxVendorNameLen = 64

// Convert parses an IEEE CSV stream (Registry,Assignment,Organization Name,...)
// filters MA-L records, sanitizes vendor names, sorts entries, and writes a gzip-compressed TSV.
func Convert(r io.Reader, w io.Writer, meta oui.Metadata) error {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // Allow variable fields in case of extra address commas
	reader.LazyQuotes = true

	dedup := make(map[[3]byte]string)

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading CSV: %w", err)
		}

		if len(record) < 3 {
			continue
		}

		registry := strings.TrimSpace(record[0])
		if registry != "MA-L" {
			continue
		}

		assignment := strings.TrimSpace(record[1])
		if len(assignment) != 6 {
			continue
		}

		prefixBytes, err := hex.DecodeString(assignment)
		if err != nil || len(prefixBytes) != 3 {
			continue
		}

		vendor := sanitizeVendorName(record[2])
		if vendor == "" {
			continue
		}

		var prefix [3]byte
		copy(prefix[:], prefixBytes)
		dedup[prefix] = vendor
	}

	// Sort prefixes
	prefixes := make([][3]byte, 0, len(dedup))
	for p := range dedup {
		prefixes = append(prefixes, p)
	}
	sort.Slice(prefixes, func(i, j int) bool {
		return bytes.Compare(prefixes[i][:], prefixes[j][:]) < 0
	})

	gz, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("initializing gzip writer: %w", err)
	}
	defer gz.Close()

	// Write metadata header
	header := fmt.Sprintf("# source=%s date=%s sha256=%s count=%d\n",
		meta.SourceURL, meta.Date, meta.SHA256, len(prefixes))
	if _, err := gz.Write([]byte(header)); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	for _, p := range prefixes {
		hexStr := strings.ToUpper(hex.EncodeToString(p[:]))
		line := fmt.Sprintf("%s\t%s\n", hexStr, dedup[p])
		if _, err := gz.Write([]byte(line)); err != nil {
			return fmt.Errorf("writing entry: %w", err)
		}
	}

	return gz.Close()
}

// sanitizeVendorName cleans vendor names: removes control characters, collapses whitespace,
// and truncates excessively long strings for UI safety.
func sanitizeVendorName(raw string) string {
	// Filter out non-printable/control characters
	var sb strings.Builder
	for _, r := range raw {
		if unicode.IsControl(r) {
			sb.WriteRune(' ')
		} else {
			sb.WriteRune(r)
		}
	}

	// Collapse repeated spaces and trim
	clean := strings.Join(strings.Fields(sb.String()), " ")
	clean = strings.Trim(clean, "\"")
	clean = strings.TrimSpace(clean)

	// Truncate to MaxVendorNameLen runes
	runes := []rune(clean)
	if len(runes) > MaxVendorNameLen {
		return strings.TrimSpace(string(runes[:MaxVendorNameLen]))
	}
	return clean
}
