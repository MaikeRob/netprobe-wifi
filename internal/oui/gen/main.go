package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"netprobe-wifi/internal/oui"
)

func main() {
	inPath := flag.String("in", "", "Path to input IEEE oui.csv file")
	outPath := flag.String("out", "", "Path to output oui.tsv.gz file")
	sourceURL := flag.String("source", "https://standards-oui.ieee.org/oui/oui.csv", "Origin URL of the database")
	date := flag.String("date", "", "Snapshot date (YYYY-MM-DD); defaults to today")
	shaFlag := flag.String("sha256", "", "Input CSV SHA256 checksum (computed if empty)")
	flag.Parse()

	if *inPath == "" || *outPath == "" {
		log.Fatalf("Both -in and -out flags are required")
	}

	inFile, err := os.Open(*inPath)
	if err != nil {
		log.Fatalf("Opening input file %s: %v", *inPath, err)
	}
	defer inFile.Close()

	var shaStr = *shaFlag
	if shaStr == "" {
		h := sha256.New()
		if _, err := io.Copy(h, inFile); err != nil {
			log.Fatalf("Computing SHA256: %v", err)
		}
		shaStr = hex.EncodeToString(h.Sum(nil))
		if _, err := inFile.Seek(0, io.SeekStart); err != nil {
			log.Fatalf("Rewinding input file: %v", err)
		}
	}

	dateStr := *date
	if dateStr == "" {
		dateStr = time.Now().UTC().Format("2006-01-02")
	}

	meta := oui.Metadata{
		SourceURL: *sourceURL,
		Date:      dateStr,
		SHA256:    shaStr,
	}

	if err := os.MkdirAll(filepath.Dir(*outPath), 0755); err != nil {
		log.Fatalf("Creating output directory: %v", err)
	}

	outFile, err := os.Create(*outPath)
	if err != nil {
		log.Fatalf("Creating output file %s: %v", *outPath, err)
	}
	defer outFile.Close()

	if err := Convert(inFile, outFile, meta); err != nil {
		log.Fatalf("Converting OUI database: %v", err)
	}

	fmt.Printf("Successfully generated %s\n", *outPath)
}
