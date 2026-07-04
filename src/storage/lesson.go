package storage

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"
)

type LessonID = string

type Lesson struct {
	ID                    UserID         `json:"id"`
	TeacherID             UserID         `json:"teacher_id"`
	DateTime              time.Time      `json:"datetime"`
	EnrolledTasks         []EnrolledTask `json:"submissions"`
	PreviousEnrolledTasks []EnrolledTask `json:"previous_submissions"`

	TeacherName          string     `json:"teacher_name"`
	Description          string     `json:"description"`
	RegistrationDeadline *time.Time `json:"registration_deadline,omitempty"`
	Capacity             int        `json:"capacity,omitempty"`
}

// IsRegistrationOpen returns true if students can still register.
// If RegistrationDeadline is set, it uses that; otherwise registration is
// open until the lesson starts.
func (l Lesson) IsRegistrationOpen() bool {
	if l.RegistrationDeadline != nil {
		return time.Now().Before(*l.RegistrationDeadline)
	}
	return l.DateTime.After(time.Now())
}

type EnrolledTask struct {
	TaskRecordID string         `json:"journal_entry_id"`
	StudentID    string         `json:"user_id"`
	TaskID       string         `json:"task_id"`
	Excerpt      string         `json:"description"`
	Status       TaskRecordType `json:"status"`
	SubmitAt     time.Time      `json:"submit_at"`
}

func (l *Lesson) RegisteredCount() int {
	count := 0
	for _, t := range l.EnrolledTasks {
		if t.Status == RegisterRecord || t.Status == ReviewedRecord {
			count++
		}
	}
	return count
}

func (l *Lesson) ReviewedCount() int {
	count := 0
	for _, t := range l.EnrolledTasks {
		if t.Status == ReviewedRecord {
			count++
		}
	}
	return count
}

func (l *Lesson) RevokedCount() int {
	count := 0
	for _, t := range l.PreviousEnrolledTasks {
		if t.Status == RevokeRecord {
			count++
		}
	}
	return count
}

const lessonsKey = "lessons"

func sortLessonsByDate(entries []*Lesson) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].DateTime.After(entries[j].DateTime)
	})
}

func (d *DB) ListLessons() ([]*Lesson, error) {
	var lessons []*Lesson
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)

		index, err := getIndex(b, lessonsKey)
		if err != nil {
			return err
		}

		for _, key := range index {
			data := b.Get([]byte(key))
			if data == nil {
				return fmt.Errorf("could not find lesson data for key %s", key)
			}
			var lesson Lesson
			if err := json.Unmarshal(data, &lesson); err != nil {
				return fmt.Errorf("could not unmarshal lesson data: %w", err)
			}
			lessons = append(lessons, &lesson)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	sortLessonsByDate(lessons)
	return lessons, nil
}

func (d *DB) GetLesson(lessonID LessonID) (*Lesson, error) {
	if lessonID == "" {
		return nil, fmt.Errorf("lesson ID cannot be empty")
	}

	var result Lesson
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)
		lesson, err := getValue[Lesson](b, lessonID)
		if err != nil {
			return err
		}
		result = *lesson
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (d *DB) AddLesson(lesson *Lesson) error {
	if lesson.TeacherID == "" || lesson.TeacherName == "" || lesson.DateTime.IsZero() {
		return fmt.Errorf("incorrect lesson")
	}

	lesson.ID = "lesson:" + lesson.TeacherID + ":" + uuid.New().String()
	lesson.EnrolledTasks = []EnrolledTask{}

	teacherLessonsKey := "teacher:" + lesson.TeacherID + ":lessons"

	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)

		if err := appendToIndex(b, teacherLessonsKey, lesson.ID); err != nil {
			return err
		}
		if err := appendToIndex(b, lessonsKey, lesson.ID); err != nil {
			return err
		}

		buf, err := json.Marshal(lesson)
		if err != nil {
			return fmt.Errorf("could not marshal lesson data: %w", err)
		}
		return b.Put([]byte(lesson.ID), buf)
	})
}

