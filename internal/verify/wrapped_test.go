package verify_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// wrappedPlanMD is the shape G2-5 found in docs/plans/feature-map.md and
// agentic-kit's decision-port.md: a checkbox row whose title wraps onto
// indented continuation lines, with the annotation block on the last of them
// (FM-1) or itself split across two of them (DP-10). Before G2-5 the parser
// was line-based, so each such row verified as a positional md-NNN row with
// no contract.
const wrappedPlanMD = `# p

- [ ] ` + "`docs/features.md`" + ` grammar: plan-grammar-style rows (id, name, keys, owner, cited)
  parsed and refused by name when malformed (§3) {id: FM-1, symbol: ParseFeatureMap, package: internal/featuremap, reachable_from: TestParseFeatureMapGrammar}
- [x] **typesafe adapter** calling the vendor API directly
  (POST ` + "`{BaseURL}/v1/systemone`" + `), one request per batch
  {id: DP-10, symbol: NewTypeSafe, package: foundations/decision,
  reachable_from: TestTypeSafe_Decide}
- [ ] One-line row {id: OL-1, symbol: Foo}
  <!-- a note under an annotated row stays a note {id: NOT-ME} -->
- [ ] Unannotated wrapped row
  whose title continues here
- [ ] Row with a nested list
  - a sub-bullet {id: SUB-1, symbol: NotTheRow}
- [ ] Lazy line is not a continuation
{id: LAZY-1, symbol: Lazy}
`

func TestParseMarkdownPlanWrappedAnnotationKeepsContract(t *testing.T) {
	p := verify.ParseMarkdownPlan("svc", []byte(wrappedPlanMD))
	if len(p.Tasks) != 6 {
		t.Fatalf("got %d tasks %v, want 6", len(p.Tasks), taskIDs(p))
	}
	byID := map[string]verify.Task{}
	for _, tk := range p.Tasks {
		byID[tk.ID] = tk
	}

	fm, ok := byID["FM-1"]
	if !ok {
		t.Fatalf("FM-1's annotation on its continuation line was not read: ids %v", taskIDs(p))
	}
	if fm.Expect.Symbol != "ParseFeatureMap" || fm.Expect.Package != "internal/featuremap" || fm.Expect.ReachableFrom != "TestParseFeatureMapGrammar" {
		t.Errorf("FM-1 contract = %+v", fm.Expect)
	}
	if want := "`docs/features.md` grammar: plan-grammar-style rows (id, name, keys, owner, cited) parsed and refused by name when malformed (§3)"; fm.Title != want {
		t.Errorf("FM-1 title = %q, want the joined title %q", fm.Title, want)
	}
	if fm.Line != 3 || fm.EndLine != 4 {
		t.Errorf("FM-1 Line/EndLine = %d/%d, want 3/4", fm.Line, fm.EndLine)
	}

	dp, ok := byID["DP-10"]
	if !ok {
		t.Fatalf("DP-10's annotation split across two continuation lines was not read: ids %v", taskIDs(p))
	}
	if dp.Expect.Symbol != "NewTypeSafe" || dp.Expect.Package != "foundations/decision" || dp.Expect.ReachableFrom != "TestTypeSafe_Decide" || !dp.Claimed {
		t.Errorf("DP-10 = %+v", dp)
	}
	if dp.Line != 5 || dp.EndLine != 8 {
		t.Errorf("DP-10 Line/EndLine = %d/%d, want 5/8", dp.Line, dp.EndLine)
	}

	// A row whose annotation sits on its checkbox line keeps exactly its old
	// reading: the note under it is not title text and its braces are not
	// the row's annotation.
	ol := byID["OL-1"]
	if ol.Expect.Symbol != "Foo" || ol.Title != "One-line row" || ol.EndLine != ol.Line {
		t.Errorf("OL-1 = %+v, want its one-line reading unchanged", ol)
	}
	if _, bad := byID["NOT-ME"]; bad {
		t.Error("an HTML comment under a row was read as its annotation")
	}

	// An unannotated wrapped row keeps its whole title and stays honestly
	// contract-less.
	var unann verify.Task
	for _, tk := range p.Tasks {
		if strings.HasPrefix(tk.Title, "Unannotated") {
			unann = tk
		}
	}
	if unann.Title != "Unannotated wrapped row whose title continues here" || unann.Expect.Symbol != "" {
		t.Errorf("unannotated wrapped row = %+v", unann)
	}

	// A nested list item is its own line, never the parent's continuation.
	if _, bad := byID["SUB-1"]; bad {
		t.Error("a sub-bullet's annotation was read as a row's")
	}
	for _, tk := range p.Tasks {
		if strings.HasPrefix(tk.Title, "Row with a nested list") && (tk.Expect.Symbol != "" || tk.EndLine != tk.Line) {
			t.Errorf("a sub-bullet was read as the parent row's continuation: %+v", tk)
		}
	}

	// An unindented line under a row is not read (a lazy paragraph line is
	// usually the next paragraph), but an annotation on one is never lost
	// silently: LintPlan names it.
	var lazy verify.Task
	for _, tk := range p.Tasks {
		if strings.HasPrefix(tk.Title, "Lazy line") {
			lazy = tk
		}
	}
	if lazy.Expect.Symbol != "" {
		t.Errorf("an unindented line was read as a continuation: %+v", lazy)
	}
	found := false
	for _, w := range verify.LintPlan(p) {
		if strings.Contains(w, lazy.ID+":") && strings.Contains(w, "continuation") {
			found = true
		}
	}
	if !found {
		t.Errorf("no lint line names %s's unindented annotation: %v", lazy.ID, verify.LintPlan(p))
	}
	for _, id := range []string{"FM-1", "DP-10", "OL-1"} {
		for _, w := range verify.LintPlan(p) {
			if strings.HasPrefix(w, id+":") {
				t.Errorf("a healthy wrapped row drew a lint line: %s", w)
			}
		}
	}
}

func taskIDs(p verify.Plan) []string {
	out := make([]string, len(p.Tasks))
	for i, tk := range p.Tasks {
		out[i] = tk.ID
	}
	return out
}
