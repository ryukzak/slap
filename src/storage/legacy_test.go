package storage

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"
)

// writeLegacyRecord writes rawJSON under recordKey in the DB bucket and appends
// recordKey to the task index "tasks:{userID}:{taskID}". Use this to inject
// synthetic old-format records without going through AppendTaskRecord.
func writeLegacyRecord(t *testing.T, db *DB, userID, taskID, recordKey string, rawJSON []byte) {
	t.Helper()
	err := db.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(db.bucketName)
		if err := b.Put([]byte(recordKey), rawJSON); err != nil {
			return err
		}
		return appendToIndex(b, "tasks:"+userID+":"+taskID, recordKey)
	})
	assert.NoError(t, err)
}

// TestLegacyFieldNames checks that old field names ("state", "entry_author_id",
// "entry_author_name") are read back as the correct TaskRecord fields.
func TestLegacyFieldNames(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	userID := "student1"
	taskID := "lab1"
	recordKey := "task:" + userID + ":" + taskID + ":legacy-key-1"

	rawJSON := []byte(fmt.Sprintf(`{
		"id": %q,
		"task_id": %q,
		"student_id": %q,
		"state": "submit",
		"entry_author_id": %q,
		"entry_author_name": "Alice",
		"content": "solution",
		"created_at": "2023-01-01T10:00:00Z"
	}`, recordKey, taskID, userID, userID))

	writeLegacyRecord(t, db, userID, taskID, recordKey, rawJSON)

	records, err := db.ListTaskRecords(userID, taskID)
	assert.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, SubmitRecord, records[0].Type)
	assert.Equal(t, userID, records[0].AuthorID)
	assert.Equal(t, "Alice", records[0].AuthorName)
}

// TestLegacyTypeRevoked checks that "state":"revoked" reads back as RevokeRecord
// from both ListTaskRecords and LatestTaskRecord.
func TestLegacyTypeRevoked(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	userID := "student2"
	taskID := "lab2"
	recordKey := "task:" + userID + ":" + taskID + ":legacy-revoked"

	rawJSON := []byte(fmt.Sprintf(`{
		"id": %q,
		"task_id": %q,
		"student_id": %q,
		"state": "revoked",
		"entry_author_id": %q,
		"entry_author_name": "Bob",
		"content": "solution",
		"created_at": "2023-02-01T10:00:00Z"
	}`, recordKey, taskID, userID, userID))

	writeLegacyRecord(t, db, userID, taskID, recordKey, rawJSON)

	records, err := db.ListTaskRecords(userID, taskID)
	assert.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, RevokeRecord, records[0].Type)

	latest, err := db.LatestTaskRecord(userID, taskID)
	assert.NoError(t, err)
	assert.NotNil(t, latest)
	assert.Equal(t, RevokeRecord, latest.Type)
}

// TestLegacyTypeReview checks that "state":"review" reads back as ReviewedRecord
// from both ListTaskRecords and LatestTaskRecord.
func TestLegacyTypeReview(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	userID := "student3"
	taskID := "lab3"
	recordKey := "task:" + userID + ":" + taskID + ":legacy-review"

	rawJSON := []byte(fmt.Sprintf(`{
		"id": %q,
		"task_id": %q,
		"student_id": %q,
		"state": "review",
		"entry_author_id": "teacher3",
		"entry_author_name": "Teacher3",
		"content": "looks good",
		"created_at": "2023-03-01T10:00:00Z"
	}`, recordKey, taskID, userID))

	writeLegacyRecord(t, db, userID, taskID, recordKey, rawJSON)

	records, err := db.ListTaskRecords(userID, taskID)
	assert.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, ReviewedRecord, records[0].Type)

	latest, err := db.LatestTaskRecord(userID, taskID)
	assert.NoError(t, err)
	assert.NotNil(t, latest)
	assert.Equal(t, ReviewedRecord, latest.Type)
}

