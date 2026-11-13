package handlers

import (
	"testing"

	"github.com/ryukzak/slap/src/storage"
)

// TestLatestScoreIgnoresTeacherDrop checks that a teacher's drop of a lesson
// registration, which copies the student's text into a record the teacher
// authors, never counts as a score.
func TestLatestScoreIgnoresTeacherDrop(t *testing.T) {
	// Newest-first, as DB.ListTaskRecords returns them.
	records := []storage.TaskRecord{
		{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "student", Content: "see comments"},
		{Type: storage.RevokeRecord, AuthorID: "teacher", StudentID: "student", Content: "9 my solution"},
		{Type: storage.SubmitRecord, AuthorID: "student", StudentID: "student", Content: "9 my solution"},
		{Type: storage.ReviewedRecord, AuthorID: "teacher", StudentID: "student", Content: "5 fix the tests"},
	}

	if got := latestScore(records); got != "5" {
		t.Errorf("latestScore() = %q, want %q", got, "5")
	}
}
