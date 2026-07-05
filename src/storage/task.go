package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/google/uuid"

	bolt "go.etcd.io/bbolt"
)

type TaskID = string
type TaskRecordID = string

// TaskRecordType is the canonical type of a task record in the event log.
type TaskRecordType string

// Record type constants. These are the canonical values stored in the Type field.
const (
	SubmitRecord   TaskRecordType = "submit"
	RegisterRecord TaskRecordType = "register"
	RevokeRecord   TaskRecordType = "revoke"
	ReviewedRecord TaskRecordType = "reviewed"
)

type TaskRecord struct {
	ID         string         `json:"id"`
	TaskID     string         `json:"task_id"`
	StudentID  string         `json:"student_id"`
	AuthorID   string         `json:"author_id"`
	AuthorName string         `json:"author_name"`
	Type       TaskRecordType `json:"type"`
	Counter    int            `json:"counter"`
	CreatedAt  time.Time      `json:"created_at"`
	SubmitAt   time.Time      `json:"submit_at"`
	Content    string         `json:"content"`
	LessonID   string         `json:"lesson_id"`
}

func (r *TaskRecord) RenderAt() string {
	return r.CreatedAt.Format("2006-01-02 15:04:05")
}

// SortTaskRecordsOldestFirst sorts a slice of TaskRecord pointers oldest-first by CreatedAt.
func SortTaskRecordsOldestFirst(records []*TaskRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})
}

// SortTaskRecordsNewestFirst reverses then stable-sorts a slice of TaskRecord
// values newest-first by CreatedAt.
func SortTaskRecordsNewestFirst(records []TaskRecord) {
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
}

// normalizeType maps legacy on-disk type strings to their canonical names.
// Safe to call on already-canonical values.
func normalizeType(t TaskRecordType) TaskRecordType {
	switch t {
	case "revoked":
		return RevokeRecord
	case "review":
		return ReviewedRecord
	default:
		return t
	}
}

// legacyTaskRecord is used only during unmarshalling to capture both old and
// new field names. Call toTaskRecord() to obtain a TaskRecord.
type legacyTaskRecord struct {
	TaskRecord
	LegacyStatus     string `json:"state"`             // old Status field (JSON tag was "state")
	LegacyAuthorID   string `json:"entry_author_id"`   // old EntryAuthorID
	LegacyAuthorName string `json:"entry_author_name"` // old EntryAuthorName
}

func (l *legacyTaskRecord) toTaskRecord() TaskRecord {
	r := l.TaskRecord
	if r.Type == "" && l.LegacyStatus != "" {
		r.Type = TaskRecordType(l.LegacyStatus) // legacy: convert plain string from old JSON
	}
	if r.AuthorID == "" && l.LegacyAuthorID != "" {
		r.AuthorID = l.LegacyAuthorID
	}
	if r.AuthorName == "" && l.LegacyAuthorName != "" {
		r.AuthorName = l.LegacyAuthorName
	}
	r.Type = normalizeType(r.Type)
	return r
}

// readRecordRaw reads and unmarshals a single record from the bucket, handling
// both legacy and current JSON field names via legacyTaskRecord.
func readRecordRaw(b *bolt.Bucket, key string) (*TaskRecord, error) {
	data := b.Get([]byte(key))
	if data == nil {
		return nil, fmt.Errorf("not found: %s", key)
	}
	var lr legacyTaskRecord
	if err := json.Unmarshal(data, &lr); err != nil {
		return nil, fmt.Errorf("could not unmarshal task record %q: %w", key, err)
	}
	r := lr.toTaskRecord()
	return &r, nil
}

// normalizeLegacyRecords fills in missing SubmitAt and Counter values and maps
// legacy Type strings to canonical names. It sorts records oldest-first and
// returns them in that order. This is a backward-compat shim; it can be
// removed once no databases with legacy records exist.
func normalizeLegacyRecords(records []TaskRecord) []TaskRecord {
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})

	var prevSubmitAt time.Time
	counter := 0

	for i := range records {
		r := &records[i]

		// Step 1: fill SubmitAt for legacy records that lack it.
		if r.SubmitAt.IsZero() {
			isStudentAuthored := r.AuthorID == r.StudentID
			switch r.Type {
			case "submit", "register", "revoked":
				// These legacy types each represent the student's submission or a
				// mutation of it; treat CreatedAt as the round-start time.
				r.SubmitAt = r.CreatedAt
			case "review", "reviewed":
				// "review" is the old on-disk value for a teacher review (now
				// "reviewed"). If the record is student-authored it is the original
				// submit that was mutated to reviewed status by the old mutable model.
				if isStudentAuthored {
					r.SubmitAt = r.CreatedAt
				} else {
					r.SubmitAt = prevSubmitAt
				}
			default:
				r.SubmitAt = prevSubmitAt
			}
		}

		// Step 2: assign Counter sequentially, resetting to 1 when SubmitAt changes.
		if !r.SubmitAt.Equal(prevSubmitAt) {
			counter = 1
		} else {
			counter++
		}
		r.Counter = counter
		prevSubmitAt = r.SubmitAt

		// Step 3: map legacy type strings to canonical names.
		switch r.Type {
		case "revoked":
			r.Type = RevokeRecord
		case "review":
			r.Type = ReviewedRecord
		}
	}

	return records
}

