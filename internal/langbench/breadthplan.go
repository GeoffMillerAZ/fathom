package langbench

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/support"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// LoadBreadthPlan reads docs/plans/language-breadth.md under root and then its
// sorted language-breadth.<wave>.md siblings (support.PlanFiles, the one
// sibling rule) into one plan. Every row keeps the file it came from in
// Task.Source; a row id repeated in a later file keeps the earlier row (the
// main plan wins). A file that does not exist contributes nothing, so a
// fixture checkout without the plan loads an empty one.
func LoadBreadthPlan(root string) (verify.Plan, error) {
	files, err := support.PlanFiles(root, []string{support.BreadthPlan})
	if err != nil {
		return verify.Plan{}, err
	}
	plan := verify.Plan{Service: "fathom"}
	seen := map[string]bool{}
	for _, name := range files {
		b, err := os.ReadFile(filepath.Join(root, "docs", "plans", name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return verify.Plan{}, fmt.Errorf("breadth plan: %w", err)
		}
		for _, t := range verify.ParseMarkdownPlan("fathom", b).Tasks {
			if seen[t.ID] {
				continue
			}
			seen[t.ID] = true
			t.Source = name
			plan.Tasks = append(plan.Tasks, t)
		}
	}
	return plan, nil
}

// planRows indexes a plan's rows by id and, for a row with a Source, by the
// cross-plan name "<file>#<id>" a queue depends_on or a needs may use.
func planRows(plan verify.Plan) map[string]verify.Task {
	rows := map[string]verify.Task{}
	for _, t := range plan.Tasks {
		rows[t.ID] = t
		if t.Source != "" {
			rows[t.Source+"#"+t.ID] = t
		}
	}
	return rows
}
