package handlers

import (
	"testing"
	"time"

	"github.com/ryukzak/slap/src/storage"
)

func TestComputeLessonStatsCheckTiming(t *testing.T) {
	base := time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC)

	reviewed := func(offset time.Duration) TaskRecordWithInfo {
		return TaskRecordWithInfo{
			TaskRecord: storage.TaskRecord{Type: storage.ReviewedRecord, CreatedAt: base.Add(offset)},
		}
	}

	t.Run("no checks", func(t *testing.T) {
		stats := computeLessonStats(nil)
		if !stats.FirstCheckAt.IsZero() || !stats.LastCheckAt.IsZero() {
			t.Errorf("expected zero check times, got first=%v last=%v", stats.FirstCheckAt, stats.LastCheckAt)
		}
		if stats.CheckIntervalCount != 0 {
			t.Errorf("expected no interval, got count=%d", stats.CheckIntervalCount)
		}
	})

	t.Run("single check has no interval", func(t *testing.T) {
		stats := computeLessonStats([]TaskRecordWithInfo{reviewed(0)})
		if !stats.FirstCheckAt.Equal(base) || !stats.LastCheckAt.Equal(base) {
			t.Errorf("expected first=last=%v, got first=%v last=%v", base, stats.FirstCheckAt, stats.LastCheckAt)
		}
		if stats.CheckIntervalCount != 0 {
			t.Errorf("expected no interval with a single check, got count=%d", stats.CheckIntervalCount)
		}
	})

	t.Run("first/last and trimmed average across unsorted checks", func(t *testing.T) {
		// 20 gaps: a zero-gap outlier at the low end, 18 one-minute gaps,
		// then a 100-minute outlier at the high end. Trimming 5% off each end
		// of 20 gaps drops exactly one from each side, so both outliers
		// should be excluded and the average should land on exactly one
		// minute.
		records := []TaskRecordWithInfo{
			reviewed(0), // r0
			reviewed(0), // r1 (gap0 = 0, outlier)
		}
		for i := 1; i <= 18; i++ {
			records = append(records, reviewed(time.Duration(i)*time.Minute)) // r2..r19
		}
		lastOffset := 18*time.Minute + 100*time.Minute
		records = append(records, reviewed(lastOffset)) // r20 (last gap = 100m, outlier)

		stats := computeLessonStats(records)

		wantFirst := base
		wantLast := base.Add(lastOffset)
		if !stats.FirstCheckAt.Equal(wantFirst) {
			t.Errorf("FirstCheckAt = %v, want %v", stats.FirstCheckAt, wantFirst)
		}
		if !stats.LastCheckAt.Equal(wantLast) {
			t.Errorf("LastCheckAt = %v, want %v", stats.LastCheckAt, wantLast)
		}
		if stats.CheckIntervalCount != 20 {
			t.Fatalf("CheckIntervalCount = %d, want 20", stats.CheckIntervalCount)
		}
		if stats.AvgCheckInterval != time.Minute {
			t.Errorf("AvgCheckInterval = %v, want %v (outliers trimmed)", stats.AvgCheckInterval, time.Minute)
		}
	})
}

func TestTrimmedMeanDuration(t *testing.T) {
	tests := []struct {
		name string
		in   []time.Duration
		want time.Duration
	}{
		{"empty", nil, 0},
		{"too few to trim keeps everything", []time.Duration{1, 2, 3}, 2},
		{"outliers trimmed with a large sample", append(append([]time.Duration{0}, repeatDuration(time.Minute, 18)...), 100*time.Minute), time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimmedMeanDuration(tt.in, checkIntervalTrim)
			if got != tt.want {
				t.Errorf("trimmedMeanDuration = %v, want %v", got, tt.want)
			}
		})
	}
}

func repeatDuration(d time.Duration, n int) []time.Duration {
	out := make([]time.Duration, n)
	for i := range out {
		out[i] = d
	}
	return out
}
