package storage

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	bolt "go.etcd.io/bbolt"
)

type TaskID = string
type TaskRecordID = string

type TaskRecordStatus string

const (
	SubmitTaskRecord   TaskRecordStatus = "submit"
	RegisterTaskRecord TaskRecordStatus = "register"
	RevokedTaskRecord  TaskRecordStatus = "revoked"
	ReviewTaskRecord   TaskRecordStatus = "review"
	ReviewedTaskRecord TaskRecordStatus = "reviewed"
)

type TaskRecord struct {
	ID              TaskRecordID     `json:"id"`
	TaskID          TaskID           `json:"task_id"`
	StudentID       UserID           `json:"student_id"`
	EntryAuthorID   UserID           `json:"entry_author_id"`
	Content         string           `json:"content"`
	Status          TaskRecordStatus `json:"state"`
	CreatedAt       time.Time        `json:"created_at"`
	EntryAuthorName string           `json:"entry_author_name"`
	LessonAt        time.Time        `json:"lesson_at"`
	LessonID        LessonID         `json:"lesson_id"`
}

func (r *TaskRecord) RenderAt() string {
	return r.CreatedAt.Format("2006-01-02 15:04:05")
}

func SortTaskRecordsOldestFirst(records []*TaskRecord) {
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})
}

func SortTaskRecordsNewestFirst(records []TaskRecord) {
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
}

func (d *DB) AddTaskRecord(record *TaskRecord) error {
	if record.TaskID == "" || record.StudentID == "" || record.EntryAuthorName == "" || record.Content == "" || record.Status == "" {
		return fmt.Errorf("task record validation error")
	}

	indexKey := "tasks:" + record.StudentID + ":" + record.TaskID
	newRecordKey := "task:" + record.StudentID + ":" + record.TaskID + ":" + uuid.New().String()
	record.ID = newRecordKey

	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)

		taskRecordKeys, err := getIndex(b, indexKey)
		if err != nil {
			return err
		}

		for _, key := range taskRecordKeys {
			existingRecord, err := getValue[TaskRecord](b, key)
			if err != nil {
				return err
			}

			if existingRecord.Status == RegisterTaskRecord {
				if record.Status == ReviewTaskRecord {
					existingRecord.Status = ReviewedTaskRecord
				} else {
					existingRecord.Status = RevokedTaskRecord
				}
				if err := setValue(b, key, existingRecord); err != nil {
					return err
				}
				// Sync status back to the EnrolledTask in the lesson
				if existingRecord.LessonID != "" {
					if lesson, err := getValue[Lesson](b, existingRecord.LessonID); err == nil {
						for i, enrolled := range lesson.EnrolledTasks {
							if enrolled.TaskRecordID == existingRecord.ID {
								lesson.EnrolledTasks[i].Status = existingRecord.Status
								_ = setValue(b, existingRecord.LessonID, *lesson)
								break
							}
						}
					}
				}
			}

		}

		err = appendToIndex(b, indexKey, newRecordKey)
		if err != nil {
			return err
		}
		return setValue(b, newRecordKey, record)
	})
}

func (d *DB) ListTaskRecords(userID string, taskID TaskID) ([]TaskRecord, error) {
	if userID == "" || taskID == "" {
		return nil, fmt.Errorf("user ID and task ID cannot be empty")
	}

	var result []TaskRecord
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)

		taskRecordKeys, err := getIndex(b, "tasks:"+userID+":"+taskID)
		if err != nil {
			return err
		}

		taskRecords := make([]TaskRecord, len(taskRecordKeys))
		for i, key := range taskRecordKeys {
			taskRecord, err := getValue[TaskRecord](b, key)
			if err != nil {
				return err
			}
			taskRecords[i] = *taskRecord
		}
		result = taskRecords
		return nil
	})

	if err != nil {
		return nil, err
	}

	SortTaskRecordsNewestFirst(result)
	return result, nil
}

// LatestTaskStatus returns the status of the most recently added record for a
// given user/task pair, or an empty string if no records exist.
func (d *DB) LatestTaskStatus(userID string, taskID TaskID) (TaskRecordStatus, error) {
	if userID == "" || taskID == "" {
		return "", fmt.Errorf("user ID and task ID cannot be empty")
	}

	var status TaskRecordStatus
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)
		keys, err := getIndex(b, "tasks:"+userID+":"+taskID)
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			return nil
		}
		record, err := getValue[TaskRecord](b, keys[len(keys)-1])
		if err != nil {
			return err
		}
		status = record.Status
		return nil
	})
	return status, err
}

