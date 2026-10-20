package langbench

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

const (
	queuePath = "../../docs/plans/language-breadth.queue.json"
	planPath  = "../../docs/plans/language-breadth.md"
)

// TestLanguageQueueMatchesPlan (LB-F22, design §15) holds the queue file and
// the plan together: every queue entry has an LB-Q-<id> row in the same
// order, the row's needs equal the entry's depends_on, and every depends_on
// names a row that exists in the plan.
func TestLanguageQueueMatchesPlan(t *testing.T) {
	q, err := LoadQueue(queuePath)
	if err != nil {
		t.Fatalf("LoadQueue: %v", err)
	}
	if len(q) == 0 {
		t.Fatal("queue is empty")
	}
	plan, err := LoadBreadthPlan("../..")
	if err != nil {
		t.Fatal(err)
	}
	if errs := CheckQueueAgainstPlan(q, plan); len(errs) > 0 {
		for _, e := range errs {
			t.Error(e)
		}
	}
}

func TestParseQueueRefusesBadEntries(t *testing.T) {
	cases := map[string]string{
		"duplicate id": `[{"id":"a","plan_row":"LB-Q-a","kind":"language","target_level":"L1","engine":"tags"},
		                  {"id":"a","plan_row":"LB-Q-a","kind":"language","target_level":"L1","engine":"tags"}]`,
		"bad id":         `[{"id":"A_b","plan_row":"LB-Q-A_b","kind":"language","target_level":"L1","engine":"tags"}]`,
		"plan_row":       `[{"id":"a","plan_row":"LB-Q-b","kind":"language","target_level":"L1","engine":"tags"}]`,
		"target":         `[{"id":"a","plan_row":"LB-Q-a","kind":"language","target_level":"L5","engine":"tags"}]`,
		"kind":           `[{"id":"a","plan_row":"LB-Q-a","kind":"other","target_level":"L1","engine":"tags"}]`,
		"engine":         `[{"id":"a","plan_row":"LB-Q-a","kind":"language","target_level":"L1","engine":"regex"}]`,
		"unknown field":  `[{"id":"a","plan_row":"LB-Q-a","kind":"language","target_level":"L1","engine":"tags","levle":"L2"}]`,
		"tool L4 claim":  `[{"id":"a","plan_row":"LB-Q-a","kind":"tooling","target_level":"L4","engine":"doc"}]`,
		"not an array":   `{"id":"a"}`,
		"tier T8":        `[{"id":"a","plan_row":"LB-Q-a","kind":"language","target_level":"L1","engine":"tags","tier":"T8"}]`,
		"tier A":         `[{"id":"a","plan_row":"LB-Q-a","kind":"language","target_level":"L1","engine":"tags","tier":"A"}]`,
		"tier lowercase": `[{"id":"a","plan_row":"LB-Q-a","kind":"language","target_level":"L1","engine":"tags","tier":"t1"}]`,
		"self-dependent": `[{"id":"a","plan_row":"LB-Q-a","kind":"language","target_level":"L1","engine":"tags","depends_on":["LB-Q-a"]}]`,
	}
	for name, src := range cases {
		if _, err := ParseQueue([]byte(src)); err == nil {
			t.Errorf("%s: ParseQueue accepted %s", name, src)
		}
	}
	ok := `[{"id":"a-1","name":"A","plan_row":"LB-Q-a-1","kind":"language","target_level":"L2","engine":"tags","depends_on":["LB-F1"]}]`
	q, err := ParseQueue([]byte(ok))
	if err != nil || len(q) != 1 || q[0].ID != "a-1" {
		t.Fatalf("ParseQueue(ok) = %v, %v", q, err)
	}
	for _, bad := range []string{"", "T0", "T8", "t1", "t7", "A"} {
		if IsProgramTier(bad) {
			t.Errorf("IsProgramTier(%q) = true", bad)
		}
	}
	for _, tier := range []string{"T1", "T2", "T3", "T4", "T5", "T6", "T7"} {
		src := `[{"id":"a","plan_row":"LB-Q-a","kind":"language","target_level":"L1","engine":"tags","tier":"` + tier + `"}]`
		if q, err := ParseQueue([]byte(src)); err != nil || q[0].Tier != tier {
			t.Errorf("tier %s: %v, %v", tier, q, err)
		}
	}
}

func TestCheckQueueAgainstPlanCatchesDrift(t *testing.T) {
	q, _ := ParseQueue([]byte(`[
	 {"id":"a","plan_row":"LB-Q-a","kind":"language","target_level":"L1","engine":"tags","depends_on":["LB-F1"]},
	 {"id":"b","plan_row":"LB-Q-b","kind":"language","target_level":"L1","engine":"tags","depends_on":["LB-F9"]}]`))
	md := strings.Join([]string{
		"- [ ] F1 {id: LB-F1, symbol: X, package: p}",
		"- [ ] b {id: LB-Q-b, oracle: lexical, path: x, pattern: y, needs: \"LB-F9\"}",
		"- [ ] a {id: LB-Q-a, oracle: lexical, path: x, pattern: y, needs: \"LB-F1\"}",
		"- [-] a rest {id: LB-Q-a-rest, disposition: deferred, reason: r}",
	}, "\n")
	errs := CheckQueueAgainstPlan(q, verify.ParseMarkdownPlan("s", []byte(md)))
	joined := strings.Join(errs, "\n")
	for _, want := range []string{"order", "LB-F9"} {
		if !strings.Contains(joined, want) {
			t.Errorf("drift report lacks %q:\n%s", want, joined)
		}
	}
}

