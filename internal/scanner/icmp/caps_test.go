package icmp_test

import (
	"testing"

	"netprobe-wifi/internal/scanner/icmp"
)

func TestCheckCapabilities(t *testing.T) {
	t.Parallel()

	caps := icmp.CheckCapabilities()
	if caps.Reason == "" {
		t.Errorf("expected non-empty diagnostic reason from CheckCapabilities")
	}

	t.Logf("ICMP Capabilities on host: CanSendICMP=%v Reason=%q", caps.CanSendICMP, caps.Reason)
}
