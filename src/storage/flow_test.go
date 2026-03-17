package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
