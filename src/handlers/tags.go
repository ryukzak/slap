package handlers

import (
	"fmt"
	"log"
	"net/http"
	"sort"

	"github.com/gorilla/mux"
	"github.com/ryukzak/slap/src/auth"
	"github.com/ryukzak/slap/src/storage"
	"github.com/ryukzak/slap/src/util"
)

// TagCount is a tag name with how many student+task pairs currently carry it.
type TagCount struct {
	Name  string
	Count int
}

// TagRow is one student+task pair currently carrying a given tag.
type TagRow struct {
	StudentID   string
	StudentName string
	TaskID      storage.TaskID
	TaskTitle   string
	Excerpt     string
	Status      storage.TaskRecordType
	// LessonID is the lesson this student+task pair is currently queued in,
	// set only when Status is "register".
	LessonID storage.LessonID
	// Score is the current numeric score for this student+task pair (the
	// leading number of the most recent teacher record that has one), or ""
	// if it has never been scored.
	Score string
}

// TagScoreFilter controls whether scored and/or unscored student+task pairs,
// and whether only queued ones, show up on the tag detail page. Scored and
// Unscored are enabled by default (no filter query params at all); once
// either "scored" or "unscored" appears in the query, both are read
// explicitly from it. Queued defaults to off (no filtering by queue status).
type TagScoreFilter struct {
	Scored   bool
	Unscored bool
	Queued   bool

	// Href* toggle just that one bucket, keeping the others as-is.
	HrefScored   string
	HrefUnscored string
	HrefQueued   string
}

func parseTagScoreFilter(r *http.Request) TagScoreFilter {
	q := r.URL.Query()
	scored, unscored := true, true
	if q.Has("scored") || q.Has("unscored") {
		scored = q.Get("scored") == "true"
		unscored = q.Get("unscored") == "true"
	}
	queued := q.Get("queued") == "true"

	href := func(s, u, qd bool) string {
		return fmt.Sprintf("?scored=%t&unscored=%t&queued=%t", s, u, qd)
	}

	return TagScoreFilter{
		Scored: scored, Unscored: unscored, Queued: queued,
		HrefScored:   href(!scored, unscored, queued),
		HrefUnscored: href(scored, !unscored, queued),
		HrefQueued:   href(scored, unscored, !queued),
	}
}

// Allows reports whether a row with the given score passes this filter.
func (f TagScoreFilter) Allows(hasScore bool) bool {
	if hasScore {
		return f.Scored
	}
	return f.Unscored
}

// latestScore returns the current numeric score for a newest-first record
// history: the leading number of the most recent teacher-authored record
// that has one, or "" if none does. Mirrors the score shown on the task page.
func latestScore(records []storage.TaskRecord) string {
	for _, r := range records {
		if r.AuthorID == r.StudentID {
			continue
		}
		if s := util.ExtractScore(r.Content); s != "" {
			return s
		}
	}
	return ""
}

// reviewedExcerpt returns the student's own submission that the most recent
// teacher-authored record actually responded to, walking newest-first
// records. This matters because a student sometimes resubmits placeholder
// content (e.g. "just for registration") purely to satisfy the state
// machine for re-registering into a lesson, without changing their real
// topic; picking the latest own record blindly would then surface that
// placeholder instead of the topic a teacher actually reviewed. Falls back
// to the latest own record when the task has never been reviewed, or to the
// newest record of any kind if the student has no own record at all.
func reviewedExcerpt(records []storage.TaskRecord) storage.TaskRecord {
	for i, r := range records {
		if r.AuthorID == r.StudentID {
			continue
		}
		for _, later := range records[i+1:] {
			if later.AuthorID == later.StudentID {
				return later
			}
		}
		break
	}
	if own := storage.LatestOwnRecord(records); own != nil {
		return *own
	}
	return records[0]
}

// canViewTagRow reports whether viewer may see a student+task pair on the
// tag browser: teachers see everything, a student always sees their own
// rows, and otherwise the task must be marked peer-visible (config.Task.Visible)
// — the same rule that gates content excerpts on the shared lesson page.
func canViewTagRow(viewer *auth.UserClaims, studentID string, taskID storage.TaskID) bool {
	if viewer.IsTeacher || viewer.ID == studentID {
		return true
	}
	task := AppConfig.GetTask(taskID)
	return task != nil && task.Visible
}