// readAndNormalizeRecords reads all records for the given index keys, applies
// legacy normalization, and returns them oldest-first.
func readAndNormalizeRecords(b *bolt.Bucket, keys []string) ([]TaskRecord, error) {
	records := make([]TaskRecord, 0, len(keys))
	for _, key := range keys {
		r, err := readRecordRaw(b, key)
		if err != nil {
			return nil, err
		}
		records = append(records, *r)
	}
	return normalizeLegacyRecords(records), nil
}

// AppendTaskRecord appends a new immutable record to the event log for a
// student/task pair. Counter and SubmitAt are derived from the latest existing
// record:
//   - Type == SubmitRecord: SubmitAt = time.Now(), Counter = 1
//   - Any other type:       SubmitAt = latest.SubmitAt, Counter = latest.Counter+1
//
// AuthorName, Content, Type, TaskID, and StudentID are all required.
func (d *DB) AppendTaskRecord(record *TaskRecord) error {
	if record.TaskID == "" || record.StudentID == "" || record.AuthorName == "" || record.Content == "" || record.Type == "" {
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

		if record.Type == SubmitRecord {
			record.SubmitAt = time.Now()
			record.Counter = 1
		} else {
			if len(taskRecordKeys) > 0 {
				existing, err := readAndNormalizeRecords(b, taskRecordKeys)
				if err != nil {
					return err
				}
				latest := existing[len(existing)-1]
				record.SubmitAt = latest.SubmitAt
				record.Counter = latest.Counter + 1

				// When a teacher adds a ReviewedRecord while the student has an
				// active lesson registration (a RegisterRecord with a LessonID),
				// update the lesson's enrolled-task status so ReviewedCount() stays
				// accurate without mutating any task record.
				if record.Type == ReviewedRecord &&
					record.AuthorID != record.StudentID &&
					latest.LessonID != "" &&
					latest.Type == RegisterRecord {
					if lesson, err := getValue[Lesson](b, latest.LessonID); err == nil {
						for j, enrolled := range lesson.EnrolledTasks {
							if enrolled.TaskRecordID == latest.ID {
								lesson.EnrolledTasks[j].Status = ReviewedRecord
								_ = setValue(b, latest.LessonID, *lesson)
								break
							}
						}
					}
				}
			} else {
				record.SubmitAt = record.CreatedAt
				record.Counter = 1
			}
		}

		if err := appendToIndex(b, indexKey, newRecordKey); err != nil {
			return err
		}
		return setValue(b, newRecordKey, record)
	})
}

// ListTaskRecords returns all task records for a student/task pair, normalized
// for legacy format and sorted newest-first.
func (d *DB) ListTaskRecords(userID string, taskID TaskID) ([]TaskRecord, error) {
	if userID == "" || taskID == "" {
		return nil, fmt.Errorf("user ID and task ID cannot be empty")
	}

	var result []TaskRecord
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)

		indexKey := "tasks:" + userID + ":" + taskID
		taskRecordKeys, err := getIndex(b, indexKey)
		if err != nil {
			// A corrupt or colliding index (e.g. a struct stored under a
			// colon-containing task ID, see issue #45) must not break the whole
			// read — degrade to "no records" so the profile page renders.
			log.Printf("Warning: unreadable task index %q, treating as empty: %v (raw value: %s)", indexKey, err, b.Get([]byte(indexKey)))
			return nil
		}

		records, err := readAndNormalizeRecords(b, taskRecordKeys)
		if err != nil {
			return err
		}
		result = records
		return nil
	})

	if err != nil {
		return nil, err
	}

	// normalizeLegacyRecords returns oldest-first; reverse to newest-first.
	SortTaskRecordsNewestFirst(result)
	return result, nil
}

// LatestTaskRecord returns the most recently appended record for a student/task
// pair (after normalization), or nil if no records exist.
func (d *DB) LatestTaskRecord(userID string, taskID TaskID) (*TaskRecord, error) {
	if userID == "" || taskID == "" {
		return nil, fmt.Errorf("user ID and task ID cannot be empty")
	}

	records, err := d.ListTaskRecords(userID, taskID)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	r := records[0] // newest-first; index 0 is the latest
	return &r, nil
}

// appendRecordInTx writes a new record and appends its key to the index inside
// an existing write transaction. Callers must set all fields including SubmitAt
// and Counter themselves.
func appendRecordInTx(b *bolt.Bucket, record *TaskRecord) error {
	indexKey := "tasks:" + record.StudentID + ":" + record.TaskID
	newKey := "task:" + record.StudentID + ":" + record.TaskID + ":" + uuid.New().String()
	record.ID = newKey
	if err := appendToIndex(b, indexKey, newKey); err != nil {
		return err
	}
	return setValue(b, newKey, *record)
}

