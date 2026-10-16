package scanner

import (
	"time"
)

// SweepStage identifies the current operational stage of an active network sweep.
type SweepStage string

const (
	// StageStarting indicates the sweep is validating targets and capturing initial neighbor snapshots.
	StageStarting SweepStage = "starting"
	// StageICMP indicates ICMP echo requests are being dispatched to the CIDR target range.
	StageICMP SweepStage = "icmp"
	// StageARP indicates silent local hosts are undergoing kernel-provoked ARP neighbor resolution.
	StageARP SweepStage = "arp"
	// StageFinished indicates the sweep completed successfully.
	StageFinished SweepStage = "finished"
	// StageCanceled indicates the sweep was aborted before completion.
	StageCanceled SweepStage = "canceled"
	// StageError indicates a fatal error occurred during the sweep.
	StageError SweepStage = "error"
)

// DiscoveredHost represents a discovered active device on the local network.
type DiscoveredHost struct {
	IP           string        `json:"ip"`
	MAC          string        `json:"mac"`
	Vendor       string        `json:"vendor"`
	Kind         string        `json:"kind"` // "vendor", "private", "unknown"
	RTT          time.Duration `json:"rtt"`
	DiscoveredBy string        `json:"discoveredBy"` // "Ativo via ICMP" or "Descoberto via ARP"
	Responded    bool          `json:"responded"`
}

// HostBatchEvent encapsulates a batch of newly discovered hosts emitted every ~100 ms.
type HostBatchEvent struct {
	Hosts []DiscoveredHost `json:"hosts"`
}

// SweepProgressEvent conveys real-time progress, stage transitions, and consolidated counters.
type SweepProgressEvent struct {
	Stage          SweepStage `json:"stage"`
	TotalHosts     int        `json:"totalHosts"`
	ScannedHosts   int        `json:"scannedHosts"`
	ActiveHosts    int        `json:"activeHosts"`
	ICMPDiscovered int        `json:"icmpDiscovered"`
	ARPDiscovered  int        `json:"arpDiscovered"`
	Percent        float64    `json:"percent"`
}

// SweepSummaryEvent conveys final execution metrics upon sweep termination.
type SweepSummaryEvent struct {
	TotalHosts     int           `json:"totalHosts"`
	ActiveHosts    int           `json:"activeHosts"`
	ICMPDiscovered int           `json:"icmpDiscovered"`
	ARPDiscovered  int           `json:"arpDiscovered"`
	Duration       time.Duration `json:"duration"`
	Reason         string        `json:"reason,omitempty"`
}