// TestQueueReadsSiblingPlans: LoadBreadthPlan reads language-breadth.md then its
// sorted language-breadth.<wave>.md siblings, rows keeping their source file. A
// queue depends_on and a row's needs may name <sibling>.md#<id>: a claimed
// sibling row satisfies it, an open or deferred one blocks with the qualified
// name, a name no file defines is an error, and a sibling -rest refusal makes
// the item need re-planning as one in the main plan does.
func TestQueueReadsSiblingPlans(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "plans")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const (
		w1 = "language-breadth.w1.md"
		w2 = "language-breadth.w2.md"
	)
	write("language-breadth.md", "- [ ] a {id: LB-Q-a, oracle: lexical, path: x, pattern: y, needs: \""+w1+"#LB-F900\"}\n"+
		"- [ ] b {id: LB-Q-b, oracle: lexical, path: x, pattern: y, needs: \""+w1+"#LB-F901\"}\n"+
		"- [ ] c {id: LB-Q-c, oracle: lexical, path: x, pattern: y, needs: \""+w1+"#LB-F902\"}\n"+
		"- [ ] r {id: LB-Q-r, oracle: lexical, path: x, pattern: y}\n")
	write(w2, "- [x] later {id: LB-F903, oracle: lexical, path: x, pattern: y}\n")
	write(w1, "- [x] done {id: LB-F900, oracle: lexical, path: x, pattern: y}\n"+
		"- [ ] open {id: LB-F901, oracle: lexical, path: x, pattern: y}\n"+
		"- [-] later {id: LB-F902, disposition: deferred, reason: \"r\", trigger: \"t\"}\n"+
		"- [-] r rest {id: LB-Q-r-rest, disposition: refused, reason: \"no\"}\n")
	write("language-breadth.queue.json", "[]") // not a plan

	plan, err := LoadBreadthPlan(root)
	if err != nil {
		t.Fatal(err)
	}
	var order, src []string
	for _, tk := range plan.Tasks {
		order = append(order, tk.ID)
		src = append(src, tk.Source)
	}
	if want := []string{"LB-Q-a", "LB-Q-b", "LB-Q-c", "LB-Q-r", "LB-F900", "LB-F901", "LB-F902", "LB-Q-r-rest", "LB-F903"}; !slices.Equal(order, want) {
		t.Fatalf("row order %v, want %v", order, want)
	}
	if src[0] != "language-breadth.md" || src[4] != w1 || src[8] != w2 {
		t.Errorf("rows lost their source file: %v", src)
	}

	q := []QueueEntry{
		{ID: "a", PlanRow: "LB-Q-a", DependsOn: []string{w1 + "#LB-F900"}},
		{ID: "b", PlanRow: "LB-Q-b", DependsOn: []string{w1 + "#LB-F901"}},
		{ID: "c", PlanRow: "LB-Q-c", DependsOn: []string{w1 + "#LB-F902"}},
		{ID: "r", PlanRow: "LB-Q-r"},
	}
	if errs := CheckQueueAgainstPlan(q, plan); len(errs) > 0 {
		t.Errorf("qualified needs rejected: %v", errs)
	}
	blocks := QueueEligibility(q, plan, func(string) (string, bool) { return "", false })
	if !blocks["a"].Eligible() {
		t.Errorf("a: a claimed sibling row should satisfy the need, got %+v", blocks["a"])
	}
	if got := blocks["b"].Blocked; !slices.Equal(got, []string{w1 + "#LB-F901"}) {
		t.Errorf("b blocked %v, want the qualified open row", got)
	}
	if got := blocks["c"].Blocked; !slices.Equal(got, []string{w1 + "#LB-F902"}) {
		t.Errorf("c blocked %v, want the qualified deferred row", got)
	}
	if got := blocks["r"].Replan; !slices.Equal(got, []string{"LB-Q-r-rest"}) {
		t.Errorf("r replan %v, want the sibling -rest refusal", got)
	}

	// A name no file defines, or a row in the wrong file, is an error.
	for _, bad := range []string{w1 + "#LB-F999", w2 + "#LB-F900", "language-breadth.w3.md#LB-F900"} {
		q2 := []QueueEntry{{ID: "a", PlanRow: "LB-Q-a", DependsOn: []string{bad}}}
		errs := CheckQueueAgainstPlan(q2, plan)
		found := false
		for _, e := range errs {
			found = found || strings.Contains(e, bad+", which is no plan row")
		}
		if !found {
			t.Errorf("%s accepted: %v", bad, errs)
		}
	}
}
