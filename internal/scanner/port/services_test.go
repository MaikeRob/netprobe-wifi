package port

import (
	"testing"
)

func TestLookupService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		port     uint16
		expected string
	}{
		{port: 21, expected: "FTP"},
		{port: 22, expected: "SSH"},
		{port: 53, expected: "DNS"},
		{port: 80, expected: "HTTP"},
		{port: 443, expected: "HTTPS"},
		{port: 445, expected: "SMB"},
		{port: 3306, expected: "MySQL"},
		{port: 3389, expected: "RDP"},
		{port: 5432, expected: "PostgreSQL"},
		{port: 8080, expected: "HTTP-Proxy"},
		{port: 59123, expected: ""}, // unmapped high port
	}

	for _, tt := range tests {
		got := LookupService(tt.port)
		if got != tt.expected {
			t.Errorf("LookupService(%d) = %q; expected %q", tt.port, got, tt.expected)
		}
	}
}
