package langbench

import (
	"os"
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
	md, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	plan := verify.ParseMarkdownPlan("fathom", md)
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
		"tier T7":        `[{"id":"a","plan_row":"LB-Q-a","kind":"language","target_level":"L1","engine":"tags","tier":"T7"}]`,
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
	for _, bad := range []string{"", "T0", "T7", "t1", "A"} {
		if IsProgramTier(bad) {
			t.Errorf("IsProgramTier(%q) = true", bad)
		}
	}
	for _, tier := range []string{"T1", "T2", "T3", "T4", "T5", "T6"} {
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
