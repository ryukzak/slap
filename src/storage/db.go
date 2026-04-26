package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// GetAllTaskRecordsForUser returns all task records for a user, organized by task ID
func (d *DB) GetAllTaskRecordsForUser(userID string) (map[TaskID][]TaskRecord, error) {
	result := make(map[TaskID][]TaskRecord)

	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)
		if b == nil {
			return nil
		}

		// Find all records with prefix "task:{userID}:"
		prefix := []byte(fmt.Sprintf("task:%s:", userID))
		cursor := b.Cursor()

		for k, v := cursor.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = cursor.Next() {
			var record TaskRecord
			if err := json.Unmarshal(v, &record); err != nil {
				continue
			}
			result[record.TaskID] = append(result[record.TaskID], record)
		}
		return nil
	})

	// Sort each task's records by CreatedAt (newest first)
	for taskID, records := range result {
		sort.Slice(records, func(i, j int) bool {
			return records[i].CreatedAt.After(records[j].CreatedAt)
		})
		result[taskID] = records
	}

	return result, err
}
