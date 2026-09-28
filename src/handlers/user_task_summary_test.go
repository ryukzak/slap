package handlers

import (
	"testing"
	"time"

	"github.com/ryukzak/slap/src/storage"
)

// TestBuildTaskSummaryFeedbackCount locks down that FeedbackCount counts
// "reviewed" records actually authored by someone other than the student —
// matching task.go's JournalSummary "c:N" figure for correctly-typed data —
// and is not thrown off by interleaved submit/register/revoke records.
func TestBuildTaskSummaryFeedbackCount(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	at := func(days int) time.Time { return base.Add(time.Duration(days) * 24 * time.Hour) }

	// Newest-first, as DB.ListTaskRecords returns them.
	records := []storage.TaskRecord{
		{Type: storage.SubmitRecord, AuthorID: "student", StudentID: "student", CreatedAt: at(8)},
		{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "student", CreatedAt: at(7)},
		{Type: storage.RevokeRecord, AuthorID: "teacher", StudentID: "student", CreatedAt: at(6)}, // admin action, not feedback
		{Type: storage.RegisterRecord, AuthorID: "student", StudentID: "student", CreatedAt: at(5)},
		{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "student", CreatedAt: at(4)},
		{Type: storage.SubmitRecord, AuthorID: "student", StudentID: "student", CreatedAt: at(3)},
		{Type: storage.RegisterRecord, AuthorID: "student", StudentID: "student", CreatedAt: at(2)},
		{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "student", CreatedAt: at(1)},
	}

	summary := buildTaskSummary(records)

	wantFeedback := 3
	if summary.FeedbackCount != wantFeedback {
		t.Errorf("FeedbackCount = %d, want %d", summary.FeedbackCount, wantFeedback)
	}

	// Cross-check against the same "checked" predicate task.go's
	// JournalSummary uses, applied to the identical record set: for this
	// data every "reviewed" record is genuinely teacher-authored, so the
	// two figures coincide (see TestBuildTaskSummaryFeedbackCountIgnoresMistypedSelfReview
	// for the case where they must NOT coincide).
	checked := 0
	for _, r := range records {
		if r.Type == storage.ReviewedRecord {
			checked++
		}
	}
	if summary.FeedbackCount != checked {
		t.Errorf("FeedbackCount = %d, want it to match JournalSummary's checked count %d", summary.FeedbackCount, checked)
	}

	if len(summary.Timeline) != len(records) {
		t.Errorf("Timeline has %d events, want %d (one per record)", len(summary.Timeline), len(records))
	}

	wantFirstSubmit := at(3) // oldest submit
	if summary.FirstSubmission == nil || !summary.FirstSubmission.Equal(wantFirstSubmit) {
		t.Errorf("FirstSubmission = %v, want %v", summary.FirstSubmission, wantFirstSubmit)
	}

	wantFirstRegister := at(2) // oldest register
	if summary.FirstRegistration == nil || !summary.FirstRegistration.Equal(wantFirstRegister) {
		t.Errorf("FirstRegistration = %v, want %v", summary.FirstRegistration, wantFirstRegister)
	}
}

// TestBuildTaskSummaryFeedbackCountIgnoresMistypedSelfReview reproduces a
// real production case: legacy/imported data can carry Type "reviewed" on a
// record the student authored themselves (AuthorID == StudentID), which
// isn't achievable through the normal submit flow (only a non-student
// author ever gets that type) but does happen in older data. Such records
// must not count as teacher feedback, even though their Type says
// "reviewed".
func TestBuildTaskSummaryFeedbackCountIgnoresMistypedSelfReview(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	at := func(days int) time.Time { return base.Add(time.Duration(days) * 24 * time.Hour) }

	// Newest-first. Mirrors the reported case: 6 records with Type
	// "reviewed", only 3 of which are actually teacher-authored.
	records := []storage.TaskRecord{
		{Type: storage.ReviewedRecord, AuthorID: "teacher", AuthorName: "teacher", StudentID: "student", CreatedAt: at(6)},
		{Type: storage.ReviewedRecord, AuthorID: "student", AuthorName: "student", StudentID: "student", CreatedAt: at(5)},
		{Type: storage.ReviewedRecord, AuthorID: "teacher", AuthorName: "teacher", StudentID: "student", CreatedAt: at(4)},
		{Type: storage.ReviewedRecord, AuthorID: "student", AuthorName: "student", StudentID: "student", CreatedAt: at(3)},
		{Type: storage.ReviewedRecord, AuthorID: "teacher", AuthorName: "teacher", StudentID: "student", CreatedAt: at(2)},
		{Type: storage.ReviewedRecord, AuthorID: "student", AuthorName: "student", StudentID: "student", CreatedAt: at(1)},
	}

	summary := buildTaskSummary(records)

	wantFeedback := 3
	if summary.FeedbackCount != wantFeedback {
		t.Errorf("FeedbackCount = %d, want %d (only teacher-authored records)", summary.FeedbackCount, wantFeedback)
	}
}

// TestBuildTaskSummaryIsolatedPerTask guards against cross-task leakage:
// buildTaskSummary must only ever see the records handed to it, so calling
// it twice with disjoint record sets must never let one call's count bleed
// into the other's.
func TestBuildTaskSummaryIsolatedPerTask(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	taskARecords := []storage.TaskRecord{
		{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "student", CreatedAt: base},
		{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "student", CreatedAt: base.Add(time.Hour)},
		{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "student", CreatedAt: base.Add(2 * time.Hour)},
	}
	taskBRecords := []storage.TaskRecord{
		{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "student", CreatedAt: base},
	}

	summaryA := buildTaskSummary(taskARecords)
	summaryB := buildTaskSummary(taskBRecords)

	if summaryA.FeedbackCount != 3 {
		t.Errorf("task A FeedbackCount = %d, want 3", summaryA.FeedbackCount)
	}
	if summaryB.FeedbackCount != 1 {
		t.Errorf("task B FeedbackCount = %d, want 1", summaryB.FeedbackCount)
	}
}
