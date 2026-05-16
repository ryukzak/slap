package handlers

// SortMode controls the order of task records on the lesson page.
type SortMode string

const (
	SortBySubmitOrd   SortMode = "submit-ord"
	SortByRegisterOrd SortMode = "register-ord"
	SortByTaskMix     SortMode = "task-mix"
	SortByStudentMix  SortMode = "student-mix"
)

// ParseSortMode returns a valid SortMode from a string, defaulting to submit-ord.
func ParseSortMode(s string) SortMode {
	switch SortMode(s) {
	case SortByRegisterOrd, SortByTaskMix, SortByStudentMix:
		return SortMode(s)
	default:
		return SortBySubmitOrd
	}
}
