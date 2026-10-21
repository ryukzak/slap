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
		Description: "\n# Lab defence\nLine one, with; chars\nLine two",
	}
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	ics := buildLessonICS(lesson, "slap", "example.com", "https://example.com/lesson/abc", 90*time.Minute, moscow, now)

	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n",
		"UID:abc@example.com\r\n",
		"DTSTAMP:20261001T000000Z\r\n",
		"BEGIN:VTIMEZONE\r\nTZID:Europe/Moscow\r\nBEGIN:STANDARD\r\n",
		"TZOFFSETTO:+0300\r\nTZNAME:MSK\r\n",
		"DTSTART;TZID=Europe/Moscow:20261021T180000\r\n",
		"DTEND;TZID=Europe/Moscow:20261021T193000\r\n",
		"SUMMARY:slap · @teacher · Lab defence\r\n",
		`DESCRIPTION:2026-10-21 18:00 MSK (Europe/Moscow)\n\n# Lab defence\nLine on`,
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

func TestICSTimezoneDST(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	// A lesson spanning the 2026-10-25 switch from CEST to CET.
	from := time.Date(2026, 10, 25, 1, 30, 0, 0, berlin)
	to := time.Date(2026, 10, 25, 4, 0, 0, 0, berlin)
	tz := strings.Join(icsTimezone(berlin, from, to), "\n")

	for _, want := range []string{
		"TZID:Europe/Berlin",
		"BEGIN:DAYLIGHT\nDTSTART:20260329T030000\nTZOFFSETFROM:+0100\nTZOFFSETTO:+0200\nTZNAME:CEST\nEND:DAYLIGHT",
		"BEGIN:STANDARD\nDTSTART:20261025T020000\nTZOFFSETFROM:+0200\nTZOFFSETTO:+0100\nTZNAME:CET\nEND:STANDARD",
	} {
		if !strings.Contains(tz, want) {
			t.Errorf("VTIMEZONE missing %q:\n%s", want, tz)
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
