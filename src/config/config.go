package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ryukzak/slap/src/storage"
	"gopkg.in/yaml.v3"
)

const DefaultWaitingPeriod = 24 * time.Hour

// Config represents the application configuration
type Config struct {
	Tasks                    []Task   `yaml:"tasks"`
	TeacherIDs               []string `yaml:"teacher_ids"`
	TitleMaxLen              int      `yaml:"title_max_len"`
	DefaultLessonDescription string   `yaml:"default_lesson_description"`
}

// IsTeacher checks if the given user ID is in the teacher list
func (c *Config) IsTeacher(userID string) bool {
	for _, id := range c.TeacherIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// Task represents a task in the system
type Task struct {
	ID            storage.TaskID `yaml:"id"`
	Title         string         `yaml:"title"`
	Description   string         `yaml:"description"`
	WaitingPeriod *time.Duration `yaml:"waiting_period,omitempty"`
}

// GetWaitingPeriod returns the task's waiting period, defaulting to 24h.
func (t *Task) GetWaitingPeriod() time.Duration {
	if t.WaitingPeriod != nil {
		return *t.WaitingPeriod
	}
	return DefaultWaitingPeriod
}

// LoadConfig loads the configuration from the specified YAML file
func LoadConfig(filePath string) (*Config, error) {
	if filePath == "" {
		return nil, fmt.Errorf("configuration file path cannot be empty")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file does not exist: %s", filePath)
	}

	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading configuration file: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing configuration file: %w", err)
	}

	// Validate tasks
	for i, task := range config.Tasks {
		if task.ID == "" {
			return nil, fmt.Errorf("task at index %d has an empty ID", i)
		}
		if task.Title == "" {
			return nil, fmt.Errorf("task at index %d has an empty title", i)
		}
	}

	return &config, nil
}

// DefaultConfig returns a default configuration for development purposes
func DefaultConfig() *Config {
	return &Config{
		Tasks: []Task{
			{
				ID:          "task1",
				Title:       "Basic Data Structures",
				Description: "Implement stack, queue, and linked list with comprehensive tests.",
			},
			{
				ID:          "task2",
				Title:       "Advanced Algorithms",
				Description: "Sorting and searching algorithm implementations with complexity analysis.",
			},
			{
				ID:          "task3",
				Title:       "Final Project",
				Description: "Implement a custom data structure solving a real-world problem.",
			},
		},
	}
}

func (c *Config) GetTask(taskID storage.TaskID) *Task {
	for _, t := range c.Tasks {
		if t.ID == taskID {
			return &t
		}
	}
	return nil
}
