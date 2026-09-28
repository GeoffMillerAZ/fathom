package specdialect

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// ghTaskListDialect reads a GitHub issue or PR body's own task list, once
// exported to a file (design §3): plain checkbox rows ("- [ ] Fix the
// retry bug in src/payments/client.go"), done as [x]/[X]. Unlike every
// other dialect, a GitHub task-list file carries no directory convention
// or content shape fathom can trust to be one — an issue or PR body is,
// after export, an arbitrary Markdown file that would otherwise collide
// with the native grammar's own checklists — so "the user passes" it
// (design's own words for this dialect): Detect never auto-claims
// anything and Claims always answers false; the only way in is
// `fathom verify --dialect gh-tasklist --plan <exported file>`, which
// bypasses detection entirely (dialectForPlan, SI-9). Anchors come from
// an inline fathom annotation, else the paths and symbols the task names,
// exactly like every other adapter's un-annotated row.
type ghTaskListDialect struct{}

// GHTaskList returns the GitHub task-list adapter.
func GHTaskList() SpecDialect { return ghTaskListDialect{} }

func (ghTaskListDialect) ID() string { return "gh-tasklist" }

// Detect never auto-claims a file: see the type doc.
func (ghTaskListDialect) Detect(_ string, _ []string) []string { return nil }

// Claims never auto-claims a file: see the type doc.
func (ghTaskListDialect) Claims(_ string, _ []byte) bool { return false }

func (ghTaskListDialect) Parse(path string, content []byte) (verify.Plan, []Span, error) {
	p, spans := ghTaskListParse(path, content)
	return p, spans, nil
}

// ghTaskListParse maps an exported GitHub issue/PR body into the
// canonical model: every checkbox row becomes a task, in document order,
// with a positional id ("t-001", …) unless an inline annotation names its
// own id:. There is no dialect-owned id scheme to strip (unlike Spec
// Kit's T###, OpenSpec's/Kiro's dotted numbers) — GitHub's own task lists
// carry none. Pure.
func ghTaskListParse(_ string, content []byte) (verify.Plan, []Span) {
	var (
		plan  verify.Plan
		spans []Span
		n     int
	)
	for i, line := range strings.Split(string(content), "\n") {
		m := checkboxLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n++
		id := fmt.Sprintf("t-%03d", n)
		t := foreignTask(id, m[3], m[2] == "x" || m[2] == "X", i+1)
		plan.Tasks = append(plan.Tasks, t)
		spans = append(spans, Span{TaskID: t.ID, Line: i + 1, Col: len(m[1]) + 1})
	}
	return plan, spans
}
