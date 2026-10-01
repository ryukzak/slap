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

func TestFormatCheckRange(t *testing.T) {
	msk := time.FixedZone("MSK", 3*60*60)
	cet := time.FixedZone("CET", 2*60*60) // CEST (summer) offset, 1h behind MSK
	format := FormatCheckRange("MSK", msk, "CET", cet)

	tests := []struct {
		name  string
		first time.Time
		last  time.Time
		want  string
	}{
		{
			name:  "same day drops the date",
			first: time.Date(2026, 9, 24, 19, 55, 0, 0, msk),
			last:  time.Date(2026, 9, 24, 20, 12, 0, 0, msk),
			want:  "19:55(MSK)/18:55(CET) - 20:12(MSK)/19:12(CET)",
		},
		{
			name:  "different days keep a short date on each end",
			first: time.Date(2026, 9, 16, 19, 55, 0, 0, msk),
			last:  time.Date(2026, 9, 28, 20, 12, 0, 0, msk),
			want:  "16.9 19:55(MSK)/18:55(CET) - 28.9 20:12(MSK)/19:12(CET)",
		},
		{
			name:  "same instant for a single check",
			first: time.Date(2026, 9, 24, 19, 55, 0, 0, msk),
			last:  time.Date(2026, 9, 24, 19, 55, 0, 0, msk),
			want:  "19:55(MSK)/18:55(CET) - 19:55(MSK)/18:55(CET)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := format(tt.first, tt.last); got != tt.want {
				t.Errorf("FormatCheckRange(%v, %v) = %q, want %q", tt.first, tt.last, got, tt.want)
			}
		})
	}
}
