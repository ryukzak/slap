package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"
)

func setupLessonFlowDB(t *testing.T) (*DB, string, *UserData, *UserData, TaskID, LessonID) {
	db, tempDir := setupTestDB(t)

	teacher := &UserData{ID: "teacher", Username: "Teacher", IsTeacher: true}
	assert.NoError(t, db.SaveUser(teacher))
	student := &UserData{ID: "student", Username: "Student", IsStudent: true}
	assert.NoError(t, db.SaveUser(student))

	lesson := &Lesson{
		DateTime:    time.Now().Add(24 * time.Hour),
		TeacherID:   teacher.ID,
		TeacherName: teacher.Username,
		Description: "test lesson",
	}
	assert.NoError(t, db.AddLesson(lesson))
	lessons, err := db.ListLessons()
	assert.NoError(t, err)
	lessonID := LessonID(lessons[0].ID)

	return db, tempDir, teacher, student, TaskID("lab1"), lessonID
}

func addSubmit(t *testing.T, db *DB, student *UserData, taskID TaskID, content string) {
	t.Helper()
	assert.NoError(t, db.AppendTaskRecord(&TaskRecord{
		TaskID:     taskID,
		StudentID:  student.ID,
		AuthorID:   student.ID,
		AuthorName: student.Username,
		Content:    content,
		CreatedAt:  time.Now(),
		Type:       SubmitRecord,
	}))
}

func TestRevokeByButtonVisible(t *testing.T) {
	db, tempDir, _, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	addSubmit(t, db, student, taskID, "first attempt")
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))

	// Student revokes via button
	assert.NoError(t, db.UnregisterFromLesson(lessonID, taskID, student.ID))

	lesson, err := db.GetLesson(lessonID)
	assert.NoError(t, err)
	assert.Empty(t, lesson.EnrolledTasks, "revoked record should not be in current enrollments")
	assert.Len(t, lesson.PreviousEnrolledTasks, 1, "revoked record should be in history")
	assert.Equal(t, RevokeRecord, lesson.PreviousEnrolledTasks[0].Status)

	prev, err := db.ListLessonPreviousTaskRecords(lesson)
	assert.NoError(t, err)
	assert.Len(t, prev, 1)
	assert.Equal(t, RevokeRecord, prev[0].Type)
	assert.Equal(t, lessonID, prev[0].LessonID, "LessonID should be preserved on revoked record")
}

// TestRevokeAllowsReRegistration verifies that after revoking, the student can
// re-register without resubmitting (RevokeRecord is a valid state to register from).
func TestRevokeAllowsReRegistration(t *testing.T) {
	db, tempDir, _, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	addSubmit(t, db, student, taskID, "first attempt")
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))
	assert.NoError(t, db.UnregisterFromLesson(lessonID, taskID, student.ID))

	// After revoke, log has 3 records: submit, register, revoke.
	records, err := db.ListTaskRecords(student.ID, taskID)
	assert.NoError(t, err)
	assert.Len(t, records, 3, "submit + register + revoke = 3 immutable records")
	assert.Equal(t, RevokeRecord, records[0].Type, "latest record is the revoke")

	// Student can re-register to the same lesson without a new submit.
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))

	lesson, err := db.GetLesson(lessonID)
	assert.NoError(t, err)
	assert.Len(t, lesson.EnrolledTasks, 1, "student is enrolled again")
	assert.Equal(t, RegisterRecord, lesson.EnrolledTasks[0].Status)
}

// TestReRegisterAfterRevokeKeepsQueuePosition verifies a student can register the
// post-revoke state to a different lesson and that the original submit time
// (SubmitAt) follows it so they are not pushed to the back of the new queue.
func TestReRegisterAfterRevokeKeepsQueuePosition(t *testing.T) {
	db, tempDir, teacher, student, taskID, lessonAID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	// Second lesson to re-register into.
	lessonB := &Lesson{
		DateTime:    time.Now().Add(48 * time.Hour),
		TeacherID:   teacher.ID,
		TeacherName: teacher.Username,
		Description: "second lesson",
	}
	assert.NoError(t, db.AddLesson(lessonB))
	lessons, err := db.ListLessons()
	assert.NoError(t, err)
	var lessonBID LessonID
	for _, l := range lessons {
		if l.ID != string(lessonAID) {
			lessonBID = LessonID(l.ID)
		}
	}
	assert.NotEmpty(t, lessonBID)

	addSubmit(t, db, student, taskID, "first attempt")
	original, err := db.ListTaskRecords(student.ID, taskID)
	assert.NoError(t, err)
	originalSubmitAt := original[0].SubmitAt

	assert.NoError(t, db.RegisterToLesson(lessonAID, taskID, student.ID))
	assert.NoError(t, db.UnregisterFromLesson(lessonAID, taskID, student.ID))

	// Re-register to lesson B (no resubmit needed — RevokeRecord is valid).
	assert.NoError(t, db.RegisterToLesson(lessonBID, taskID, student.ID))

	lessonB, err = db.GetLesson(lessonBID)
	assert.NoError(t, err)
	assert.Len(t, lessonB.EnrolledTasks, 1, "resubmission is queued on lesson B")

	queued, err := db.ListLessonTaskRecords(lessonB)
	assert.NoError(t, err)
	assert.Len(t, queued, 1)
	assert.Equal(t, RegisterRecord, queued[0].Type)
	assert.True(t, queued[0].SubmitAt.Equal(originalSubmitAt),
		"submit time (SubmitAt) is preserved so queue position is not lost on re-registration")

	// Lesson A still shows the drop in its history.
	lessonA, err := db.GetLesson(lessonAID)
	assert.NoError(t, err)
	prevA, err := db.ListLessonPreviousTaskRecords(lessonA)
	assert.NoError(t, err)
	assert.Len(t, prevA, 1)
	assert.Equal(t, RevokeRecord, prevA[0].Type)
}

