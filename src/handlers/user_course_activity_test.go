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

	// Course span is base .. base+16d, so 3 weekly buckets (0, 1, 2).
	if len(row.Cells) != 3 {
		t.Fatalf("Cells = %d, want 3", len(row.Cells))
	}

	// week0 and week2 both have 3 events, tied for the busiest cell -> both
	// sit at the fully-saturated color despite their different type mixes:
	// color encodes volume only, not type.
	saturated := lerpRGB(courseActivityFillAnchor, 1)

	week0 := row.Cells[0]
	if week0.Total != 3 {
		t.Errorf("week0.Total = %d, want 3", week0.Total)
	}
	if week0.FillRGB != saturated {
		t.Errorf("week0.FillRGB = %q, want %q (tied for busiest cell)", week0.FillRGB, saturated)
	}
	if week0.Tooltip != "week of 5 Jan: 3 submitted" {
		t.Errorf("week0.Tooltip = %q, want the type breakdown text", week0.Tooltip)
	}

	week1 := row.Cells[1]
	if week1.Total != 0 {
		t.Errorf("week1.Total = %d, want 0 (no activity that week)", week1.Total)
	}
	if week1.FillRGB != "" {
		t.Errorf("week1.FillRGB = %q, want empty for an empty cell", week1.FillRGB)
	}

	week2 := row.Cells[2]
	if week2.Total != 3 {
		t.Errorf("week2.Total = %d, want 3", week2.Total)
	}
	if week2.FillRGB != saturated {
		t.Errorf("week2.FillRGB = %q, want %q (tied for busiest cell, same color as week0 despite a different type mix)", week2.FillRGB, saturated)
	}
	if week2.Tooltip != "week of 19 Jan: 3 checked" {
		t.Errorf("week2.Tooltip = %q, want the type breakdown text", week2.Tooltip)
	}
}

func TestLerpRGB(t *testing.T) {
	anchor := courseActivityAnchor{lowR: 191, lowG: 219, lowB: 254, highR: 37, highG: 99, highB: 235}

	if got := lerpRGB(anchor, 0); got != "191,219,254" {
		t.Errorf("lerpRGB(0) = %q, want the pale anchor %q", got, "191,219,254")
	}
	if got := lerpRGB(anchor, 1); got != "37,99,235" {
		t.Errorf("lerpRGB(1) = %q, want the saturated anchor %q", got, "37,99,235")
	}
	// Out-of-range ratios must clamp rather than extrapolate past either anchor.
	if got := lerpRGB(anchor, -5); got != "191,219,254" {
		t.Errorf("lerpRGB(-5) = %q, want clamped to the pale anchor", got)
	}
	if got := lerpRGB(anchor, 5); got != "37,99,235" {
		t.Errorf("lerpRGB(5) = %q, want clamped to the saturated anchor", got)
	}
}

// TestBuildCourseActivityIntensityIsDistinct guards the actual bug this was
// built to fix: a low-volume week and a high-volume week must resolve to
// visibly different colors (not both collapse toward the same faint tone),
// regardless of the viewer's light/dark theme -- which is why cells are
// fully opaque, distinct colors rather than one color faded via alpha
// toward an unknown page background.
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
	if low.FillRGB == high.FillRGB {
		t.Errorf("low-volume (1 event) and high-volume (10 events) cells share the same color %q -- intensity isn't distinguishable", low.FillRGB)
	}
	if low.FillRGB != lerpRGB(courseActivityFillAnchor, 0.1) {
		t.Errorf("low.FillRGB = %q, want the ratio-0.1 blend", low.FillRGB)
	}
	if high.FillRGB != lerpRGB(courseActivityFillAnchor, 1) {
		t.Errorf("high.FillRGB = %q, want the fully saturated anchor", high.FillRGB)
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
