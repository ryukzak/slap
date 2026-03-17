package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
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

	user = User{
		Username:         dbUser.Username,
		ID:               dbUser.ID,
		SessionUserID:    sessionUser.ID,
		SessionIsTeacher: sessionUser.IsTeacher,
		IsStudent:        dbUser.IsStudent,
		IsTeacher:        dbUser.IsTeacher,
		Tasks:            AppConfig.Tasks,
		TaskStatuses:     taskStatuses,
		TaskScores:       taskScores,
		Journals:         journals,
		Lessons:          []*storage.Lesson{},
		Now:              time.Now(),
		DefaultDateTime:  getTomorrowNoon(),
		TZName:           PrimaryTZName,
	}

	// Load lessons for all users
	lessons, err := DB.ListLessons()
	if err != nil {
		log.Printf("Error loading lessons: %v", err)
	} else {
		user.Lessons = lessons
	}

	renderPage(w, "templates/user.html", user)
}

// UserListHandler shows all registered users. Teacher-only.
func UserListHandler(w http.ResponseWriter, r *http.Request) {
	user := teacherSession(w, r)
	if user == nil {
		return
	}

	users, err := DB.ListUsers()
	if err != nil {
		log.Printf("Error listing users: %v", err)
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	renderPage(w, "templates/users.html", struct {
		SessionUserID string
		Users         []*storage.UserData
	}{
		SessionUserID: user.ID,
		Users:         users,
	})
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
