package handlers

import (
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/gorilla/mux"
	"github.com/ryukzak/slap/src/analytics"
	"github.com/ryukzak/slap/src/storage"
)

func CreateLessonHandler(w http.ResponseWriter, r *http.Request) {
	userClaim := teacherSession(w, r)
	if userClaim == nil {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}
	dateStr := r.FormValue("date")
	timeStr := r.FormValue("time")
	description := r.FormValue("description")
	if dateStr == "" || timeStr == "" || description == "" {
		http.Error(w, "Date, time and description are required", http.StatusBadRequest)
		return
	}

	datetime, err := time.ParseInLocation("2006-01-02 15:04", dateStr+" "+timeStr, PrimaryLoc)
	if err != nil {
		http.Error(w, "Invalid date/time format", http.StatusBadRequest)
		return
	}

	lesson := &storage.Lesson{
		TeacherID:   userClaim.ID,
		DateTime:    datetime,
		TeacherName: userClaim.Username,
		Description: description,
	}

	err = DB.AddLesson(lesson)
	if err != nil {
		log.Printf("action=add_lesson user=%s error=%v", userClaim.ID, err)
		http.Error(w, "Error creating lesson", http.StatusInternalServerError)
		return
	}

	log.Printf("action=add_lesson user=%s lesson=%s datetime=%s", userClaim.ID, lesson.ID, lesson.DateTime.Format("2006-01-02T15:04"))
	analytics.Track(userClaim.ID, "lesson_created", map[string]any{"lesson_id": lesson.ID})
	http.Redirect(w, r, "/lesson/"+string(lesson.ID), http.StatusSeeOther)
}

// TaskRecordWithInfo wraps a TaskRecord with additional task metadata.
type TaskRecordWithInfo struct {
	storage.TaskRecord
	TaskTitle       string
	TaskDescription string
	PreviousRecords []storage.TaskRecord
}

// lessonRecordsData is the data passed to the lesson_task_records partial.
type lessonRecordsData struct {
	TaskRecords      []TaskRecordWithInfo
	ShowRevoked      bool
	TotalRecords     int
	SessionIsTeacher bool
}

func buildLessonRecords(lesson *storage.Lesson, showRevoked bool) ([]TaskRecordWithInfo, int, error) {
	taskRecords, err := DB.ListLessonTaskRecords(lesson)
	if err != nil {
		return nil, 0, err
	}

	previousTaskRecords, err := DB.ListLessonPreviousTaskRecords(lesson)
	if err != nil {
		return nil, 0, err
	}

	// Reviewed previous records go inside the current enrollment's accordion.
	// Revoked previous records are merged into the main list.
	reviewedByKey := map[string][]storage.TaskRecord{}
	for _, pr := range previousTaskRecords {
		if pr.Status == storage.ReviewedTaskRecord {
			key := pr.StudentID + ":" + pr.TaskID
			reviewedByKey[key] = append(reviewedByKey[key], *pr)
		}
	}

	allRecords := []TaskRecordWithInfo{}
	for _, taskRecord := range taskRecords {
		task := AppConfig.GetTask(taskRecord.TaskID)
		taskTitle := ""
		taskDescription := ""
		if task != nil {
			taskTitle = task.Title
			taskDescription = task.Description
		}
		key := taskRecord.StudentID + ":" + taskRecord.TaskID
		allRecords = append(allRecords, TaskRecordWithInfo{
			TaskRecord:      *taskRecord,
			TaskTitle:       taskTitle,
			TaskDescription: taskDescription,
			PreviousRecords: reviewedByKey[key],
		})
	}
	for _, pr := range previousTaskRecords {
		if pr.Status != storage.RevokedTaskRecord {
			continue
		}
		task := AppConfig.GetTask(pr.TaskID)
		taskTitle := ""
		if task != nil {
			taskTitle = task.Title
		}
		allRecords = append(allRecords, TaskRecordWithInfo{
			TaskRecord: *pr,
			TaskTitle:  taskTitle,
		})
	}

	sort.Slice(allRecords, func(i, j int) bool {
		return allRecords[i].CreatedAt.Before(allRecords[j].CreatedAt)
	})

	totalRecords := len(allRecords)
	var visible []TaskRecordWithInfo
	for _, r := range allRecords {
		if showRevoked || r.Status != storage.RevokedTaskRecord {
			visible = append(visible, r)
		}
	}
	return visible, totalRecords, nil
}

