package handlers

import (
	"testing"
	"time"

	"github.com/ryukzak/slap/src/storage"
)

// rec is a small helper to build a TaskRecord with the fields latestCheckedInfo
// looks at. SubmitAt defaults to the record's own timestamp; use recAt to model
// a record (e.g. a teacher review) whose submission time differs from when it
// was created.
func rec(typ storage.TaskRecordType, author, student, content string, at time.Time) storage.TaskRecord {
	return recAt(typ, author, student, content, at, at)
}

// recAt builds a TaskRecord with independent CreatedAt (when the record was
// written) and SubmitAt (the submission time it belongs to).
func recAt(typ storage.TaskRecordType, author, student, content string, createdAt, submitAt time.Time) storage.TaskRecord {
	return storage.TaskRecord{
		Type:      typ,
		AuthorID:  author,
		StudentID: student,
		Content:   content,
		CreatedAt: createdAt,
		SubmitAt:  submitAt,
	}
}

func TestLatestCheckedInfo(t *testing.T) {
	t0 := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(24 * time.Hour)
	t2 := t1.Add(24 * time.Hour)

	tests := []struct {
		name      string
		records   []storage.TaskRecord // newest-first
		wantAt    *time.Time
		wantState string
	}{
		{
			name:      "no records",
			records:   nil,
			wantAt:    nil,
			wantState: "not submitted",
		},
		{
			name: "submitted but not checked",
			records: []storage.TaskRecord{
				rec(storage.SubmitRecord, "s", "s", "work", t0),
			},
			wantAt:    nil,
			wantState: "not checked (Pending)",
		},
		{
			// The teacher's review is written at t2 but the work was submitted at
			// t1; scoring must use the submission time (t1), not the review time.
			name: "reviewed by teacher uses submission time",
			records: []storage.TaskRecord{
				recAt(storage.ReviewedRecord, "teacher", "s", "8 ok", t2, t1),
				rec(storage.RegisterRecord, "s", "s", "work", t1),
			},
			wantAt:    &t1,
			wantState: "Checked",
		},
		{
			name: "registered but not yet reviewed",
			records: []storage.TaskRecord{
				rec(storage.RegisterRecord, "s", "s", "work", t1),
			},
			wantAt:    nil,
			wantState: "not checked (Queued)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			at, state := latestCheckedInfo(tt.records)
			if tt.wantAt == nil {
				if at != nil {
					t.Errorf("checked time = %v, want nil", at)
				}
			} else if at == nil || !at.Equal(*tt.wantAt) {
				t.Errorf("checked time = %v, want %v", at, *tt.wantAt)
			}
			if state != tt.wantState {
				t.Errorf("state = %q, want %q", state, tt.wantState)
			}
		})
	}
}