func (d *DB) RegisterToLesson(lessonID LessonID, taskID TaskID, authorID UserID) error {
	if lessonID == "" || authorID == "" || taskID == "" {
		return fmt.Errorf("taskID and lessonID should provided")
	}

	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)

		keys, err := getIndex(b, "tasks:"+authorID+":"+taskID)
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			return fmt.Errorf("no task entries found for author %s and task %s", authorID, taskID)
		}

		lesson, err := getValue[Lesson](b, lessonID)
		if err != nil {
			return err
		}

		if !lesson.IsRegistrationOpen() {
			return fmt.Errorf("registration is closed")
		}

		// Check for existing enrollment for same task+author
		existingIdx := -1
		for i, enrolled := range lesson.EnrolledTasks {
			if enrolled.TaskID == taskID && enrolled.AuthorID == authorID {
				existingIdx = i
				break
			}
		}
		if existingIdx >= 0 {
			// Duplicate: same record already registered
			if lesson.EnrolledTasks[existingIdx].TaskRecordID == keys[len(keys)-1] {
				return fmt.Errorf("already registered")
			}
			// Move old enrollment to history before replacing
			lesson.PreviousEnrolledTasks = append(lesson.PreviousEnrolledTasks, lesson.EnrolledTasks[existingIdx])
			lesson.EnrolledTasks = append(lesson.EnrolledTasks[:existingIdx], lesson.EnrolledTasks[existingIdx+1:]...)
		}

		lastTaskRecord, err := getValue[TaskRecord](b, keys[len(keys)-1])
		if err != nil {
			return err
		}
		if lastTaskRecord.Status != SubmitTaskRecord {
			return fmt.Errorf("unexpected task state for registration on lesson: %s", lastTaskRecord.Status)
		}
		lastTaskRecord.Status = RegisterTaskRecord

		lesson.EnrolledTasks = append(lesson.EnrolledTasks, EnrolledTask{
			TaskID:         taskID,
			AuthorID:       authorID,
			TaskRecordID:   lastTaskRecord.ID,
			TaskRecordDesc: lastTaskRecord.Content,
			Status:         RegisterTaskRecord,
		})
		if err := setValue(b, lessonID, *lesson); err != nil {
			return err
		}

		lastTaskRecord.LessonAt = lesson.DateTime
		lastTaskRecord.LessonID = lessonID
		return setValue(b, lastTaskRecord.ID, lastTaskRecord)
	})
}

func (d *DB) UnregisterAllFromLesson(lessonID LessonID) (int, error) {
	if lessonID == "" {
		return 0, fmt.Errorf("lessonID must be provided")
	}

	var count int
	err := d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)

		lesson, err := getValue[Lesson](b, lessonID)
		if err != nil {
			return err
		}

		var remaining []EnrolledTask
		for _, enrolled := range lesson.EnrolledTasks {
			if enrolled.Status != RegisterTaskRecord {
				remaining = append(remaining, enrolled)
				continue
			}

			taskRecord, err := getValue[TaskRecord](b, enrolled.TaskRecordID)
			if err != nil {
				return err
			}
			taskRecord.Status = RevokedTaskRecord
			if err := setValue(b, taskRecord.ID, *taskRecord); err != nil {
				return err
			}

			enrolled.Status = RevokedTaskRecord
			lesson.PreviousEnrolledTasks = append(lesson.PreviousEnrolledTasks, enrolled)
			count++
		}
		lesson.EnrolledTasks = remaining
		return setValue(b, lessonID, *lesson)
	})
	return count, err
}

func (d *DB) UnregisterFromLesson(lessonID LessonID, taskID TaskID, authorID UserID) error {
	if lessonID == "" || taskID == "" || authorID == "" {
		return fmt.Errorf("lessonID, taskID, and authorID must be provided")
	}

	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)

		lesson, err := getValue[Lesson](b, lessonID)
		if err != nil {
			return err
		}

		existingIdx := -1
		for i, enrolled := range lesson.EnrolledTasks {
			if enrolled.TaskID == taskID && enrolled.AuthorID == authorID {
				existingIdx = i
				break
			}
		}
		if existingIdx < 0 {
			return fmt.Errorf("not registered")
		}

		taskRecord, err := getValue[TaskRecord](b, lesson.EnrolledTasks[existingIdx].TaskRecordID)
		if err != nil {
			return err
		}
		if taskRecord.Status != RegisterTaskRecord {
			return fmt.Errorf("task record is not in register state")
		}

		taskRecord.Status = RevokedTaskRecord
		if err := setValue(b, taskRecord.ID, *taskRecord); err != nil {
			return err
		}

		enrolledTask := lesson.EnrolledTasks[existingIdx]
		enrolledTask.Status = RevokedTaskRecord
		lesson.PreviousEnrolledTasks = append(lesson.PreviousEnrolledTasks, enrolledTask)
		lesson.EnrolledTasks = append(lesson.EnrolledTasks[:existingIdx], lesson.EnrolledTasks[existingIdx+1:]...)
		return setValue(b, lessonID, *lesson)
	})
}
