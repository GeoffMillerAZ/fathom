package support

// planrows.go — the checkbox of every row in the plans the target rows
// belong to, for the console's What's next panel (SM-24, design §5 panel 4:
// the `plans next --eligible-only` rule — a need is met when its row is
// claimed or dispositioned). A lexical read of committed plan files: the
// claim, not its proof, which `fathom verify` owns.

import (
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// RowMark is one plan row's checkbox.
type RowMark string

const (
	RowMarkOpen          RowMark = "open"
	RowMarkClaimed       RowMark = "claimed"
	RowMarkDispositioned RowMark = "dispositioned"
)

// PlanRowMarks reads docs/plans/<name> under root for each name and returns
// every row id's checkbox. A plan file that does not exist contributes
// nothing.
func PlanRowMarks(root string, names ...string) (map[string]RowMark, error) {
	out := map[string]RowMark{}
	for _, name := range names {
		b, err := os.ReadFile(filepath.Join(root, "docs", "plans", name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, t := range verify.ParseMarkdownPlan("", b).Tasks {
			m := RowMarkOpen
			switch {
			case t.Claimed:
				m = RowMarkClaimed
			case t.Skipped:
				m = RowMarkDispositioned
			}
			out[t.ID] = m
		}
	}
	return out, nil
}
