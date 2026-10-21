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

// icsUTCFormat renders a UTC timestamp for iCalendar ("Z" suffix).
const icsUTCFormat = "20060102T150405Z"

// icsLocalFormat renders a wall-clock time to be paired with a TZID.
const icsLocalFormat = "20060102T150405"

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

	ics := buildLessonICS(lesson, AppConfig.GetName(), r.Host, lessonURL, AppConfig.GetLessonDuration(), PrimaryLoc, time.Now())

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="lesson-%s.ics"`, lesson.ID))
	_, _ = w.Write([]byte(ics))
}

// buildLessonICS renders the iCalendar (RFC 5545) document for a lesson.
// Start and end are pinned to loc (the server's primary timezone) via TZID
// and a matching VTIMEZONE, so calendar apps show the lesson both in its own
// zone and converted to the viewer's local time.
func buildLessonICS(lesson *storage.Lesson, instanceName, host, lessonURL string, duration time.Duration, loc *time.Location, now time.Time) string {
	start := lesson.DateTime.In(loc)
	end := start.Add(duration)
	zoneName, _ := start.Zone()
	description := fmt.Sprintf("%s %s (%s)\n\n%s\n\n%s",
		start.Format("2006-01-02 15:04"), zoneName, loc.String(),
		strings.TrimSpace(lesson.Description), lessonURL)

	lines := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//SLAP//Lesson Calendar//EN",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
	}
	lines = append(lines, icsTimezone(loc, start, end)...)
	lines = append(lines,
		"BEGIN:VEVENT",
		"UID:"+icsEscape(lesson.ID+"@"+host),
		"DTSTAMP:"+now.UTC().Format(icsUTCFormat),
		"DTSTART;TZID="+loc.String()+":"+start.Format(icsLocalFormat),
		"DTEND;TZID="+loc.String()+":"+end.Format(icsLocalFormat),
		"SUMMARY:"+icsEscape(lessonSummary(lesson, instanceName)),
		"DESCRIPTION:"+icsEscape(description),
		"URL:"+lessonURL,
		"END:VEVENT",
		"END:VCALENDAR",
	)

	var b strings.Builder
	for _, line := range lines {
		b.WriteString(icsFold(line))
		b.WriteString("\r\n")
	}
	return b.String()
}

// lessonSummary builds the event title: instance name, teacher, and the
// first non-empty line of the description with any Markdown heading marker
// stripped (e.g. "slap · @teacher · Lab defence").
func lessonSummary(lesson *storage.Lesson, instanceName string) string {
	parts := []string{instanceName, "@" + lesson.TeacherName}
	for _, line := range strings.Split(lesson.Description, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if line != "" {
			parts = append(parts, line)
			break
		}
	}
	return strings.Join(parts, " · ")
}

// icsTimezone renders a VTIMEZONE block for loc covering [from, to]. It
// lists every offset period overlapping that range as an explicit
// STANDARD/DAYLIGHT observance (no RRULE), derived from Go's tz database.
func icsTimezone(loc *time.Location, from, to time.Time) []string {
	lines := []string{"BEGIN:VTIMEZONE", "TZID:" + loc.String()}
	for t := from.In(loc); ; {
		name, offset := t.Zone()
		begin, end := t.ZoneBounds()
		kind := "STANDARD"
		if t.IsDST() {
			kind = "DAYLIGHT"
		}
		prevOffset := offset
		dtstart := "19700101T000000"
		if !begin.IsZero() {
			_, prevOffset = begin.Add(-time.Second).Zone()
			dtstart = begin.Format(icsLocalFormat)
		}
		lines = append(lines,
			"BEGIN:"+kind,
			"DTSTART:"+dtstart,
			"TZOFFSETFROM:"+icsOffset(prevOffset),
			"TZOFFSETTO:"+icsOffset(offset),
			"TZNAME:"+name,
			"END:"+kind,
		)
		if end.IsZero() || end.After(to) {
			break
		}
		t = end.In(loc)
	}
	return append(lines, "END:VTIMEZONE")
}

// icsOffset renders a UTC offset in seconds as +HHMM / -HHMM.
func icsOffset(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	return fmt.Sprintf("%s%02d%02d", sign, seconds/3600, seconds%3600/60)
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
