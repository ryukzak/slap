package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/ryukzak/slap/src/analytics"
	"github.com/ryukzak/slap/src/config"
	"github.com/ryukzak/slap/src/storage"
	"github.com/ryukzak/slap/src/util"
)

func TaskDetailHandler(w http.ResponseWriter, r *http.Request) {
	user := userSession(w, r)
	if user == nil {
		return
	}

	vars := mux.Vars(r)
	userIDFromURL := vars["userID"]
	if userIDFromURL != user.ID && !user.IsTeacher {
		http.Error(w, "Unauthorized: You can only view tasks for your own profile", http.StatusForbidden)
		return
	}
	taskID := vars["taskID"]
	if taskID == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	task := AppConfig.GetTask(storage.TaskID(taskID))
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	userData, err := DB.GetUser(userIDFromURL)
	if err != nil {
		log.Printf("Error retrieving user data: %v", err)
	}

	type TaskViewModel struct {
		config.Task
		UserID           storage.UserID
		StudentID        storage.UserID
		SessionUserID    storage.UserID
		StudentName      string
		TaskRecords      []storage.TaskRecord
		LatestRecord     *storage.TaskRecord
		TaskID           storage.TaskID
		Score            string
		JournalSummary   string
		IsTeacher        bool
		RegisteredLesson *storage.Lesson
	}

	model := TaskViewModel{
		Task:          *task,
		UserID:        user.ID,
		StudentID:     userIDFromURL,
		SessionUserID: user.ID,
		TaskID:        storage.TaskID(taskID),
		IsTeacher:     user.IsTeacher,
	}

	if userData != nil {
		model.StudentName = userData.Username
	}

	rawRecords, err := DB.ListTaskRecords(userIDFromURL, storage.TaskID(taskID))
	if err != nil {
		log.Printf("Error retrieving task records: %v", err)
	}

	if len(rawRecords) > 0 {
		model.LatestRecord = &rawRecords[0]
		if rawRecords[0].Status == storage.RegisterTaskRecord && rawRecords[0].LessonID != "" {
			lesson, err := DB.GetLesson(rawRecords[0].LessonID)
			if err != nil {
				log.Printf("Error fetching registered lesson %s: %v", rawRecords[0].LessonID, err)
			} else {
				model.RegisteredLesson = lesson
			}
		}
	}
	model.TaskRecords = rawRecords

	var pending, queued, dropped, feedback, checked int
	for _, r := range rawRecords {
		if r.EntryAuthorID != r.StudentID {
			if score := util.ExtractScore(r.Content); score != "" && model.Score == "" {
				model.Score = score
			}
		}
		switch r.Status {
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
		parts = append(parts, fmt.Sprintf("r:%d", queued))
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
	model.JournalSummary = strings.Join(parts, " ")

	renderPage(w, "templates/task.html", model)
}

func AddTaskRecordHandler(w http.ResponseWriter, r *http.Request) {
	user := userSession(w, r)
	if user == nil {
		http.Error(w, "Authentication required, please login first", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	userIDFromURL, taskID := vars["userID"], vars["taskID"]
	if userIDFromURL != user.ID && !user.IsTeacher {
		http.Error(w, "Unauthorized: You can only add records to your own tasks", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}
	content := r.PostForm.Get("content")
	if content == "" {
		http.Error(w, "Record content is required", http.StatusBadRequest)
		return
	}
	const maxContentLength = 64 * 1024 // 64 KB
	if len(content) > maxContentLength {
		http.Error(w, "Record content exceeds maximum allowed length", http.StatusBadRequest)
		return
	}

	record := &storage.TaskRecord{
		TaskID:          storage.TaskID(taskID),
		StudentID:       userIDFromURL,
		Content:         content,
		CreatedAt:       time.Now(),
		EntryAuthorID:   user.ID,
		EntryAuthorName: user.Username,
	}
	if userIDFromURL == user.ID {
		record.Status = storage.SubmitTaskRecord
	} else {
		record.Status = storage.ReviewTaskRecord
	}

	if err := DB.AddTaskRecord(record); err != nil {
		log.Printf("action=add_task_record author=%s student=%s task=%s status=%s error=%v", user.ID, userIDFromURL, taskID, record.Status, err)
		http.Error(w, "Failed to save journal record", http.StatusInternalServerError)
		return
	}

	log.Printf("action=add_task_record author=%s student=%s task=%s status=%s", user.ID, userIDFromURL, taskID, record.Status)
	analytics.Track(user.ID, "task_record_added", map[string]any{
		"task_id":    taskID,
		"student_id": userIDFromURL,
		"role":       record.Status,
	})
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Trigger", "lessonRecordsRefresh")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/user/"+userIDFromURL+"/task/"+taskID, http.StatusSeeOther)
}