// TestMultipleSubmissionsStackInLog verifies that each new student submission
// appends a fresh SubmitRecord to the immutable log (no collapsing).
func TestMultipleSubmissionsStackInLog(t *testing.T) {
	db, tempDir, _, student, taskID, _ := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	addSubmit(t, db, student, taskID, "first attempt")
	addSubmit(t, db, student, taskID, "second attempt")

	records, err := db.ListTaskRecords(student.ID, taskID)
	assert.NoError(t, err)
	assert.Len(t, records, 2, "each submit appends a new record")

	// Newest-first: records[0] is the second submit.
	assert.Equal(t, SubmitRecord, records[0].Type)
	assert.Equal(t, "second attempt", records[0].Content)
	assert.Equal(t, SubmitRecord, records[1].Type)
	assert.Equal(t, "first attempt", records[1].Content)

	// SubmitAt resets on each new submission (different rounds).
	assert.False(t, records[0].SubmitAt.Equal(records[1].SubmitAt),
		"each submission starts a new round with its own SubmitAt")

	latest, err := db.LatestTaskRecord(student.ID, taskID)
	assert.NoError(t, err)
	assert.Equal(t, SubmitRecord, latest.Type)
	assert.Equal(t, "second attempt", latest.Content)
}

func TestResubmitAfterCheckVisible(t *testing.T) {
	db, tempDir, teacher, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	// Student submits and registers
	addSubmit(t, db, student, taskID, "first attempt")
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))

	records, err := db.ListTaskRecords(student.ID, taskID)
	assert.NoError(t, err)
	// Newest-first: records[0] is the RegisterRecord.
	firstRegisterID := records[0].ID

	// Teacher reviews (appends ReviewedRecord; lesson enrollment status updated)
	assert.NoError(t, db.AppendTaskRecord(&TaskRecord{
		TaskID:     taskID,
		StudentID:  student.ID,
		AuthorID:   teacher.ID,
		AuthorName: teacher.Username,
		Content:    "looks good",
		CreatedAt:  time.Now(),
		Type:       ReviewedRecord,
	}))

	// Student submits new entry and re-registers
	addSubmit(t, db, student, taskID, "second attempt")
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))

	lesson, err := db.GetLesson(lessonID)
	assert.NoError(t, err)
	assert.Len(t, lesson.EnrolledTasks, 1, "only new registration should be current")
	assert.Len(t, lesson.PreviousEnrolledTasks, 1, "reviewed submission should be in history")
	assert.Equal(t, ReviewedRecord, lesson.PreviousEnrolledTasks[0].Status)
	assert.Equal(t, firstRegisterID, lesson.PreviousEnrolledTasks[0].TaskRecordID)

	prev, err := db.ListLessonPreviousTaskRecords(lesson)
	assert.NoError(t, err)
	assert.Len(t, prev, 1)
	assert.Equal(t, ReviewedRecord, prev[0].Type)
	assert.Equal(t, "first attempt", prev[0].Content)
}

// TestReadTolerateCorruptIndex reproduces issue #45: a task index key holds a
// JSON object instead of the expected []string. Display reads must degrade to
// "no records" rather than failing the whole request.
func TestReadTolerateCorruptIndex(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	userID := "409529"
	taskID := TaskID("ac:2026:scheme")
	indexKey := "tasks:" + userID + ":" + taskID

	// Write a JSON object where a []string index is expected.
	err := db.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(db.bucketName)
		return b.Put([]byte(indexKey), []byte(`{"id":"409529","journals":{}}`))
	})
	assert.NoError(t, err)

	records, err := db.ListTaskRecords(userID, taskID)
	assert.NoError(t, err, "ListTaskRecords must not error on a corrupt index")
	assert.Empty(t, records)

	rec, err := db.LatestTaskRecord(userID, taskID)
	assert.NoError(t, err, "LatestTaskRecord must not error on a corrupt index")
	assert.Nil(t, rec)
}

