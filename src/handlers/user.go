package handlers

import (
	"encoding/csv"
	"fmt"
	"log"
	"math"
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

type ScoreRuleWithStatus struct {
	config.ScoreRule
	Status      string // "active", "applied", "not_applied"
	StatusColor string // "yellow", "red", "green", "gray"
	EffectColor string // "yellow", "red", "green", "gray"
}

// TaskTimelineEvent is one point on a task's activity timeline. PositionPct
// places it on the page-wide timeline scale shared by every task on the
// student page (0 = scale start, 100 = scale end; see computeTimelineScale),
// so positions are directly comparable across a student's tasks.
type TaskTimelineEvent struct {
	Type        storage.TaskRecordType
	CreatedAt   time.Time
	AuthorName  string
	Content     string
	PositionPct float64
}

// TaskSummary aggregates a task's full record history for the student page:
// when the student first submitted and first registered into a lesson, how
// many times a teacher has actually left feedback (a "reviewed" record —
// not administrative register/revoke actions), and the events making up its
// timeline bar.
type TaskSummary struct {
	FirstSubmission   *time.Time
	FirstRegistration *time.Time
	FeedbackCount     int
	// Score is the current numeric score for this task (the leading number
	// of the most recent teacher-authored record that has one), or "" if
	// it has never been scored.
	Score    string
	Timeline []TaskTimelineEvent
}

// buildTaskSummary derives a TaskSummary from a task's full record history.
// records must be newest-first, as returned by DB.ListTaskRecords.
// PositionPct on the returned events is left unset (0) — call
// applyTimelineScale afterward once the page-wide scale is known.
func buildTaskSummary(records []storage.TaskRecord) TaskSummary {
	var summary TaskSummary
	if len(records) == 0 {
		return summary
	}

	summary.Timeline = make([]TaskTimelineEvent, 0, len(records))

	// Walk oldest-first so "first submission"/"first registration" and the
	// timeline order come out naturally.
	for i := len(records) - 1; i >= 0; i-- {
		r := records[i]
		// Require the author to actually differ from the student, not just
		// a "reviewed" Type: legacy/imported data can carry a Type of
		// "reviewed" on a record the student authored themselves (Type
		// alone isn't a reliable signal there), which would otherwise
		// overcount teacher feedback.
		if r.Type == storage.ReviewedRecord && r.AuthorID != r.StudentID {
			summary.FeedbackCount++
		}
		// Walking oldest-to-newest and overwriting on every match leaves
		// the most recent teacher-authored score by the time the loop ends.
		if r.AuthorID != r.StudentID {
			if s := util.ExtractScore(r.Content); s != "" {
				summary.Score = s
			}
		}
		if r.Type == storage.SubmitRecord && summary.FirstSubmission == nil {
			t := r.CreatedAt
			summary.FirstSubmission = &t
		}
		if r.Type == storage.RegisterRecord && summary.FirstRegistration == nil {
			t := r.CreatedAt
			summary.FirstRegistration = &t
		}

		summary.Timeline = append(summary.Timeline, TaskTimelineEvent{
			Type:       r.Type,
			CreatedAt:  r.CreatedAt,
			AuthorName: r.AuthorName,
			Content:    r.Content,
		})
	}

	return summary
}

// computeTimelineScale picks the shared time scale for a student page's task
// timelines: the configured course start/end when both are set (so every
// student's timeline is plotted on the same, comparable scale), falling
// back to that student's own earliest-to-latest task activity otherwise.
// Returns the zero Value twice if there are no events and no configured
// course bounds.
func computeTimelineScale(cfg *config.Config, summaries map[storage.TaskID]TaskSummary) (time.Time, time.Time) {
	if cfg.CourseStart != nil && cfg.CourseEnd != nil {
		return *cfg.CourseStart, *cfg.CourseEnd
	}

	var start, end time.Time
	for _, summary := range summaries {
		for _, ev := range summary.Timeline {
			if start.IsZero() || ev.CreatedAt.Before(start) {
				start = ev.CreatedAt
			}
			if end.IsZero() || ev.CreatedAt.After(end) {
				end = ev.CreatedAt
			}
		}
	}
	return start, end
}

// applyTimelineScale sets PositionPct on every event across summaries,
// placing each event between start and end (0 = start, 100 = end, clamped).
// Mutates summaries in place.
func applyTimelineScale(summaries map[storage.TaskID]TaskSummary, start, end time.Time) {
	span := end.Sub(start)
	for _, summary := range summaries {
		for i := range summary.Timeline {
			ev := &summary.Timeline[i]
			pct := 50.0
			if span > 0 {
				pct = float64(ev.CreatedAt.Sub(start)) / float64(span) * 100
				pct = math.Round(pct*10) / 10
				pct = math.Max(0, math.Min(100, pct))
			}
			ev.PositionPct = pct
		}
	}
}

var courseActivityTypeLabel = map[storage.TaskRecordType]string{
	storage.SubmitRecord:   "submitted",
	storage.RegisterRecord: "queued",
	storage.ReviewedRecord: "checked",
	storage.RevokeRecord:   "dropped",
}

// courseActivityTypeOrder is a stable iteration order for the map above, so
// tooltip text is deterministic.
var courseActivityTypeOrder = []storage.TaskRecordType{
	storage.SubmitRecord, storage.RegisterRecord, storage.ReviewedRecord, storage.RevokeRecord,
}

// courseActivityGlyphs are Unicode shade blocks encoding a count as glyph
// density instead of color or height — density reads the same regardless
// of theme or how well a viewer distinguishes color/size differences.
// Ordered lightest (lowest nonzero level) to densest (highest).
var courseActivityGlyphs = []string{"░", "▒", "▓", "█"}

// courseActivityGlyphForCount picks the glyph for count, where step is how
// many counts each level spans: 1..step is the lightest glyph, step+1..2*step
// the next, and so on, capping at the densest glyph for anything beyond the
// second-to-last boundary. Returns "" for count <= 0 (no glyph at all).
func courseActivityGlyphForCount(count, step int) string {
	if count <= 0 || step <= 0 {
		return ""
	}
	level := (count - 1) / step
	level = min(level, len(courseActivityGlyphs)-1)
	return courseActivityGlyphs[level]
}

// CourseActivityLevel is one legend entry: a glyph and the count range it
// represents at the current step (e.g. "3–4" for the second level at
// step 2, or "7+" for the last, open-ended level).
type CourseActivityLevel struct {
	Glyph string
	Range string
}

// courseActivityLevels builds the legend entries for a given step.
func courseActivityLevels(step int) []CourseActivityLevel {
	levels := make([]CourseActivityLevel, len(courseActivityGlyphs))
	for i, g := range courseActivityGlyphs {
		lo := i*step + 1
		if i == len(courseActivityGlyphs)-1 {
			levels[i] = CourseActivityLevel{Glyph: g, Range: fmt.Sprintf("≥%d", lo)}
		} else {
			levels[i] = CourseActivityLevel{Glyph: g, Range: fmt.Sprintf("%d–%d", lo, (i+1)*step)}
		}
	}
	return levels
}

// courseActivitySteps are the selectable counts-per-level offered by the
// step selector. The default (used when the query param is absent,
// non-numeric, or non-positive) is 2.
var courseActivitySteps = []int{1, 2, 5, 10, 25}

const courseActivityDefaultStep = 2

// CourseActivityStepOption is one link in the step selector.
type CourseActivityStepOption struct {
	Label    string
	Href     string
	Selected bool
}

// CourseActivityCell is one task/week cell in the course-wide activity
// heatmap, rendered as two glyphs: submissions on the left, teacher reviews
// on the right, each independently bucketed by count (see
// courseActivityGlyphForCount) — so the two are never forced onto a shared
// scale the way a page-wide "busiest cell" comparison would. Tooltip
// carries the full type breakdown, including queued/dropped, which don't
// get their own glyph.
type CourseActivityCell struct {
	SubmitCount int
	CheckCount  int
	SubmitGlyph string // one of courseActivityGlyphs; "" when SubmitCount == 0
	CheckGlyph  string
	Tooltip     string
}

// CourseActivityRow is one task's row in the heatmap: one cell per week
// across the course.
type CourseActivityRow struct {
	TaskID    storage.TaskID
	TaskTitle string
	Cells     []CourseActivityCell
}

// CourseActivity is the course-wide, per-task activity heatmap shown on the
// teacher dashboard: rows are tasks, columns are weeks spanning the whole
// course.
type CourseActivity struct {
	Rows        []CourseActivityRow
	RangeStart  string
	RangeEnd    string
	Step        int
	Levels      []CourseActivityLevel
	StepOptions []CourseActivityStepOption
	// Expanded keeps the section's <details> open across a step-link
	// navigation (a plain page reload) instead of snapping shut — true
	// whenever the request carries an activity_step param, which every
	// step link includes.
	Expanded bool
}

// buildCourseActivity aggregates every student's task record history into a
// per-task, per-week heatmap spanning the whole course. userRecords maps
// each student's ID to their records by task, as cached from
// DB.GetAllTaskRecordsForUser while building the students table. The course
// scale is cfg.CourseStart/CourseEnd when both are set (see
// computeTimelineScale for the same convention on the student page),
// falling back to the earliest-to-latest record seen across every student.
// step is how many counts each glyph level spans (see
// courseActivityGlyphForCount).
func buildCourseActivity(cfg *config.Config, userRecords map[storage.UserID]map[storage.TaskID][]storage.TaskRecord, step int) CourseActivity {
	start, end := cfg.CourseStart, cfg.CourseEnd
	var scaleStart, scaleEnd time.Time
	if start != nil && end != nil {
		scaleStart, scaleEnd = *start, *end
	} else {
		for _, byTask := range userRecords {
			for _, records := range byTask {
				for _, r := range records {
					if scaleStart.IsZero() || r.CreatedAt.Before(scaleStart) {
						scaleStart = r.CreatedAt
					}
					if scaleEnd.IsZero() || r.CreatedAt.After(scaleEnd) {
						scaleEnd = r.CreatedAt
					}
				}
			}
		}
	}
	if scaleStart.IsZero() || scaleEnd.IsZero() || !scaleStart.Before(scaleEnd) {
		return CourseActivity{Step: step, Levels: courseActivityLevels(step)}
	}

	const bucketDays = 7
	weeks := int(scaleEnd.Sub(scaleStart).Hours()/(24*bucketDays)) + 1

	type cellCounts struct {
		byType map[storage.TaskRecordType]int
	}
	grid := make(map[storage.TaskID][]cellCounts, len(cfg.Tasks))
	for _, task := range cfg.Tasks {
		cells := make([]cellCounts, weeks)
		for i := range cells {
			cells[i].byType = make(map[storage.TaskRecordType]int)
		}
		grid[task.ID] = cells
	}

	for _, byTask := range userRecords {
		for taskID, records := range byTask {
			cells, ok := grid[taskID]
			if !ok {
				continue // record belongs to a task no longer in config
			}
			for _, r := range records {
				idx := int(r.CreatedAt.Sub(scaleStart).Hours() / (24 * bucketDays))
				idx = max(0, min(weeks-1, idx))
				cells[idx].byType[r.Type]++
			}
		}
	}

	weekLabels := make([]string, weeks)
	for i := range weeks {
		weekLabels[i] = scaleStart.AddDate(0, 0, i*bucketDays).Format("2 Jan")
	}

	rows := make([]CourseActivityRow, 0, len(cfg.Tasks))
	for _, task := range cfg.Tasks {
		row := CourseActivityRow{TaskID: task.ID, TaskTitle: task.Title, Cells: make([]CourseActivityCell, weeks)}
		for i, c := range grid[task.ID] {
			submit, check := c.byType[storage.SubmitRecord], c.byType[storage.ReviewedRecord]
			var parts []string
			for _, t := range courseActivityTypeOrder {
				if n := c.byType[t]; n > 0 {
					parts = append(parts, fmt.Sprintf("%d %s", n, courseActivityTypeLabel[t]))
				}
			}
			if len(parts) == 0 {
				continue
			}
			row.Cells[i] = CourseActivityCell{
				SubmitCount: submit,
				CheckCount:  check,
				SubmitGlyph: courseActivityGlyphForCount(submit, step),
				CheckGlyph:  courseActivityGlyphForCount(check, step),
				Tooltip:     fmt.Sprintf("week of %s: %s", weekLabels[i], strings.Join(parts, ", ")),
			}
		}
		rows = append(rows, row)
	}

	return CourseActivity{
		Rows:       rows,
		RangeStart: scaleStart.Format("2 Jan"),
		RangeEnd:   scaleEnd.Format("2 Jan"),
		Step:       step,
		Levels:     courseActivityLevels(step),
	}
}

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

	taskStatuses := make(map[storage.TaskID]storage.TaskRecordType)
	taskPreview := make(map[storage.TaskID]*storage.TaskRecord)
	taskTags := make(map[storage.TaskID][]storage.Tag)
	taskSummaries := make(map[storage.TaskID]TaskSummary)
	for _, task := range AppConfig.Tasks {
		rec, err := DB.LatestTaskRecord(profileUserID, task.ID)
		if err != nil {
			log.Printf("Error fetching task status for user %s task %s: %v", profileUserID, task.ID, err)
			continue
		}
		if rec != nil {
			taskStatuses[task.ID] = rec.Type
		}
		records, err := DB.ListTaskRecords(profileUserID, task.ID)
		if err != nil {
			log.Printf("Error fetching task records for user %s task %s: %v", profileUserID, task.ID, err)
			continue
		}
		if preview := storage.LatestOwnRecord(records); preview != nil {
			taskPreview[task.ID] = preview
		} else if len(records) > 0 {
			taskPreview[task.ID] = &records[0]
		}
		taskSummaries[task.ID] = buildTaskSummary(records)

		if tags, err := DB.TaskTags(profileUserID, task.ID); err != nil {
			log.Printf("Error computing tags for user %s task %s: %v", profileUserID, task.ID, err)
		} else {
			taskTags[task.ID] = tags
		}
	}

	timelineStart, timelineEnd := computeTimelineScale(AppConfig, taskSummaries)
	applyTimelineScale(taskSummaries, timelineStart, timelineEnd)

	var rulesWithStatus []ScoreRuleWithStatus
	totalEffect := 0

	if dbUser.IsStudent {
		getCheckedTime := func(taskID storage.TaskID) (*time.Time, error) {
			records, err := DB.ListTaskRecords(profileUserID, taskID)
			if err != nil {
				return nil, err
			}
			at, _ := latestCheckedInfo(records)
			return at, nil
		}

		evaluator := NewEvaluator(AppConfig)
		now := time.Now()

		for _, rule := range AppConfig.ScoreRules {
			eval, err := evaluator.EvaluateForStudent(rule, now, getCheckedTime)
			if err != nil {
				log.Printf("Error evaluating rule %s for user %s: %v", rule.Name, profileUserID, err)
				http.Error(w, "Failed to evaluate score rules", http.StatusInternalServerError)
				return
			}

			rulesWithStatus = append(rulesWithStatus, ScoreRuleWithStatus{
				ScoreRule:   rule,
				Status:      eval.Status(),
				StatusColor: eval.Color(),
				EffectColor: eval.Color(),
			})

			if eval.Applies {
				totalEffect += rule.Effect
			}
		}
	}

	var timelineStartPtr, timelineEndPtr *time.Time
	if !timelineStart.IsZero() {
		timelineStartPtr, timelineEndPtr = &timelineStart, &timelineEnd
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
		TaskPreview:              taskPreview,
		TaskTags:                 taskTags,
		TaskSummaries:            taskSummaries,
		TimelineStart:            timelineStartPtr,
		TimelineEnd:              timelineEndPtr,
		Lessons:                  []*storage.Lesson{},
		ShowPastLessons:          showPast,
		Now:                      now,
		DefaultDateTime:          getTomorrowNoon(),
		TZName:                   PrimaryTZName,
		DefaultLessonDescription: AppConfig.DefaultLessonDescription,
		ScoreRules:               rulesWithStatus,
		TotalEffect:              totalEffect,
		TaskTitles:               taskTitles,
		Notes:                    dbUser.Notes,
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

// AddUserNoteHandler appends a teacher's note about a user. Teacher-only;
// any teacher may add a note, and notes are shared between teachers.
func AddUserNoteHandler(w http.ResponseWriter, r *http.Request) {
	sessionUser := teacherSession(w, r)
	if sessionUser == nil {
		return
	}

	profileUserID := mux.Vars(r)["userID"]
	if profileUserID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if _, err := DB.GetUser(profileUserID); err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}
	content := strings.TrimSpace(r.FormValue("note"))
	if content == "" {
		http.Error(w, "Note cannot be empty", http.StatusBadRequest)
		return
	}

	note := storage.UserNote{
		Content:    content,
		AuthorID:   sessionUser.ID,
		AuthorName: sessionUser.Username,
	}
	if err := DB.AddUserNote(profileUserID, note); err != nil {
		log.Printf("action=add_user_note teacher=%s user=%s error=%v", sessionUser.ID, profileUserID, err)
		http.Error(w, "Failed to add note", http.StatusInternalServerError)
		return
	}

	log.Printf("action=add_user_note teacher=%s user=%s", sessionUser.ID, profileUserID)
	http.Redirect(w, r, "/user/"+profileUserID, http.StatusSeeOther)
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
	Pending int // submitted (or dropped, or a stale registration), not yet checked
	Queued  int // registered for a lesson that has not passed yet
	Checked int // reviewed by a teacher (scored or not)
	Scores  *ScoreStats
}

type UserTaskSummary struct {
	Count     int
	Score     string
	Status    storage.TaskRecordType
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

// effectiveTaskStatus collapses a student's record history for one task into a
// single display state:
//   - ReviewedRecord (Checked) if the teacher has reviewed it at all;
//   - RegisterRecord (Queued) only while the lesson is still upcoming;
//   - SubmitRecord  (Pending) for everything else — a plain submission, a
//     dropped registration, or a *stale* registration whose lesson day has
//     already passed without a review (the student registered but was never
//     checked, so the work is effectively still pending).
//
// records is newest-first. startOfToday is midnight today in PrimaryLoc, so a
// registration only counts as stale the day after its lesson.
func effectiveTaskStatus(records []storage.TaskRecord, lessonByID map[storage.LessonID]*storage.Lesson, startOfToday time.Time) storage.TaskRecordType {
	for _, rec := range records {
		if rec.Type == storage.ReviewedRecord {
			return storage.ReviewedRecord
		}
	}
	latest := records[0]
	if latest.Type == storage.RegisterRecord {
		if l, ok := lessonByID[latest.LessonID]; ok && l.DateTime.Before(startOfToday) {
			return storage.SubmitRecord // stale registration -> Pending
		}
		return storage.RegisterRecord
	}
	return storage.SubmitRecord
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
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, PrimaryLoc)
	checkedByDay := make(map[string]int)

	// Lesson lookup for detecting stale registrations (registered, lesson passed,
	// never reviewed) so they no longer count as Queued.
	lessonByID := make(map[storage.LessonID]*storage.Lesson)
	if allLessons, err := DB.ListLessons(); err != nil {
		log.Printf("Error listing lessons for user stats: %v", err)
	} else {
		for _, l := range allLessons {
			lessonByID[l.ID] = l
		}
	}

	userNames := make(map[string]string) // userID -> username
	for _, u := range users {
		userNames[u.ID] = u.Username
	}

	// dayKey -> teacherID -> count
	checkedByDayTeacher := make(map[string]map[string]int)

	// userRecords caches each student's full record history (by task) so
	// buildCourseActivity can aggregate it below without a second DB pass.
	userRecords := make(map[storage.UserID]map[storage.TaskID][]storage.TaskRecord)

	rows := make([]UserTableRow, 0, len(users))
	for _, u := range users {
		row := UserTableRow{
			UserData: *u,
			TaskData: make(map[storage.TaskID]UserTaskSummary),
		}
		if u.IsStudent {

			allRecords, err := DB.GetAllTaskRecordsForUser(u.ID)
			if err != nil {
				log.Printf("Error fetching task records for user %s: %v", u.ID, err)
				row.TotalEffect = 0
				rows = append(rows, row)
				continue
			}
			userRecords[u.ID] = allRecords

			for _, task := range AppConfig.Tasks {
				records, ok := allRecords[task.ID]
				if !ok || len(records) == 0 {
					continue
				}
				bestStatus := effectiveTaskStatus(records, lessonByID, startOfToday)
				summary := UserTaskSummary{Count: len(records), Status: bestStatus, WaitSince: records[0].CreatedAt}
				var pending, queued, checked int
				for _, rec := range records {
					if rec.AuthorID != rec.StudentID {
						if score := util.ExtractScore(rec.Content); score != "" && summary.Score == "" {
							summary.Score = score
						}
						dayKey := rec.CreatedAt.In(PrimaryLoc).Format("2006-01-02")
						checkedByDay[dayKey]++
						if checkedByDayTeacher[dayKey] == nil {
							checkedByDayTeacher[dayKey] = make(map[string]int)
						}
						checkedByDayTeacher[dayKey][rec.AuthorID]++
					}
					switch rec.Type {
					case storage.SubmitRecord:
						pending++
					case storage.RegisterRecord:
						queued++
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
				summary.Summary = strings.Join(parts, "\u00a0")
				row.TaskData[task.ID] = summary
			}

			totalEffect, err := calculateStudentTotalEffectWithRecords(allRecords)
			if err != nil {
				log.Printf("Error calculating total effect for user %s: %v", u.ID, err)
				row.TotalEffect = 0
			} else {
				row.TotalEffect = totalEffect
			}
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
			case storage.SubmitRecord:
				ts.Pending++
			case storage.RegisterRecord:
				ts.Queued++
			case storage.ReviewedRecord:
				ts.Checked++
				// A numeric score still feeds the min/avg/med/max summary below,
				// but scored and unscored reviews share the Checked bucket.
				if td.Score != "" && td.Score != "0" {
					if v, err := strconv.Atoi(td.Score); err == nil {
						scoreValues[task.ID] = append(scoreValues[task.ID], v)
					}
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
			if td.Status != storage.SubmitRecord && td.Status != storage.RegisterRecord {
				continue
			}
			wait := now.Sub(td.WaitSince)
			wb := pendingByTask[task.ID]
			// Count skipped lessons for stall detection.
			isStall := false
			if td.Status == storage.SubmitRecord {
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

	activityStepParam := r.URL.Query().Get("activity_step")
	activityStep := courseActivityDefaultStep
	if v, err := strconv.Atoi(activityStepParam); err == nil && v > 0 {
		activityStep = v
	}

	stepOptions := make([]CourseActivityStepOption, len(courseActivitySteps))
	for i, s := range courseActivitySteps {
		stepOptions[i] = CourseActivityStepOption{
			Label:    strconv.Itoa(s),
			Href:     fmt.Sprintf("?activity_step=%d", s),
			Selected: s == activityStep,
		}
	}
	courseActivity := buildCourseActivity(AppConfig, userRecords, activityStep)
	courseActivity.StepOptions = stepOptions
	// Every step link carries activity_step, so its presence means the user
	// got here by clicking one — keep the section open rather than
	// snapping shut on the resulting page reload.
	courseActivity.Expanded = activityStepParam != ""

	renderPage(w, "templates/users.html", struct {
		SessionUserID  string
		Users          []UserTableRow
		Tasks          []config.Task
		StudentCount   int
		TaskStats      map[storage.TaskID]TaskStats
		PendingByTask  map[storage.TaskID]WaitBucket
		PendingTotal   WaitBucket
		Timeline       []TimelineEntry
		MaxBar         int
		CourseActivity CourseActivity
	}{
		SessionUserID:  sessionUser.ID,
		Users:          rows,
		Tasks:          AppConfig.Tasks,
		StudentCount:   studentCount,
		TaskStats:      taskStats,
		PendingByTask:  pendingByTask,
		PendingTotal:   pendingTotal,
		Timeline:       timeline,
		MaxBar:         maxBar,
		CourseActivity: courseActivity,
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
	cw.Comma = ','

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

		allRecords, err := DB.GetAllTaskRecordsForUser(u.ID)
		if err != nil {
			log.Printf("Error loading records for user %s: %v", u.ID, err)

			// Fall back to empty
			for range AppConfig.Tasks {
				row = append(row, "")
			}
			row = append(row, "0")
			if err := cw.Write(row); err != nil {
				log.Printf("Error writing CSV row: %v", err)
			}

			continue
		}

		for _, task := range AppConfig.Tasks {
			records, ok := allRecords[task.ID]
			if !ok {
				row = append(row, "")
				continue
			}
			score := ""
			for _, rec := range records {
				if rec.AuthorID != rec.StudentID {
					if s := util.ExtractScore(rec.Content); s != "" && s != "0" {
						score = s
						break
					}
				}
			}
			row = append(row, score)
		}

		totalEffect, err := calculateStudentTotalEffectWithRecords(allRecords)
		if err != nil {
			log.Printf("Error calculating total effect for user %s: %v", u.ID, err)
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

// calculateStudentTotalEffectWithRecords calculates total effect using preloaded records
func calculateStudentTotalEffectWithRecords(allRecords map[storage.TaskID][]storage.TaskRecord) (int, error) {
	getCheckedTime := func(taskID storage.TaskID) (*time.Time, error) {
		records, ok := allRecords[taskID]
		if !ok {
			return nil, nil
		}
		at, _ := latestCheckedInfo(records)
		return at, nil
	}

	evaluator := NewEvaluator(AppConfig)
	now := time.Now()
	total := 0
	for _, rule := range AppConfig.ScoreRules {
		eval, err := evaluator.EvaluateForStudent(rule, now, getCheckedTime)
		if err != nil {
			// Propagate error — caller decides how to handle
			return 0, fmt.Errorf("failed to evaluate rule %s: %w", rule.Name, err)
		}
		if eval.Applies {
			total += rule.Effect
		}
	}
	return total, nil
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
