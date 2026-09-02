package verify

// crossplan.go — `needs:` across plans (PC-10, docs/plans/proof-credibility.md).
// A row may depend on a row in a sibling plan by writing
// `needs: custodian.md#SD-2`. LintPlan (pure) checks only the shape; the
// CLI resolves the target by reading the sibling file next to the plan and
// reports a missing plan or row as a lint line. The sibling's row STATE is
// authored (claimed / skipped-with-disposition / open) — its verdict is
// not recomputed here, and callers say so when they treat "claimed" as
// satisfied.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CrossPlanNeed is a parsed "<sibling>.md#<id>" needs: target.
type CrossPlanNeed struct {
	Plan string // the sibling plan's file name, relative to the plan's directory
	ID   string // the row id inside it
}

// ParseCrossPlanNeed splits ref into its plan file and row id. ok is false
// for a bare id (same-plan target) or a malformed cross-plan shape: an
// empty side, a plan that is not a .md file, or a plan with a directory
// component (sibling plans live flat under docs/plans/).
func ParseCrossPlanNeed(ref string) (CrossPlanNeed, bool) {
	i := strings.Index(ref, "#")
	if i < 0 {
		return CrossPlanNeed{}, false
	}
	plan, id := strings.TrimSpace(ref[:i]), strings.TrimSpace(ref[i+1:])
	if plan == "" || id == "" || !strings.HasSuffix(plan, ".md") || strings.ContainsAny(plan, `/\`) {
		return CrossPlanNeed{}, false
	}
	return CrossPlanNeed{Plan: plan, ID: id}, true
}

// resolveCrossPlanNeed reads the sibling plan named by ref from planDir and
// returns the row it points at, with the row's authored state. Errors name
// which half failed — the plan file or the row id — so the lint line a
// caller prints is actionable.
func resolveCrossPlanNeed(planDir, ref string) (Task, error) {
	need, ok := ParseCrossPlanNeed(ref)
	if !ok {
		return Task{}, fmt.Errorf("malformed cross-plan target %q (want <sibling>.md#<id>)", ref)
	}
	path := filepath.Join(planDir, need.Plan)
	raw, err := os.ReadFile(path)
	if err != nil {
		return Task{}, fmt.Errorf("sibling plan %s: %w", need.Plan, err)
	}
	sibling := ParseMarkdownPlan("", raw)
	for _, t := range sibling.Tasks {
		if t.ID == need.ID {
			return t, nil
		}
	}
	return Task{}, fmt.Errorf("sibling plan %s has no row %q", need.Plan, need.ID)
}

// ResolveCrossPlanNeed is resolveCrossPlanNeed's exported face for the CLI.
func ResolveCrossPlanNeed(planDir, ref string) (Task, error) {
	return resolveCrossPlanNeed(planDir, ref)
}
