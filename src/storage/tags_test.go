package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestComputeTags(t *testing.T) {
	base := time.Now()
	rec := func(offset time.Duration, authorID, studentID, content string) TaskRecord {
		return TaskRecord{
			StudentID:  studentID,
			AuthorID:   authorID,
			AuthorName: authorID,
			Content:    content,
			CreatedAt:  base.Add(offset),
			Type:       SubmitRecord,
		}
	}

	t.Run("empty input", func(t *testing.T) {
		assert.Empty(t, ComputeTags(nil))
	})

	t.Run("student cannot add a tag on their own submission", func(t *testing.T) {
		tags := ComputeTags([]TaskRecord{
			rec(0, "student", "student", "topic #group-3"),
		})
		assert.Empty(t, tags)
	})

	t.Run("teacher add", func(t *testing.T) {
		tags := ComputeTags([]TaskRecord{
			rec(0, "student", "student", "topic"),
			rec(time.Minute, "teacher", "student", "looks good #approve"),
		})
		assert.Equal(t, []Tag{{Name: "approve"}}, tags)
	})

	t.Run("student cannot remove a teacher-added tag", func(t *testing.T) {
		tags := ComputeTags([]TaskRecord{
			rec(0, "teacher", "student", "#approve"),
			rec(time.Minute, "student", "student", "-#approve"),
		})
		assert.Equal(t, []Tag{{Name: "approve"}}, tags)
	})

	t.Run("teacher remove drops tag", func(t *testing.T) {
		tags := ComputeTags([]TaskRecord{
			rec(0, "teacher", "student", "#group-3"),
			rec(time.Minute, "teacher", "student", "-#group-3"),
		})
		assert.Empty(t, tags)
	})

	t.Run("removal of an absent tag is a no-op", func(t *testing.T) {
		tags := ComputeTags([]TaskRecord{
			rec(0, "teacher", "student", "-#nope"),
		})
		assert.Empty(t, tags)
	})

	t.Run("order reflects add order, independent of input slice order", func(t *testing.T) {
		newest := rec(time.Minute, "teacher", "student", "#approve")
		oldest := rec(0, "teacher", "student", "#group-3")
		// Pass newest first to prove ComputeTags sorts internally.
		tags := ComputeTags([]TaskRecord{newest, oldest})
		assert.Equal(t, []Tag{
			{Name: "group-3"},
			{Name: "approve"},
		}, tags)
	})

	t.Run("re-add after remove moves tag to the end", func(t *testing.T) {
		tags := ComputeTags([]TaskRecord{
			rec(0, "teacher", "student", "#group-3 #approve"),
			rec(time.Minute, "teacher", "student", "-#group-3"),
			rec(2*time.Minute, "teacher", "student", "#group-3"),
		})
		assert.Equal(t, []Tag{
			{Name: "approve"},
			{Name: "group-3"},
		}, tags)
	})
}

func TestTaskTagsCacheAndInvalidation(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	student := &UserData{ID: "s1", Username: "S1", IsStudent: true}
	assert.NoError(t, db.SaveUser(student))
	teacherID := "t1"
	taskID := TaskID("taskX")

	assert.NoError(t, db.AppendTaskRecord(&TaskRecord{
		TaskID:     taskID,
		StudentID:  student.ID,
		AuthorID:   student.ID,
		AuthorName: student.Username,
		Content:    "topic",
		CreatedAt:  time.Now(),
		Type:       SubmitRecord,
	}))
	assert.NoError(t, db.AppendTaskRecord(&TaskRecord{
		TaskID:     taskID,
		StudentID:  student.ID,
		AuthorID:   teacherID,
		AuthorName: "Teacher",
		Content:    "looks good #group-3",
		CreatedAt:  time.Now(),
		Type:       ReviewedRecord,
	}))

	key := student.ID + ":" + taskID
	if _, ok := db.tagCache.Load(key); ok {
		t.Fatal("cache should be empty before the first TaskTags call")
	}

	tags, err := db.TaskTags(student.ID, taskID)
	assert.NoError(t, err)
	assert.Equal(t, []Tag{{Name: "group-3"}}, tags)

	cached, ok := db.tagCache.Load(key)
	assert.True(t, ok, "TaskTags should populate the cache")
	assert.Equal(t, []Tag{{Name: "group-3"}}, cached)

	// A second append must invalidate the cache so the next TaskTags call sees
	// the new tag rather than a stale cached value.
	assert.NoError(t, db.AppendTaskRecord(&TaskRecord{
		TaskID:     taskID,
		StudentID:  student.ID,
		AuthorID:   teacherID,
		AuthorName: "Teacher",
		Content:    "also #approve",
		CreatedAt:  time.Now(),
		Type:       ReviewedRecord,
	}))

	if _, ok := db.tagCache.Load(key); ok {
		t.Fatal("cache should be invalidated after AppendTaskRecord")
	}

	tags, err = db.TaskTags(student.ID, taskID)
	assert.NoError(t, err)
	assert.Len(t, tags, 2)
}

func TestTaskTagsInvalidatedByLessonLifecycle(t *testing.T) {
	db, tempDir, teacher, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	addSubmit(t, db, student, taskID, "topic")
	assert.NoError(t, db.AppendTaskRecord(&TaskRecord{
		TaskID:     taskID,
		StudentID:  student.ID,
		AuthorID:   teacher.ID,
		AuthorName: teacher.Username,
		Content:    "looks good #group-3",
		CreatedAt:  time.Now(),
		Type:       ReviewedRecord,
	}))
	// A fresh submit resets the task to a registerable state (submit/revoke)
	// after the review, and also exercises AppendTaskRecord's invalidation.
	addSubmit(t, db, student, taskID, "topic v2")

	key := student.ID + ":" + taskID
	_, err := db.TaskTags(student.ID, taskID)
	assert.NoError(t, err)
	_, ok := db.tagCache.Load(key)
	assert.True(t, ok)

	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))
	_, ok = db.tagCache.Load(key)
	assert.False(t, ok, "RegisterToLesson should invalidate the cache")

	tags, err := db.TaskTags(student.ID, taskID)
	assert.NoError(t, err)
	assert.Equal(t, []Tag{{Name: "group-3"}}, tags)

	assert.NoError(t, db.UnregisterFromLesson(lessonID, taskID, student.ID))
	_, ok = db.tagCache.Load(key)
	assert.False(t, ok, "UnregisterFromLesson should invalidate the cache")
}
