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

	TeacherName string `json:"teacher_name"`
	Description string `json:"description"`
}

type EnrolledTask struct {
	TaskRecordID TaskRecordID     `json:"journal_entry_id"`
	Status       TaskRecordStatus `json:"status"`

	AuthorID       UserID `json:"user_id"`
	TaskID         TaskID `json:"task_id"`
	TaskRecordDesc string `json:"description"`
}

func (l *Lesson) RegisteredCount() int {
	count := 0
	for _, t := range l.EnrolledTasks {
		if t.Status == RegisterTaskRecord || t.Status == ReviewedTaskRecord {
			count++
		}
	}
	return count
}

func (l *Lesson) ReviewedCount() int {
	count := 0
	for _, t := range l.EnrolledTasks {
		if t.Status == ReviewedTaskRecord {
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

		// Reset registered task records back to submit state
		for _, enrolled := range lesson.EnrolledTasks {
			if enrolled.Status == RegisterTaskRecord {
				taskRecord, err := getValue[TaskRecord](b, enrolled.TaskRecordID)
				if err != nil {
					return err
				}
				taskRecord.Status = SubmitTaskRecord
				taskRecord.LessonAt = time.Time{}
				taskRecord.LessonID = ""
				if err := setValue(b, taskRecord.ID, *taskRecord); err != nil {
					return err
				}
			}
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

func (d *DB) ListLessonTaskRecords(lesson *Lesson) ([]*TaskRecord, error) {
	var result []*TaskRecord
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)
		for _, enrolledTask := range lesson.EnrolledTasks {
			taskRecordID := enrolledTask.TaskRecordID
			taskRecord, err := getValue[TaskRecord](b, taskRecordID)
			if err != nil {
				return err
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
			taskRecord, err := getValue[TaskRecord](b, enrolledTask.TaskRecordID)
			if err != nil {
				return err
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
