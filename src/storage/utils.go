package storage

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

func getIndex(b *bolt.Bucket, indexKey string) ([]string, error) {
	data := b.Get([]byte(indexKey))
	if data == nil {
		return []string{}, nil
	}

	var result []string
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("could not unmarshal task entries: %w", err)
	}
	return result, nil
}

func appendToIndex(b *bolt.Bucket, indexKey, newKey string) error {
	index, err := getIndex(b, indexKey)
	if err != nil {
		return fmt.Errorf("could not collect old index: %w", err)
	}
	index = append(index, newKey)

	buf, err := json.Marshal(index)
	if err != nil {
		return fmt.Errorf("could not marshal index data: %w", err)
	}

	if err := b.Put([]byte(indexKey), buf); err != nil {
		return fmt.Errorf("could not write error: %w", err)
	}
	return nil
}

func removeFromIndex(b *bolt.Bucket, indexKey, removeKey string) error {
	index, err := getIndex(b, indexKey)
	if err != nil {
		return fmt.Errorf("could not collect old index: %w", err)
	}
	filtered := index[:0]
	for _, k := range index {
		if k != removeKey {
			filtered = append(filtered, k)
		}
	}
	buf, err := json.Marshal(filtered)
	if err != nil {
		return fmt.Errorf("could not marshal index data: %w", err)
	}
	if err := b.Put([]byte(indexKey), buf); err != nil {
		return fmt.Errorf("could not write index: %w", err)
	}
	return nil
}

func getValue[T any](b *bolt.Bucket, key string) (*T, error) {
	data := b.Get([]byte(key))
	if data == nil {
		return nil, fmt.Errorf("not found: %s", key)
	}

	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("could not unmarshal: %w", err)
	}
	return &result, nil
}

func setValue[T any](b *bolt.Bucket, key string, data T) error {
	dataBuf, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("could not marshal data: %w", err)
	}
	if err := b.Put([]byte(key), dataBuf); err != nil {
		return fmt.Errorf("could not write data: %w", err)
	}
	return nil
}