func LessonDetailHandler(w http.ResponseWriter, r *http.Request) {
	user := userSession(w, r)
	if user == nil {
		return
	}

	vars := mux.Vars(r)
	lessonID := vars["lessonID"]
	showRevoked := r.URL.Query().Get("showRevoked") == "true"

	lesson, err := DB.GetLesson(storage.LessonID(lessonID))
	if err != nil {
		log.Printf("Error fetching lesson: %v", err)
		http.Error(w, "Lesson not found", http.StatusNotFound)
		return
	}

	visibleTaskRecords, totalRecords, err := buildLessonRecords(lesson, showRevoked)
	if err != nil {
		log.Printf("Error fetching task records for lesson %s: %v", lesson.ID, err)
		http.Error(w, "Error fetching task records", http.StatusInternalServerError)
		return
	}

	renderPage(w, "templates/lesson.html", struct {
		Lesson           *storage.Lesson
		TeacherID        string
		SessionUserID    string
		SessionIsTeacher bool
		TaskRecords      []TaskRecordWithInfo
		ShowRevoked      bool
		TotalRecords     int
		DefaultDateTime  time.Time
		TZName           string
	}{
		Lesson:           lesson,
		TeacherID:        lesson.TeacherID,
		SessionUserID:    user.ID,
		SessionIsTeacher: user.IsTeacher,
		TaskRecords:      visibleTaskRecords,
		ShowRevoked:      showRevoked,
		TotalRecords:     totalRecords,
		DefaultDateTime:  time.Now().In(PrimaryLoc),
		TZName:           PrimaryTZName,
	})
}

func LessonTaskRecordsPartialHandler(w http.ResponseWriter, r *http.Request) {
	user := userSession(w, r)
	if user == nil {
		return
	}

	lessonID := mux.Vars(r)["lessonID"]
	showRevoked := r.URL.Query().Get("showRevoked") == "true"

	lesson, err := DB.GetLesson(storage.LessonID(lessonID))
	if err != nil {
		log.Printf("Error fetching lesson: %v", err)
		http.Error(w, "Lesson not found", http.StatusNotFound)
		return
	}

	visibleTaskRecords, totalRecords, err := buildLessonRecords(lesson, showRevoked)
	if err != nil {
		log.Printf("Error fetching task records for lesson %s: %v", lesson.ID, err)
		http.Error(w, "Error fetching task records", http.StatusInternalServerError)
		return
	}

	data := lessonRecordsData{
		TaskRecords:      visibleTaskRecords,
		ShowRevoked:      showRevoked,
		TotalRecords:     totalRecords,
		SessionIsTeacher: user.IsTeacher,
	}

	t, err := BaseTemplates.Clone()
	if err != nil {
		http.Error(w, "Failed to clone template", http.StatusInternalServerError)
		log.Printf("Template clone error: %v", err)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	if err := t.ExecuteTemplate(w, "lesson_task_records.html", data); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("Template execution error: %v", err)
	}
}

