package util

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
)

var scoreRe = regexp.MustCompile(`^\d+`)

// ExtractScore returns the leading integer from s (e.g. "10 good" → "10"),
// or "" if s doesn't start with a number.
func ExtractScore(s string) string {
	return scoreRe.FindString(strings.TrimSpace(s))
}

// BoldScore wraps a leading integer in <strong> if present, returning safe HTML.
func BoldScore(s string) string {
	loc := scoreRe.FindStringIndex(strings.TrimSpace(s))
	if loc == nil {
		return html.EscapeString(s)
	}
	return "<strong>" + s[:loc[1]] + "</strong>" + html.EscapeString(s[loc[1]:])
}

func GetTitle(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	firstLine := lines[0]
	if len(firstLine) <= 80 {
		return firstLine
	}
	return firstLine[:77] + "..."
}

func GetRestText(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	firstLine := lines[0]
	if len(firstLine) <= 80 {
		if len(lines) <= 1 {
			return ""
		}
		rest := strings.Join(lines[1:], "\n")
		return strings.TrimLeft(rest, "\n")
	}
	rest := "..." + firstLine[77:]
	if len(lines) > 1 {
		rest += "\n" + strings.Join(lines[1:], "\n")
	}
	return rest
}

func FormatDateTime(fstName string, fstLoc *time.Location, sndName string, sndLoc *time.Location) func(time.Time) string {
	return func(t time.Time) string {
		return fmt.Sprintf("%s(%s) / %s(%s)",
			t.In(fstLoc).Format("Mon 2.1.2006 15:04"),
			fstName,
			t.In(sndLoc).Format("15:04"), sndName,
		)
	}
}
