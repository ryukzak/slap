package handlers

import (
	"fmt"
	"time"

	"github.com/ryukzak/slap/src/config"
	"github.com/ryukzak/slap/src/storage"
	"github.com/ryukzak/slap/src/util"
)

// taskStatusLabel maps a stored record status to the label shown in the UI.
func taskStatusLabel(s storage.TaskRecordStatus) string {
	switch s {
	case storage.SubmitTaskRecord:
		return "Pending"
	case storage.RegisterTaskRecord:
		return "Queued"
	case storage.RevokedTaskRecord:
		return "Dropped"
	case storage.ReviewTaskRecord:
		return "Feedback"
	case storage.ReviewedTaskRecord:
		return "Checked"
	default:
		return string(s)
	}
}

func latestCheckedInfo(records []storage.TaskRecord) (*time.Time, string) {
	if len(records) == 0 {
		return nil, "not submitted"
	}

	for i := range records {
		if records[i].Status == storage.ReviewedTaskRecord {
			return &records[i].CreatedAt, "Checked"
		}
	}

	// No accepted record. If a teacher left a scored review, treat the work as
	// checked at the time of the student's latest submission.
	scored := false
	for i := range records {
		r := records[i]
		if r.EntryAuthorID != r.StudentID && util.ExtractScore(r.Content) != "" {
			scored = true
			break
		}
	}
	if scored {
		for i := range records {
			if records[i].EntryAuthorID == records[i].StudentID {
				return &records[i].CreatedAt, "scored without lesson (submission " + taskStatusLabel(records[i].Status) + ")"
			}
		}
		return nil, "scored, but no student submission found"
	}

	return nil, "not checked (" + taskStatusLabel(records[0].Status) + ")"
}

type Evaluation struct {
	Rule        config.ScoreRule
	Applies     bool
	IsActive    bool
	CountBefore int
	// Debug captures the per-task and per-rule details of this evaluation so
	// the score-rules debug endpoint can show why a rule did or did not apply.
	// It is populated on the real evaluation path, so it always reflects the
	// actual behaviour rather than a re-implementation.
	Debug EvaluationDebug
}

// TaskCheckDebug records, for one task referenced by a rule, the timestamp the
// evaluator compared against the rule's deadline and whether that task passed
// the rule's per-task predicate.
type TaskCheckDebug struct {
	TaskID    string     `json:"task_id"`
	CheckedAt *time.Time `json:"checked_at"`      // nil = task was never accepted (Checked)
	State     string     `json:"state,omitempty"` // record state the checked time came from (or why none)
	Pass      bool       `json:"pass"`            // satisfies this condition's per-task predicate
}

// EvaluationDebug is a self-describing trace of a single rule evaluation.
type EvaluationDebug struct {
	Rule          string           `json:"rule"`
	ConditionKind string           `json:"condition_kind"` // min_checked_before | after | before | interval | none
	Now           time.Time        `json:"now"`
	CheckedAfter  *time.Time       `json:"checked_after,omitempty"`
	CheckedBefore *time.Time       `json:"checked_before,omitempty"`
	MinRequired   int              `json:"min_required,omitempty"`
	Tasks         []TaskCheckDebug `json:"tasks"`
	Count         int              `json:"count"` // number of tasks that passed the per-task predicate
	IsActive      bool             `json:"is_active"`
	Applies       bool             `json:"applies"`
	Status        string           `json:"status"`
	Effect        int              `json:"effect"`         // the rule's configured effect
	AppliedEffect int              `json:"applied_effect"` // effect actually contributed (0 unless Applies)
}

type Evaluator struct {
	config *config.Config
}

func NewEvaluator(cfg *config.Config) *Evaluator {
	return &Evaluator{config: cfg}
}

