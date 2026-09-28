package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// PE-3 (docs/design/progress-and-economics.md §4, spec interop §4): `plans
// status --weighted` rolls proven weight over total weight up by fleet,
// project, plan and section; a dispositioned row carries no weight.
func TestPlansStatusWeighted(t *testing.T) {
	pf := filepath.Join("testdata", "plans", "fixture-weighted.md")
	raw, err := os.ReadFile(pf)
	if err != nil {
		t.Fatal(err)
	}
	// The rollup over known verdicts: W-1 proven, W-2 failing.
	parsed := verify.ParseMarkdownPlan("payment-service", raw)
	verdict := map[string]verify.CriterionStatus{"W-1": verify.Pass, "W-2": verify.Fail, "W-3": verify.Pass}
	secs := planSectionsOf(raw)
	var rows []planRow
	for _, tk := range parsed.Tasks {
		rows = append(rows, planRow{PlanFile: pf, Task: tk, Verdict: verify.TaskVerdict{TaskID: tk.ID, Status: verdict[tk.ID]}, Section: sectionAt(secs, tk.Line)})
	}
	fleet := weightedRollup("payment-service", []string{pf}, [][]planRow{rows})
	if fleet.Scope != "fleet" || fleet.Proven != 3 || fleet.Total != 6 || len(fleet.Children) != 1 {
		t.Fatalf("fleet rollup wrong: %+v", fleet)
	}
	proj := fleet.Children[0]
	if proj.Scope != "project" || proj.Name != "payment-service" || proj.Proven != 3 || proj.Total != 6 {
		t.Fatalf("project rollup wrong: %+v", proj)
	}
	plan := proj.Children[0]
	if plan.Scope != "plan" || !strings.HasSuffix(plan.Name, "fixture-weighted.md") || plan.Proven != 3 || plan.Total != 6 || plan.Rows != 4 {
		t.Fatalf("plan rollup wrong: %+v", plan)
	}
	if len(plan.Children) != 2 {
		t.Fatalf("want two sections, got %+v", plan.Children)
	}
	w1, w2 := plan.Children[0], plan.Children[1]
	if w1.Scope != "section" || w1.Name != "Wave 1: the heavy row" || w1.Proven != 3 || w1.Total != 4 {
		t.Fatalf("section 1 wrong: %+v", w1)
	}
	if w2.Name != "Wave 2: open work" || w2.Proven != 0 || w2.Total != 2 {
		t.Fatalf("section 2 wrong: %+v", w2)
	}

	// The verb: --weighted --json emits the same tree over the gate's own
	// verdicts (here the fixture's skip_test rows read self-skipped, so
	// nothing is proven), and the human table names every level.
	storePath := seedPlansFixtureStore(t)
	files := []string{pf}
	var out, errBuf bytes.Buffer
	if code := run(planArgs("status", storePath, files, "--weighted"), &out, &errBuf); code != 0 {
		t.Fatalf("plans status --weighted exit = %d, stderr: %s", code, errBuf.String())
	}
	var got weightedScope
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("--weighted --json not valid JSON: %v\n%s", err, out.String())
	}
	if got.Scope != "fleet" || got.Total != 6 || got.Rows != 4 || len(got.Children[0].Children[0].Children) != 2 {
		t.Fatalf("verb rollup wrong: %+v", got)
	}
	out.Reset()
	if code := run(removeArg(planArgs("status", storePath, files, "--weighted"), "--json"), &out, &errBuf); code != 0 {
		t.Fatalf("plans status --weighted (text) exit = %d: %s", code, errBuf.String())
	}
	for _, want := range []string{"fleet", "payment-service", "fixture-weighted.md", "Wave 1: the heavy row", "/6"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("text table missing %q:\n%s", want, out.String())
		}
	}
}

func removeArg(args []string, drop string) []string {
	out := args[:0:0]
	for _, a := range args {
		if a != drop {
			out = append(out, a)
		}
	}
	return out
}