// TestLegacyMissingSubmitAtAndCounter verifies that normalizeLegacyRecords
// correctly fills SubmitAt and Counter for old records that lack these fields.
// A student submit followed by a teacher review should result in:
//   - submit: SubmitAt == CreatedAt, Counter == 1
//   - teacher review: SubmitAt == submit.CreatedAt (inherited), Counter == 2
func TestLegacyMissingSubmitAtAndCounter(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	studentID := "student4"
	teacherID := "teacher4"
	taskID := "lab4"

	submitCreatedAt := time.Date(2023, 4, 1, 10, 0, 0, 0, time.UTC)
	reviewCreatedAt := time.Date(2023, 4, 1, 12, 0, 0, 0, time.UTC)

	submitKey := "task:" + studentID + ":" + taskID + ":legacy-submit"
	reviewKey := "task:" + studentID + ":" + taskID + ":legacy-review"

	submitJSON := []byte(fmt.Sprintf(`{
		"id": %q,
		"task_id": %q,
		"student_id": %q,
		"state": "submit",
		"entry_author_id": %q,
		"entry_author_name": "Student4",
		"content": "solution",
		"created_at": %q
	}`, submitKey, taskID, studentID, studentID, submitCreatedAt.Format(time.RFC3339)))

	reviewJSON := []byte(fmt.Sprintf(`{
		"id": %q,
		"task_id": %q,
		"student_id": %q,
		"state": "review",
		"entry_author_id": %q,
		"entry_author_name": "Teacher4",
		"content": "looks good",
		"created_at": %q
	}`, reviewKey, taskID, studentID, teacherID, reviewCreatedAt.Format(time.RFC3339)))

	writeLegacyRecord(t, db, studentID, taskID, submitKey, submitJSON)
	writeLegacyRecord(t, db, studentID, taskID, reviewKey, reviewJSON)

	// ListTaskRecords returns newest-first: records[0] = review, records[1] = submit.
	records, err := db.ListTaskRecords(studentID, taskID)
	assert.NoError(t, err)
	assert.Len(t, records, 2)

	submitRecord := records[1] // oldest
	reviewRecord := records[0] // newest

	// Submit: SubmitAt filled from its own CreatedAt; Counter = 1.
	assert.Equal(t, SubmitRecord, submitRecord.Type)
	assert.True(t, submitRecord.SubmitAt.Equal(submitCreatedAt),
		"submit SubmitAt should equal its own CreatedAt, got %v want %v",
		submitRecord.SubmitAt, submitCreatedAt)
	assert.Equal(t, 1, submitRecord.Counter)

	// Teacher review: SubmitAt inherited from previous submit's SubmitAt; Counter = 2.
	assert.Equal(t, ReviewedRecord, reviewRecord.Type)
	assert.True(t, reviewRecord.SubmitAt.Equal(submitCreatedAt),
		"teacher review SubmitAt should equal submit CreatedAt, got %v want %v",
		reviewRecord.SubmitAt, submitCreatedAt)
	assert.Equal(t, 2, reviewRecord.Counter)
}

