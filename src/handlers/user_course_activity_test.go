package handlers

import (
	"testing"
	"time"

	"github.com/ryukzak/slap/src/config"
	"github.com/ryukzak/slap/src/storage"
)

func TestBuildCourseActivityEmptyWithoutRecords(t *testing.T) {
	cfg := &config.Config{Tasks: []config.Task{{ID: "task1", Title: "Lab 1"}}}

	activity := buildCourseActivity(cfg, map[storage.UserID]map[storage.TaskID][]storage.TaskRecord{})

	if len(activity.Rows) != 0 {
		t.Errorf("Rows = %v, want empty (no records, no configured course bounds)", activity.Rows)
	}
}

func TestBuildCourseActivityBucketsByWeek(t *testing.T) {
	cfg := &config.Config{Tasks: []config.Task{{ID: "task1", Title: "Lab 1"}}}
	base := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC) // a Monday

	userRecords := map[storage.UserID]map[storage.TaskID][]storage.TaskRecord{
		"alice": {
			"task1": {
				// Week 0: two submits.
				{Type: storage.SubmitRecord, AuthorID: "alice", StudentID: "alice", CreatedAt: base},
				{Type: storage.SubmitRecord, AuthorID: "alice", StudentID: "alice", CreatedAt: base.Add(2 * 24 * time.Hour)},
			},
		},
		"bob": {
			"task1": {
				// Week 0: one more submit (adds to week 0's total).
				{Type: storage.SubmitRecord, AuthorID: "bob", StudentID: "bob", CreatedAt: base.Add(24 * time.Hour)},
				// Week 2 (14-20 days in): three reviews -- a different type mix,
				// but tied with week 0 for busiest cell overall.
				{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "bob", CreatedAt: base.AddDate(0, 0, 14)},
				{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "bob", CreatedAt: base.AddDate(0, 0, 15)},
				{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "bob", CreatedAt: base.AddDate(0, 0, 16)},
			},
		},
	}

	activity := buildCourseActivity(cfg, userRecords)

	if len(activity.Rows) != 1 {
		t.Fatalf("Rows = %d, want 1", len(activity.Rows))
	}
	row := activity.Rows[0]
	if row.TaskID != "task1" || row.TaskTitle != "Lab 1" {
		t.Errorf("row = %+v, want task1/Lab 1", row)
	}
	if activity.MaxTotal != 3 {
		t.Errorf("MaxTotal = %d, want 3 (the legend's upper bound)", activity.MaxTotal)
	}

	// Course span is base .. base+16d, so 3 weekly buckets (0, 1, 2).
	if len(row.Cells) != 3 {
		t.Fatalf("Cells = %d, want 3", len(row.Cells))
	}

	// week0 and week2 both have 3 events, tied for the busiest cell -> both
	// get the densest glyph despite their different type mixes: the glyph
	// encodes volume only, not type.
	week0 := row.Cells[0]
	if week0.Total != 3 {
		t.Errorf("week0.Total = %d, want 3", week0.Total)
	}
	if week0.Glyph != "█" {
		t.Errorf("week0.Glyph = %q, want the densest glyph (tied for busiest cell)", week0.Glyph)
	}
	if week0.Tooltip != "week of 5 Jan: 3 submitted" {
		t.Errorf("week0.Tooltip = %q, want the type breakdown text", week0.Tooltip)
	}

	week1 := row.Cells[1]
	if week1.Total != 0 {
		t.Errorf("week1.Total = %d, want 0 (no activity that week)", week1.Total)
	}
	if week1.Glyph != "" {
		t.Errorf("week1.Glyph = %q, want empty for an empty cell", week1.Glyph)
	}

	week2 := row.Cells[2]
	if week2.Total != 3 {
		t.Errorf("week2.Total = %d, want 3", week2.Total)
	}
	if week2.Glyph != "█" {
		t.Errorf("week2.Glyph = %q, want the densest glyph (tied for busiest cell, same as week0 despite a different type mix)", week2.Glyph)
	}
	if week2.Tooltip != "week of 19 Jan: 3 checked" {
		t.Errorf("week2.Tooltip = %q, want the type breakdown text", week2.Tooltip)
	}
}

