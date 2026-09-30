package handlers

import (
	"testing"
	"time"

	"github.com/ryukzak/slap/src/config"
	"github.com/ryukzak/slap/src/storage"
)

func TestBuildCourseActivityEmptyWithoutRecords(t *testing.T) {
	cfg := &config.Config{Tasks: []config.Task{{ID: "task1", Title: "Lab 1"}}}

	activity := buildCourseActivity(cfg, map[storage.UserID]map[storage.TaskID][]storage.TaskRecord{}, 2)

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
				// Week 0: two submits, one review.
				{Type: storage.SubmitRecord, AuthorID: "alice", StudentID: "alice", CreatedAt: base},
				{Type: storage.SubmitRecord, AuthorID: "alice", StudentID: "alice", CreatedAt: base.Add(2 * 24 * time.Hour)},
				{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "alice", CreatedAt: base.Add(3 * 24 * time.Hour)},
			},
		},
		"bob": {
			"task1": {
				// Week 2 (14-20 days in): three reviews.
				{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "bob", CreatedAt: base.AddDate(0, 0, 14)},
				{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "bob", CreatedAt: base.AddDate(0, 0, 15)},
				{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "bob", CreatedAt: base.AddDate(0, 0, 16)},
			},
		},
	}

	activity := buildCourseActivity(cfg, userRecords, 2) // step 2: 1-2=░, 3-4=▒, 5-6=▓, 7+=█

	if len(activity.Rows) != 1 {
		t.Fatalf("Rows = %d, want 1", len(activity.Rows))
	}
	row := activity.Rows[0]
	if row.TaskID != "task1" || row.TaskTitle != "Lab 1" {
		t.Errorf("row = %+v, want task1/Lab 1", row)
	}
	if activity.Step != 2 {
		t.Errorf("Step = %d, want 2", activity.Step)
	}

	// Course span is base .. base+16d, so 3 weekly buckets (0, 1, 2).
	if len(row.Cells) != 3 {
		t.Fatalf("Cells = %d, want 3", len(row.Cells))
	}

	week0 := row.Cells[0]
	if week0.SubmitCount != 2 || week0.CheckCount != 1 {
		t.Errorf("week0 submit/check = %d/%d, want 2/1", week0.SubmitCount, week0.CheckCount)
	}
	if week0.SubmitGlyph != "░" { // 2 submits at step 2 -> level 0
		t.Errorf("week0.SubmitGlyph = %q, want %q (2 submits at step 2)", week0.SubmitGlyph, "░")
	}
	if week0.CheckGlyph != "░" { // 1 check at step 2 -> level 0
		t.Errorf("week0.CheckGlyph = %q, want %q (1 check at step 2)", week0.CheckGlyph, "░")
	}
	if week0.Tooltip != "week of 5 Jan: 2 submitted, 1 checked" {
		t.Errorf("week0.Tooltip = %q, want the type breakdown text", week0.Tooltip)
	}

	week1 := row.Cells[1]
	if week1.SubmitCount != 0 || week1.CheckCount != 0 {
		t.Errorf("week1 submit/check = %d/%d, want 0/0 (no activity that week)", week1.SubmitCount, week1.CheckCount)
	}
	if week1.SubmitGlyph != "" || week1.CheckGlyph != "" {
		t.Errorf("week1 glyphs = %q/%q, want empty for an empty cell", week1.SubmitGlyph, week1.CheckGlyph)
	}
	if week1.Tooltip != "" {
		t.Errorf("week1.Tooltip = %q, want empty for an empty cell", week1.Tooltip)
	}

	week2 := row.Cells[2]
	if week2.SubmitCount != 0 || week2.CheckCount != 3 {
		t.Errorf("week2 submit/check = %d/%d, want 0/3", week2.SubmitCount, week2.CheckCount)
	}
	if week2.CheckGlyph != "▒" { // 3 checks at step 2 -> level 1
		t.Errorf("week2.CheckGlyph = %q, want %q (3 checks at step 2)", week2.CheckGlyph, "▒")
	}
	if week2.Tooltip != "week of 19 Jan: 3 checked" {
		t.Errorf("week2.Tooltip = %q, want the type breakdown text", week2.Tooltip)
	}
}

// TestBuildCourseActivityUsesConfiguredCourseBounds checks that
// course_start/course_end from config override the record-derived scale,
// and that a record falling outside that window still clamps into the
// nearest bucket rather than being dropped or panicking.
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

	activity := buildCourseActivity(cfg, userRecords, 2)

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
	if lastCell.SubmitCount != 1 {
		t.Errorf("last cell SubmitCount = %d, want 1 (out-of-range record clamped into it)", lastCell.SubmitCount)
	}
}

func TestCourseActivityGlyphForCount(t *testing.T) {
	tests := []struct {
		count, step int
		want        string
	}{
		{0, 2, ""}, {-1, 2, ""},
		{1, 2, "░"}, {2, 2, "░"},
		{3, 2, "▒"}, {4, 2, "▒"},
		{5, 2, "▓"}, {6, 2, "▓"},
		{7, 2, "█"}, {100, 2, "█"}, // caps at the densest glyph
		{1, 1, "░"}, {2, 1, "▒"}, {4, 1, "█"}, {50, 1, "█"},
	}
	for _, tt := range tests {
		if got := courseActivityGlyphForCount(tt.count, tt.step); got != tt.want {
			t.Errorf("courseActivityGlyphForCount(%d, %d) = %q, want %q", tt.count, tt.step, got, tt.want)
		}
	}
}

func TestCourseActivityLevels(t *testing.T) {
	got := courseActivityLevels(2)
	want := []CourseActivityLevel{
		{Glyph: "░", Range: "1–2"},
		{Glyph: "▒", Range: "3–4"},
		{Glyph: "▓", Range: "5–6"},
		{Glyph: "█", Range: "≥7"},
	}
	if len(got) != len(want) {
		t.Fatalf("len(levels) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("levels[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
