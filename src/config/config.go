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
	Tasks                    []Task      `yaml:"tasks"`
	TeacherIDs               []string    `yaml:"teacher_ids"`
	TitleMaxLen              int         `yaml:"title_max_len"`
	DefaultLessonDescription string      `yaml:"default_lesson_description"`
	ScoreRules               []ScoreRule `yaml:"score_rules"`
}

// ScoreRule defines a rule that adds effect to student's total score
type ScoreRule struct {
	Name      string           `yaml:"name"`
	TaskIDs   []storage.TaskID `yaml:"task_ids"`
	Condition Condition        `yaml:"condition"`
	Effect    int              `yaml:"effect"`
}

// Condition defines when the rule applies
type Condition struct {
	CheckedAfter     *time.Time `yaml:"checked_after,omitempty"`
	CheckedBefore    *time.Time `yaml:"checked_before,omitempty"`
	MinCheckedBefore int        `yaml:"min_checked_before,omitempty"`
}

// Task represents a task in the system
type Task struct {
	ID          storage.TaskID `yaml:"id"`
	Title       string         `yaml:"title"`
	Description string         `yaml:"description"`
	// WaitingPeriodHours is the minimum number of hours that must pass since the
	// last teacher review before a student may re-register the task for a lesson.
	// When unset, DefaultWaitingPeriod applies. A value of 0 disables the check.
	WaitingPeriodHours *int `yaml:"waiting_period_hours,omitempty"`
}

func (c *Config) IsTeacher(userID string) bool {
	for _, id := range c.TeacherIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// GetWaitingPeriod returns the task's waiting period, defaulting to 24h.
// A configured value of 0 hours disables the check (no threshold).
func (t *Task) GetWaitingPeriod() time.Duration {
	if t.WaitingPeriodHours != nil {
		return time.Duration(*t.WaitingPeriodHours) * time.Hour
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
		if task.WaitingPeriodHours != nil && *task.WaitingPeriodHours < 0 {
			return nil, fmt.Errorf("task %s has a negative waiting_period_hours (use 0 to disable)", task.ID)
		}
	}

	// Check all tasks are exist
	taskExists := make(map[storage.TaskID]bool)
	for _, task := range config.Tasks {
		taskExists[task.ID] = true
	}

	// Validate score rules
	for i, rule := range config.ScoreRules {
		if rule.Name == "" {
			return nil, fmt.Errorf("score rule at index %d has an empty name", i)
		}
		if len(rule.TaskIDs) == 0 {
			return nil, fmt.Errorf("score rule %s has no task_ids", rule.Name)
		}

		hasMin := rule.Condition.MinCheckedBefore > 0
		hasAfter := rule.Condition.CheckedAfter != nil
		hasBefore := rule.Condition.CheckedBefore != nil

		// Count conditions
		condCount := 0
		if hasMin {
			condCount++
		}
		if hasAfter {
			condCount++
		}
		if hasBefore {
			condCount++
		}

		// No conditions
		if condCount == 0 {
			return nil, fmt.Errorf("score rule %s: at least one condition is required", rule.Name)
		}

		// More than 2 conditions
		if condCount > 2 {
			return nil, fmt.Errorf("score rule %s: too many conditions (max 2)", rule.Name)
		}

		// Determine allowed combinations
		switch condCount {
		case 1:
			// allowed single conditions: after, before, min+before (min requires before)
			allowed := (hasAfter && !hasBefore && !hasMin) ||
				(hasBefore && !hasAfter && !hasMin) ||
				(hasMin && hasBefore && !hasAfter)
			if !allowed {
				return nil, fmt.Errorf("score rule %s: invalid single condition (allowed: after, before, min+before)", rule.Name)
			}
		case 2:
			// allowed two-condition combinations: after+before, min+before
			allowed := (hasAfter && hasBefore && !hasMin) ||
				(hasMin && hasBefore && !hasAfter)
			if !allowed {
				return nil, fmt.Errorf("score rule %s: invalid two-condition combination (allowed: after+before, min+before)", rule.Name)
			}
		}

		// Interval validity
		if hasAfter && hasBefore {
			if rule.Condition.CheckedAfter.After(*rule.Condition.CheckedBefore) {
				return nil, fmt.Errorf("score rule %s: checked_after must be before checked_before", rule.Name)
			}
		}

		// Validate effect non-zero
		if rule.Effect == 0 {
			return nil, fmt.Errorf("score rule %s: effect cannot be zero", rule.Name)
		}

		// Validate all task IDs exist
		for _, taskID := range rule.TaskIDs {
			if !taskExists[taskID] {
				return nil, fmt.Errorf("score rule %s references non-existent task: %s", rule.Name, taskID)
			}
		}
	}

	return &config, nil
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
		ScoreRules: []ScoreRule{},
	}
}
