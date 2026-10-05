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

	task := AppConfig.GetTask(taskID)
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
		QueuePosition    int
		QueueTotal       int
		WaitingMessage   string
		Tags             []storage.Tag
		// CheckCountsForLesson tells a teacher looking at a queued task whether
		// reviewing it here would count as the lesson's check or drop it from the
		// lesson queue first. Uses the same predicate as the review handler.
		CheckCountsForLesson bool
	}

	model := TaskViewModel{
		Task:          *task,
		UserID:        user.ID,
		StudentID:     userIDFromURL,
		SessionUserID: user.ID,
		TaskID:        taskID,
		IsTeacher:     user.IsTeacher,
	}

	if userData != nil {
		model.StudentName = userData.Username
	}

	rawRecords, err := DB.ListTaskRecords(userIDFromURL, taskID)
	if err != nil {
		log.Printf("Error retrieving task records: %v", err)
	}

	if len(rawRecords) > 0 {
		model.LatestRecord = &rawRecords[0]
		if rawRecords[0].Type == storage.RegisterRecord && rawRecords[0].LessonID != "" {
			lesson, err := DB.GetLesson(rawRecords[0].LessonID)
			if err != nil {
				log.Printf("Error fetching registered lesson %s: %v", rawRecords[0].LessonID, err)
			} else {
				model.RegisteredLesson = lesson
				model.QueuePosition, model.QueueTotal = queuePosition(lesson, userIDFromURL, taskID, SortBySubmitOrd)
				if user.IsTeacher {
					model.CheckCountsForLesson = belongsToLesson(rawRecords[0].LessonID, "", user.ID)
				}
			}
		}
	}
	model.TaskRecords = rawRecords

	if tags, err := DB.TaskTags(userIDFromURL, taskID); err != nil {
		log.Printf("Error computing tags for user %s task %s: %v", userIDFromURL, taskID, err)
	} else {
		model.Tags = tags
	}

	if remaining := waitingPeriodRemaining(task, rawRecords); remaining > 0 {
		model.WaitingMessage = formatWaitingMessage(remaining)
	}

	var pending, queued, dropped, checked int
	for _, r := range rawRecords {
		if r.AuthorID != r.StudentID {
			if score := util.ExtractScore(r.Content); score != "" && model.Score == "" {
				model.Score = score
			}
		}
		switch r.Type {
		case storage.SubmitRecord:
			pending++
		case storage.RegisterRecord:
			queued++
		case storage.RevokeRecord:
			dropped++
		case storage.ReviewedRecord:
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
	if checked > 0 {
		parts = append(parts, fmt.Sprintf("c:%d", checked))
	}
	if dropped > 0 {
		parts = append(parts, fmt.Sprintf("d:%d", dropped))
	}
	model.JournalSummary = strings.Join(parts, " ")

	renderPage(w, "templates/task.html", model)
}

// waitingPeriodRemaining returns how long until the student may re-register the
// task, based on the most recent teacher review and the task's configured
// waiting period. Records are expected newest-first. Returns 0 when no waiting
// period is active.
func waitingPeriodRemaining(task *config.Task, records []storage.TaskRecord) time.Duration {
	if task == nil {
		return 0
	}
	wp := task.GetWaitingPeriod()
	if wp <= 0 {
		return 0
	}
	for _, rec := range records {
		if rec.Type == storage.ReviewedRecord {
			if elapsed := time.Since(rec.CreatedAt); elapsed < wp {
				return wp - elapsed
			}
			return 0
		}
	}
	return 0
}

// formatWaitingMessage renders the user-facing waiting period notice.
func formatWaitingMessage(remaining time.Duration) string {
	hours := int(remaining.Hours())
	minutes := int(remaining.Minutes()) % 60
	return fmt.Sprintf("Waiting period: %dh%dm remaining since last check", hours, minutes)
}

func AddTaskRecordHandler(w http.ResponseWriter, r *http.Request) {
	user := userSession(w, r)
	if user == nil {
		renderAuthRequired(w)
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
		TaskID:     taskID,
		StudentID:  userIDFromURL,
		Content:    content,
		CreatedAt:  time.Now(),
		AuthorID:   user.ID,
		AuthorName: user.Username,
	}
	if user.IsTeacher || userIDFromURL != user.ID {
		record.Type = storage.ReviewedRecord
	} else {
		record.Type = storage.SubmitRecord
	}

	// When a student submits new content while registered to a lesson, auto-revoke
	// the existing registration so the lesson queue stays consistent.
	if record.Type == storage.SubmitRecord {
		if latest, err := DB.LatestTaskRecord(userIDFromURL, storage.TaskID(taskID)); err == nil && latest != nil && latest.Type == storage.RegisterRecord && latest.LessonID != "" {
			if rerr := DB.UnregisterFromLesson(latest.LessonID, latest.TaskID, latest.StudentID); rerr != nil {
				log.Printf("action=auto_revoke_lesson author=%s student=%s task=%s error=%v", user.ID, userIDFromURL, taskID, rerr)
			}
		}
	}

	// A teacher's review of a task that is still queued for a lesson belongs to
	// that lesson only when it was left from the lesson page, or from the task
	// page while the teacher's own lesson is running. Any other check was taken
	// outside the lesson, so drop the registration first: that severs the link
	// AppendTaskRecord follows, leaving the lesson neither holding the student in
	// its queue nor claiming the check.
	origin := "direct"
	if record.Type == storage.ReviewedRecord {
		if latest, err := DB.LatestTaskRecord(userIDFromURL, storage.TaskID(taskID)); err == nil && latest != nil && latest.Type == storage.RegisterRecord && latest.LessonID != "" {
			if belongsToLesson(latest.LessonID, r.PostForm.Get("lesson_id"), user.ID) {
				origin = "lesson"
			} else {
				origin = "outside_lesson"
				if rerr := DB.DropFromLessonAsTeacher(latest.LessonID, latest.TaskID, latest.StudentID, user.ID, user.Username); rerr != nil {
					log.Printf("action=drop_for_outside_lesson_check teacher=%s student=%s task=%s lesson=%s error=%v", user.ID, userIDFromURL, taskID, latest.LessonID, rerr)
					origin = "lesson"
				} else {
					log.Printf("action=drop_for_outside_lesson_check teacher=%s student=%s task=%s lesson=%s", user.ID, userIDFromURL, taskID, latest.LessonID)
				}
			}
		}
	}

	// Stamp the record only now: reads order an event log by CreatedAt, and any
	// auto-revoke or outside-lesson drop above happened before this record, so a
	// timestamp taken earlier would sort the review ahead of the drop that
	// preceded it.
	record.CreatedAt = time.Now()

	if err := DB.AppendTaskRecord(record); err != nil {
		log.Printf("action=add_task_record author=%s student=%s task=%s type=%s error=%v", user.ID, userIDFromURL, taskID, record.Type, err)
		http.Error(w, "Failed to save journal record", http.StatusInternalServerError)
		return
	}

	log.Printf("action=add_task_record author=%s student=%s task=%s type=%s", user.ID, userIDFromURL, taskID, record.Type)
	analytics.Track(user.ID, "task_record_added", map[string]any{
		"task_id":    taskID,
		"student_id": userIDFromURL,
		"role":       record.Type,
		"origin":     origin,
	})
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Trigger", "lessonRecordsRefresh")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/user/"+userIDFromURL+"/task/"+taskID, http.StatusSeeOther)
}

