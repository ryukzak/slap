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
	Tasks                    []Task       `yaml:"tasks"`
	TasksGroups              []TasksGroup `yaml:"tasks_groups"`
	TeacherIDs               []string     `yaml:"teacher_ids"`
	TitleMaxLen              int          `yaml:"title_max_len"`
	DefaultLessonDescription string       `yaml:"default_lesson_description"`
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

// Deadline defines a single deadline with penalty
type Deadline struct {
	Date    time.Time `yaml:"date"`
	Penalty int       `yaml:"penalty"`
}

// Task represents a task in the system
type Task struct {
	ID            storage.TaskID `yaml:"id"`
	Title         string         `yaml:"title"`
	Description   string         `yaml:"description"`
	WaitingPeriod *time.Duration `yaml:"waiting_period,omitempty"`
	Deadlines     []Deadline     `yaml:"deadlines,omitempty"`
}

// TasksGroup represents a group of tasks with collective deadlines
type TasksGroup struct {
	GroupID   string           `yaml:"group_id"`
	TasksIDs  []storage.TaskID `yaml:"tasks_ids"`
	Deadlines []Deadline       `yaml:"deadlines"`
}

// GetWaitingPeriod returns the task's waiting period, defaulting to 24h.
func (t *Task) GetWaitingPeriod() time.Duration {
	if t.WaitingPeriod != nil {
		return *t.WaitingPeriod
	}
	return DefaultWaitingPeriod
}

// CalculatePenalty calculates penalty for a submission based on individual deadlines
func (t *Task) CalculatePenalty(submissionTime time.Time) int {
	if len(t.Deadlines) == 0 {
		return 0
	}

	// Get first missed deadline
	for _, deadline := range t.Deadlines {
		if submissionTime.After(deadline.Date) {
			return deadline.Penalty
		}
	}
	return 0
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

		// Validate deadlines are in chronological order
		for j := 1; j < len(task.Deadlines); j++ {
			if task.Deadlines[j].Date.Before(task.Deadlines[j-1].Date) {
				return nil, fmt.Errorf("task %s: deadlines must be in chronological order", task.ID)
			}
		}
	}

	// Validate task groups
	for i, group := range config.TasksGroups {
		if group.GroupID == "" {
			return nil, fmt.Errorf("task group at index %d has an empty group_id", i)
		}
		if len(group.TasksIDs) == 0 {
			return nil, fmt.Errorf("task group %s has no tasks", group.GroupID)
		}

		// Check all tasks_ids are exist
		taskExists := make(map[storage.TaskID]bool)
		for _, task := range config.Tasks {
			taskExists[task.ID] = true
		}

		for _, taskID := range group.TasksIDs {
			if !taskExists[taskID] {
				return nil, fmt.Errorf("task group %s references non-existent task: %s", group.GroupID, taskID)
			}
		}

		// Validate deadlines are in chronological order
		for j := 1; j < len(group.Deadlines); j++ {
			if group.Deadlines[j].Date.Before(group.Deadlines[j-1].Date) {
				return nil, fmt.Errorf("task group %s: deadlines must be in chronological order", group.GroupID)
			}
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
		TasksGroups: []TasksGroup{},
	}
}

// GetTask returns task by ID
func (c *Config) GetTask(taskID storage.TaskID) *Task {
	for i := range c.Tasks {
		if c.Tasks[i].ID == taskID {
			return &c.Tasks[i]
		}
	}
	return nil
}

// GetTaskGroup returns task group by ID
func (c *Config) GetTaskGroup(groupID string) *TasksGroup {
	for i := range c.TasksGroups {
		if c.TasksGroups[i].GroupID == groupID {
			return &c.TasksGroups[i]
		}
	}
	return nil
}

// GetTaskGroupForTask returns the group that contains the given task
func (c *Config) GetTaskGroupForTask(taskID storage.TaskID) *TasksGroup {
	for i := range c.TasksGroups {
		for _, id := range c.TasksGroups[i].TasksIDs {
			if id == taskID {
				return &c.TasksGroups[i]
			}
		}
	}
	return nil
}

// CalculateGroupPenalty calculates penalty based on group deadlines
func (g *TasksGroup) CalculateGroupPenalty(completedCount int, submissionTime time.Time) int {
	if len(g.Deadlines) == 0 {
		return 0
	}

	// Apply penalty to first missed deadline
	for _, deadline := range g.Deadlines {
		if submissionTime.After(deadline.Date) {
			return deadline.Penalty
		}
	}
	return 0
}