func TestIsRegistrationOpen(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	past := time.Now().Add(-24 * time.Hour)
	deadlineFuture := time.Now().Add(1 * time.Hour)
	deadlinePast := time.Now().Add(-1 * time.Hour)

	// No deadline: open iff DateTime is in the future
	assert.True(t, Lesson{DateTime: future}.IsRegistrationOpen())
	assert.False(t, Lesson{DateTime: past}.IsRegistrationOpen())

	// Deadline set: DateTime is ignored
	assert.True(t, Lesson{DateTime: past, RegistrationDeadline: &deadlineFuture}.IsRegistrationOpen())
	assert.False(t, Lesson{DateTime: future, RegistrationDeadline: &deadlinePast}.IsRegistrationOpen())
}

func TestSetLessonDeadline(t *testing.T) {
	db, tempDir, _, _, _, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	deadline := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	assert.NoError(t, db.SetLessonDeadline(lessonID, deadline))

	lesson, err := db.GetLesson(lessonID)
	assert.NoError(t, err)
	assert.NotNil(t, lesson.RegistrationDeadline)
	assert.WithinDuration(t, deadline, *lesson.RegistrationDeadline, time.Second)
	assert.True(t, lesson.IsRegistrationOpen())
}

func TestLessonFlow(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	teacherID := "teacher"
	studentID := "student"
	taskID := TaskID("lab1")
	lessonDesc := "lesson-description"

	// Add Teacher & Student
	teacher := &UserData{
		ID:        teacherID,
		Username:  strings.ToTitle(teacherID),
		IsTeacher: true,
		IsStudent: false,
	}
	assert.NoError(t, db.SaveUser(teacher))

	student := &UserData{
		ID:        studentID,
		Username:  strings.ToTitle(studentID),
		IsTeacher: false,
		IsStudent: true,
	}
	assert.NoError(t, db.SaveUser(student))

	// Create lesson
	lesson := &Lesson{
		DateTime:    time.Now().Add(24 * time.Hour),
		TeacherID:   teacher.ID,
		TeacherName: teacher.Username,
		Description: lessonDesc,
	}
	assert.NoError(t, db.AddLesson(lesson))

	// List lesson
	lessons, err := db.ListLessons()
	assert.NoError(t, err)
	assert.Len(t, lessons, 1)
	assert.Len(t, lessons[0].EnrolledTasks, 0)
	lessonID := LessonID(lessons[0].ID)
	assert.NotNil(t, lessonID)
	assert.Regexp(t, "^lesson:teacher:.{8}-.{4}-.{4}-.{4}-.{12}$", lessonID)

	// Create task record (student submits)
	submitRecord := &TaskRecord{
		TaskID:     taskID,
		StudentID:  student.ID,
		AuthorID:   student.ID,
		AuthorName: student.Username,
		Content:    taskID + " submission",
		CreatedAt:  time.Date(2023, 8, 15, 14, 30, 0, 0, time.UTC),
		Type:       SubmitRecord,
	}
	assert.NoError(t, db.AppendTaskRecord(submitRecord))

	records, err := db.ListTaskRecords(student.ID, taskID)
	assert.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, taskID, records[0].TaskID)
	assert.Equal(t, SubmitRecord, records[0].Type)
	assert.NotNil(t, records[0].ID)
	assert.Regexp(t, "^task:student:lab1:.{8}-.{4}-.{4}-.{4}-.{12}$", records[0].ID)

	// Register to the lesson (appends a RegisterRecord)
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, studentID))
	lesson, err = db.GetLesson(lessonID)
	assert.NoError(t, err)
	assert.Len(t, lesson.EnrolledTasks, 1)
	assert.Equal(t, taskID, lesson.EnrolledTasks[0].TaskID)
	assert.Equal(t, student.ID, lesson.EnrolledTasks[0].StudentID)

	records, err = db.ListTaskRecords(student.ID, taskID)
	assert.NoError(t, err)
	assert.Len(t, records, 2, "submit + register = 2 immutable records")
	assert.Equal(t, taskID, records[0].TaskID)
	assert.Equal(t, RegisterRecord, records[0].Type, "newest record is the register")
	// Enrolled task points to the RegisterRecord.
	assert.Equal(t, records[0].ID, lesson.EnrolledTasks[0].TaskRecordID)

	// Teacher submits review
	assert.NoError(t, db.AppendTaskRecord(&TaskRecord{
		TaskID:     taskID,
		StudentID:  student.ID,
		AuthorID:   teacher.ID,
		AuthorName: teacher.Username,
		Content:    taskID + " review",
		CreatedAt:  time.Now(),
		Type:       ReviewedRecord,
	}))

	// List updated task history (3 records: reviewed, register, submit)
	records, err = db.ListTaskRecords(student.ID, taskID)
	assert.NoError(t, err)
	assert.Len(t, records, 3)

	assert.Equal(t, student.ID, records[0].StudentID)
	assert.Equal(t, teacher.ID, records[0].AuthorID)
	assert.Equal(t, ReviewedRecord, records[0].Type)

	assert.Equal(t, student.ID, records[1].StudentID)
	assert.Equal(t, student.ID, records[1].AuthorID)
	assert.Equal(t, RegisterRecord, records[1].Type)
}