// EvaluateForStudent evaluates a rule at a specific point in time.
//
// The branch order below is significant and preserved as-is: a rule with both
// checked_after and checked_before but no min is handled by the "before" branch,
// so the "interval" branch is only reachable in theory. The debug trace reports
// whichever branch actually ran, so it stays faithful to real behaviour.
func (e *Evaluator) EvaluateForStudent(rule config.ScoreRule, now time.Time, getCheckedTime func(taskID storage.TaskID) (*time.Time, error)) (Evaluation, error) {
	eval := Evaluation{
		Rule:    rule,
		Applies: false,
	}
	dbg := EvaluationDebug{
		Rule:          rule.Name,
		Now:           now,
		CheckedAfter:  rule.Condition.CheckedAfter,
		CheckedBefore: rule.Condition.CheckedBefore,
		MinRequired:   rule.Condition.MinCheckedBefore,
		Effect:        rule.Effect,
	}

	// Collect checked times in rule order so the debug trace is stable.
	tasks := make([]TaskCheckDebug, 0, len(rule.TaskIDs))
	for _, taskID := range rule.TaskIDs {
		t, err := getCheckedTime(taskID)
		if err != nil {
			return Evaluation{}, fmt.Errorf("failed to get checked time for task %s: %w", taskID, err)
		}
		tasks = append(tasks, TaskCheckDebug{TaskID: taskID, CheckedAt: t})
	}

	switch {
	// Min checked before
	case rule.Condition.MinCheckedBefore > 0:
		dbg.ConditionKind = "min_checked_before"
		countBefore := 0
		for i := range tasks {
			pass := tasks[i].CheckedAt != nil && tasks[i].CheckedAt.Before(*rule.Condition.CheckedBefore)
			tasks[i].Pass = pass
			if pass {
				countBefore++
			}
		}
		eval.CountBefore = countBefore

		if countBefore < rule.Condition.MinCheckedBefore {
			eval.IsActive = !now.After(*rule.Condition.CheckedBefore)
			eval.Applies = !eval.IsActive
		}

	// After
	case rule.Condition.CheckedAfter != nil && rule.Condition.CheckedBefore == nil:
		dbg.ConditionKind = "after"
		allCheckedBefore := true
		for i := range tasks {
			onTime := tasks[i].CheckedAt != nil && tasks[i].CheckedAt.Before(*rule.Condition.CheckedAfter)
			tasks[i].Pass = onTime
			if !onTime {
				allCheckedBefore = false
			}
		}
		eval.IsActive = !now.After(*rule.Condition.CheckedAfter)
		eval.Applies = !eval.IsActive && !allCheckedBefore

	// Before
	case rule.Condition.CheckedBefore != nil && rule.Condition.MinCheckedBefore == 0:
		dbg.ConditionKind = "before"
		hasCheckedBefore := false
		for i := range tasks {
			pass := tasks[i].CheckedAt != nil && tasks[i].CheckedAt.Before(*rule.Condition.CheckedBefore)
			tasks[i].Pass = pass
			if pass {
				hasCheckedBefore = true
			}
		}
		eval.IsActive = !now.After(*rule.Condition.CheckedBefore)
		eval.Applies = hasCheckedBefore

	// Interval
	case rule.Condition.CheckedAfter != nil && rule.Condition.CheckedBefore != nil:
		dbg.ConditionKind = "interval"
		for i := range tasks {
			t := tasks[i].CheckedAt
			pass := t != nil && t.After(*rule.Condition.CheckedAfter) && t.Before(*rule.Condition.CheckedBefore)
			tasks[i].Pass = pass
			if pass {
				eval.Applies = true
			}
		}

	default:
		dbg.ConditionKind = "none"
	}

	for _, tc := range tasks {
		if tc.Pass {
			dbg.Count++
		}
	}
	dbg.Tasks = tasks
	dbg.IsActive = eval.IsActive
	dbg.Applies = eval.Applies
	dbg.Status = eval.Status()
	if eval.Applies {
		dbg.AppliedEffect = rule.Effect
	}
	eval.Debug = dbg

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
