package handlers

import (
	"testing"
	"time"

	"github.com/ryukzak/slap/src/storage"
)

func TestComputeLessonStatsCheckTiming(t *testing.T) {
	base := time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC)

	// reviewed builds a Checked row the way buildLessonRecords does: the
	// underlying TaskRecord.CreatedAt is the registration time (see
	// storage.ListLessonTaskRecords), while the actual check happens at
	// ReviewRecords[0].CreatedAt/AuthorName — the two are intentionally
	// different here to catch regressions that read the wrong field.
	reviewed := func(teacher string, checkOffset time.Duration) TaskRecordWithInfo {
		registeredAt := base.Add(checkOffset).Add(-time.Hour)
		return TaskRecordWithInfo{
			TaskRecord:    storage.TaskRecord{Type: storage.ReviewedRecord, CreatedAt: registeredAt},
			ReviewRecords: []storage.TaskRecord{{Type: storage.ReviewedRecord, AuthorName: teacher, CreatedAt: base.Add(checkOffset)}},
		}
	}

	t.Run("no checks", func(t *testing.T) {
		stats := computeLessonStats(nil, base)
		if len(stats.CheckTimingByTeacher) != 0 {
			t.Errorf("expected no per-teacher timing, got %+v", stats.CheckTimingByTeacher)
		}
	})

	t.Run("single check has no interval", func(t *testing.T) {
		stats := computeLessonStats([]TaskRecordWithInfo{reviewed("Mia", 0)}, base)
		if len(stats.CheckTimingByTeacher) != 1 {
			t.Fatalf("expected one teacher entry, got %d", len(stats.CheckTimingByTeacher))
		}
		ts := stats.CheckTimingByTeacher[0]
		if ts.TeacherName != "Mia" || ts.Checked != 1 {
			t.Errorf("got %+v, want TeacherName=Mia Checked=1", ts)
		}
		if !ts.FirstCheckAt.Equal(base) || !ts.LastCheckAt.Equal(base) {
			t.Errorf("expected first=last=%v, got first=%v last=%v", base, ts.FirstCheckAt, ts.LastCheckAt)
		}
		if ts.CheckIntervalCount != 0 {
			t.Errorf("expected no interval with a single check, got count=%d", ts.CheckIntervalCount)
		}
		if ts.Duration != 0 {
			t.Errorf("Duration = %v, want 0 (check coincides with lesson start)", ts.Duration)
		}
	})

	t.Run("first/last and trimmed average across unsorted checks for one teacher", func(t *testing.T) {
		// 20 gaps: a zero-gap outlier at the low end, 18 one-minute gaps,
		// then a 100-minute outlier at the high end. Trimming 5% off each end
		// of 20 gaps drops exactly one from each side, so both outliers
		// should be excluded and the average should land on exactly one
		// minute.
		records := []TaskRecordWithInfo{
			reviewed("Mia", 0), // r0
			reviewed("Mia", 0), // r1 (gap0 = 0, outlier)
		}
		for i := 1; i <= 18; i++ {
			records = append(records, reviewed("Mia", time.Duration(i)*time.Minute)) // r2..r19
		}
		lastOffset := 18*time.Minute + 100*time.Minute
		records = append(records, reviewed("Mia", lastOffset)) // r20 (last gap = 100m, outlier)

		stats := computeLessonStats(records, base)
		if len(stats.CheckTimingByTeacher) != 1 {
			t.Fatalf("expected one teacher entry, got %d", len(stats.CheckTimingByTeacher))
		}
		ts := stats.CheckTimingByTeacher[0]

		wantFirst := base
		wantLast := base.Add(lastOffset)
		if !ts.FirstCheckAt.Equal(wantFirst) {
			t.Errorf("FirstCheckAt = %v, want %v", ts.FirstCheckAt, wantFirst)
		}
		if !ts.LastCheckAt.Equal(wantLast) {
			t.Errorf("LastCheckAt = %v, want %v", ts.LastCheckAt, wantLast)
		}
		if ts.CheckIntervalCount != 20 {
			t.Fatalf("CheckIntervalCount = %d, want 20", ts.CheckIntervalCount)
		}
		if ts.AvgCheckInterval != time.Minute {
			t.Errorf("AvgCheckInterval = %v, want %v (outliers trimmed)", ts.AvgCheckInterval, time.Minute)
		}
		if ts.Duration != lastOffset {
			t.Errorf("Duration = %v, want %v (last check minus lesson start)", ts.Duration, lastOffset)
		}
	})

	t.Run("uses the review record's timestamp, not the registration record's", func(t *testing.T) {
		r := reviewed("Mia", 3*time.Hour)
		wantCheckAt := base.Add(3 * time.Hour)
		if r.CreatedAt.Equal(wantCheckAt) {
			t.Fatalf("test fixture is broken: TaskRecord.CreatedAt should differ from the review time")
		}

		stats := computeLessonStats([]TaskRecordWithInfo{r}, base)
		if len(stats.CheckTimingByTeacher) != 1 {
			t.Fatalf("expected one teacher entry, got %d", len(stats.CheckTimingByTeacher))
		}
		if got := stats.CheckTimingByTeacher[0].FirstCheckAt; !got.Equal(wantCheckAt) {
			t.Errorf("FirstCheckAt = %v, want the review time %v (not registration time %v)", got, wantCheckAt, r.CreatedAt)
		}
	})

	t.Run("falls back to CreatedAt/AuthorName when no review record is available", func(t *testing.T) {
		r := TaskRecordWithInfo{TaskRecord: storage.TaskRecord{Type: storage.ReviewedRecord, AuthorName: "Mia", CreatedAt: base}}
		stats := computeLessonStats([]TaskRecordWithInfo{r}, base)
		if len(stats.CheckTimingByTeacher) != 1 {
			t.Fatalf("expected one teacher entry, got %d", len(stats.CheckTimingByTeacher))
		}
		ts := stats.CheckTimingByTeacher[0]
		if ts.TeacherName != "Mia" || !ts.FirstCheckAt.Equal(base) {
			t.Errorf("got %+v, want TeacherName=Mia FirstCheckAt=%v", ts, base)
		}
	})

	t.Run("keeps each teacher's timing and duration separate instead of mixing them", func(t *testing.T) {
		// Anna's two checks are an hour apart; Mia's two checks are a minute
		// apart. A combined-average bug would blend these into one meaningless
		// number — each teacher's own stats must stay independent.
		records := []TaskRecordWithInfo{
			reviewed("Anna", 0),
			reviewed("Anna", time.Hour),
			reviewed("Mia", 10*time.Minute),
			reviewed("Mia", 11*time.Minute),
		}

		stats := computeLessonStats(records, base)
		if len(stats.CheckTimingByTeacher) != 2 {
			t.Fatalf("expected 2 teacher entries, got %d: %+v", len(stats.CheckTimingByTeacher), stats.CheckTimingByTeacher)
		}

		// Sorted alphabetically: Anna before Mia.
		anna, mia := stats.CheckTimingByTeacher[0], stats.CheckTimingByTeacher[1]
		if anna.TeacherName != "Anna" || mia.TeacherName != "Mia" {
			t.Fatalf("expected Anna then Mia, got %q then %q", anna.TeacherName, mia.TeacherName)
		}

		if anna.Checked != 2 || anna.AvgCheckInterval != time.Hour {
			t.Errorf("Anna = %+v, want Checked=2 AvgCheckInterval=1h", anna)
		}
		if mia.Checked != 2 || mia.AvgCheckInterval != time.Minute {
			t.Errorf("Mia = %+v, want Checked=2 AvgCheckInterval=1m", mia)
		}

		// Duration is each teacher's own LastCheckAt minus the lesson start —
		// Anna's last check is 1h after lessonStart, Mia's is 11m after.
		if anna.Duration != time.Hour {
			t.Errorf("Anna.Duration = %v, want 1h", anna.Duration)
		}
		if mia.Duration != 11*time.Minute {
			t.Errorf("Mia.Duration = %v, want 11m", mia.Duration)
		}
	})

	t.Run("Duration is relative to the lesson's scheduled start, not the first check", func(t *testing.T) {
		lessonStart := base.Add(-30 * time.Minute)
		stats := computeLessonStats([]TaskRecordWithInfo{reviewed("Mia", 0)}, lessonStart)
		ts := stats.CheckTimingByTeacher[0]
		if ts.Duration != 30*time.Minute {
			t.Errorf("Duration = %v, want 30m (check happened 30m after lesson start)", ts.Duration)
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
