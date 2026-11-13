package handlers

import (
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/ryukzak/slap/src/storage"
	"github.com/ryukzak/slap/src/util"
)

type TeacherRow struct {
	ID         storage.UserID
	Username   string
	Lessons    int
	Reviews    int
	Queued     int
	LastLesson *time.Time
	NextLesson *time.Time
}

type TeacherSortMode string

const (
	TeacherSortByLessons TeacherSortMode = "lessons"
	TeacherSortByReviews TeacherSortMode = "reviews"
)

func ParseTeacherSortMode(s string) TeacherSortMode {
	switch TeacherSortMode(s) {
	case TeacherSortByReviews:
		return TeacherSortByReviews
	default:
		return TeacherSortByLessons
	}
}

// ScoreLogEntry is one teacher-authored review that carried a numeric score,
// shown newest-first in the score log on the teachers page.
type ScoreLogEntry struct {
	TeacherID   string
	TeacherName string
	StudentID   string
	StudentName string
	TaskID      string
	TaskTitle   string
	Score       string
	CreatedAt   time.Time
}

// CheckedTaskEntry is one review a teacher left on a student's task, shown
// newest-first on that teacher's profile page. Score is "" for a review
// without a numeric score. RecordNumber is the review's "#N" position in the
// task page's journal, used to link straight to it.
type CheckedTaskEntry struct {
	StudentID    string
	StudentName  string
	TaskID       string
	TaskTitle    string
	RecordNumber int
	Score        string
	CreatedAt    time.Time
}

// listCheckedTasks returns every review teacherID left on another user's
// task, newest first, limited to taskID unless it is empty.
func listCheckedTasks(teacherID storage.UserID, taskID storage.TaskID) ([]CheckedTaskEntry, error) {
	users, err := DB.ListUsers()
	if err != nil {
		return nil, err
	}

	var checked []CheckedTaskEntry
	for _, u := range users {
		if !u.IsStudent || u.ID == teacherID {
			continue
		}
		for _, task := range AppConfig.Tasks {
			if taskID != "" && task.ID != taskID {
				continue
			}
			records, err := DB.ListTaskRecords(u.ID, task.ID)
			if err != nil {
				log.Printf("Error fetching task records for user %s task %s: %v", u.ID, task.ID, err)
				continue
			}
			for i, rec := range records {
				if rec.Type != storage.ReviewedRecord || rec.AuthorID != teacherID {
					continue
				}
				checked = append(checked, CheckedTaskEntry{
					StudentID:    u.ID,
					StudentName:  u.Username,
					TaskID:       string(task.ID),
					TaskTitle:    task.Title,
					RecordNumber: len(records) - i,
					Score:        util.ExtractScore(rec.Content),
					CreatedAt:    rec.CreatedAt,
				})
			}
		}
	}

	sort.Slice(checked, func(i, j int) bool {
		return checked[i].CreatedAt.After(checked[j].CreatedAt)
	})
	return checked, nil
}

// allowedLogLimits are the selectable score-log sizes, in display order.
var allowedLogLimits = []int{10, 100, 500}

// parseLogLimit validates s against allowedLogLimits, defaulting to the
// first (smallest) one.
func parseLogLimit(s string) int {
	if n, err := strconv.Atoi(s); err == nil {
		for _, allowed := range allowedLogLimits {
			if n == allowed {
				return n
			}
		}
	}
	return allowedLogLimits[0]
}

func TeacherListHandler(w http.ResponseWriter, r *http.Request) {
	sessionUser := teacherSession(w, r)
	if sessionUser == nil {
		return
	}

	users, err := DB.ListUsers()
	if err != nil {
		log.Printf("Error listing users: %v", err)
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	lessons, err := DB.ListLessons()
	if err != nil {
		log.Printf("Error listing lessons: %v", err)
		http.Error(w, "Failed to list lessons", http.StatusInternalServerError)
		return
	}

	teachers := make(map[storage.UserID]*TeacherRow)
	for _, u := range users {
		if !AppConfig.IsTeacher(u.ID) {
			continue
		}
		teachers[u.ID] = &TeacherRow{ID: u.ID, Username: u.Username}
	}

	now := time.Now()
	for _, l := range lessons {
		t, ok := teachers[l.TeacherID]
		if !ok {
			// Lessons authored by a non-teacher user (role revoked etc.) — surface anyway.
			t = &TeacherRow{ID: l.TeacherID, Username: l.TeacherName}
			teachers[l.TeacherID] = t
		}
		t.Lessons++
		for _, e := range l.EnrolledTasks {
			if e.Status == storage.RegisterRecord {
				t.Queued++
			}
		}
		when := l.DateTime
		if when.Before(now) {
			if t.LastLesson == nil || when.After(*t.LastLesson) {
				cp := when
				t.LastLesson = &cp
			}
		} else {
			if t.NextLesson == nil || when.Before(*t.NextLesson) {
				cp := when
				t.NextLesson = &cp
			}
		}
	}

	var scoreLog []ScoreLogEntry
	for _, u := range users {
		if !u.IsStudent {
			continue
		}
		for _, task := range AppConfig.Tasks {
			records, err := DB.ListTaskRecords(u.ID, task.ID)
			if err != nil {
				log.Printf("Error fetching task records for user %s task %s: %v", u.ID, task.ID, err)
				continue
			}
			for _, rec := range records {
				if rec.AuthorID == rec.StudentID {
					continue
				}
				if t, ok := teachers[rec.AuthorID]; ok {
					t.Reviews++
				}
				if score := util.ExtractScore(rec.Content); rec.Type == storage.ReviewedRecord && score != "" {
					scoreLog = append(scoreLog, ScoreLogEntry{
						TeacherID:   rec.AuthorID,
						TeacherName: rec.AuthorName,
						StudentID:   u.ID,
						StudentName: u.Username,
						TaskID:      string(task.ID),
						TaskTitle:   task.Title,
						Score:       score,
						CreatedAt:   rec.CreatedAt,
					})
				}
			}
		}
	}

	sort.Slice(scoreLog, func(i, j int) bool {
		return scoreLog[i].CreatedAt.After(scoreLog[j].CreatedAt)
	})
	logLimit := parseLogLimit(r.URL.Query().Get("logLimit"))
	if logLimit < len(scoreLog) {
		scoreLog = scoreLog[:logLimit]
	}

	rows := make([]*TeacherRow, 0, len(teachers))
	for _, t := range teachers {
		rows = append(rows, t)
	}

	sortMode := ParseTeacherSortMode(r.URL.Query().Get("sort"))
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch sortMode {
		case TeacherSortByReviews:
			if a.Reviews != b.Reviews {
				return a.Reviews > b.Reviews
			}
			if a.Lessons != b.Lessons {
				return a.Lessons > b.Lessons
			}
		default:
			if a.Lessons != b.Lessons {
				return a.Lessons > b.Lessons
			}
			if a.Reviews != b.Reviews {
				return a.Reviews > b.Reviews
			}
		}
		return a.Username < b.Username
	})

	renderPage(w, "templates/teachers.html", struct {
		SessionUserID string
		Teachers      []*TeacherRow
		SortMode      TeacherSortMode
		ScoreLog      []ScoreLogEntry
		LogLimit      int
		LogLimits     []int
	}{
		SessionUserID: sessionUser.ID,
		Teachers:      rows,
		SortMode:      sortMode,
		ScoreLog:      scoreLog,
		LogLimit:      logLimit,
		LogLimits:     allowedLogLimits,
	})
}
