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
	ID            storage.TaskID `yaml:"id"`
	Title         string         `yaml:"title"`
	Description   string         `yaml:"description"`
	WaitingPeriod *time.Duration `yaml:"waiting_period,omitempty"`
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

	// Validate score rules
	for i, rule := range config.ScoreRules {
		if rule.Name == "" {
			return nil, fmt.Errorf("score rule at index %d has an empty name", i)
		}
		if len(rule.TaskIDs) == 0 {
			return nil, fmt.Errorf("score rule %s has no task_ids", rule.Name)
		}

		// Check all tasks are exist
		taskExists := make(map[storage.TaskID]bool)
		for _, task := range config.Tasks {
			taskExists[task.ID] = true
		}

		for _, taskID := range rule.TaskIDs {
			if !taskExists[taskID] {
				return nil, fmt.Errorf("score rule %s references non-existent task: %s", rule.Name, taskID)
			}
		}
	}

	return &config, nil
}

// CalculateScoreEffects calculates total effect of all rules
func (c *Config) CalculateScoreEffects(getCheckedTime func(taskID storage.TaskID) (*time.Time, error)) (int, error) {
	totalEffect := 0

	for _, rule := range c.ScoreRules {
		applies, err := c.RuleApplies(rule, getCheckedTime)
		if err != nil {
			return 0, fmt.Errorf("error checking rule %s: %w", rule.Name, err)
		}
		if applies {
			totalEffect += rule.Effect
		}
	}

	return totalEffect, nil
}

// RuleApplies checks if a rule applies to a student
func (c *Config) RuleApplies(rule ScoreRule, getCheckedTime func(taskID storage.TaskID) (*time.Time, error)) (bool, error) {
	checkedCount := 0
	var checkedTimes []time.Time

	// Collect checked tasks data
	for _, taskID := range rule.TaskIDs {
		checkedTime, err := getCheckedTime(taskID)
		if err != nil {
			// Error equals task was not checked
			continue
		}
		if checkedTime != nil {
			checkedCount++
			checkedTimes = append(checkedTimes, *checkedTime)
		}
	}

	now := time.Now()

	// Check conditions
	if rule.Condition.CheckedAfter != nil && rule.Condition.CheckedBefore != nil {
		// Interval: checked between dates
		for _, t := range checkedTimes {
			if t.After(*rule.Condition.CheckedAfter) && t.Before(*rule.Condition.CheckedBefore) {
				return true, nil
			}
		}
		return false, nil
	}

	if rule.Condition.CheckedAfter != nil && rule.Condition.CheckedBefore == nil {
		// Checked after date (deadline penalty)
		deadlinePassed := now.After(*rule.Condition.CheckedAfter)
		if !deadlinePassed {
			// Deadline hasn't passed yet
			return false, nil
		}
		// Deadline passed - check if there's any submission before deadline
		for _, t := range checkedTimes {
			if t.Before(*rule.Condition.CheckedAfter) {
				// Submitted on time - no penalty
				return false, nil
			}
		}
		// No submissions or all submissions after deadline - penalty applies
		return true, nil
	}

	if rule.Condition.CheckedBefore != nil {
		if rule.Condition.MinCheckedBefore > 0 {
			// Minimum checked before date (group penalty)
			deadlinePassed := now.After(*rule.Condition.CheckedBefore)
			if !deadlinePassed {
				// Deadline hasn't passed yet
				return false, nil
			}
			// Deadline passed - check if enough tasks were checked before deadline
			countBefore := 0
			for _, t := range checkedTimes {
				if t.Before(*rule.Condition.CheckedBefore) {
					countBefore++
				}
			}
			return countBefore < rule.Condition.MinCheckedBefore, nil
		}

		// Checked before date (early bird bonus)
		for _, t := range checkedTimes {
			if t.Before(*rule.Condition.CheckedBefore) {
				return true, nil
			}
		}
		return false, nil
	}

	return false, nil
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
