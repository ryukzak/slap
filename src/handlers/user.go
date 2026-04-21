package handlers

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/ryukzak/slap/src/config"
	"github.com/ryukzak/slap/src/storage"
	"github.com/ryukzak/slap/src/util"
)

const stallThreshold = 4 // lessons skipped before marking as stalled

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

	var relevantRules []config.ScoreRule
	ruleApplies := make(map[string]bool)
	totalEffect := 0

	if dbUser.IsStudent {
		// Функция для получения времени проверки задания
		getCheckedTime := func(taskID storage.TaskID) (*time.Time, error) {
			records, err := DB.ListTaskRecords(profileUserID, taskID)
			if err != nil {
				return nil, err
			}
			for _, record := range records {
				if record.Status == storage.ReviewedTaskRecord {
					return &record.CreatedAt, nil
				}
			}
			return nil, nil
		}

		// Собираем правила, которые относятся к задачам студента
		for _, rule := range AppConfig.ScoreRules {
			// Проверяем, есть ли у студента хотя бы одно задание из правила
			hasTask := false
			for _, taskID := range rule.TaskIDs {
				for _, studentTask := range AppConfig.Tasks {
					if studentTask.ID == taskID {
						hasTask = true
						break
					}
				}
				if hasTask {
					break
				}
			}

			if hasTask {
				relevantRules = append(relevantRules, rule)
				applies, err := AppConfig.RuleApplies(rule, getCheckedTime)
				if err == nil && applies {
					ruleApplies[rule.Name] = true
					totalEffect += rule.Effect
				} else {
					ruleApplies[rule.Name] = false
				}
			}
		}
	}

	showPast := r.URL.Query().Get("showPast") == "true"
	now := time.Now()

	taskTitles := make(map[storage.TaskID]string)
	for _, task := range AppConfig.Tasks {
		taskTitles[task.ID] = task.Title
	}

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
		ScoreRules:               relevantRules,
		RuleApplies:              ruleApplies,
		TotalEffect:              totalEffect,
		TaskTitles:               taskTitles,
	}

	// Load lessons for all users
	lessons, err := DB.ListLessons()
	if err != nil {
		log.Printf("Error loading lessons: %v", err)
	} else {
		if showPast {
			user.Lessons = lessons
		} else {
			startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			for _, l := range lessons {
				if !l.DateTime.Before(startOfDay) {
					user.Lessons = append(user.Lessons, l)
				}
			}
		}
	}

	renderPage(w, "templates/user.html", user)
}

type ScoreStats struct {
	Min    int
	Avg    float64
	Median float64
	Max    int
}

type WaitBucket struct {
	Day1          int // <= 1 day
	Days3         int // 1-3 days
	Week1         int // 3-7 days
	WeekPlus      int // > 7 days
	Day1Stall     int // stalled in <= 1 day bucket
	Days3Stall    int // stalled in 1-3 days bucket
	Week1Stall    int // stalled in 3-7 days bucket
	WeekPlusStall int // stalled in > 7 days bucket
}

func (w WaitBucket) Total() int {
	return w.Day1 + w.Days3 + w.Week1 + w.WeekPlus
}

func (w WaitBucket) TotalStall() int {
	return w.Day1Stall + w.Days3Stall + w.Week1Stall + w.WeekPlusStall
}

type TaskStats struct {
	Pending   int
	Queued    int
	Dropped   int
	Feedback  int
	Checked   int
	Evaluated int
	Scores    *ScoreStats
}

type UserTaskSummary struct {
	Count     int
	Score     string
	Status    storage.TaskRecordStatus
	Summary   string // compact status counts e.g. "p:2 r:1 c:1"
	WaitSince time.Time
}

type UserTableRow struct {
	storage.UserData
	TaskData    map[storage.TaskID]UserTaskSummary
	TotalEffect int
}

type TimelineLesson struct {
	ID         string
	Registered int
	Reviewed   int
	Revoked    int
	Teacher    string
}

type TimelineTeacherReview struct {
	Teacher string
	Checked int
}

