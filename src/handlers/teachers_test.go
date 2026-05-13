package handlers

import "testing"

func TestParseTeacherSortMode(t *testing.T) {
	tests := []struct {
		in   string
		want TeacherSortMode
	}{
		{"lessons", TeacherSortByLessons},
		{"reviews", TeacherSortByReviews},
		{"", TeacherSortByLessons},
		{"unknown", TeacherSortByLessons},
		{"LESSONS", TeacherSortByLessons},
	}
	for _, tt := range tests {
		if got := ParseTeacherSortMode(tt.in); got != tt.want {
			t.Errorf("ParseTeacherSortMode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
