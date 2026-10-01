// Package langbench is the language-breadth program's instrument (ADR-048,
// docs/design/language-breadth.md §1, §10, §12, §15): the queue, the
// reading schema, the level computation and the generated tracker. It never
// types a level: every level is computed from metrics, thresholds and
// oracle tiers.
package langbench

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// QueueEntry is one item of docs/plans/language-breadth.queue.json (§15).
type QueueEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"` // language | tooling
	// Tier is the excellence program's target-set tier, T1..T6: an entry
	// without one is outside the target set. Membership is this data alone.
	Tier                string   `json:"tier,omitempty"`
	TargetLevel         string   `json:"target_level"`
	GrammarSource       string   `json:"grammar_source"`
	Licence             string   `json:"licence"`
	Oracle              []string `json:"oracle"`
	CalibrationSubjects []string `json:"calibration_subjects"`
	DependsOn           []string `json:"depends_on"`
	Wave                int      `json:"wave"`
	Engine              string   `json:"engine"`
	PlanRow             string   `json:"plan_row"`
	WorkerModel         string   `json:"worker_model"`
	PortOf              string   `json:"port_of,omitempty"`
	Notes               string   `json:"notes,omitempty"`
	Retargeted          string   `json:"retargeted,omitempty"`
}

// QueueIDPattern is the id grammar every queue entry and dialect directory
// obeys (§2.2, §15).
var QueueIDPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

var (
	validKinds   = []string{"language", "tooling"}
	validLevels  = []string{"L1", "L2", "L3", "L4"}
	validEngines = []string{"tags", "doc", "native", "tags+doc"}
	// programTiers is the closed set of excellence-program tiers; reading.go's
	// validTiers is a different thing (oracle tiers A-D).
	programTiers = []string{"T1", "T2", "T3", "T4", "T5", "T6"}
)

// LoadQueue reads and validates the queue file at path.
func LoadQueue(path string) ([]QueueEntry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	q, err := ParseQueue(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return q, nil
}

// ParseQueue decodes a queue (a top-level JSON array, unknown fields
// refused) and checks each entry's own shape: unique [a-z0-9-]+ ids,
// plan_row = LB-Q-<id>, a known kind/engine/target, no L4 claim for a tool
// format (§1.1), and no entry depending on its own row.
func ParseQueue(b []byte) ([]QueueEntry, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var q []QueueEntry
	if err := dec.Decode(&q); err != nil {
		return nil, fmt.Errorf("queue: %w", err)
	}
	seen := map[string]bool{}
	for i, e := range q {
		where := fmt.Sprintf("queue[%d] %q", i, e.ID)
		switch {
		case !QueueIDPattern.MatchString(e.ID):
			return nil, fmt.Errorf("%s: id must match %s", where, QueueIDPattern)
		case seen[e.ID]:
			return nil, fmt.Errorf("%s: duplicate id", where)
		case e.PlanRow != "LB-Q-"+e.ID:
			return nil, fmt.Errorf("%s: plan_row %q, want LB-Q-%s", where, e.PlanRow, e.ID)
		case !slices.Contains(validKinds, e.Kind):
			return nil, fmt.Errorf("%s: kind %q not in %v", where, e.Kind, validKinds)
		case !slices.Contains(validLevels, e.TargetLevel):
			return nil, fmt.Errorf("%s: target_level %q not in %v", where, e.TargetLevel, validLevels)
		case !slices.Contains(validEngines, e.Engine):
			return nil, fmt.Errorf("%s: engine %q not in %v", where, e.Engine, validEngines)
		case e.Tier != "" && !slices.Contains(programTiers, e.Tier):
			return nil, fmt.Errorf("%s: tier %q not in %v", where, e.Tier, programTiers)
		case e.Kind == "tooling" && e.TargetLevel == "L4":
			return nil, fmt.Errorf("%s: L4 is not-applicable for tool formats (§1.1)", where)
		case slices.Contains(e.DependsOn, e.PlanRow):
			return nil, fmt.Errorf("%s: depends on its own row", where)
		}
		seen[e.ID] = true
	}
	return q, nil
}

// isQueueRow reports whether a plan row id is a queue row proper — an
// LB-Q-<id> row, not a §14.2 "-rest" disposition sibling.
func isQueueRow(id string, queueIDs map[string]bool) bool {
	if !strings.HasPrefix(id, "LB-Q-") {
		return false
	}
	rest := strings.TrimPrefix(id, "LB-Q-")
	if queueIDs[rest] {
		return true
	}
	return !strings.HasSuffix(rest, "-rest")
}

// CheckQueueAgainstPlan returns every disagreement between the queue and
// the parsed plan (sorted by discovery order, deterministic): a missing or
// extra LB-Q row, a row out of queue order, a row whose needs differ from
// the entry's depends_on, and a depends_on naming no plan row.
func CheckQueueAgainstPlan(q []QueueEntry, plan verify.Plan) []string {
	var errs []string
	queueIDs := map[string]bool{}
	for _, e := range q {
		queueIDs[e.ID] = true
	}
	rows := map[string]verify.Task{}
	var planOrder []string
	for _, t := range plan.Tasks {
		rows[t.ID] = t
		if isQueueRow(t.ID, queueIDs) {
			planOrder = append(planOrder, t.ID)
		}
	}
	var queueOrder []string
	for _, e := range q {
		queueOrder = append(queueOrder, e.PlanRow)
		row, ok := rows[e.PlanRow]
		if !ok {
			errs = append(errs, fmt.Sprintf("queue entry %q has no plan row %s", e.ID, e.PlanRow))
			continue
		}
		want := append([]string(nil), e.DependsOn...)
		got := append([]string(nil), row.Needs...)
		sort.Strings(want)
		sort.Strings(got)
		if !slices.Equal(want, got) {
			errs = append(errs, fmt.Sprintf("%s: plan needs %v, queue depends_on %v", e.PlanRow, got, want))
		}
		for _, d := range e.DependsOn {
			if _, ok := rows[d]; !ok {
				errs = append(errs, fmt.Sprintf("queue entry %q depends_on %s, which is no plan row", e.ID, d))
			}
		}
	}
	for _, id := range planOrder {
		if !queueIDs[strings.TrimPrefix(id, "LB-Q-")] {
			errs = append(errs, fmt.Sprintf("plan row %s has no queue entry", id))
		}
	}
	if !slices.Equal(planOrder, queueOrder) {
		for i := range queueOrder {
			if i >= len(planOrder) || planOrder[i] != queueOrder[i] {
				got := "<none>"
				if i < len(planOrder) {
					got = planOrder[i]
				}
				errs = append(errs, fmt.Sprintf("order: queue position %d is %s, plan has %s", i, queueOrder[i], got))
				break
			}
		}
	}
	return errs
}
