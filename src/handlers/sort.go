package handlers

// SortMode controls the order of task records on the lesson page.
type SortMode string

const (
	SortBySubmitOrd  SortMode = "submit-ord"
	SortByTaskMix    SortMode = "task-mix"
	SortByStudentMix SortMode = "student-mix"
)

// ParseSortMode returns a valid SortMode from a string, defaulting to date.
func ParseSortMode(s string) SortMode {
	switch SortMode(s) {
	case SortByTaskMix, SortByStudentMix:
		return SortMode(s)
	default:
		return SortBySubmitOrd
	}
}
