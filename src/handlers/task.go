package handlers

import (
	"log"
	"net/http"
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

	type TaskRecordView struct {
		storage.TaskRecord
		Reviews []storage.TaskRecord
	}

	type TaskViewModel struct {
		config.Task
		UserID        storage.UserID
		StudentID     storage.UserID
		SessionUserID storage.UserID
		StudentName   string
		TaskRecords   []TaskRecordView
		TaskID        storage.TaskID
		Score         string
	}

	model := TaskViewModel{
		Task:          *task,
		UserID:        user.ID,
		StudentID:     userIDFromURL,
		SessionUserID: user.ID,
		TaskID:        storage.TaskID(taskID),
	}

	if userData != nil {
		model.StudentName = userData.Username
	}

	rawRecords, err := DB.ListTaskRecords(userIDFromURL, storage.TaskID(taskID))
	if err != nil {
		log.Printf("Error retrieving task records: %v", err)
	}

	// Pair all consecutive review records with the reviewed record they follow (newest first).
	for i := 0; i < len(rawRecords); {
		r := rawRecords[i]
		if r.Status == storage.ReviewTaskRecord {
			var reviews []storage.TaskRecord
			for i < len(rawRecords) && rawRecords[i].Status == storage.ReviewTaskRecord {
				reviews = append(reviews, rawRecords[i])
				i++
			}
			if i < len(rawRecords) && rawRecords[i].Status == storage.ReviewedTaskRecord {
				model.TaskRecords = append(model.TaskRecords, TaskRecordView{
					TaskRecord: rawRecords[i],
					Reviews:    reviews,
				})
				i++
			} else {
				for _, rev := range reviews {
					model.TaskRecords = append(model.TaskRecords, TaskRecordView{TaskRecord: rev})
				}
			}
		} else {
			model.TaskRecords = append(model.TaskRecords, TaskRecordView{TaskRecord: r})
			i++
		}
	}

	for _, r := range rawRecords {
		if r.EntryAuthorID != r.StudentID {
			if score := util.ExtractScore(r.Content); score != "" {
				model.Score = score
				break
			}
		}
	}

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
