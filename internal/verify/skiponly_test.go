package verify

import (
	"strings"
	"testing"
)

// skiponly_test.go — a claimed row whose only annotation is a skip waiver
// (`skip_test: true` / `skip: "…"`) carries no contract: VerifyPlan reads it
// no-contract (Unknown), which blocks only under --strict. Twenty-eight such
// rows sat claimed across the plans because the default (pre-push) gate lets
// an Unknown claimed row through silently — authors reasonably read
// `skip_test` plus a detailed `skip:` as closing an "(op)" row. The lint names
// the shape at the moment of the claim.

const skipOnlyMD = `# L
- [x] Measured op row {id: S-1, skip_test: true, skip: "operator spend; evidence is the ledger row"}
- [x] Reason only {id: S-2, skip: "measured: docs/x.md"}
- [x] With symbol {id: S-3, symbol: X, skip_test: true, skip: "cited"}
- [x] With oracle {id: S-4, oracle: lexical, path: notes.txt, pattern: x, skip_test: true, skip: "docs"}
- [x] With gone {id: S-5, gone: "import example.com/x", skip: "retired"}
- [x] With spec [FR-001] {id: S-6, skip_test: true, skip: "spec row"}
- [ ] Open op row {id: S-7, skip_test: true, skip: "operator spend"}
- [-] Dispositioned {id: S-8, disposition: deferred, reason: r, trigger: t, skip_test: true, skip: "x"}
- [~] In progress {id: S-9, skip: "measuring"}
- [x] No annotation at all {id: S-10}
`

func TestLintPlan_ClaimedSkipOnlyRowHasNoContract(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte(skipOnlyMD))
	var got []string
	for _, f := range LintPlan(p) {
		if strings.Contains(f, "not a contract") {
			got = append(got, f)
		}
	}
	want := []string{"S-1: ", "S-2: "}
	if len(got) != len(want) {
		t.Fatalf("skip-only findings = %v, want exactly rows %v", got, want)
	}
	for i, prefix := range want {
		if !strings.HasPrefix(got[i], prefix) {
			t.Errorf("finding[%d] = %q, want prefix %q", i, got[i], prefix)
		}
		for _, word := range []string{"no-contract", "symbol", "oracle", "gone"} {
			if !strings.Contains(got[i], word) {
				t.Errorf("finding[%d] = %q does not name %q (the remedy must be in the message)", i, got[i], word)
			}
		}
	}
}

// The lint and the gate must agree on the shape: every row the lint names is
// one VerifyPlan reports no-contract, and every claimed no-contract row that
// carries a skip waiver is one the lint names.
func TestLintPlan_SkipOnlyAgreesWithVerifyNoContract(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte(skipOnlyMD))
	named := map[string]bool{}
	for _, f := range LintPlan(p) {
		if strings.Contains(f, "not a contract") {
			named[strings.SplitN(f, ":", 2)[0]] = true
		}
	}
	byID := map[string]Task{}
	for _, tk := range p.Tasks {
		byID[tk.ID] = tk
	}
	rep := VerifyPlan(p, noEvidence{})
	for _, v := range rep.Tasks {
		tk := byID[v.TaskID]
		waiver := tk.Expect.SkipTest || tk.Expect.SkipReason != ""
		noContract := v.Claimed && v.Label == "no-contract"
		if named[v.TaskID] && !noContract {
			t.Errorf("%s: lint names it skip-only but VerifyPlan labels it %q (claimed=%v)", v.TaskID, v.Label, v.Claimed)
		}
		if noContract && waiver && !named[v.TaskID] {
			t.Errorf("%s: claimed no-contract row with a skip waiver was not named by the lint", v.TaskID)
		}
	}
}
