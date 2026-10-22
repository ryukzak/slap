package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func addReview(t *testing.T, db *DB, teacher, student *UserData, taskID TaskID, content string) {
	t.Helper()
	assert.NoError(t, db.AppendTaskRecord(&TaskRecord{
		TaskID:     taskID,
		StudentID:  student.ID,
		AuthorID:   teacher.ID,
		AuthorName: teacher.Username,
		Content:    content,
		CreatedAt:  time.Now(),
		Type:       ReviewedRecord,
	}))
}

// A teacher dropping a registration and then reviewing the task — the shape of
// a check taken outside the lesson — must read back as CheckedElsewhere, with
// the teacher named as the author of the drop so it is not mistaken for the
// student withdrawing.
func TestOutsideLessonCheckReadsAsCheckedElsewhere(t *testing.T) {
	db, tempDir, teacher, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	addSubmit(t, db, student, taskID, "submission")
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))

	assert.NoError(t, db.DropFromLessonAsTeacher(lessonID, taskID, student.ID, teacher.ID, teacher.Username))
	addReview(t, db, teacher, student, taskID, "7 checked between lessons")

	lesson, err := db.GetLesson(lessonID)
	assert.NoError(t, err)
	assert.Empty(t, lesson.EnrolledTasks, "the student should have left the queue")
	assert.Zero(t, lesson.ReviewedCount(), "the lesson must not claim a check it did not do")

	prev, err := db.ListLessonPreviousTaskRecords(lesson)
	assert.NoError(t, err)
	assert.Len(t, prev, 1)
	assert.Equal(t, CheckedElsewhereRecord, prev[0].Type)
	assert.Equal(t, teacher.ID, prev[0].AuthorID, "the teacher ended the registration")
	assert.Equal(t, student.ID, prev[0].StudentID)
}

// The author of a revoke cannot by itself mean "checked elsewhere": clearing a
// queue is a teacher action too, and must still read as a drop.
func TestBulkRevokeStaysDropped(t *testing.T) {
	db, tempDir, _, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	addSubmit(t, db, student, taskID, "submission")
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))

	count, err := db.UnregisterAllFromLesson(lessonID)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	lesson, err := db.GetLesson(lessonID)
	assert.NoError(t, err)
	prev, err := db.ListLessonPreviousTaskRecords(lesson)
	assert.NoError(t, err)
	assert.Len(t, prev, 1)
	assert.Equal(t, RevokeRecord, prev[0].Type)
}

// A student withdrawing and later resubmitting must not be read as a check,
// even though a review eventually follows further down the log.
func TestStudentWithdrawalThenResubmitStaysDropped(t *testing.T) {
	db, tempDir, teacher, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	addSubmit(t, db, student, taskID, "first attempt")
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))
	assert.NoError(t, db.UnregisterFromLesson(lessonID, taskID, student.ID))
	addSubmit(t, db, student, taskID, "second attempt")
	addReview(t, db, teacher, student, taskID, "8 better")

	lesson, err := db.GetLesson(lessonID)
	assert.NoError(t, err)
	prev, err := db.ListLessonPreviousTaskRecords(lesson)
	assert.NoError(t, err)
	assert.Len(t, prev, 1)
	assert.Equal(t, RevokeRecord, prev[0].Type, "a resubmission came between the revoke and the review")
	assert.Equal(t, student.ID, prev[0].AuthorID, "the student withdrew")
}

// A plain student revoke with nothing after it stays a drop.
func TestStudentRevokeStaysDropped(t *testing.T) {
	db, tempDir, _, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	addSubmit(t, db, student, taskID, "submission")
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))
	assert.NoError(t, db.UnregisterFromLesson(lessonID, taskID, student.ID))

	lesson, err := db.GetLesson(lessonID)
	assert.NoError(t, err)
	prev, err := db.ListLessonPreviousTaskRecords(lesson)
	assert.NoError(t, err)
	assert.Len(t, prev, 1)
	assert.Equal(t, RevokeRecord, prev[0].Type)
	assert.Equal(t, student.ID, prev[0].AuthorID)
}

// A lesson check still flips the enrollment to reviewed: the drop-first path
// must not have disturbed the ordinary flow.
func TestLessonCheckStillCountsForLesson(t *testing.T) {
	db, tempDir, teacher, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	addSubmit(t, db, student, taskID, "submission")
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))
	addReview(t, db, teacher, student, taskID, "9 good")

	lesson, err := db.GetLesson(lessonID)
	assert.NoError(t, err)
	assert.Equal(t, 1, lesson.ReviewedCount())
	assert.Empty(t, lesson.PreviousEnrolledTasks)
}
