package handlers

import (
	"testing"
	"time"
)

func TestWithinLessonCheckWindow(t *testing.T) {
	start := time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC)
	const before = 1 * time.Hour
	const after = 6 * time.Hour

	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"during the lesson", start.Add(30 * time.Minute), true},
		{"shortly before the start", start.Add(-30 * time.Minute), true},
		{"exactly at the leading edge", start.Add(-before), true},
		{"just outside the leading edge", start.Add(-before - time.Minute), false},
		{"late but inside the trailing edge", start.Add(5 * time.Hour), true},
		{"exactly at the trailing edge", start.Add(after), true},
		{"next morning", start.Add(after + 8*time.Hour), false},
		{"long before the lesson exists", start.Add(-30 * 24 * time.Hour), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := withinLessonCheckWindow(tt.now, start, before, after); got != tt.want {
				t.Errorf("withinLessonCheckWindow(%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}

// A lesson with no scheduled time can never own a check: there is no window to
// fall inside of.
func TestWithinLessonCheckWindowZeroStart(t *testing.T) {
	if withinLessonCheckWindow(time.Now(), time.Time{}, time.Hour, 6*time.Hour) {
		t.Error("a zero lesson start must not count as inside the window")
	}
}

// A zero-width window (both sides disabled in config) accepts only the exact
// start instant, so configuring 0/0 effectively requires the lesson page.
func TestWithinLessonCheckWindowDisabled(t *testing.T) {
	start := time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC)
	if !withinLessonCheckWindow(start, start, 0, 0) {
		t.Error("the start instant itself must be inside a zero-width window")
	}
	if withinLessonCheckWindow(start.Add(time.Minute), start, 0, 0) {
		t.Error("a minute past the start must be outside a zero-width window")
	}
}
