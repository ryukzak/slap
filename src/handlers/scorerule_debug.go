package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/ryukzak/slap/src/storage"
)

// ScoreRulesDebugResponse is the full machine-readable trace returned by the
// score-rules debug endpoint.
type ScoreRulesDebugResponse struct {
	UserID      string            `json:"user_id"`
	Username    string            `json:"username"`
	Now         time.Time         `json:"now"`
	Timezone    string            `json:"timezone"`
	TotalEffect int               `json:"total_effect"`
	Rules       []EvaluationDebug `json:"rules"`
}

// ScoreRulesDebugHandler exposes the per-task, per-rule reasoning behind a
// student's score-rule evaluation so the actual checked time, the expected
// deadline, the pass/fail of every task, the counted number and the rule result
// can be inspected. It returns JSON by default, or plain text with ?format=text.
//
// Access mirrors the profile page: a student may inspect only their own
// evaluation; teachers may inspect anyone.
func ScoreRulesDebugHandler(w http.ResponseWriter, r *http.Request) {
	sessionUser := userSession(w, r)
	if sessionUser == nil {
		return
	}

	profileUserID := mux.Vars(r)["userID"]
	if profileUserID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}
	if profileUserID != sessionUser.ID && !sessionUser.IsTeacher {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	dbUser, err := DB.GetUser(profileUserID)
	if err != nil || dbUser == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Same checked-time source as the profile page (src/handlers/user.go): the
	// CreatedAt of the newest accepted (Checked) record. The state label (and
	// read) is cached so each task is loaded only once.
	stateCache := map[storage.TaskID]string{}
	getCheckedTime := func(taskID storage.TaskID) (*time.Time, error) {
		records, err := DB.ListTaskRecords(profileUserID, taskID)
		if err != nil {
			return nil, err
		}
		at, state := debugCheckedInfo(records)
		stateCache[taskID] = state
		return at, nil
	}

	evaluator := NewEvaluator(AppConfig)
	now := time.Now()

	resp := ScoreRulesDebugResponse{
		UserID:   dbUser.ID,
		Username: dbUser.Username,
		Now:      now,
		Timezone: PrimaryTZName,
	}

	for _, rule := range AppConfig.ScoreRules {
		eval, err := evaluator.EvaluateForStudent(rule, now, getCheckedTime)
		if err != nil {
			log.Printf("Error evaluating rule %s for user %s: %v", rule.Name, profileUserID, err)
			http.Error(w, "Failed to evaluate score rules", http.StatusInternalServerError)
			return
		}
		for i := range eval.Debug.Tasks {
			eval.Debug.Tasks[i].State = stateCache[eval.Debug.Tasks[i].TaskID]
		}
		resp.Rules = append(resp.Rules, eval.Debug)
		resp.TotalEffect += eval.Debug.AppliedEffect
	}

	if r.URL.Query().Get("format") == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if _, err := io.WriteString(w, renderScoreDebugText(resp)); err != nil {
			log.Printf("Error writing score debug text for user %s: %v", profileUserID, err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resp); err != nil {
		log.Printf("Error encoding score debug for user %s: %v", profileUserID, err)
	}
}

// debugCheckedInfo returns the checked time (CreatedAt of the newest accepted
// record, matching the profile page) plus a label describing the record state
// it came from, or why none exists. records must be newest-first.
func debugCheckedInfo(records []storage.TaskRecord) (*time.Time, string) {
	if len(records) == 0 {
		return nil, "not submitted"
	}
	for i := range records {
		if records[i].Status == storage.ReviewedTaskRecord {
			return &records[i].CreatedAt, "Checked"
		}
	}
	return nil, "not checked (" + taskStatusLabel(records[0].Status) + ")"
}

// fmtDebugTime renders a timestamp in the primary timezone, or "<never>" for a
// task that was never accepted.
func fmtDebugTime(t *time.Time) string {
	if t == nil {
		return "<never accepted>"
	}
	loc := PrimaryLoc
	if loc == nil {
		loc = time.UTC
	}
	return t.In(loc).Format("2006-01-02 15:04:05 MST")
}

func renderScoreDebugText(resp ScoreRulesDebugResponse) string {
	var b strings.Builder
	fmt.Fprintf(&b, "score-rule debug for @%s (id %s)\n", resp.Username, resp.UserID)
	fmt.Fprintf(&b, "now: %s  (timezone: %s)\n", fmtDebugTime(&resp.Now), resp.Timezone)
	fmt.Fprintf(&b, "checked time = submission time (CreatedAt) of the newest accepted (Checked) record\n")
	b.WriteString(strings.Repeat("=", 72) + "\n\n")

	for _, rule := range resp.Rules {
		fmt.Fprintf(&b, "Rule: %s  [effect %+d]\n", rule.Rule, rule.Effect)
		fmt.Fprintf(&b, "  condition: %s", rule.ConditionKind)
		if rule.MinRequired > 0 {
			fmt.Fprintf(&b, "  (min required: %d)", rule.MinRequired)
		}
		b.WriteString("\n")
		if rule.CheckedAfter != nil {
			fmt.Fprintf(&b, "  checked_after:  %s\n", fmtDebugTime(rule.CheckedAfter))
		}
		if rule.CheckedBefore != nil {
			fmt.Fprintf(&b, "  checked_before: %s\n", fmtDebugTime(rule.CheckedBefore))
		}

		b.WriteString("  tasks:\n")
		for _, tc := range rule.Tasks {
			mark := "FAIL"
			if tc.Pass {
				mark = "PASS"
			}
			fmt.Fprintf(&b, "    [%s] %-24s state=%-40s actual=%s  expected=%s\n",
				mark, tc.TaskID, tc.State, fmtDebugTime(tc.CheckedAt), expectedDesc(rule, tc))
		}

		fmt.Fprintf(&b, "  count (passed): %d", rule.Count)
		if rule.MinRequired > 0 {
			fmt.Fprintf(&b, " / required %d", rule.MinRequired)
		}
		b.WriteString("\n")
		switch {
		case rule.Applies:
			fmt.Fprintf(&b, "  => APPLIED: %+d point(s)\n\n", rule.AppliedEffect)
		case rule.IsActive:
			b.WriteString("  => not applied yet (deadline still open)\n\n")
		default:
			b.WriteString("  => not applied\n\n")
		}
	}

	fmt.Fprintf(&b, "%s\nTOTAL applied effect: %+d point(s)\n", strings.Repeat("=", 72), resp.TotalEffect)
	return b.String()
}

// expectedDesc describes the deadline a task is compared against for the rule's
// condition, so each task line is self-explanatory.
func expectedDesc(rule EvaluationDebug, tc TaskCheckDebug) string {
	switch rule.ConditionKind {
	case "min_checked_before", "before":
		return "before " + fmtDebugTime(rule.CheckedBefore)
	case "after":
		return "before " + fmtDebugTime(rule.CheckedAfter) + " (else late)"
	case "interval":
		return "between " + fmtDebugTime(rule.CheckedAfter) + " and " + fmtDebugTime(rule.CheckedBefore)
	default:
		return "n/a"
	}
}
