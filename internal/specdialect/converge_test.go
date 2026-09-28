package specdialect

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestConvergeThreeSections (SI-5, design §5): claimed-unproven tasks with
// their failing criterion, unanchored tasks, and changed code no task
// anchors — deterministic, from a spec, its verify report and the range's
// changed symbols.
func TestConvergeThreeSections(t *testing.T) {
	spec := "- [x] T001 Charge handler {symbol: Charge, package: svc/pay}\n" +
		"- [x] T002 Ghost {symbol: Ghost}\n" +
		"- [ ] T003 Write the runbook\n" +
		"- [x] T004 Retry lives in svc/pay/retry.go\n" +
		"- [x] T005 Prose only, claimed\n"
	plan, _ := specKitParse("specs/001/tasks.md", []byte(spec))
	rep := verify.Report{Tasks: []verify.TaskVerdict{
		{TaskID: "T001", Status: verify.Pass},
		{TaskID: "T002", Status: verify.Fail, Criteria: []verify.Criterion{
			{Name: "runtime_pass", Status: verify.Unknown, Detail: "n/a"},
			{Name: "symbol_present", Status: verify.Fail, Detail: `"Ghost" not found`},
		}},
		{TaskID: "T003", Status: verify.Unknown},
		{TaskID: "T004", Status: verify.Pass},
		{TaskID: "T005", Status: verify.Unknown},
	}}

	diff := "diff --git a/svc/pay/charge.go b/svc/pay/charge.go\n" +
		"--- a/svc/pay/charge.go\n+++ b/svc/pay/charge.go\n" +
		"@@ -10,0 +11,3 @@ func Charge\n+x\n+y\n+z\n" +
		"@@ -40,2 +42,0 @@\n-a\n-b\n" +
		"--- a/svc/pay/retry.go\n+++ b/svc/pay/retry.go\n@@ -1 +1 @@\n-a\n+b\n" +
		"--- a/svc/pay/refund.go\n+++ b/svc/pay/refund.go\n@@ -5 +5,2 @@\n-a\n+b\n+c\n" +
		"--- a/old.go\n+++ /dev/null\n@@ -1,3 +0,0 @@\n-a\n"
	hunks := ParseHunks(diff)
	if want := []LineRange{{11, 13}, {42, 42}}; !reflect.DeepEqual(hunks["svc/pay/charge.go"], want) {
		t.Fatalf("hunks = %v, want %v", hunks["svc/pay/charge.go"], want)
	}
	if _, ok := hunks["old.go"]; ok {
		t.Fatal("a deleted file has no head-side lines")
	}
	symbols := []SymbolSpan{
		{Name: "Charge", Package: "pay", File: "svc/pay/charge.go", Line: 10, EndLine: 20},
		{Name: "Client.Void", Package: "pay", File: "svc/pay/charge.go", Line: 40, EndLine: 45},
		{Name: "untouched", Package: "pay", File: "svc/pay/charge.go", Line: 25, EndLine: 30},
		{Name: "Backoff", Package: "pay", File: "svc/pay/retry.go", Line: 1, EndLine: 9},
		{Name: "Refund", Package: "pay", File: "svc/pay/refund.go", Line: 5},
		{Name: "Refund", Package: "pay", File: "svc/pay/refund.go", Line: 5},
	}
	changed := ChangedSymbols(hunks, symbols)
	if len(changed) != 4 {
		t.Fatalf("changed = %+v, want Charge, Client.Void, Refund (once), Backoff", changed)
	}

	r := Converge(plan, rep, changed)
	if len(r.ClaimedUnproven) != 1 || r.ClaimedUnproven[0].TaskID != "T002" ||
		r.ClaimedUnproven[0].Criterion != `symbol_present: "Ghost" not found` || r.ClaimedUnproven[0].Line != 2 {
		t.Errorf("claimed-unproven = %+v", r.ClaimedUnproven)
	}
	var unanchored []string
	for _, it := range r.Unanchored {
		unanchored = append(unanchored, it.TaskID)
		if len(it.Proposals) != 0 {
			t.Errorf("%s carries proposals before SI-M4 passed", it.TaskID)
		}
	}
	if !reflect.DeepEqual(unanchored, []string{"T003", "T005"}) {
		t.Errorf("unanchored = %v, want T003 T005", unanchored)
	}
	// Charge is anchored by T001's symbol; retry.go by T004's named path.
	var names []string
	for _, s := range r.Unclaimed {
		names = append(names, s.File+":"+s.Name)
	}
	if want := []string{"svc/pay/charge.go:Client.Void", "svc/pay/refund.go:Refund"}; !reflect.DeepEqual(names, want) {
		t.Errorf("unclaimed = %v, want %v", names, want)
	}

	// Deterministic: shuffled inputs, same report.
	rev := append([]ChangedSymbol(nil), changed...)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	if again := Converge(plan, rep, rev); !reflect.DeepEqual(again, r) {
		t.Fatal("Converge depends on input order")
	}

	r.Spec, r.Dialect = "specs/001/tasks.md", "speckit"
	var buf bytes.Buffer
	RenderConverge(&buf, r)
	for _, want := range []string{"claimed-unproven (1)", "unanchored (2)", "unclaimed code (2)", "specs/001/tasks.md:2 [T002]", "svc/pay/refund.go:5 Refund"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("render lacks %q:\n%s", want, buf.String())
		}
	}
	r.UnclaimedNote = "needs --store"
	buf.Reset()
	RenderConverge(&buf, r)
	if !strings.Contains(buf.String(), "unclaimed code: not evaluated — needs --store") {
		t.Errorf("an unevaluated section must say so:\n%s", buf.String())
	}
}
