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

const newlineSeparator = " ↵ "

// GetTitle returns a preview of s suitable for display in a table cell.
// Newlines are replaced with a visible separator, and the result is capped at
// maxLen runes (unicode-safe). No trailing "…" is added — use CSS truncation
// (e.g. Tailwind "truncate") to clip the text visually.
func GetTitle(s string, maxLen int) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\n", newlineSeparator)
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

// GetRestText returns the portion of s that follows the first maxLen runes
// (after newline replacement), i.e. the overflow not shown by GetTitle.
func GetRestText(s string, maxLen int) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\n", newlineSeparator)
	runes := []rune(s)
	if len(runes) <= maxLen {
		return ""
	}
	return string(runes[maxLen:])
}

func FormatDateTime(fstName string, fstLoc *time.Location, sndName string, sndLoc *time.Location) func(time.Time) string {
	return func(t time.Time) string {
		return fmt.Sprintf("%s\u00a0%s(%s)/%s(%s)",
			t.In(fstLoc).Format("Mon\u00a02.1.2006"),
			t.In(fstLoc).Format("15:04"),
			fstName,
			t.In(sndLoc).Format("15:04"),
			sndName,
		)
	}
}
