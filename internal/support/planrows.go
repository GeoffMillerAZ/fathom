package support

// planrows.go — the checkbox of every row in the plans the target rows
// belong to, for the console's What's next panel (SM-24, design §5 panel 4:
// the `plans next --eligible-only` rule since G2-6 — a need is met only when
// its row is claimed; a deferred [-] row blocks and a refused one means
// re-planning). A lexical read of committed plan files: the claim, not its
// proof, which `fathom verify` owns.

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
	RowMarkDispositioned RowMark = "dispositioned" // [-] deferred, or with no disposition
	RowMarkRefused       RowMark = "refused"       // [-] with any disposition but deferred (G2-6)
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
			case t.Skipped && t.Disposition != "" && t.Disposition != "deferred":
				m = RowMarkRefused
			case t.Skipped:
				m = RowMarkDispositioned
			}
			out[t.ID] = m
		}
	}
	return out, nil
}
