package icmp

// Capabilities describes the current system ability to perform ICMP operations without elevation.
type Capabilities struct {
	CanSendICMP bool
	Reason      string
}
