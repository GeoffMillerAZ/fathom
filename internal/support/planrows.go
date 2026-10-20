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
	"sort"
	"strings"

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

// breadthPlan is the one plan whose waves add sibling plans
// (language-breadth.<wave>.md, design language-breadth.md section 13).
const breadthPlan = "language-breadth.md"

// planFiles expands names: breadthPlan is followed by its sorted
// language-breadth.<wave>.md siblings (the queue .json is not a plan).
func planFiles(root string, names []string) ([]string, error) {
	var out []string
	for _, name := range names {
		out = append(out, name)
		if name != breadthPlan {
			continue
		}
		hits, err := filepath.Glob(filepath.Join(root, "docs", "plans", "language-breadth.*.md"))
		if err != nil {
			return nil, err
		}
		sort.Strings(hits)
		for _, h := range hits {
			out = append(out, filepath.Base(h))
		}
	}
	return out, nil
}

// readPlanMarks calls fn for every row of every plan file that exists.
func readPlanMarks(root string, names []string, fn func(file, id string, m RowMark)) error {
	files, err := planFiles(root, names)
	if err != nil {
		return err
	}
	for _, name := range files {
		b, err := os.ReadFile(filepath.Join(root, "docs", "plans", name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
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
			fn(name, t.ID, m)
		}
	}
	return nil
}

// PlanRowMarks reads docs/plans/<name> under root for each name and returns
// every row id's checkbox. A plan file that does not exist contributes
// nothing. language-breadth.md also reads its language-breadth.<wave>.md
// siblings, sorted. A row id in both a sibling and an earlier file keeps the
// earlier file's mark (the main plan wins).
func PlanRowMarks(root string, names ...string) (map[string]RowMark, error) {
	out := map[string]RowMark{}
	seen := map[string]string{}
	err := readPlanMarks(root, names, func(file, id string, m RowMark) {
		if f, ok := seen[id]; ok && f != file && strings.HasPrefix(file, "language-breadth.") && file != breadthPlan {
			return
		}
		seen[id] = file
		out[id] = m
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PlanRowDuplicates names each sibling row whose id an earlier plan already
// holds, as "<id> (<sibling file>)", sorted.
func PlanRowDuplicates(root string, names ...string) ([]string, error) {
	seen := map[string]string{}
	var dups []string
	err := readPlanMarks(root, names, func(file, id string, _ RowMark) {
		if f, ok := seen[id]; ok && f != file {
			dups = append(dups, id+" ("+file+")")
			return
		}
		seen[id] = file
	})
	sort.Strings(dups)
	return dups, err
}
