package handlers

import (
	"testing"
	"time"

	"github.com/ryukzak/slap/src/storage"
)

func TestLastInteraction(t *testing.T) {
	at := func(day int) time.Time { return time.Date(2026, 10, day, 12, 0, 0, 0, time.UTC) }

	tests := []struct {
		name    string
		records map[storage.TaskID][]storage.TaskRecord
		want    time.Time
	}{
		{"no records", nil, time.Time{}},
		{
			"only registration and revoke don't count",
			map[storage.TaskID][]storage.TaskRecord{
				"task1": {
					{Type: storage.RegisterRecord, CreatedAt: at(1)},
					{Type: storage.RevokeRecord, CreatedAt: at(2)},
				},
			},
			time.Time{},
		},
		{
			"latest across tasks, later registration ignored",
			map[storage.TaskID][]storage.TaskRecord{
				"task1": {
					{Type: storage.SubmitRecord, CreatedAt: at(1)},
					{Type: storage.ReviewedRecord, CreatedAt: at(5)},
				},
				"task2": {
					{Type: storage.SubmitRecord, CreatedAt: at(3)},
					{Type: storage.RegisterRecord, CreatedAt: at(9)},
				},
			},
			at(5),
		},
		{
			"submission newer than check",
			map[storage.TaskID][]storage.TaskRecord{
				"task1": {
					{Type: storage.ReviewedRecord, CreatedAt: at(2)},
					{Type: storage.SubmitRecord, CreatedAt: at(7)},
				},
			},
			at(7),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lastInteraction(tt.records); !got.Equal(tt.want) {
				t.Errorf("lastInteraction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatCSVTime(t *testing.T) {
	prev := PrimaryLoc
	t.Cleanup(func() { PrimaryLoc = prev })
	PrimaryLoc = time.FixedZone("MSK", 3*60*60)

	if got := formatCSVTime(time.Time{}); got != "" {
		t.Errorf("formatCSVTime(zero) = %q, want empty", got)
	}
	got := formatCSVTime(time.Date(2026, 10, 27, 9, 30, 0, 0, time.UTC))
	if want := "2026-10-27 12:30"; got != want {
		t.Errorf("formatCSVTime() = %q, want %q", got, want)
	}
}
