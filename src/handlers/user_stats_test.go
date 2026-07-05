package handlers

import (
	"testing"
	"time"

	"github.com/ryukzak/slap/src/storage"
)

func TestEffectiveTaskStatus(t *testing.T) {
	// startOfToday is midnight; a lesson before it is "passed".
	startOfToday := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	pastLesson := &storage.Lesson{ID: "lesson:past", DateTime: startOfToday.Add(-24 * time.Hour)}
	todayLesson := &storage.Lesson{ID: "lesson:today", DateTime: startOfToday.Add(10 * time.Hour)}
	futureLesson := &storage.Lesson{ID: "lesson:future", DateTime: startOfToday.Add(48 * time.Hour)}
	lessons := map[storage.LessonID]*storage.Lesson{
		pastLesson.ID:   pastLesson,
		todayLesson.ID:  todayLesson,
		futureLesson.ID: futureLesson,
	}

	reg := func(lessonID string) storage.TaskRecord {
		return storage.TaskRecord{Type: storage.RegisterRecord, AuthorID: "s", StudentID: "s", LessonID: lessonID}
	}

	tests := []struct {
		name    string
		records []storage.TaskRecord // newest-first
		want    storage.TaskRecordType
	}{
		{
			name:    "reviewed wins over everything",
			records: []storage.TaskRecord{reg(todayLesson.ID), {Type: storage.ReviewedRecord, AuthorID: "t", StudentID: "s"}},
			want:    storage.ReviewedRecord,
		},
		{
			name:    "plain submission is pending",
			records: []storage.TaskRecord{{Type: storage.SubmitRecord, AuthorID: "s", StudentID: "s"}},
			want:    storage.SubmitRecord,
		},
		{
			name:    "dropped registration is pending",
			records: []storage.TaskRecord{{Type: storage.RevokeRecord, AuthorID: "s", StudentID: "s"}},
			want:    storage.SubmitRecord,
		},
		{
			name:    "registered for a future lesson is queued",
			records: []storage.TaskRecord{reg(futureLesson.ID)},
			want:    storage.RegisterRecord,
		},
		{
			name:    "registered for today's lesson is still queued",
			records: []storage.TaskRecord{reg(todayLesson.ID)},
			want:    storage.RegisterRecord,
		},
		{
			name:    "stale registration (lesson passed, never reviewed) falls back to pending",
			records: []storage.TaskRecord{reg(pastLesson.ID)},
			want:    storage.SubmitRecord,
		},
		{
			name:    "registration to an unknown lesson stays queued",
			records: []storage.TaskRecord{reg("lesson:missing")},
			want:    storage.RegisterRecord,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveTaskStatus(tt.records, lessons, startOfToday)
			if got != tt.want {
				t.Errorf("effectiveTaskStatus = %q, want %q", got, tt.want)
			}
		})
	}
}
