package verify_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// findingContains reports whether any LintPlan finding contains every substr.
func findingContains(findings []string, substrs ...string) bool {
	for _, f := range findings {
		ok := true
		for _, s := range substrs {
			if !strings.Contains(f, s) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// TestCtxAnnotationParsedNonGating pins WF-1: ctx: is a discovery-pack
// pointer list, carried on the row, and invisible to verify semantics.
func TestCtxAnnotationParsedNonGating(t *testing.T) {
	// (a) ctx parses and is carried as space-separated pointers.
	md := []byte("- [ ] Row with ctx {id: T-1, symbol: Foo, ctx: internal/verify.ParseMarkdownPlan markdown.go concern-9}\n")
	p := verify.ParseMarkdownPlan("svc", md)
	if len(p.Tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(p.Tasks))
	}
	wantCtx := []string{"internal/verify.ParseMarkdownPlan", "markdown.go", "concern-9"}
	if !reflect.DeepEqual(p.Tasks[0].Ctx, wantCtx) {
		t.Fatalf("Ctx = %v, want %v", p.Tasks[0].Ctx, wantCtx)
	}

	// (b) a plan whose ONLY difference is ctx: verifies identically.
	withCtx := []byte("- [x] Same row {id: T-1, symbol: Foo, ctx: internal/verify.ParseMarkdownPlan}\n")
	withoutCtx := []byte("- [x] Same row {id: T-1, symbol: Foo}\n")
	ev := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"Foo": {node("Foo", "a.go", 1, core.TierVerified)}},
		tests:   map[string][]string{"Foo": {"TestFoo"}},
	}
	repWith := verify.VerifyPlan(verify.ParseMarkdownPlan("svc", withCtx), ev)
	repWithout := verify.VerifyPlan(verify.ParseMarkdownPlan("svc", withoutCtx), ev)
	if !reflect.DeepEqual(repWith, repWithout) {
		t.Fatalf("ctx: changed the verify verdict:\nwith:    %+v\nwithout: %+v", repWith, repWithout)
	}

	// (c) empty ctx: is a lint warning, never an error — and it does not
	// void the rest of the row's contract (the O4 failure mode).
	empty := []byte("- [ ] Empty ctx {id: T-2, symbol: Bar, ctx: }\n")
	pEmpty := verify.ParseMarkdownPlan("svc", empty)
	if pEmpty.Tasks[0].Expect.Symbol != "Bar" {
		t.Fatalf("empty ctx: voided the row's other keys: %+v", pEmpty.Tasks[0])
	}
	if !findingContains(verify.LintPlan(pEmpty), "T-2", "ctx") {
		t.Fatalf("no ctx warning in %v", verify.LintPlan(pEmpty))
	}
}

// TestNeedsLintCatchesUnknownSelfAndCycle pins WF-3: needs: builds the
// eligibility DAG; LintPlan errors on an unknown id, a self-reference, and a
// dependency cycle, and lints a valid chain (including a quoted multi-id
// fan-in) clean.
func TestNeedsLintCatchesUnknownSelfAndCycle(t *testing.T) {
	unknown := verify.ParseMarkdownPlan("svc", []byte(
		"- [ ] A {id: T-1, needs: T-9}\n",
	))
	if got := verify.LintPlan(unknown); !findingContains(got, "T-1", "unknown id") {
		t.Fatalf("unknown-id case: got %v", got)
	}

	selfRef := verify.ParseMarkdownPlan("svc", []byte(
		"- [ ] A {id: T-1, needs: T-1}\n",
	))
	if got := verify.LintPlan(selfRef); !findingContains(got, "T-1", "self-reference") {
		t.Fatalf("self-reference case: got %v", got)
	}

	cycle := verify.ParseMarkdownPlan("svc", []byte(
		"- [ ] A {id: T-1, needs: T-2}\n"+
			"- [ ] B {id: T-2, needs: T-1}\n",
	))
	if got := verify.LintPlan(cycle); !findingContains(got, "cycle") {
		t.Fatalf("cycle case: got %v", got)
	}

	chain := verify.ParseMarkdownPlan("svc", []byte(
		"- [ ] Base {id: T-1}\n"+
			"- [ ] Middle {id: T-2, needs: T-1}\n"+
			"- [ ] Top {id: T-3, needs: T-2}\n"+
			`- [ ] Fan-in {id: T-4, needs: "T-1, T-2"}`+"\n",
	))
	if got := verify.LintPlan(chain); len(got) != 0 {
		t.Fatalf("valid chain should lint clean, got %v", got)
	}
	wantNeeds := []string{"T-1", "T-2"}
	if !reflect.DeepEqual(chain.Tasks[3].Needs, wantNeeds) {
		t.Fatalf("Fan-in Needs = %v, want %v", chain.Tasks[3].Needs, wantNeeds)
	}
}
