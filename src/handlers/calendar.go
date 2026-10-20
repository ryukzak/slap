package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/ryukzak/slap/src/storage"
)

// icsTimeFormat renders a UTC timestamp for iCalendar. Times are always
// emitted in UTC ("Z" suffix) so calendar apps convert the lesson to the
// viewer's own timezone, without needing a VTIMEZONE block.
const icsTimeFormat = "20060102T150405Z"

// LessonCalendarHandler serves a lesson as a single-event iCalendar file, so
// it can be added to Apple Calendar, Google Calendar, Outlook, etc. It is
// public like the lesson preview page: it exposes only what the preview does.
func LessonCalendarHandler(w http.ResponseWriter, r *http.Request) {
	lessonID := mux.Vars(r)["lessonID"]

	lesson, err := DB.GetLesson(storage.LessonID(lessonID))
	if err != nil {
		log.Printf("Error fetching lesson: %v", err)
		http.Error(w, "Lesson not found", http.StatusNotFound)
		return
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	lessonURL := fmt.Sprintf("%s://%s/lesson/%s", scheme, r.Host, lesson.ID)

	ics := buildLessonICS(lesson, AppConfig.GetName(), r.Host, lessonURL, AppConfig.GetLessonDuration(), time.Now())

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="lesson-%s.ics"`, lesson.ID))
	_, _ = w.Write([]byte(ics))
}

// buildLessonICS renders the iCalendar (RFC 5545) document for a lesson.
func buildLessonICS(lesson *storage.Lesson, instanceName, host, lessonURL string, duration time.Duration, now time.Time) string {
	start := lesson.DateTime.UTC()
	end := start.Add(duration)
	description := strings.TrimSpace(lesson.Description) + "\n\n" + lessonURL

	lines := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//SLAP//Lesson Calendar//EN",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"BEGIN:VEVENT",
		"UID:" + icsEscape(lesson.ID+"@"+host),
		"DTSTAMP:" + now.UTC().Format(icsTimeFormat),
		"DTSTART:" + start.Format(icsTimeFormat),
		"DTEND:" + end.Format(icsTimeFormat),
		"SUMMARY:" + icsEscape(fmt.Sprintf("%s lesson · @%s", instanceName, lesson.TeacherName)),
		"DESCRIPTION:" + icsEscape(description),
		"URL:" + lessonURL,
		"END:VEVENT",
		"END:VCALENDAR",
	}

	var b strings.Builder
	for _, line := range lines {
		b.WriteString(icsFold(line))
		b.WriteString("\r\n")
	}
	return b.String()
}

// icsEscape escapes a TEXT value per RFC 5545 §3.3.11.
func icsEscape(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.NewReplacer(
		`\`, `\\`,
		";", `\;`,
		",", `\,`,
		"\n", `\n`,
		"\r", `\n`,
	).Replace(s)
}

// icsFold splits a content line into chunks of at most 75 octets, continuing
// each with CRLF + space (RFC 5545 §3.1), without breaking UTF-8 sequences.
func icsFold(line string) string {
	const limit = 75
	if len(line) <= limit {
		return line
	}
	var b strings.Builder
	width := 0
	for _, r := range line {
		n := len(string(r))
		if width+n > limit {
			b.WriteString("\r\n ")
			width = 1 // the leading space counts toward the next line
		}
		b.WriteRune(r)
		width += n
	}
	return b.String()
}