func TestCourseActivityGlyph(t *testing.T) {
	tests := []struct {
		ratio float64
		want  string
	}{
		{0.01, "░"}, {0.25, "░"},
		{0.26, "▒"}, {0.5, "▒"},
		{0.51, "▓"}, {0.75, "▓"},
		{0.76, "█"}, {1.0, "█"},
	}
	for _, tt := range tests {
		if got := courseActivityGlyph(tt.ratio); got != tt.want {
			t.Errorf("courseActivityGlyph(%v) = %q, want %q", tt.ratio, got, tt.want)
		}
	}
}

// TestBuildCourseActivityIntensityIsDistinct guards the actual bug this was
// built to fix: a low-volume week and a high-volume week must resolve to
// visibly different glyphs (not both collapse toward the same faint tone).
// Unicode shade blocks were chosen over a color gradient specifically
// because density reads the same regardless of theme or how well a viewer
// distinguishes subtly different shades of one color.
func TestBuildCourseActivityIntensityIsDistinct(t *testing.T) {
	cfg := &config.Config{Tasks: []config.Task{{ID: "task1", Title: "Lab 1"}}}
	base := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

	records := []storage.TaskRecord{
		{Type: storage.SubmitRecord, AuthorID: "alice", StudentID: "alice", CreatedAt: base}, // week 0: 1 event
	}
	// Week 3: 10 events -- the busiest cell.
	for i := 0; i < 10; i++ {
		records = append(records, storage.TaskRecord{
			Type: storage.SubmitRecord, AuthorID: "alice", StudentID: "alice", CreatedAt: base.AddDate(0, 0, 21+i%3),
		})
	}

	activity := buildCourseActivity(cfg, map[storage.UserID]map[storage.TaskID][]storage.TaskRecord{
		"alice": {"task1": records},
	})

	row := activity.Rows[0]
	low, high := row.Cells[0], row.Cells[3]
	if low.Total != 1 || high.Total != 10 {
		t.Fatalf("low/high totals = %d/%d, want 1/10", low.Total, high.Total)
	}
	if low.Glyph == high.Glyph {
		t.Errorf("low-volume (1 event) and high-volume (10 events) cells share the same glyph %q -- intensity isn't distinguishable", low.Glyph)
	}
	if low.Glyph != "░" {
		t.Errorf("low.Glyph = %q, want the lightest glyph (ratio 0.1)", low.Glyph)
	}
	if high.Glyph != "█" {
		t.Errorf("high.Glyph = %q, want the densest glyph (the busiest cell)", high.Glyph)
	}
}

func TestBuildCourseActivityUsesConfiguredCourseBounds(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC) // 3 weeks later
	cfg := &config.Config{
		Tasks:       []config.Task{{ID: "task1", Title: "Lab 1"}},
		CourseStart: &start,
		CourseEnd:   &end,
	}

	// A single record, far after the configured course end -- the scale
	// must still come from config, not from this record.
	userRecords := map[storage.UserID]map[storage.TaskID][]storage.TaskRecord{
		"alice": {
			"task1": {
				{Type: storage.SubmitRecord, AuthorID: "alice", StudentID: "alice", CreatedAt: end.AddDate(0, 0, 30)},
			},
		},
	}

	activity := buildCourseActivity(cfg, userRecords)

	if activity.RangeStart != start.Format("2 Jan") {
		t.Errorf("RangeStart = %q, want %q (from config, not the outlier record)", activity.RangeStart, start.Format("2 Jan"))
	}
	if activity.RangeEnd != end.Format("2 Jan") {
		t.Errorf("RangeEnd = %q, want %q (from config, not the outlier record)", activity.RangeEnd, end.Format("2 Jan"))
	}
	// The record falls after the configured window -- it must clamp into
	// the last bucket rather than being dropped or panicking.
	row := activity.Rows[0]
	lastCell := row.Cells[len(row.Cells)-1]
	if lastCell.Total != 1 {
		t.Errorf("last cell Total = %d, want 1 (out-of-range record clamped into it)", lastCell.Total)
	}
}
