package handlers

import (
	"fmt"
	"log"
	"net/http"
	"sort"

	"github.com/gorilla/mux"
	"github.com/ryukzak/slap/src/auth"
	"github.com/ryukzak/slap/src/storage"
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
}

// TagStatusFilter controls which record statuses show up on the tag detail
// page. All four are enabled by default (no filter query params at all); once
// any one of pending/queued/checked/dropped appears in the query, all four
// are read explicitly from it.
type TagStatusFilter struct {
	Pending bool
	Queued  bool
	Checked bool
	Dropped bool

	// Href* toggle just that one status, keeping the other three as-is.
	HrefPending string
	HrefQueued  string
	HrefChecked string
	HrefDropped string
}

func parseTagStatusFilter(r *http.Request) TagStatusFilter {
	q := r.URL.Query()
	pending, queued, checked, dropped := true, true, true, true
	if q.Has("pending") || q.Has("queued") || q.Has("checked") || q.Has("dropped") {
		pending = q.Get("pending") == "true"
		queued = q.Get("queued") == "true"
		checked = q.Get("checked") == "true"
		dropped = q.Get("dropped") == "true"
	}

	href := func(p, qd, c, d bool) string {
		return fmt.Sprintf("?pending=%t&queued=%t&checked=%t&dropped=%t", p, qd, c, d)
	}

	return TagStatusFilter{
		Pending: pending, Queued: queued, Checked: checked, Dropped: dropped,
		HrefPending: href(!pending, queued, checked, dropped),
		HrefQueued:  href(pending, !queued, checked, dropped),
		HrefChecked: href(pending, queued, !checked, dropped),
		HrefDropped: href(pending, queued, checked, !dropped),
	}
}

// Allows reports whether a record of the given status passes this filter.
func (f TagStatusFilter) Allows(t storage.TaskRecordType) bool {
	switch t {
	case storage.SubmitRecord:
		return f.Pending
	case storage.RegisterRecord:
		return f.Queued
	case storage.ReviewedRecord:
		return f.Checked
	case storage.RevokeRecord:
		return f.Dropped
	default:
		return true
	}
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
	filter := parseTagStatusFilter(r)

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

			totalRows++
			if !filter.Allows(records[0].Type) {
				continue
			}

			excerpt := records[0]
			if own := storage.LatestOwnRecord(records); own != nil {
				excerpt = *own
			}

			rows = append(rows, TagRow{
				StudentID:   u.ID,
				StudentName: u.Username,
				TaskID:      taskID,
				TaskTitle:   taskTitle,
				Excerpt:     excerpt.Content,
				Status:      records[0].Type,
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
		Filter        TagStatusFilter
	}{
		SessionUserID: user.ID,
		TagName:       tagName,
		Rows:          rows,
		TotalRows:     totalRows,
		Filter:        filter,
	})
}
