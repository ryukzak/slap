package storage

import (
	"sort"

	"github.com/ryukzak/slap/src/util"
)

// Tag is a currently active tag on a task, derived from its record history.
type Tag struct {
	Name string
}

// ComputeTags derives the current set of active tags for a task from its full
// record history, applying #tag/-#tag operations found in each record's
// Content in chronological (oldest-first) order, regardless of record order
// passed in. Only teacher-authored records (AuthorID != StudentID) can add or
// remove tags — a student cannot tag their own work, so #tag tokens in a
// student's own submission are plain text and have no effect.
func ComputeTags(records []TaskRecord) []Tag {
	ordered := make([]TaskRecord, len(records))
	copy(ordered, records)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
	})

	active := make(map[string]bool)
	var order []string

	for _, r := range ordered {
		if r.AuthorID == r.StudentID {
			continue
		}
		for _, op := range util.ExtractTagOps(r.Content) {
			if op.Remove {
				if active[op.Name] {
					delete(active, op.Name)
					order = removeTagName(order, op.Name)
				}
				continue
			}
			if !active[op.Name] {
				order = append(order, op.Name)
			}
			active[op.Name] = true
		}
	}

	tags := make([]Tag, 0, len(order))
	for _, name := range order {
		tags = append(tags, Tag{Name: name})
	}
	return tags
}

func removeTagName(order []string, name string) []string {
	for i, n := range order {
		if n == name {
			return append(order[:i], order[i+1:]...)
		}
	}
	return order
}

// TaskTags returns the active tags for a student/task pair, backed by an
// in-memory cache keyed by "studentID:taskID". The cache is populated on miss
// from ListTaskRecords and invalidated by invalidateTags whenever a new
// record is appended for that pair.
func (d *DB) TaskTags(studentID, taskID string) ([]Tag, error) {
	key := studentID + ":" + taskID
	if cached, ok := d.tagCache.Load(key); ok {
		return cached.([]Tag), nil
	}

	records, err := d.ListTaskRecords(studentID, taskID)
	if err != nil {
		return nil, err
	}
	tags := ComputeTags(records)
	d.tagCache.Store(key, tags)
	return tags, nil
}

// invalidateTags drops the cached tags for a student/task pair. Call after
// appending any record that could change the pair's tag set.
func (d *DB) invalidateTags(studentID, taskID string) {
	d.tagCache.Delete(studentID + ":" + taskID)
}