// TestLegacyEnrolledTaskStatusRevoked simulates an old database where the lesson
// JSON contains "status":"revoked" (legacy value) rather than "status":"revoke".
// It verifies that EnrolledTask.UnmarshalJSON normalizes the value on read and
// that RevokedCount() and ListLessonPreviousTaskRecords return correct results.
func TestLegacyEnrolledTaskStatusRevoked(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	teacher := &UserData{ID: "teacher5", Username: "Teacher5", IsTeacher: true}
	assert.NoError(t, db.SaveUser(teacher))
	student := &UserData{ID: "student5", Username: "Student5", IsStudent: true}
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

	taskID := TaskID("lab5")

	// Student submits, registers, then unregisters; this writes "revoke" to disk.
	assert.NoError(t, db.AppendTaskRecord(&TaskRecord{
		TaskID:     taskID,
		StudentID:  student.ID,
		AuthorID:   student.ID,
		AuthorName: student.Username,
		Content:    "my solution",
		CreatedAt:  time.Now(),
		Type:       SubmitRecord,
	}))
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))
	assert.NoError(t, db.UnregisterFromLesson(lessonID, taskID, student.ID))

	// Overwrite the lesson's bucket value: replace "revoke" with "revoked" to
	// simulate the legacy on-disk format.
	err = db.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(db.bucketName)
		data := b.Get([]byte(lessonID))
		if data == nil {
			return fmt.Errorf("lesson %q not found in bucket", lessonID)
		}
		corrupted := bytes.ReplaceAll(data, []byte(`"revoke"`), []byte(`"revoked"`))
		return b.Put([]byte(lessonID), corrupted)
	})
	assert.NoError(t, err)

	// GetLesson must normalize "revoked" -> RevokeRecord via EnrolledTask.UnmarshalJSON.
	updatedLesson, err := db.GetLesson(lessonID)
	assert.NoError(t, err)
	assert.Equal(t, 1, updatedLesson.RevokedCount())

	// ListLessonPreviousTaskRecords must return the record with Type == RevokeRecord.
	prev, err := db.ListLessonPreviousTaskRecords(updatedLesson)
	assert.NoError(t, err)
	assert.Len(t, prev, 1)
	assert.Equal(t, RevokeRecord, prev[0].Type)
}

// TestLegacyMutableModelSingleRecord simulates the old mutable model where a
// student's submit was mutated in-place to "reviewed". A single record with
// "state":"reviewed" and student authorship should:
//   - Read back as ReviewedRecord with SubmitAt == CreatedAt.
//   - Block RegisterToLesson (ReviewedRecord is not a valid registration state).
//   - Allow RegisterToLesson after a new SubmitRecord is appended.
func TestLegacyMutableModelSingleRecord(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	teacher := &UserData{ID: "teacher6", Username: "Teacher6", IsTeacher: true}
	assert.NoError(t, db.SaveUser(teacher))
	student := &UserData{ID: "student6", Username: "Student6", IsStudent: true}
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

	taskID := TaskID("lab6")
	createdAt := time.Date(2023, 6, 1, 10, 0, 0, 0, time.UTC)
	recordKey := "task:" + student.ID + ":" + taskID + ":legacy-mutable"

	// Simulate old mutable model: single record with "state":"reviewed",
	// student-authored (student_id == entry_author_id).
	rawJSON := []byte(fmt.Sprintf(`{
		"id": %q,
		"task_id": %q,
		"student_id": %q,
		"state": "reviewed",
		"entry_author_id": %q,
		"entry_author_name": %q,
		"content": "my solution",
		"created_at": %q
	}`, recordKey, taskID, student.ID, student.ID, student.Username, createdAt.Format(time.RFC3339)))

	writeLegacyRecord(t, db, student.ID, taskID, recordKey, rawJSON)

	// LatestTaskRecord should return ReviewedRecord.
	latest, err := db.LatestTaskRecord(student.ID, taskID)
	assert.NoError(t, err)
	assert.NotNil(t, latest)
	assert.Equal(t, ReviewedRecord, latest.Type)

	// Student-authored reviewed: SubmitAt should equal CreatedAt.
	assert.True(t, latest.SubmitAt.Equal(createdAt),
		"student-authored reviewed: SubmitAt should equal CreatedAt, got %v want %v",
		latest.SubmitAt, createdAt)

	// RegisterToLesson should fail: ReviewedRecord is not a valid registration state.
	err = db.RegisterToLesson(lessonID, taskID, student.ID)
	assert.Error(t, err, "RegisterToLesson should fail when latest record is ReviewedRecord")

	// After appending a new SubmitRecord, RegisterToLesson should succeed.
	assert.NoError(t, db.AppendTaskRecord(&TaskRecord{
		TaskID:     taskID,
		StudentID:  student.ID,
		AuthorID:   student.ID,
		AuthorName: student.Username,
		Content:    "updated solution",
		CreatedAt:  time.Now(),
		Type:       SubmitRecord,
	}))
	assert.NoError(t, db.RegisterToLesson(lessonID, taskID, student.ID))
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
