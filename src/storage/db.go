package storage

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	DefaultBucketName = "slap"
)

type DB struct {
	db         *bolt.DB
	bucketName []byte
}

func NewDB(dbPath string, bucketName string) (*DB, error) {
	if bucketName == "" {
		bucketName = DefaultBucketName
	}

	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("could not create directory for database: %w", err)
		}
	}

	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("could not open db: %w", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return fmt.Errorf("could not create bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not initialize bucket: %w", err)
	}

	return &DB{
		db:         db,
		bucketName: []byte(bucketName),
	}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

// GetAllTaskRecordsForUser returns all task records for a user, organized by
// task ID. Records within each task are sorted newest-first. Legacy records are
// normalized via normalizeLegacyRecords.
func (d *DB) GetAllTaskRecordsForUser(userID string) (map[TaskID][]TaskRecord, error) {
	rawByTask := make(map[TaskID][]TaskRecord)

	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)
		if b == nil {
			return nil
		}

		prefix := []byte(fmt.Sprintf("task:%s:", userID))
		cursor := b.Cursor()

		for k, _ := cursor.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = cursor.Next() {
			r, err := readRecordRaw(b, string(k))
			if err != nil {
				log.Printf("Warning: failed to read task record for key %s: %v", k, err)
				continue
			}
			rawByTask[r.TaskID] = append(rawByTask[r.TaskID], *r)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result := make(map[TaskID][]TaskRecord, len(rawByTask))
	for taskID, records := range rawByTask {
		normalized := normalizeLegacyRecords(records) // returns oldest-first
		SortTaskRecordsNewestFirst(normalized)
		result[taskID] = normalized
	}

	return result, nil
}
