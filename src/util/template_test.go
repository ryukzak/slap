package util

import (
	"testing"
	"time"
)

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{"zero", 0, "0h00m"},
		{"sub-hour", 7 * time.Minute, "0h07m"},
		{"hours only", 5*time.Hour + 12*time.Minute, "5h12m"},
		{"exactly one day", 24 * time.Hour, "1d0h00m"},
		{"days and hours", 2*24*time.Hour + 5*time.Hour + 12*time.Minute, "2d5h12m"},
		{"days, no extra hours", 3*24*time.Hour + 7*time.Minute, "3d0h07m"},
		{"negative clamps to zero", -time.Hour, "0h00m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatUptime(tt.in); got != tt.want {
				t.Errorf("FormatUptime(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
