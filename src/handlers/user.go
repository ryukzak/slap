package handlers

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/ryukzak/slap/src/config"
	"github.com/ryukzak/slap/src/storage"
	"github.com/ryukzak/slap/src/util"
)

// UserInfoHandler displays the user information and available tasks
func UserInfoHandler(w http.ResponseWriter, r *http.Request) {
	sessionUser := userSession(w, r)
	if sessionUser == nil {
		return
	}

	profileUserID := mux.Vars(r)["userID"]
	if profileUserID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Students can only view their own profile
	if profileUserID != sessionUser.ID && !sessionUser.IsTeacher {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var user User
	dbUser, err := DB.GetUser(profileUserID)
	if err != nil || dbUser == nil {
		log.Printf("User %s not found in database: %v", profileUserID, err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	taskStatuses := make(map[storage.TaskID]storage.TaskRecordStatus)
	taskScores := make(map[storage.TaskID]string)
	journals := make(map[storage.TaskID][]storage.TaskRecord)
	for _, task := range AppConfig.Tasks {
		status, err := DB.LatestTaskStatus(profileUserID, task.ID)
		if err != nil {
			log.Printf("Error fetching task status for user %s task %s: %v", profileUserID, task.ID, err)
			continue
		}
		if status != "" {
			taskStatuses[task.ID] = status
		}
		records, err := DB.ListTaskRecords(profileUserID, task.ID)
		if err != nil {
			log.Printf("Error fetching task records for user %s task %s: %v", profileUserID, task.ID, err)
			continue
		}
		journals[task.ID] = records
		for _, r := range records {
			if r.EntryAuthorID != r.StudentID {
				if score := util.ExtractScore(r.Content); score != "" {
					taskScores[task.ID] = score
					break
				}
			}
		}
	}

	showPast := r.URL.Query().Get("showPast") == "true"
	now := time.Now()

	user = User{
		Username:                 dbUser.Username,
		ID:                       dbUser.ID,
		SessionUserID:            sessionUser.ID,
		SessionIsTeacher:         sessionUser.IsTeacher,
		IsStudent:                dbUser.IsStudent,
		IsTeacher:                dbUser.IsTeacher,
		Tasks:                    AppConfig.Tasks,
		TaskStatuses:             taskStatuses,
		TaskScores:               taskScores,
		Journals:                 journals,
		Lessons:                  []*storage.Lesson{},
		ShowPastLessons:          showPast,
		Now:                      now,
		DefaultDateTime:          getTomorrowNoon(),
		TZName:                   PrimaryTZName,
		DefaultLessonDescription: AppConfig.DefaultLessonDescription,
	}

	// Load lessons for all users
	lessons, err := DB.ListLessons()
	if err != nil {
		log.Printf("Error loading lessons: %v", err)
	} else {
		if showPast {
			user.Lessons = lessons
		} else {
			for _, l := range lessons {
				if !l.DateTime.Before(now) {
					user.Lessons = append(user.Lessons, l)
				}
			}
		}
	}

	renderPage(w, "templates/user.html", user)
}

type TaskStats struct {
	Pending  int
	Queued   int
	Dropped  int
	Feedback int
	Checked  int
}

type UserTaskSummary struct {
	Count   int
	Score   string
	Status  storage.TaskRecordStatus
	Summary string // compact status counts e.g. "p:2 r:1 c:1"
}

type UserTableRow struct {
	storage.UserData
	TaskData map[storage.TaskID]UserTaskSummary
}

// UserListHandler shows all registered users with task summaries. Teacher-only.
func UserListHandler(w http.ResponseWriter, r *http.Request) {
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

	rows := make([]UserTableRow, 0, len(users))
	for _, u := range users {
		row := UserTableRow{
			UserData: *u,
			TaskData: make(map[storage.TaskID]UserTaskSummary),
		}
		if u.IsStudent {
			for _, task := range AppConfig.Tasks {
				records, err := DB.ListTaskRecords(u.ID, task.ID)
				if err != nil {
					log.Printf("Error fetching task records for user %s task %s: %v", u.ID, task.ID, err)
					continue
				}
				if len(records) == 0 {
					continue
				}
				// Prefer "reviewed" status so the table shows "Checked" when the
				// task has been reviewed, not just "Feedback".
				bestStatus := records[0].Status
				for _, rec := range records {
					if rec.Status == storage.ReviewedTaskRecord {
						bestStatus = storage.ReviewedTaskRecord
						break
					}
				}
				summary := UserTaskSummary{Count: len(records), Status: bestStatus}
				var pending, queued, dropped, feedback, checked int
				for _, rec := range records {
					if rec.EntryAuthorID != rec.StudentID {
						if score := util.ExtractScore(rec.Content); score != "" && summary.Score == "" {
							summary.Score = score
						}
					}
					switch rec.Status {
					case storage.SubmitTaskRecord:
						pending++
					case storage.RegisterTaskRecord:
						queued++
					case storage.RevokedTaskRecord:
						dropped++
					case storage.ReviewTaskRecord:
						feedback++
					case storage.ReviewedTaskRecord:
						checked++
					}
				}
				var parts []string
				if pending > 0 {
					parts = append(parts, fmt.Sprintf("p:%d", pending))
				}
				if queued > 0 {
					parts = append(parts, fmt.Sprintf("q:%d", queued))
				}
				if feedback > 0 {
					parts = append(parts, fmt.Sprintf("f:%d", feedback))
				}
				if checked > 0 {
					parts = append(parts, fmt.Sprintf("c:%d", checked))
				}
				if dropped > 0 {
					parts = append(parts, fmt.Sprintf("d:%d", dropped))
				}
				summary.Summary = strings.Join(parts, " ")
				row.TaskData[task.ID] = summary
			}
		}
		rows = append(rows, row)
	}

	// Compute per-task aggregate stats (students only).
	studentCount := 0
	taskStats := make(map[storage.TaskID]TaskStats)
	for _, row := range rows {
		if !row.IsStudent {
			continue
		}
		studentCount++
		for _, task := range AppConfig.Tasks {
			ts := taskStats[task.ID]
			td, ok := row.TaskData[task.ID]
			if !ok || td.Count == 0 {
				taskStats[task.ID] = ts
				continue
			}
			switch td.Status {
			case storage.SubmitTaskRecord:
				ts.Pending++
			case storage.RegisterTaskRecord:
				ts.Queued++
			case storage.RevokedTaskRecord:
				ts.Dropped++
			case storage.ReviewTaskRecord:
				ts.Feedback++
			case storage.ReviewedTaskRecord:
				ts.Checked++
			}
			taskStats[task.ID] = ts
		}
	}

	renderPage(w, "templates/users.html", struct {
		SessionUserID string
		Users         []UserTableRow
		Tasks         []config.Task
		StudentCount  int
		TaskStats     map[storage.TaskID]TaskStats
	}{
		SessionUserID: sessionUser.ID,
		Users:         rows,
		Tasks:         AppConfig.Tasks,
		StudentCount:  studentCount,
		TaskStats:     taskStats,
	})
}

// UserListCSVHandler returns a CSV of students with their scores per task. Teacher-only.
func UserListCSVHandler(w http.ResponseWriter, r *http.Request) {
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

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=\"students.csv\"")

	cw := csv.NewWriter(w)
	cw.Comma = '|'

	header := []string{"ID", "Name"}
	for _, task := range AppConfig.Tasks {
		header = append(header, "Score "+task.Title)
	}
	if err := cw.Write(header); err != nil {
		log.Printf("Error writing CSV header: %v", err)
		return
	}

	for _, u := range users {
		if !u.IsStudent {
			continue
		}
		row := []string{string(u.ID), u.Username}
		for _, task := range AppConfig.Tasks {
			records, err := DB.ListTaskRecords(u.ID, task.ID)
			if err != nil {
				log.Printf("Error fetching task records for user %s task %s: %v", u.ID, task.ID, err)
				row = append(row, "")
				continue
			}
			score := ""
			for _, rec := range records {
				if rec.EntryAuthorID != rec.StudentID {
					if s := util.ExtractScore(rec.Content); s != "" {
						score = s
						break
					}
				}
			}
			row = append(row, score)
		}
		if err := cw.Write(row); err != nil {
			log.Printf("Error writing CSV row: %v", err)
			return
		}
	}

	cw.Flush()
}

// getTomorrowNoon returns tomorrow's date at 12:00 PM in PrimaryLoc
func getTomorrowNoon() time.Time {
	tomorrow := time.Now().In(PrimaryLoc).AddDate(0, 0, 1)
	return time.Date(
		tomorrow.Year(),
		tomorrow.Month(),
		tomorrow.Day(),
		12, 0, 0, 0,
		PrimaryLoc,
	)
}
