package handlers

import (
	"testing"
	"time"

	"github.com/ryukzak/slap/src/config"
	"github.com/ryukzak/slap/src/storage"
)

// These tests pin the observable behaviour of EvaluateForStudent (Applies /
// IsActive / Status) for every condition kind, so the evaluation core can be
// refactored without changing what students see.
func TestEvaluateForStudent(t *testing.T) {
	d := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	beforeD := d.Add(-time.Hour)
	afterD := d.Add(time.Hour)
	nowBefore := d.Add(-2 * time.Hour)
	nowAfter := d.Add(2 * time.Hour)

	tp := func(t time.Time) *time.Time { return &t }

	tests := []struct {
		name       string
		rule       config.ScoreRule
		now        time.Time
		checked    map[storage.TaskID]*time.Time
		wantApply  bool
		wantActive bool
		wantStatus string
	}{
		{
			name: "min_checked_before: enough checked -> not applied",
			rule: config.ScoreRule{TaskIDs: []storage.TaskID{"a", "b", "c"}, Effect: -1,
				Condition: config.Condition{MinCheckedBefore: 2, CheckedBefore: &d}},
			now:        nowAfter,
			checked:    map[storage.TaskID]*time.Time{"a": tp(beforeD), "b": tp(beforeD)},
			wantApply:  false,
			wantActive: false,
			wantStatus: "not_applied",
		},
		{
			name: "min_checked_before: too few and deadline passed -> applied",
			rule: config.ScoreRule{TaskIDs: []storage.TaskID{"a", "b", "c"}, Effect: -1,
				Condition: config.Condition{MinCheckedBefore: 2, CheckedBefore: &d}},
			now:        nowAfter,
			checked:    map[storage.TaskID]*time.Time{"a": tp(beforeD)},
			wantApply:  true,
			wantActive: false,
			wantStatus: "applied",
		},
		{
			name: "min_checked_before: too few but deadline open -> active",
			rule: config.ScoreRule{TaskIDs: []storage.TaskID{"a", "b", "c"}, Effect: -1,
				Condition: config.Condition{MinCheckedBefore: 2, CheckedBefore: &d}},
			now:        nowBefore,
			checked:    map[storage.TaskID]*time.Time{"a": tp(beforeD)},
			wantApply:  false,
			wantActive: true,
			wantStatus: "active",
		},
		{
			name: "after: all on time -> not applied",
			rule: config.ScoreRule{TaskIDs: []storage.TaskID{"a", "b"}, Effect: -2,
				Condition: config.Condition{CheckedAfter: &d}},
			now:        nowAfter,
			checked:    map[storage.TaskID]*time.Time{"a": tp(beforeD), "b": tp(beforeD)},
			wantApply:  false,
			wantActive: false,
			wantStatus: "not_applied",
		},
		{
			name: "after: one late and deadline passed -> applied",
			rule: config.ScoreRule{TaskIDs: []storage.TaskID{"a", "b"}, Effect: -2,
				Condition: config.Condition{CheckedAfter: &d}},
			now:        nowAfter,
			checked:    map[storage.TaskID]*time.Time{"a": tp(beforeD), "b": tp(afterD)},
			wantApply:  true,
			wantActive: false,
			wantStatus: "applied",
		},
		{
			name: "after: deadline still open -> active",
			rule: config.ScoreRule{TaskIDs: []storage.TaskID{"a"}, Effect: -2,
				Condition: config.Condition{CheckedAfter: &d}},
			now:        nowBefore,
			checked:    map[storage.TaskID]*time.Time{"a": tp(afterD)},
			wantApply:  false,
			wantActive: true,
			wantStatus: "active",
		},
		{
			name: "before (bonus): checked in time -> applied immediately",
			rule: config.ScoreRule{TaskIDs: []storage.TaskID{"a"}, Effect: 1,
				Condition: config.Condition{CheckedBefore: &d}},
			now:        nowBefore,
			checked:    map[storage.TaskID]*time.Time{"a": tp(beforeD)},
			wantApply:  true,
			wantActive: true,
			wantStatus: "applied",
		},
		{
			name: "before (bonus): not checked in time -> not applied",
			rule: config.ScoreRule{TaskIDs: []storage.TaskID{"a"}, Effect: 1,
				Condition: config.Condition{CheckedBefore: &d}},
			now:        nowAfter,
			checked:    map[storage.TaskID]*time.Time{"a": tp(afterD)},
			wantApply:  false,
			wantActive: false,
			wantStatus: "not_applied",
		},
		{
			// Documents CURRENT behaviour: a rule with both bounds and no min is
			// handled by the "before" branch (the "interval" branch is shadowed),
			// so a task checked before the lower bound still counts.
			name: "both bounds, no min: currently treated as before",
			rule: config.ScoreRule{TaskIDs: []storage.TaskID{"a"}, Effect: -1,
				Condition: config.Condition{CheckedAfter: &d, CheckedBefore: tp(d.Add(10 * time.Hour))}},
			now:        nowAfter,
			checked:    map[storage.TaskID]*time.Time{"a": tp(beforeD)}, // before the lower bound
			wantApply:  true,
			wantActive: true, // IsActive comes from the far-future CheckedBefore in the "before" branch
			wantStatus: "applied",
		},
	}

	evaluator := NewEvaluator(&config.Config{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			get := func(id storage.TaskID) (*time.Time, error) { return tt.checked[id], nil }
			ev, err := evaluator.EvaluateForStudent(tt.rule, tt.now, get)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ev.Applies != tt.wantApply {
				t.Errorf("Applies = %v, want %v", ev.Applies, tt.wantApply)
			}
			if ev.IsActive != tt.wantActive {
				t.Errorf("IsActive = %v, want %v", ev.IsActive, tt.wantActive)
			}
			if got := ev.Status(); got != tt.wantStatus {
				t.Errorf("Status = %q, want %q", got, tt.wantStatus)
			}
		})
	}
}
