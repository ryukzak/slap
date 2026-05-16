package handlers

import (
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/ryukzak/slap/src/storage"
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
		if !u.IsTeacher {
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
			if e.Status == storage.RegisterTaskRecord {
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
				if rec.EntryAuthorID == rec.StudentID {
					continue
				}
				if t, ok := teachers[rec.EntryAuthorID]; ok {
					t.Reviews++
				}
			}
		}
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
	}{
		SessionUserID: sessionUser.ID,
		Teachers:      rows,
		SortMode:      sortMode,
	})
}
