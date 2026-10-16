//go:build windows

package arp

// NewDefaultTableReader instantiates the default TableReader for Windows.
func NewDefaultTableReader() TableReader {
	return NewWindowsTableReader()
}

// NewDefaultTrigger instantiates the default Trigger for Windows.
func NewDefaultTrigger() Trigger {
	return NewWindowsTrigger()
}