func (d *DB) DeleteLesson(lessonID LessonID, teacherID UserID) error {
	if lessonID == "" {
		return fmt.Errorf("lesson ID cannot be empty")
	}

	teacherLessonsKey := "teacher:" + teacherID + ":lessons"

	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)

		lesson, err := getValue[Lesson](b, lessonID)
		if err != nil {
			return err
		}

		// Append a RevokeRecord for each currently-registered enrollment so
		// students can re-register to another lesson without resubmitting.
		for _, enrolled := range lesson.EnrolledTasks {
			if enrolled.Status != RegisterRecord {
				continue
			}
			regRecord, err := readRecordRaw(b, enrolled.TaskRecordID)
			if err != nil {
				continue // best-effort; skip on error
			}
			revokeRecord := TaskRecord{
				TaskID:     enrolled.TaskID,
				StudentID:  enrolled.StudentID,
				AuthorID:   enrolled.StudentID,
				AuthorName: regRecord.AuthorName,
				Type:       RevokeRecord,
				LessonID:   lessonID,
				CreatedAt:  time.Now(),
				SubmitAt:   regRecord.SubmitAt,
				Counter:    regRecord.Counter + 1,
				Content:    regRecord.Content,
			}
			_ = appendRecordInTx(b, &revokeRecord) // best-effort
		}

		if err := removeFromIndex(b, lessonsKey, lessonID); err != nil {
			return err
		}
		if err := removeFromIndex(b, teacherLessonsKey, lessonID); err != nil {
			return err
		}
		return b.Delete([]byte(lessonID))
	})
}

func (d *DB) UpdateLessonDescription(lessonID LessonID, description string) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)
		lesson, err := getValue[Lesson](b, lessonID)
		if err != nil {
			return err
		}
		lesson.Description = description
		return setValue(b, lessonID, *lesson)
	})
}

func (d *DB) SetLessonDeadline(lessonID LessonID, deadline time.Time) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)
		lesson, err := getValue[Lesson](b, lessonID)
		if err != nil {
			return err
		}
		lesson.RegistrationDeadline = &deadline
		return setValue(b, lessonID, *lesson)
	})
}

func (d *DB) ListLessonTaskRecords(lesson *Lesson) ([]*TaskRecord, error) {
	var result []*TaskRecord
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)
		for _, enrolledTask := range lesson.EnrolledTasks {
			taskRecord, err := readRecordRaw(b, enrolledTask.TaskRecordID)
			if err != nil {
				return err
			}
			// In the immutable model the record at TaskRecordID is a RegisterRecord.
			// Use EnrolledTask.Status as the effective status for lesson display so
			// handlers can detect reviewed/revoked states correctly.
			taskRecord.Type = enrolledTask.Status
			if taskRecord.SubmitAt.IsZero() {
				taskRecord.SubmitAt = enrolledTask.SubmitAt
			}
			result = append(result, taskRecord)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	SortTaskRecordsOldestFirst(result)
	return result, nil
}

func (d *DB) ListLessonPreviousTaskRecords(lesson *Lesson) ([]*TaskRecord, error) {
	var result []*TaskRecord
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)
		for _, enrolledTask := range lesson.PreviousEnrolledTasks {
			taskRecord, err := readRecordRaw(b, enrolledTask.TaskRecordID)
			if err != nil {
				return err
			}
			// Use EnrolledTask.Status as the effective status so handlers can
			// distinguish reviewed from revoked previous enrollments.
			taskRecord.Type = enrolledTask.Status
			if taskRecord.SubmitAt.IsZero() {
				taskRecord.SubmitAt = enrolledTask.SubmitAt
			}
			result = append(result, taskRecord)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	SortTaskRecordsOldestFirst(result)
	return result, nil
}
