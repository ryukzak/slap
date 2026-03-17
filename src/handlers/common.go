package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ryukzak/slap/src/auth"
	"github.com/ryukzak/slap/src/config"
	"github.com/ryukzak/slap/src/storage"
)

// User represents the user data structure
type User struct {
	Username         string
	ID               storage.UserID // profile user's ID
	SessionUserID    storage.UserID // logged-in user's ID
	SessionIsTeacher bool
	IsStudent        bool
	IsTeacher        bool
	RegisterMode     bool
	Tasks            []config.Task
	TaskStatuses     map[storage.TaskID]storage.TaskRecordStatus
	TaskScores       map[storage.TaskID]string
	Journals         map[storage.TaskID][]storage.TaskRecord
	Lessons          []*storage.Lesson
	Now              time.Time
	DefaultDateTime  time.Time
	TZName           string
}

var JwtAuth *auth.JWTConfig
var Templates *template.Template
var BaseTemplates *template.Template
var DB *storage.DB
var AppConfig *config.Config
var Version string
var StartTime time.Time
var PrimaryLoc *time.Location
var PrimaryTZName string

func renderPage(w http.ResponseWriter, pageFile string, data any) {
	t, err := BaseTemplates.Clone()
	if err != nil {
		http.Error(w, "Failed to clone template", http.StatusInternalServerError)
		log.Printf("Template clone error: %v", err)
		return
	}
	t, err = t.ParseFiles(pageFile)
	if err != nil {
		http.Error(w, "Failed to parse template", http.StatusInternalServerError)
		log.Printf("Template parse error (%s): %v", pageFile, err)
		return
	}
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("Template execution error (%s): %v", pageFile, err)
	}
}

func UserLineHandler(w http.ResponseWriter, r *http.Request) {
	userClaim := getUserClaim(w, r)
	if userClaim == nil {
		return
	}

	dbUser, err := DB.GetUser(userClaim.ID)
	if err != nil {
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}

	var roleList []string
	if AppConfig.IsTeacher(dbUser.ID) {
		roleList = append(roleList, "Teacher")
	}
	if dbUser.IsStudent {
		roleList = append(roleList, "Student")
	}
	roles := strings.Join(roleList, ":")

	t, err := BaseTemplates.Clone()
	if err != nil {
		http.Error(w, "Failed to clone template", http.StatusInternalServerError)
		log.Printf("Template clone error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(w, "user_line.html", struct {
		Username string
		ID       storage.UserID
		Roles    string
	}{
		Username: dbUser.Username,
		ID:       dbUser.ID,
		Roles:    roles,
	}); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("Template execution error: %v", err)
	}
}
