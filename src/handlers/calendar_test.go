package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/ryukzak/slap/src/storage"
)

func TestBuildLessonICS(t *testing.T) {
	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	lesson := &storage.Lesson{
		ID:          "abc",
		TeacherName: "teacher",
		DateTime:    time.Date(2026, 10, 21, 18, 0, 0, 0, moscow),
		Description: "Line one, with; chars\nLine two",
	}
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	ics := buildLessonICS(lesson, "slap", "example.com", "https://example.com/lesson/abc", 90*time.Minute, now)

	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n",
		"UID:abc@example.com\r\n",
		"DTSTAMP:20261001T000000Z\r\n",
		"DTSTART:20261021T150000Z\r\n",
		"DTEND:20261021T163000Z\r\n",
		"SUMMARY:slap lesson · @teacher\r\n",
		`DESCRIPTION:Line one\, with\; chars\nLine two\n\n`,
		"END:VCALENDAR\r\n",
	} {
		if !strings.Contains(ics, want) {
			t.Errorf("ICS missing %q:\n%s", want, ics)
		}
	}
	for _, line := range strings.Split(strings.TrimSuffix(ics, "\r\n"), "\r\n") {
		if len(line) > 75 {
			t.Errorf("line longer than 75 octets: %q", line)
		}
	}
}

func TestICSFold(t *testing.T) {
	line := "DESCRIPTION:" + strings.Repeat("я", 100)
	folded := icsFold(line)
	for _, part := range strings.Split(folded, "\r\n") {
		if len(part) > 75 {
			t.Errorf("part longer than 75 octets: %d", len(part))
		}
	}
	if unfolded := strings.ReplaceAll(folded, "\r\n ", ""); unfolded != line {
		t.Errorf("unfolding does not restore the line")
	}
}
