package handlers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ryukzak/slap/src/storage"
	"github.com/ryukzak/slap/src/util"
)

func TestParseSortMode_Valid(t *testing.T) {
	tests := []struct {
		input string
		want  SortMode
	}{
		{"date", SortByDate},
		{"task-mix", SortByTaskMix},
		{"student-mix", SortByStudentMix},
	}
	for _, tt := range tests {
		got := ParseSortMode(tt.input)
		if got != tt.want {
			t.Errorf("ParseSortMode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseSortMode_Unknown(t *testing.T) {
	for _, input := range []string{"", "unknown", "TASK-MIX", "Date"} {
		got := ParseSortMode(input)
		if got != SortByDate {
			t.Errorf("ParseSortMode(%q) = %q, want %q", input, got, SortByDate)
		}
	}
}

// reg is a shorthand for a registration: student + task, ordered by creation time.
type reg struct{ student, task string }

func (r reg) String() string { return r.student + ":" + r.task }

func makeRecords(regs []reg) []TaskRecordWithInfo {
	out := make([]TaskRecordWithInfo, len(regs))
	for i, r := range regs {
		out[i] = TaskRecordWithInfo{
			TaskRecord: storage.TaskRecord{
				TaskID:    storage.TaskID(r.task),
				StudentID: storage.UserID(r.student),
				CreatedAt: time.Date(2025, 1, 1, 0, i, 0, 0, time.UTC),
			},
		}
	}
	return out
}

func applySortMode(records []TaskRecordWithInfo, mode SortMode) []TaskRecordWithInfo {
	switch mode {
	case SortByTaskMix:
		return util.InterleaveByKey(records, func(r TaskRecordWithInfo) string { return string(r.TaskID) })
	case SortByStudentMix:
		return util.InterleaveByKey(records, func(r TaskRecordWithInfo) string { return r.StudentID })
	default:
		return records
	}
}

func formatSequence(records []TaskRecordWithInfo) string {
	parts := make([]string, len(records))
	for i, r := range records {
		parts[i] = fmt.Sprintf("%s:%s", r.StudentID, r.TaskID)
	}
	return strings.Join(parts, " ")
}

func TestSortModes(t *testing.T) {
	tests := []struct {
		name     string
		input    []reg
		mode     SortMode
		expected string // "student:task student:task ..."
	}{
		{
			name:     "date: keeps original order",
			input:    []reg{{"Alice", "T1"}, {"Bob", "T1"}, {"Alice", "T2"}, {"Bob", "T2"}},
			mode:     SortByDate,
			expected: "Alice:T1 Bob:T1 Alice:T2 Bob:T2",
		},
		{
			name:     "task-mix: alternates between tasks",
			input:    []reg{{"Alice", "T1"}, {"Bob", "T1"}, {"Alice", "T2"}, {"Bob", "T2"}},
			mode:     SortByTaskMix,
			expected: "Alice:T1 Alice:T2 Bob:T1 Bob:T2",
		},
		{
			name:     "student-mix: alternates between students",
			input:    []reg{{"Alice", "T1"}, {"Bob", "T1"}, {"Alice", "T2"}, {"Bob", "T2"}},
			mode:     SortByStudentMix,
			expected: "Alice:T1 Bob:T1 Alice:T2 Bob:T2",
		},
		{
			name:     "task-mix: three tasks, unequal sizes",
			input:    []reg{{"Alice", "T1"}, {"Bob", "T1"}, {"Carol", "T1"}, {"Dave", "T2"}, {"Eve", "T3"}},
			mode:     SortByTaskMix,
			expected: "Alice:T1 Dave:T2 Eve:T3 Bob:T1 Carol:T1",
		},
		{
			name:     "student-mix: three students, unequal sizes",
			input:    []reg{{"Alice", "T1"}, {"Alice", "T2"}, {"Alice", "T3"}, {"Bob", "T4"}, {"Carol", "T5"}},
			mode:     SortByStudentMix,
			expected: "Alice:T1 Bob:T4 Carol:T5 Alice:T2 Alice:T3",
		},
		{
			name:     "task-mix: single task unchanged",
			input:    []reg{{"Alice", "T1"}, {"Bob", "T1"}, {"Carol", "T1"}},
			mode:     SortByTaskMix,
			expected: "Alice:T1 Bob:T1 Carol:T1",
		},
		{
			name:     "student-mix: single student unchanged",
			input:    []reg{{"Alice", "T1"}, {"Alice", "T2"}, {"Alice", "T3"}},
			mode:     SortByStudentMix,
			expected: "Alice:T1 Alice:T2 Alice:T3",
		},
		{
			name:     "empty input",
			input:    []reg{},
			mode:     SortByTaskMix,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records := makeRecords(tt.input)
			result := applySortMode(records, tt.mode)
			got := formatSequence(result)
			if got != tt.expected {
				t.Errorf("\n  input:    %v\n  mode:     %s\n  expected: %s\n  got:      %s",
					tt.input, tt.mode, tt.expected, got)
			}
		})
	}
}
