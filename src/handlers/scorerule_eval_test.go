package handlers

import (
	"testing"
	"time"

	"github.com/ryukzak/slap/src/storage"
)

// rec is a small helper to build a TaskRecord with the fields latestCheckedInfo
// looks at.
func rec(typ storage.TaskRecordType, author, student, content string, at time.Time) storage.TaskRecord {
	return storage.TaskRecord{
		Type:      typ,
		AuthorID:  author,
		StudentID: student,
		Content:   content,
		CreatedAt: at,
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
			name: "reviewed by teacher",
			records: []storage.TaskRecord{
				rec(storage.ReviewedRecord, "teacher", "s", "8 ok", t2),
				rec(storage.RegisterRecord, "s", "s", "work", t1),
			},
			wantAt:    &t2,
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
