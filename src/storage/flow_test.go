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
	assert.NoError(t, db.AddTaskRecord(&TaskRecord{
		TaskID:          taskID,
		StudentID:       student.ID,
		EntryAuthorID:   student.ID,
		EntryAuthorName: student.Username,
		Content:         content,
		CreatedAt:       time.Now(),
		Status:          SubmitTaskRecord,
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
	assert.Equal(t, RevokedTaskRecord, lesson.PreviousEnrolledTasks[0].Status)

	prev, err := db.ListLessonPreviousTaskRecords(lesson)
	assert.NoError(t, err)
	assert.Len(t, prev, 1)
	assert.Equal(t, RevokedTaskRecord, prev[0].Status)
	assert.Equal(t, lessonID, prev[0].LessonID, "LessonID should be preserved on revoked record")
}

// TestRevokeResubmitsAsPending verifies that revoking leaves the dropped record
// intact as history and appends a fresh pending record (copied content, original
// CreatedAt, no lesson) so the student is back in the queue without resubmitting.
func TestRevokeResubmitsAsPending(t *testing.T) {
	db, tempDir, _, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	addSubmit(t, db, student, taskID, "first attempt")
	before, err := db.ListTaskRecords(student.ID, taskID)
	assert.NoError(t, err)
	assert.Len(t, before, 1)
	originalID := before[0].ID
	originalCreatedAt := before[0].CreatedAt

	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))
	assert.NoError(t, db.UnregisterFromLesson(lessonID, taskID, student.ID))

	records, err := db.ListTaskRecords(student.ID, taskID)
	assert.NoError(t, err)
	assert.Len(t, records, 2, "revoke should append a new pending record, keeping the dropped one")
	assert.Equal(t, SubmitTaskRecord, records[0].Status,
		"the pending resubmission must sort as the newest record despite sharing CreatedAt with the dropped one")

	// records are newest-first; locate each by status.
	var dropped, pending *TaskRecord
	for i := range records {
		switch records[i].Status {
		case RevokedTaskRecord:
			dropped = &records[i]
		case SubmitTaskRecord:
			pending = &records[i]
		}
	}
	assert.NotNil(t, dropped, "the original record stays Dropped")
	assert.NotNil(t, pending, "a fresh Pending record is created")
	assert.Equal(t, originalID, dropped.ID, "dropped record is the original, untouched")
	assert.Equal(t, lessonID, dropped.LessonID, "dropped record keeps its lesson for faithful history")

	assert.NotEqual(t, originalID, pending.ID, "resubmission is a distinct record")
	assert.Equal(t, "first attempt", pending.Content, "content is carried over")
	assert.True(t, pending.CreatedAt.Equal(originalCreatedAt), "queue position (CreatedAt) is preserved")
	assert.Equal(t, LessonID(""), pending.LessonID, "resubmission is not tied to any lesson yet")

	status, err := db.LatestTaskStatus(student.ID, taskID)
	assert.NoError(t, err)
	assert.Equal(t, SubmitTaskRecord, status, "latest record is the new pending one")
}

// TestReRegisterAfterRevokeKeepsQueuePosition verifies a student can register the
// post-revoke pending record to a different lesson, and that the original submit
// time follows it so they are not pushed to the back of the new queue.
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
	originalCreatedAt := original[0].CreatedAt

	assert.NoError(t, db.RegisterToLesson(lessonAID, taskID, student.ID))
	assert.NoError(t, db.UnregisterFromLesson(lessonAID, taskID, student.ID))

	// Re-register the pending resubmission to lesson B (no resubmit needed).
	assert.NoError(t, db.RegisterToLesson(lessonBID, taskID, student.ID))

	lessonB, err = db.GetLesson(lessonBID)
	assert.NoError(t, err)
	assert.Len(t, lessonB.EnrolledTasks, 1, "resubmission is queued on lesson B")

	queued, err := db.ListLessonTaskRecords(lessonB)
	assert.NoError(t, err)
	assert.Len(t, queued, 1)
	assert.Equal(t, RegisterTaskRecord, queued[0].Status)
	assert.True(t, queued[0].CreatedAt.Equal(originalCreatedAt),
		"submit time is preserved so queue position is not lost on re-registration")

	// Lesson A still shows the drop in its history, no status override required.
	lessonA, err := db.GetLesson(lessonAID)
	assert.NoError(t, err)
	prevA, err := db.ListLessonPreviousTaskRecords(lessonA)
	assert.NoError(t, err)
	assert.Len(t, prevA, 1)
	assert.Equal(t, RevokedTaskRecord, prevA[0].Status)
}

// TestNewSubmissionCollapsesPending verifies that adding a genuinely new
// submission drops the previous pending record so only the newest stays pending.
func TestNewSubmissionCollapsesPending(t *testing.T) {
	db, tempDir, _, student, taskID, _ := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	addSubmit(t, db, student, taskID, "first attempt")
	addSubmit(t, db, student, taskID, "second attempt")

	records, err := db.ListTaskRecords(student.ID, taskID)
	assert.NoError(t, err)
	assert.Len(t, records, 2)

	var pending, dropped int
	for _, r := range records {
		switch r.Status {
		case SubmitTaskRecord:
			pending++
		case RevokedTaskRecord:
			dropped++
		}
	}
	assert.Equal(t, 1, pending, "only the newest submission stays pending")
	assert.Equal(t, 1, dropped, "the older pending submission is collapsed to dropped")

	status, err := db.LatestTaskStatus(student.ID, taskID)
	assert.NoError(t, err)
	assert.Equal(t, SubmitTaskRecord, status)
}

