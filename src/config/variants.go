package config

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ryukzak/slap/src/storage"
)

// Variants maps a student ID to that student's assigned variant per task.
type Variants map[storage.UserID]map[storage.TaskID]string

// loadVariants reads a `;`-separated CSV whose header row is `id;<task-id>;...`
// and whose other rows are `<student-id>;<variant>;...`. Empty cells mean no
// variant for that task. Every task column must name a configured task.
func loadVariants(path string, taskExists map[storage.TaskID]bool) (Variants, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening variants file: %w", err)
	}
	defer func() { _ = f.Close() }()
	return parseVariants(f, taskExists)
}

func parseVariants(r io.Reader, taskExists map[storage.TaskID]bool) (Variants, error) {
	reader := csv.NewReader(r)
	reader.Comma = ';'
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err == io.EOF {
		return nil, fmt.Errorf("variants file is empty")
	}
	if err != nil {
		return nil, fmt.Errorf("error parsing variants file: %w", err)
	}
	if len(header) < 2 {
		return nil, fmt.Errorf("variants file header must be `id;<task-id>;...`")
	}

	taskIDs := make([]storage.TaskID, len(header)-1)
	for i, col := range header[1:] {
		taskID := strings.TrimSpace(col)
		if !taskExists[taskID] {
			return nil, fmt.Errorf("variants file references non-existent task: %s", taskID)
		}
		taskIDs[i] = taskID
	}

	variants := Variants{}
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error parsing variants file: %w", err)
		}
		studentID := strings.TrimSpace(row[0])
		if studentID == "" {
			continue
		}
		if _, dup := variants[studentID]; dup {
			return nil, fmt.Errorf("variants file lists student %s more than once", studentID)
		}
		if len(row)-1 > len(taskIDs) {
			return nil, fmt.Errorf("variants file row for student %s has more cells than the header", studentID)
		}
		perTask := map[storage.TaskID]string{}
		for i, cell := range row[1:] {
			if v := strings.TrimSpace(cell); v != "" {
				perTask[taskIDs[i]] = v
			}
		}
		variants[studentID] = perTask
	}
	return variants, nil
}

// GetVariant returns the student's assigned variant for the task, or "" when
// none is assigned.
func (c *Config) GetVariant(studentID storage.UserID, taskID storage.TaskID) string {
	return c.Variants[studentID][taskID]
}