func (d *DB) RegisterToLesson(lessonID LessonID, taskID TaskID, authorID UserID, waitingPeriod ...time.Duration) error {
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

		records, err := readAndNormalizeRecords(b, keys)
		if err != nil {
			return err
		}
		// records is oldest-first; the latest is the last element.
		latest := records[len(records)-1]

		if latest.Type == RegisterRecord {
			return fmt.Errorf("already registered")
		}
		if latest.Type != SubmitRecord && latest.Type != RevokeRecord {
			return fmt.Errorf("unexpected task state for registration on lesson: %s", latest.Type)
		}

		// Check waiting period since last teacher review.
		if len(waitingPeriod) > 0 && waitingPeriod[0] > 0 {
			for i := len(records) - 1; i >= 0; i-- {
				if records[i].Type == ReviewedRecord {
					if time.Since(records[i].CreatedAt) < waitingPeriod[0] {
						remaining := waitingPeriod[0] - time.Since(records[i].CreatedAt)
						hours := int(remaining.Hours())
						minutes := int(remaining.Minutes()) % 60
						return fmt.Errorf("waiting period: %dh%dm remaining since last check", hours, minutes)
					}
					break
				}
			}
		}

		lesson, err := getValue[Lesson](b, lessonID)
		if err != nil {
			return err
		}

		if !lesson.IsRegistrationOpen() {
			return fmt.Errorf("registration is closed")
		}

		// Check for an existing enrollment for the same task+student.
		existingIdx := -1
		for i, enrolled := range lesson.EnrolledTasks {
			if enrolled.TaskID == taskID && enrolled.StudentID == authorID {
				existingIdx = i
				break
			}
		}
		if existingIdx >= 0 {
			// Move old enrollment to history before replacing.
			lesson.PreviousEnrolledTasks = append(lesson.PreviousEnrolledTasks, lesson.EnrolledTasks[existingIdx])
			lesson.EnrolledTasks = append(lesson.EnrolledTasks[:existingIdx], lesson.EnrolledTasks[existingIdx+1:]...)
		}

		// Append a new immutable RegisterRecord.
		newRecord := TaskRecord{
			TaskID:     taskID,
			StudentID:  authorID,
			AuthorID:   authorID,
			AuthorName: latest.AuthorName,
			Type:       RegisterRecord,
			LessonID:   lessonID,
			CreatedAt:  time.Now(),
			SubmitAt:   latest.SubmitAt,
			Counter:    latest.Counter + 1,
			Content:    latest.Content,
		}
		if err := appendRecordInTx(b, &newRecord); err != nil {
			return err
		}

		lesson.EnrolledTasks = append(lesson.EnrolledTasks, EnrolledTask{
			TaskID:       taskID,
			StudentID:    authorID,
			TaskRecordID: newRecord.ID,
			Excerpt:      newRecord.Content,
			Status:       RegisterRecord,
			SubmitAt:     newRecord.SubmitAt,
		})
		return setValue(b, lessonID, *lesson)
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
			if enrolled.Status != RegisterRecord {
				remaining = append(remaining, enrolled)
				continue
			}

			// Read the RegisterRecord to copy content, AuthorName, and SubmitAt.
			regRecord, err := readRecordRaw(b, enrolled.TaskRecordID)
			if err != nil {
				return err
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
			if err := appendRecordInTx(b, &revokeRecord); err != nil {
				return err
			}

			enrolled.Status = RevokeRecord
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
			if enrolled.TaskID == taskID && enrolled.StudentID == authorID {
				existingIdx = i
				break
			}
		}
		if existingIdx < 0 {
			return fmt.Errorf("not registered")
		}

		enrolled := lesson.EnrolledTasks[existingIdx]
		if enrolled.Status != RegisterRecord {
			return fmt.Errorf("task record is not in register state")
		}

		// Read the RegisterRecord to copy content, AuthorName, and SubmitAt.
		regRecord, err := readRecordRaw(b, enrolled.TaskRecordID)
		if err != nil {
			return err
		}

		revokeRecord := TaskRecord{
			TaskID:     taskID,
			StudentID:  authorID,
			AuthorID:   authorID,
			AuthorName: regRecord.AuthorName,
			Type:       RevokeRecord,
			LessonID:   lessonID,
			CreatedAt:  time.Now(),
			SubmitAt:   regRecord.SubmitAt,
			Counter:    regRecord.Counter + 1,
			Content:    regRecord.Content,
		}
		if err := appendRecordInTx(b, &revokeRecord); err != nil {
			return err
		}

		enrolled.Status = RevokeRecord
		lesson.PreviousEnrolledTasks = append(lesson.PreviousEnrolledTasks, enrolled)
		lesson.EnrolledTasks = append(lesson.EnrolledTasks[:existingIdx], lesson.EnrolledTasks[existingIdx+1:]...)
		return setValue(b, lessonID, *lesson)
	})
}
