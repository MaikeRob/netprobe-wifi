package oui

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
)

//go:generate go run ./gen -in ../../scratch/oui.csv -out ./data/oui.tsv.gz

//go:embed data/oui.tsv.gz
var embeddedData []byte

// Kind classifies the type of MAC address owner.
type Kind int

const (
	// KindUnknown indicates a public EUI-48 not matched in the local IEEE database.
	KindUnknown Kind = iota
	// KindVendor indicates a known vendor matched from the IEEE MA-L registry.
	KindVendor
	// KindPrivate indicates a locally administered / randomized MAC (RFC 7042 / IEEE 802).
	KindPrivate
)

func (k Kind) String() string {
	switch k {
	case KindVendor:
		return "vendor"
	case KindPrivate:
		return "private"
	default:
		return "unknown"
	}
}

// Result holds the vendor lookup resolution for a MAC address.
type Result struct {
	Vendor string
	Kind   Kind
}

// Metadata contains provenance details of the embedded OUI database.
type Metadata struct {
	SourceURL  string
	Date       string
	SHA256     string
	EntryCount int
}

type entry struct {
	prefix [3]byte
	vendor string
}

// DB provides thread-safe, read-only OUI lookups via binary search.
type DB struct {
	entries  []entry
	metadata Metadata
}

// Load decompress and parses the embedded IEEE OUI database.
func Load() (*DB, error) {
	if len(embeddedData) == 0 {
		return nil, errors.New("embedded OUI database is empty")
	}
	return Parse(bytes.NewReader(embeddedData))
}

// Parse reads a gzip-compressed TSV stream and constructs a DB.
func Parse(r io.Reader) (*DB, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("decompressing OUI database: %w", err)
	}
	defer gz.Close()

	scanner := bufio.NewScanner(gz)
	var meta Metadata
	var entries []entry

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Metadata header
		if strings.HasPrefix(line, "#") {
			parseHeader(line, &meta)
			continue
		}

		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}

		prefixHex := strings.TrimSpace(parts[0])
		vendor := strings.TrimSpace(parts[1])

		if len(prefixHex) != 6 {
			continue
		}

		b, err := hex.DecodeString(prefixHex)
		if err != nil || len(b) != 3 {
			continue
		}

		entries = append(entries, entry{
			prefix: [3]byte{b[0], b[1], b[2]},
			vendor: vendor,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading OUI database: %w", err)
	}

	// Ensure sorted by prefix for binary search
	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare(entries[i].prefix[:], entries[j].prefix[:]) < 0
	})

	meta.EntryCount = len(entries)
	return &DB{
		entries:  entries,
		metadata: meta,
	}, nil
}

func parseHeader(line string, meta *Metadata) {
	// Example: # source=https://... date=2026-10-16 sha256=abc...
	tokens := strings.Fields(strings.TrimPrefix(line, "#"))
	for _, tok := range tokens {
		kv := strings.SplitN(tok, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "source":
			meta.SourceURL = kv[1]
		case "date":
			meta.Date = kv[1]
		case "sha256":
			meta.SHA256 = kv[1]
		case "count":
			c, _ := strconv.Atoi(kv[1])
			meta.EntryCount = c
		}
	}
}

// Metadata returns database provenance details.
func (db *DB) Metadata() Metadata {
	return db.metadata
}

// Lookup resolves the manufacturer or categorization of the given MAC address.
func (db *DB) Lookup(hw net.HardwareAddr) Result {
	if len(hw) < 3 {
		return Result{Kind: KindUnknown}
	}

	// 1. Check for Locally Administered (randomized / private) MAC
	if IsLocallyAdministered(hw) {
		return Result{
			Vendor: "Private / Randomized MAC",
			Kind:   KindPrivate,
		}
	}

	// 2. Binary search on the 3-byte OUI prefix
	target := [3]byte{hw[0], hw[1], hw[2]}
	idx := sort.Search(len(db.entries), func(i int) bool {
		return bytes.Compare(db.entries[i].prefix[:], target[:]) >= 0
	})

	if idx < len(db.entries) && db.entries[idx].prefix == target {
		return Result{
			Vendor: db.entries[idx].vendor,
			Kind:   KindVendor,
		}
	}

	return Result{
		Vendor: "",
		Kind:   KindUnknown,
	}
}
