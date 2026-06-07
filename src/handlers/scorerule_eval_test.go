package handlers

import (
	"testing"
	"time"

	"github.com/ryukzak/slap/src/storage"
)

// rec is a small helper to build a TaskRecord with the fields latestCheckedInfo
// looks at.
func rec(status storage.TaskRecordStatus, author, student, content string, at time.Time) storage.TaskRecord {
	return storage.TaskRecord{
		Status:        status,
		EntryAuthorID: author,
		StudentID:     student,
		Content:       content,
		CreatedAt:     at,
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
				rec(storage.SubmitTaskRecord, "s", "s", "work", t0),
			},
			wantAt:    nil,
			wantState: "not checked (Pending)",
		},
		{
			name: "accepted via lesson uses submission time",
			records: []storage.TaskRecord{
				rec(storage.ReviewTaskRecord, "teacher", "s", "8 ok", t2),
				rec(storage.ReviewedTaskRecord, "s", "s", "work", t1),
			},
			wantAt:    &t1,
			wantState: "Checked",
		},
		{
			name: "scored without lesson falls back to latest submission",
			records: []storage.TaskRecord{
				rec(storage.ReviewTaskRecord, "teacher", "s", "8 good", t2),
				rec(storage.RevokedTaskRecord, "s", "s", "work", t1),
			},
			wantAt:    &t1,
			wantState: "scored without lesson (submission Dropped)",
		},
		{
			name: "feedback without a score does not count",
			records: []storage.TaskRecord{
				rec(storage.ReviewTaskRecord, "teacher", "s", "please revise", t2),
				rec(storage.RevokedTaskRecord, "s", "s", "work", t1),
			},
			wantAt:    nil,
			wantState: "not checked (Feedback)",
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