// belongsToLesson reports whether a teacher's review counts as the work of the
// lesson the task is queued for. formLessonID is the lesson the review form
// declared, naming it the lesson page's own review form; a task-page review
// declares nothing and qualifies only while the teacher's own lesson is within
// the configured check window. On a lesson read error it returns true, so a
// transient failure leaves the registration alone rather than dropping it.
func belongsToLesson(registeredLessonID, formLessonID string, teacherID storage.UserID) bool {
	if formLessonID != "" && formLessonID == registeredLessonID {
		return true
	}
	lesson, err := DB.GetLesson(storage.LessonID(registeredLessonID))
	if err != nil {
		log.Printf("Error fetching lesson %s to classify a check: %v", registeredLessonID, err)
		return true
	}
	if lesson.TeacherID != teacherID {
		return false
	}
	before, after := AppConfig.GetLessonCheckWindow()
	return withinLessonCheckWindow(time.Now(), lesson.DateTime, before, after)
}

// withinLessonCheckWindow reports whether now falls in
// [lessonStart-before, lessonStart+after]. A zero-length side disables that
// side of the window.
func withinLessonCheckWindow(now, lessonStart time.Time, before, after time.Duration) bool {
	if lessonStart.IsZero() {
		return false
	}
	return !now.Before(lessonStart.Add(-before)) && !now.After(lessonStart.Add(after))
}

func queuePosition(lesson *storage.Lesson, studentID storage.UserID, taskID storage.TaskID, sortMode SortMode) (int, int) {
	visible, _, _, err := buildLessonRecords(lesson, false, sortMode)
	if err != nil {
		log.Printf("Error building lesson records for queue position: %v", err)
		return 0, 0
	}
	for i, rec := range visible {
		if rec.StudentID == studentID && rec.TaskID == taskID {
			return i + 1, len(visible)
		}
	}
	return 0, len(visible)
}
