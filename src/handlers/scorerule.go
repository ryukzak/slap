package handlers

import (
	"time"

	"github.com/ryukzak/slap/src/config"
	"github.com/ryukzak/slap/src/storage"
)

type Evaluation struct {
	Rule        config.ScoreRule
	Applies     bool
	IsActive    bool
	CountBefore int
}

type Evaluator struct {
	config *config.Config
}

func NewEvaluator(cfg *config.Config) *Evaluator {
	return &Evaluator{config: cfg}
}

// EvaluateForStudent evaluates a rule at a specific point in time
func (e *Evaluator) EvaluateForStudent(rule config.ScoreRule, now time.Time, getCheckedTime func(taskID storage.TaskID) (*time.Time, error)) (Evaluation, error) {
	eval := Evaluation{
		Rule:    rule,
		Applies: false,
	}

	// Collect checked times
	checkedTimes := make(map[storage.TaskID]*time.Time)
	for _, taskID := range rule.TaskIDs {
		t, err := getCheckedTime(taskID)
		if err == nil {
			checkedTimes[taskID] = t
		}
	}

	// Min checked before
	if rule.Condition.MinCheckedBefore > 0 {
		countBefore := 0
		for _, t := range checkedTimes {
			if t != nil && t.Before(*rule.Condition.CheckedBefore) {
				countBefore++
			}
		}
		eval.CountBefore = countBefore

		if countBefore >= rule.Condition.MinCheckedBefore {
			return eval, nil
		}

		eval.IsActive = !now.After(*rule.Condition.CheckedBefore)
		eval.Applies = !eval.IsActive
		return eval, nil
	}

	// After
	if rule.Condition.CheckedAfter != nil && rule.Condition.CheckedBefore == nil {
		allCheckedBefore := true
		for _, t := range checkedTimes {
			if t == nil || !t.Before(*rule.Condition.CheckedAfter) {
				allCheckedBefore = false
				break
			}
		}
		eval.IsActive = !now.After(*rule.Condition.CheckedAfter)
		eval.Applies = !eval.IsActive && !allCheckedBefore
		return eval, nil
	}

	// Before
	if rule.Condition.CheckedBefore != nil && rule.Condition.MinCheckedBefore == 0 {
		hasCheckedBefore := false
		for _, t := range checkedTimes {
			if t != nil && t.Before(*rule.Condition.CheckedBefore) {
				hasCheckedBefore = true
				break
			}
		}
		eval.IsActive = !now.After(*rule.Condition.CheckedBefore)
		eval.Applies = hasCheckedBefore
		return eval, nil
	}

	// Interval
	if rule.Condition.CheckedAfter != nil && rule.Condition.CheckedBefore != nil {
		for _, t := range checkedTimes {
			if t != nil && t.After(*rule.Condition.CheckedAfter) && t.Before(*rule.Condition.CheckedBefore) {
				eval.Applies = true
				return eval, nil
			}
		}
		return eval, nil
	}

	return eval, nil
}

// Status returns "applied", "active", or "not_applied"
func (e Evaluation) Status() string {
	if e.Applies {
		return "applied"
	}
	if e.IsActive {
		return "active"
	}
	return "not_applied"
}

// Color returns color for status and effect
func (e Evaluation) Color() string {
	if e.Applies {
		if e.Rule.Effect < 0 {
			return "red"
		}
		return "green"
	}
	if e.IsActive {
		return "yellow"
	}
	return "gray"
}