type TimelineEntry struct {
	Date           string // "Mon 02 Jan"
	Checked        int    // teacher reviews that day (past only)
	Lessons        []TimelineLesson
	TeacherReviews []TimelineTeacherReview
	Registered     int // total registered across all lessons (future only)
	Reviewed       int // total reviewed across all lessons (future only)
	Revoked        int // total revoked across all lessons
	IsToday        bool
	IsFuture       bool
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

	now := time.Now().In(PrimaryLoc)
	todayKey := now.Format("2006-01-02")
	checkedByDay := make(map[string]int)

	userNames := make(map[string]string) // userID -> username
	for _, u := range users {
		userNames[u.ID] = u.Username
	}

	// dayKey -> teacherID -> count
	checkedByDayTeacher := make(map[string]map[string]int)

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
				summary := UserTaskSummary{Count: len(records), Status: bestStatus, WaitSince: records[0].CreatedAt}
				var pending, queued, dropped, feedback, checked int
				for _, rec := range records {
					if rec.EntryAuthorID != rec.StudentID {
						if score := util.ExtractScore(rec.Content); score != "" && summary.Score == "" {
							summary.Score = score
						}
						dayKey := rec.CreatedAt.In(PrimaryLoc).Format("2006-01-02")
						checkedByDay[dayKey]++
						if checkedByDayTeacher[dayKey] == nil {
							checkedByDayTeacher[dayKey] = make(map[string]int)
						}
						checkedByDayTeacher[dayKey][rec.EntryAuthorID]++
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
				summary.Summary = strings.Join(parts, "\u00a0")
				row.TaskData[task.ID] = summary
			}

			// Calculate total effect for this student
			getCheckedTime := func(taskID storage.TaskID) (*time.Time, error) {
				records, err := DB.ListTaskRecords(u.ID, taskID)
				if err != nil {
					return nil, err
				}
				for _, record := range records {
					if record.Status == storage.ReviewedTaskRecord {
						return &record.CreatedAt, nil
					}
				}
				return nil, nil
			}

			totalEffect, err := AppConfig.CalculateTotalEffect(getCheckedTime)
			if err != nil {
				totalEffect = 0
			}
			row.TotalEffect = totalEffect
		}
		rows = append(rows, row)
	}

	// Compute per-task aggregate stats (students only).
	studentCount := 0
	taskStats := make(map[storage.TaskID]TaskStats)
	scoreValues := make(map[storage.TaskID][]int)
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
				if td.Score != "" && td.Score != "0" {
					ts.Evaluated++
					if v, err := strconv.Atoi(td.Score); err == nil {
						scoreValues[task.ID] = append(scoreValues[task.ID], v)
					}
				} else {
					ts.Checked++
				}
			}
			taskStats[task.ID] = ts
		}
	}
	for taskID, vals := range scoreValues {
		if len(vals) == 0 {
			continue
		}
		sort.Ints(vals)
		sum := 0
		for _, v := range vals {
			sum += v
		}
		n := len(vals)
		var median float64
		if n%2 == 0 {
			median = float64(vals[n/2-1]+vals[n/2]) / 2
		} else {
			median = float64(vals[n/2])
		}
		ts := taskStats[taskID]
		ts.Scores = &ScoreStats{
			Min:    vals[0],
			Avg:    float64(sum) / float64(n),
			Median: median,
			Max:    vals[n-1],
		}
		taskStats[taskID] = ts
	}

	// Build activity timeline
	lessons, err := DB.ListLessons()
	if err != nil {
		log.Printf("Error loading lessons for timeline: %v", err)
	}

	// Collect past lesson dates for stall detection.
	var pastLessonDates []time.Time
	for _, l := range lessons {
		if l.DateTime.Before(now) {
			pastLessonDates = append(pastLessonDates, l.DateTime)
		}
	}

	// Compute pending wait buckets (students with "submit" or "register" status).
	pendingByTask := make(map[storage.TaskID]WaitBucket)
	for _, task := range AppConfig.Tasks {
		pendingByTask[task.ID] = WaitBucket{}
	}
	var pendingTotal WaitBucket
	for _, row := range rows {
		if !row.IsStudent {
			continue
		}
		for _, task := range AppConfig.Tasks {
			td, ok := row.TaskData[task.ID]
			if !ok || td.Count == 0 {
				continue
			}
			if td.Status != storage.SubmitTaskRecord && td.Status != storage.RegisterTaskRecord {
				continue
			}
			wait := now.Sub(td.WaitSince)
			wb := pendingByTask[task.ID]
			// Count skipped lessons for stall detection.
			isStall := false
			if td.Status == storage.SubmitTaskRecord {
				skipped := 0
				for _, ld := range pastLessonDates {
					if ld.After(td.WaitSince) {
						skipped++
					}
				}
				isStall = skipped >= stallThreshold
			}
			switch {
			case wait <= 24*time.Hour:
				wb.Day1++
				pendingTotal.Day1++
				if isStall {
					wb.Day1Stall++
					pendingTotal.Day1Stall++
				}
			case wait <= 3*24*time.Hour:
				wb.Days3++
				pendingTotal.Days3++
				if isStall {
					wb.Days3Stall++
					pendingTotal.Days3Stall++
				}
			case wait <= 7*24*time.Hour:
				wb.Week1++
				pendingTotal.Week1++
				if isStall {
					wb.Week1Stall++
					pendingTotal.Week1Stall++
				}
			default:
				wb.WeekPlus++
				pendingTotal.WeekPlus++
				if isStall {
					wb.WeekPlusStall++
					pendingTotal.WeekPlusStall++
				}
			}
			pendingByTask[task.ID] = wb
		}
	}

	lessonsByDay := make(map[string][]TimelineLesson)
	for _, l := range lessons {
		dayKey := l.DateTime.In(PrimaryLoc).Format("2006-01-02")
		lessonsByDay[dayKey] = append(lessonsByDay[dayKey], TimelineLesson{
			ID:         l.ID,
			Registered: l.RegisteredCount(),
			Reviewed:   l.ReviewedCount(),
			Revoked:    l.RevokedCount(),
			Teacher:    l.TeacherName,
		})
	}

	timelineMap := make(map[string]*TimelineEntry)
	for day, count := range checkedByDay {
		isFuture := day > todayKey
		var reviews []TimelineTeacherReview
		for teacherID, c := range checkedByDayTeacher[day] {
			name := userNames[teacherID]
			if name == "" {
				name = teacherID
			}
			reviews = append(reviews, TimelineTeacherReview{Teacher: name, Checked: c})
		}
		sort.Slice(reviews, func(i, j int) bool {
			return reviews[i].Checked > reviews[j].Checked
		})
		e := &TimelineEntry{
			Checked:        count,
			TeacherReviews: reviews,
			IsToday:        day == todayKey,
			IsFuture:       isFuture,
		}
		timelineMap[day] = e
	}
	for day, tl := range lessonsByDay {
		if e, ok := timelineMap[day]; ok {
			e.Lessons = tl
		} else {
			isFuture := day > todayKey
			timelineMap[day] = &TimelineEntry{
				Lessons:  tl,
				IsToday:  day == todayKey,
				IsFuture: isFuture,
			}
		}
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, PrimaryLoc)
	rangeStart := today.AddDate(0, 0, -14)
	rangeEnd := today.AddDate(0, 0, 7)

	var timeline []TimelineEntry
	for d := rangeStart; !d.After(rangeEnd); d = d.AddDate(0, 0, 1) {
		day := d.Format("2006-01-02")
		e, ok := timelineMap[day]
		if !ok {
			e = &TimelineEntry{}
		}
		e.IsToday = day == todayKey
		e.IsFuture = day > todayKey
		e.Date = d.Format("Mon 02 Jan")
		for _, l := range e.Lessons {
			e.Registered += l.Registered
			e.Reviewed += l.Reviewed
			e.Revoked += l.Revoked
		}
		timeline = append(timeline, *e)
	}

	maxBar := 0
	for _, e := range timeline {
		var total int
		if e.IsFuture {
			total = e.Registered
		} else {
			queued := e.Registered - e.Reviewed
			if queued < 0 {
				queued = 0
			}
			total = e.Checked + queued
		}
		if total > maxBar {
			maxBar = total
		}
	}

	renderPage(w, "templates/users.html", struct {
		SessionUserID string
		Users         []UserTableRow
		Tasks         []config.Task
		StudentCount  int
		TaskStats     map[storage.TaskID]TaskStats
		PendingByTask map[storage.TaskID]WaitBucket
		PendingTotal  WaitBucket
		Timeline      []TimelineEntry
		MaxBar        int
	}{
		SessionUserID: sessionUser.ID,
		Users:         rows,
		Tasks:         AppConfig.Tasks,
		StudentCount:  studentCount,
		TaskStats:     taskStats,
		PendingByTask: pendingByTask,
		PendingTotal:  pendingTotal,
		Timeline:      timeline,
		MaxBar:        maxBar,
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
	header = append(header, "Bonus/Penalty")
	if err := cw.Write(header); err != nil {
		log.Printf("Error writing CSV header: %v", err)
		return
	}

	for _, u := range users {
		if !u.IsStudent {
			continue
		}
		row := []string{u.ID, u.Username}
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

		getCheckedTime := func(taskID storage.TaskID) (*time.Time, error) {
			records, err := DB.ListTaskRecords(u.ID, taskID)
			if err != nil {
				return nil, err
			}
			for _, record := range records {
				if record.Status == storage.ReviewedTaskRecord {
					return &record.CreatedAt, nil
				}
			}
			return nil, nil
		}

		totalEffect, err := AppConfig.CalculateTotalEffect(getCheckedTime)
		if err != nil {
			totalEffect = 0
		}
		row = append(row, fmt.Sprintf("%d", totalEffect))

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
