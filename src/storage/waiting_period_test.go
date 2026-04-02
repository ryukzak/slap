package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWaitingPeriod_BlocksRegistrationAfterRecentReview(t *testing.T) {
	db, tempDir, teacher, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	// Student submits and registers
	addSubmit(t, db, student, taskID, "first attempt")
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))

	// Teacher reviews
	assert.NoError(t, db.AddTaskRecord(&TaskRecord{
		TaskID:          taskID,
		StudentID:       student.ID,
		EntryAuthorID:   teacher.ID,
		EntryAuthorName: teacher.Username,
		Content:         "needs work",
		CreatedAt:       time.Now(),
		Status:          ReviewTaskRecord,
	}))

	// Student submits again
	addSubmit(t, db, student, taskID, "second attempt")

	// Registration should be blocked by 24h waiting period
	err := db.RegisterToLesson(lessonID, taskID, student.ID, 24*time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "waiting period")
}

func TestWaitingPeriod_AllowsRegistrationAfterPeriodExpires(t *testing.T) {
	db, tempDir, teacher, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	// Student submits and registers
	addSubmit(t, db, student, taskID, "first attempt")
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))

	// Teacher reviews with a CreatedAt in the past (beyond waiting period)
	assert.NoError(t, db.AddTaskRecord(&TaskRecord{
		TaskID:          taskID,
		StudentID:       student.ID,
		EntryAuthorID:   teacher.ID,
		EntryAuthorName: teacher.Username,
		Content:         "needs work",
		CreatedAt:       time.Now().Add(-25 * time.Hour),
		Status:          ReviewTaskRecord,
	}))

	// Student submits again
	addSubmit(t, db, student, taskID, "second attempt")

	// Registration should succeed (waiting period expired)
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID, 24*time.Hour))
}

func TestWaitingPeriod_ZeroDurationSkipsCheck(t *testing.T) {
	db, tempDir, teacher, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	// Student submits and registers
	addSubmit(t, db, student, taskID, "first attempt")
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))

	// Teacher reviews just now
	assert.NoError(t, db.AddTaskRecord(&TaskRecord{
		TaskID:          taskID,
		StudentID:       student.ID,
		EntryAuthorID:   teacher.ID,
		EntryAuthorName: teacher.Username,
		Content:         "needs work",
		CreatedAt:       time.Now(),
		Status:          ReviewTaskRecord,
	}))

	// Student submits again
	addSubmit(t, db, student, taskID, "second attempt")

	// Zero waiting period should allow immediate re-registration
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID, 0))
}

func TestWaitingPeriod_NoWaitingPeriodParam(t *testing.T) {
	db, tempDir, teacher, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	// Student submits and registers
	addSubmit(t, db, student, taskID, "first attempt")
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))

	// Teacher reviews just now
	assert.NoError(t, db.AddTaskRecord(&TaskRecord{
		TaskID:          taskID,
		StudentID:       student.ID,
		EntryAuthorID:   teacher.ID,
		EntryAuthorName: teacher.Username,
		Content:         "needs work",
		CreatedAt:       time.Now(),
		Status:          ReviewTaskRecord,
	}))

	// Student submits again
	addSubmit(t, db, student, taskID, "second attempt")

	// No waiting period param — should allow registration (backward compat)
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))
}

func TestWaitingPeriod_FirstSubmissionNoReview(t *testing.T) {
	db, tempDir, _, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	// Student submits for the first time (no prior reviews)
	addSubmit(t, db, student, taskID, "first attempt")

	// Should register fine even with waiting period
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID, 24*time.Hour))
}