func TestResubmitAfterCheckVisible(t *testing.T) {
	db, tempDir, teacher, student, taskID, lessonID := setupLessonFlowDB(t)
	defer cleanupTestDB(db, tempDir)

	// Student submits and registers
	addSubmit(t, db, student, taskID, "first attempt")
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))

	records, err := db.ListTaskRecords(student.ID, taskID)
	assert.NoError(t, err)
	firstRecordID := records[0].ID

	// Teacher checks it (adds review → first record becomes ReviewedTaskRecord)
	assert.NoError(t, db.AddTaskRecord(&TaskRecord{
		TaskID:          taskID,
		StudentID:       student.ID,
		EntryAuthorID:   teacher.ID,
		EntryAuthorName: teacher.Username,
		Content:         "looks good",
		CreatedAt:       time.Now(),
		Status:          ReviewTaskRecord,
	}))

	// Student submits new entry and re-registers
	addSubmit(t, db, student, taskID, "second attempt")
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))

	lesson, err := db.GetLesson(lessonID)
	assert.NoError(t, err)
	assert.Len(t, lesson.EnrolledTasks, 1, "only new registration should be current")
	assert.Len(t, lesson.PreviousEnrolledTasks, 1, "checked submission should be in history")
	assert.Equal(t, ReviewedTaskRecord, lesson.PreviousEnrolledTasks[0].Status)
	assert.Equal(t, firstRecordID, lesson.PreviousEnrolledTasks[0].TaskRecordID)

	prev, err := db.ListLessonPreviousTaskRecords(lesson)
	assert.NoError(t, err)
	assert.Len(t, prev, 1)
	assert.Equal(t, ReviewedTaskRecord, prev[0].Status)
	assert.Equal(t, "first attempt", prev[0].Content)
}

// TestReadTolerateCorruptIndex reproduces issue #45: a task index key holds a
// JSON object (e.g. from a key collision with a colon-containing task ID)
// instead of the expected []string. Display reads must degrade to "no records"
// rather than failing the whole request, which previously 500'd the profile
// page via the score-rule evaluation path.
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

	status, err := db.LatestTaskStatus(userID, taskID)
	assert.NoError(t, err, "LatestTaskStatus must not error on a corrupt index")
	assert.Equal(t, TaskRecordStatus(""), status)
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

	// Create task record
	submitedTaskRecord := &TaskRecord{
		TaskID:          taskID,
		StudentID:       student.ID,
		EntryAuthorID:   student.ID,
		EntryAuthorName: student.Username,
		Content:         taskID + " submittion",
		CreatedAt:       time.Date(2023, 8, 15, 14, 30, 0, 0, time.UTC),
		Status:          SubmitTaskRecord,
	}
	assert.NoError(t, db.AddTaskRecord(submitedTaskRecord))

	records, err := db.ListTaskRecords(student.ID, taskID)
	assert.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, taskID, records[0].TaskID)
	assert.Equal(t, SubmitTaskRecord, records[0].Status)
	assert.NotNil(t, records[0].ID)
	assert.Regexp(t, "^task:student:lab1:.{8}-.{4}-.{4}-.{4}-.{12}$", records[0].ID)

	// Register to the lesson
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, studentID))
	lesson, err = db.GetLesson(lessonID)
	assert.NoError(t, err)
	assert.Len(t, lesson.EnrolledTasks, 1)
	assert.Equal(t, taskID, lesson.EnrolledTasks[0].TaskID)
	assert.Equal(t, student.ID, lesson.EnrolledTasks[0].AuthorID)
	assert.Equal(t, submitedTaskRecord.ID, lesson.EnrolledTasks[0].TaskRecordID)

	records, err = db.ListTaskRecords(student.ID, taskID)
	assert.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, taskID, records[0].TaskID)
	assert.Equal(t, RegisterTaskRecord, records[0].Status)

	// Teacher submit review on the student work
	reviewedTaskRecord := &TaskRecord{
		TaskID:          taskID,
		StudentID:       student.ID,
		EntryAuthorID:   teacher.ID,
		EntryAuthorName: teacher.Username,
		Content:         taskID + " review",
		CreatedAt:       time.Date(2023, 8, 15, 15, 30, 0, 0, time.UTC),
		Status:          ReviewTaskRecord,
	}
	assert.NoError(t, db.AddTaskRecord(reviewedTaskRecord))

	// List updated task history
	records, err = db.ListTaskRecords(student.ID, taskID)
	assert.NoError(t, err)
	assert.Len(t, records, 2)

	assert.Equal(t, student.ID, records[0].StudentID)
	assert.Equal(t, teacher.ID, records[0].EntryAuthorID)
	assert.Equal(t, ReviewTaskRecord, records[0].Status)

	assert.Equal(t, student.ID, records[1].StudentID)
	assert.Equal(t, student.ID, records[1].EntryAuthorID)
	assert.Equal(t, ReviewedTaskRecord, records[1].Status)
}