// TagsIndexHandler lists every currently active tag with a count of
// student+task pairs carrying it. Open to any signed-in user; a non-teacher
// only sees counts for their own tasks and peer-visible tasks (see
// canViewTagRow).
func TagsIndexHandler(w http.ResponseWriter, r *http.Request) {
	user := userSession(w, r)
	if user == nil {
		return
	}

	users, err := DB.ListUsers()
	if err != nil {
		log.Printf("Error listing users: %v", err)
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	counts := map[string]int{}
	for _, u := range users {
		if !u.IsStudent {
			continue
		}
		allRecords, err := DB.GetAllTaskRecordsForUser(u.ID)
		if err != nil {
			log.Printf("Error fetching records for user %s: %v", u.ID, err)
			continue
		}
		for taskID := range allRecords {
			if !canViewTagRow(user, u.ID, taskID) {
				continue
			}
			tags, err := DB.TaskTags(u.ID, taskID)
			if err != nil {
				log.Printf("Error computing tags for user %s task %s: %v", u.ID, taskID, err)
				continue
			}
			for _, t := range tags {
				counts[t.Name]++
			}
		}
	}

	tagList := make([]TagCount, 0, len(counts))
	for name, count := range counts {
		tagList = append(tagList, TagCount{Name: name, Count: count})
	}
	sort.Slice(tagList, func(i, j int) bool {
		if tagList[i].Count != tagList[j].Count {
			return tagList[i].Count > tagList[j].Count
		}
		return tagList[i].Name < tagList[j].Name
	})

	renderPage(w, "templates/tags.html", struct {
		SessionUserID    string
		SessionIsTeacher bool
		Tags             []TagCount
	}{
		SessionUserID:    user.ID,
		SessionIsTeacher: user.IsTeacher,
		Tags:             tagList,
	})
}

// TagDetailHandler lists every student+task pair currently carrying the given
// tag. Open to any signed-in user; a non-teacher only sees their own rows and
// rows for peer-visible tasks (see canViewTagRow) — this is what lets a
// teacher, or a student self-checking for duplicate topics, browse who else
// carries a tag.
func TagDetailHandler(w http.ResponseWriter, r *http.Request) {
	user := userSession(w, r)
	if user == nil {
		return
	}

	tagName := mux.Vars(r)["tag"]
	filter := parseTagScoreFilter(r)

	users, err := DB.ListUsers()
	if err != nil {
		log.Printf("Error listing users: %v", err)
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	var rows []TagRow
	totalRows := 0
	for _, u := range users {
		if !u.IsStudent {
			continue
		}
		allRecords, err := DB.GetAllTaskRecordsForUser(u.ID)
		if err != nil {
			log.Printf("Error fetching records for user %s: %v", u.ID, err)
			continue
		}
		for taskID, records := range allRecords {
			if len(records) == 0 {
				continue
			}
			if !canViewTagRow(user, u.ID, taskID) {
				continue
			}
			tags, err := DB.TaskTags(u.ID, taskID)
			if err != nil {
				log.Printf("Error computing tags for user %s task %s: %v", u.ID, taskID, err)
				continue
			}
			hasTag := false
			for _, t := range tags {
				if t.Name == tagName {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}

			taskTitle := string(taskID)
			if task := AppConfig.GetTask(taskID); task != nil {
				taskTitle = task.Title
			}

			score := latestScore(records)

			totalRows++
			if !filter.Allows(score != "") {
				continue
			}
			if filter.Queued && records[0].Type != storage.RegisterRecord {
				continue
			}

			excerpt := reviewedExcerpt(records)

			rows = append(rows, TagRow{
				StudentID:   u.ID,
				StudentName: u.Username,
				TaskID:      taskID,
				TaskTitle:   taskTitle,
				Excerpt:     excerpt.Content,
				Status:      records[0].Type,
				LessonID:    records[0].LessonID,
				Score:       score,
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].StudentName != rows[j].StudentName {
			return rows[i].StudentName < rows[j].StudentName
		}
		return rows[i].TaskTitle < rows[j].TaskTitle
	})

	renderPage(w, "templates/tag_detail.html", struct {
		SessionUserID string
		TagName       string
		Rows          []TagRow
		TotalRows     int
		Filter        TagScoreFilter
	}{
		SessionUserID: user.ID,
		TagName:       tagName,
		Rows:          rows,
		TotalRows:     totalRows,
		Filter:        filter,
	})
}
