package ui

import "testing"

func TestFormatRate(t *testing.T) {
	tests := []struct {
		bytesPerSec float64
		want        string
	}{
		{0, "0 Kbps"},
		{100, "1 Kbps"},       // 800 bits → "%.0f" = 1
		{124999, "1000 Kbps"}, // 999992 bits < 1e6
		{125000, "1.0 Mbps"},  // 1e6 bits
		{1_250_000, "10.0 Mbps"},
	}
	for _, tt := range tests {
		if got := formatRate(tt.bytesPerSec); got != tt.want {
			t.Errorf("formatRate(%v) = %q, want %q", tt.bytesPerSec, got, tt.want)
		}
	}
}
