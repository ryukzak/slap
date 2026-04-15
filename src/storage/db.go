package storage

import (
	"fmt"
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

// GetCompletedTasksCountInGroup returns number of reviewed tasks in a group for a user
func (d *DB) GetCompletedTasksCountInGroup(userID string, taskIDs []TaskID) (int, error) {
	completed := 0
	for _, taskID := range taskIDs {
		records, err := d.ListTaskRecords(userID, TaskID(taskID))
		if err != nil {
			continue
		}

		for _, record := range records {
			if record.Status == ReviewedTaskRecord {
				completed++
				break
			}
		}
	}
	return completed, nil
}
