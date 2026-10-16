//go:build linux

package arp

// NewDefaultTableReader instantiates the default TableReader for Linux.
func NewDefaultTableReader() TableReader {
	return NewLinuxTableReader()
}

// NewDefaultTrigger instantiates the default Trigger for Linux.
func NewDefaultTrigger() Trigger {
	return NewLinuxTrigger()
}