func RenderLessonListHandler(w http.ResponseWriter, r *http.Request) {
	userClaim := userSession(w, r)
	if userClaim == nil {
		return
	}

	filter := r.URL.Query().Get("filter")
	filterLessons := filter != "false"

	lessons, err := DB.ListLessons()
	if err != nil {
		log.Printf("Error listing lessons: %v", err)
		http.Error(w, "Error fetching lessons", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	availableLessons := []*storage.Lesson{}
	for _, lesson := range lessons {
		if !filterLessons || !lesson.DateTime.Before(today) {
			availableLessons = append(availableLessons, lesson)
		}
	}

	w.Header().Set("Content-Type", "text/html")

	data := struct {
		Lessons      []*storage.Lesson
		RegisterMode bool
	}{
		Lessons:      availableLessons,
		RegisterMode: r.URL.Query().Get("register") == "1",
	}

	t, err := BaseTemplates.Clone()
	if err != nil {
		http.Error(w, "Failed to clone template", http.StatusInternalServerError)
		log.Printf("Template clone error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(w, "lesson_list.html", data); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("Template execution error: %v", err)
	}
}

func ExtendLessonDeadlineHandler(w http.ResponseWriter, r *http.Request) {
	user := teacherSession(w, r)
	if user == nil {
		return
	}

	lessonID := mux.Vars(r)["lessonID"]
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	dateStr := r.FormValue("date")
	timeStr := r.FormValue("time")
	if dateStr == "" || timeStr == "" {
		http.Error(w, "Date and time are required", http.StatusBadRequest)
		return
	}

	deadline, err := time.ParseInLocation("2006-01-02 15:04", dateStr+" "+timeStr, PrimaryLoc)
	if err != nil {
		http.Error(w, "Invalid date/time format", http.StatusBadRequest)
		return
	}

	lesson, err := DB.GetLesson(storage.LessonID(lessonID))
	if err != nil {
		http.Error(w, "Lesson not found", http.StatusNotFound)
		return
	}
	if lesson.TeacherID != user.ID {
		http.Error(w, "Only the lesson's teacher can extend the deadline", http.StatusForbidden)
		return
	}

	if err := DB.SetLessonDeadline(lessonID, deadline); err != nil {
		log.Printf("action=extend_deadline user=%s lesson=%s error=%v", user.ID, lessonID, err)
		http.Error(w, "Failed to extend deadline", http.StatusInternalServerError)
		return
	}

	log.Printf("action=extend_deadline user=%s lesson=%s deadline=%s", user.ID, lessonID, deadline.Format("2006-01-02T15:04"))
	http.Redirect(w, r, "/lesson/"+lessonID, http.StatusSeeOther)
}

func DeleteLessonHandler(w http.ResponseWriter, r *http.Request) {
	user := teacherSession(w, r)
	if user == nil {
		return
	}

	lessonID := mux.Vars(r)["lessonID"]

	lesson, err := DB.GetLesson(storage.LessonID(lessonID))
	if err != nil {
		http.Error(w, "Lesson not found", http.StatusNotFound)
		return
	}
	if lesson.TeacherID != user.ID {
		http.Error(w, "Only the lesson's teacher can delete it", http.StatusForbidden)
		return
	}

	if err := DB.DeleteLesson(lessonID, user.ID); err != nil {
		log.Printf("action=delete_lesson user=%s lesson=%s error=%v", user.ID, lessonID, err)
		http.Error(w, "Failed to delete lesson", http.StatusInternalServerError)
		return
	}

	log.Printf("action=delete_lesson user=%s lesson=%s", user.ID, lessonID)
	analytics.Track(user.ID, "lesson_deleted", map[string]any{"lesson_id": lessonID})
	w.WriteHeader(http.StatusOK)
}

func RegisterTaskRecordToLessonHandler(w http.ResponseWriter, r *http.Request) {
	user := studentSession(w, r)
	if user == nil {
		return
	}

	vars := mux.Vars(r)
	lessonID := vars["lessonID"]
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form data", http.StatusBadRequest)
		return
	}
	taskID := r.FormValue("taskRecordID")
	studentID := r.FormValue("studentID")

	if studentID != user.ID {
		http.Error(w, "Unauthorized: You can only register your own task records", http.StatusForbidden)
		return
	}

	if err := DB.RegisterToLesson(storage.LessonID(lessonID), storage.TaskID(taskID), studentID); err != nil {
		log.Printf("action=register_to_lesson user=%s lesson=%s task=%s error=%v", studentID, lessonID, taskID, err)
		http.Error(w, "Failed to register: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("action=register_to_lesson user=%s lesson=%s task=%s", studentID, lessonID, taskID)
	analytics.Track(studentID, "lesson_registered", map[string]any{"lesson_id": lessonID, "task_id": taskID})
	w.Header().Set("HX-Redirect", "/user/"+studentID+"/task/"+taskID)
}

func UnregisterFromLessonHandler(w http.ResponseWriter, r *http.Request) {
	user := studentSession(w, r)
	if user == nil {
		return
	}

	lessonID := mux.Vars(r)["lessonID"]
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form data", http.StatusBadRequest)
		return
	}
	taskID := r.FormValue("taskID")

	if err := DB.UnregisterFromLesson(storage.LessonID(lessonID), storage.TaskID(taskID), user.ID); err != nil {
		log.Printf("action=unregister_from_lesson user=%s lesson=%s task=%s error=%v", user.ID, lessonID, taskID, err)
		http.Error(w, "Failed to revoke registration: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("action=unregister_from_lesson user=%s lesson=%s task=%s", user.ID, lessonID, taskID)
	analytics.Track(user.ID, "lesson_unregistered", map[string]any{"lesson_id": lessonID, "task_id": taskID})
	http.Redirect(w, r, "/user/"+user.ID+"/task/"+taskID, http.StatusSeeOther)
}
