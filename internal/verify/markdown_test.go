package verify_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func TestParseMarkdownPlan(t *testing.T) {
	md := []byte(`# My plan

Some prose that is not a task.

- [x] Implement token validation [US-042] {symbol: ValidateToken, reachable_from: Router, min_tier: verified}
- [ ] Refund endpoint {symbol: RefundHandler, skip_test: true}
* [-] Deferred thing
- [x] Just a vibe with no contract
- [x] Covered spec only [EP-013]
`)
	plan := verify.ParseMarkdownPlan("svc", md)

	if plan.Service != "svc" {
		t.Fatalf("service = %q", plan.Service)
	}
	if len(plan.Tasks) != 5 {
		t.Fatalf("want 5 tasks, got %d: %+v", len(plan.Tasks), plan.Tasks)
	}

	// IDs are deterministic + zero-padded in document order.
	if plan.Tasks[0].ID != "md-001" || plan.Tasks[4].ID != "md-005" {
		t.Fatalf("unexpected IDs: %s … %s", plan.Tasks[0].ID, plan.Tasks[4].ID)
	}

	// Task 1: symbol + reachable_from + spec id; annotation stripped from title.
	t1 := plan.Tasks[0]
	if t1.Expect.Symbol != "ValidateToken" || t1.Expect.ReachableFrom != "Router" {
		t.Fatalf("t1 expect = %+v", t1.Expect)
	}
	if t1.Expect.MinTier != core.TierVerified {
		t.Fatalf("t1 min_tier = %q", t1.Expect.MinTier)
	}
	if len(t1.SpecIDs) != 1 || t1.SpecIDs[0] != "[US-042]" {
		t.Fatalf("t1 spec_ids = %v", t1.SpecIDs)
	}
	if t1.Title != "Implement token validation [US-042]" {
		t.Fatalf("t1 title = %q", t1.Title)
	}

	// Task 2: skip_test parsed.
	if !plan.Tasks[1].Expect.SkipTest || plan.Tasks[1].Expect.Symbol != "RefundHandler" {
		t.Fatalf("t2 expect = %+v", plan.Tasks[1].Expect)
	}

	// Task 4: no contract → no symbol, no specs.
	if plan.Tasks[3].Expect.Symbol != "" || len(plan.Tasks[3].SpecIDs) != 0 {
		t.Fatalf("t4 should have no contract: %+v", plan.Tasks[3])
	}

	// Task 5: spec only.
	if len(plan.Tasks[4].SpecIDs) != 1 || plan.Tasks[4].SpecIDs[0] != "[EP-013]" {
		t.Fatalf("t5 spec_ids = %v", plan.Tasks[4].SpecIDs)
	}
}

// TestParseMarkdownPlan_CheckboxSemantics: [ ] open, [x]/[X] claimed, [-] skipped.
func TestParseMarkdownPlan_CheckboxSemantics(t *testing.T) {
	md := []byte("- [ ] open task\n- [x] lower x claimed\n- [X] upper X claimed\n- [-] skipped task\n")
	plan := verify.ParseMarkdownPlan("svc", md)
	if len(plan.Tasks) != 4 {
		t.Fatalf("want 4 tasks, got %d: %+v", len(plan.Tasks), plan.Tasks)
	}

	open := plan.Tasks[0]
	if open.Claimed || open.Skipped {
		t.Errorf("[ ] task: Claimed=%v Skipped=%v, want both false", open.Claimed, open.Skipped)
	}

	lowerX := plan.Tasks[1]
	if !lowerX.Claimed || lowerX.Skipped {
		t.Errorf("[x] task: Claimed=%v Skipped=%v, want Claimed=true Skipped=false", lowerX.Claimed, lowerX.Skipped)
	}

	upperX := plan.Tasks[2]
	if !upperX.Claimed || upperX.Skipped {
		t.Errorf("[X] task: Claimed=%v Skipped=%v, want Claimed=true Skipped=false", upperX.Claimed, upperX.Skipped)
	}

	skipped := plan.Tasks[3]
	if skipped.Claimed || !skipped.Skipped {
		t.Errorf("[-] task: Claimed=%v Skipped=%v, want Claimed=false Skipped=true", skipped.Claimed, skipped.Skipped)
	}
}

// TestParseMarkdownPlan_EndToEnd: a parsed markdown plan verifies correctly.
func TestParseMarkdownPlan_EndToEnd(t *testing.T) {
	md := []byte("- [x] do it {symbol: Foo}\n- [x] ghost {symbol: Ghost}\n")
	plan := verify.ParseMarkdownPlan("svc", md)
	ev := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"Foo": {node("Foo", "a.go", 1, core.TierVerified)}},
		tests:   map[string][]string{"Foo": {"t"}},
	}
	rep := verify.VerifyPlan(plan, ev)
	if rep.Summary.Total != 2 {
		t.Fatalf("total = %d", rep.Summary.Total)
	}
	byID := map[string]verify.TaskVerdict{}
	for _, tv := range rep.Tasks {
		byID[tv.TaskID] = tv
	}
	if byID["md-001"].Label != "verified" {
		t.Fatalf("md-001 = %s", byID["md-001"].Label)
	}
	if byID["md-002"].Label != "claimed-but-absent" {
		t.Fatalf("md-002 = %s", byID["md-002"].Label)
	}
}

// TestParseAnnotation_BracesInQuotedValues pins the O4 fix: the feature's own
// vocabulary is the landmine — an author explaining the notation writes
// "{symbol, package}" inside a quoted skip: value, and the brace-blind
// scanner used to hand the row's whole contract (id included) to the void.
func TestParseAnnotation_BracesInQuotedValues(t *testing.T) {
	md := []byte("# p\n\n" +
		"- [ ] Never write braces in values {id: T-1, symbol: Foo, package: pkg/foo, skip_test: true, skip: \"an inner {symbol, package} block silently voids the row\"}\n" +
		"- [ ] Close-brace inside a value {id: T-2, skip_test: true, skip: \"see the {a, b} rule\"}\n")
	p := verify.ParseMarkdownPlan("svc", md)
	if len(p.Tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(p.Tasks))
	}
	t1 := p.Tasks[0]
	if t1.ID != "T-1" {
		t.Fatalf("T-1 lost its author-owned id (got %q) — the O4 md-NNN downgrade", t1.ID)
	}
	if t1.Expect.Symbol != "Foo" || t1.Expect.Package != "pkg/foo" {
		t.Errorf("T-1 contract keys lost: symbol=%q pkg=%q", t1.Expect.Symbol, t1.Expect.Package)
	}
	if !t1.Expect.SkipTest || !strings.Contains(t1.Expect.SkipReason, "{symbol, package}") {
		t.Errorf("T-1 skip contract mangled: skipTest=%v reason=%q", t1.Expect.SkipTest, t1.Expect.SkipReason)
	}
	if strings.Contains(t1.Title, "{") || strings.Contains(t1.Title, "skip") {
		t.Errorf("T-1 annotation leaked into the title: %q", t1.Title)
	}
	if p.Tasks[1].ID != "T-2" {
		t.Errorf("T-2 lost its id (got %q)", p.Tasks[1].ID)
	}
}

// TestLintPlan_UnparseableAnnotationIsLoud pins the second half of O4: a
// trailing brace block that parses to zero keys is a named lint warning,
// never a silent no-contract downgrade.
func TestLintPlan_UnparseableAnnotationIsLoud(t *testing.T) {
	md := []byte("# p\n\n- [ ] Broken block {id T-9 symbol Foo}\n")
	p := verify.ParseMarkdownPlan("svc", md)
	if len(p.Tasks) != 1 {
		t.Fatalf("got %d tasks", len(p.Tasks))
	}
	warns := verify.LintPlan(p)
	found := false
	for _, w := range warns {
		if strings.Contains(w, "annotation") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no annotation warning in %v — the silent-void failure mode survives", warns)
	}
}

// TestOracleAnnotationMangledLintsLoud (EP-2): intent conditions carry
// `{oracle: …}` blocks parsed by this same tokenizer (ADR-026 §3). The
// suspect-key set must know the oracle keys, or a mangled oracle block
// yields zero keys, reads as prose, and vanishes — the O4 silent-void class
// that once cost a plan row its id and its whole contract.
func TestOracleAnnotationMangledLintsLoud(t *testing.T) {
	// Well-formed: the keys parse and nothing is suspect.
	kv, title, suspect := verify.ParseAnnotation(
		`XT-6b lands {oracle: index, service: fathom, symbol: tsengineCppExtractor, present: true}`,
	)
	if suspect != "" {
		t.Errorf("a well-formed oracle block must not be suspect: %q", suspect)
	}
	if kv["oracle"] != "index" || kv["service"] != "fathom" || kv["present"] != "true" {
		t.Errorf("oracle keys not parsed: %#v", kv)
	}
	if title != "XT-6b lands" {
		t.Errorf("title = %q, want the condition text without the block", title)
	}

	// Mangled (no colon after the key) yields no keys — but the block names
	// an oracle key, so it must be reported as suspect, never dropped.
	_, _, suspect = verify.ParseAnnotation(`a second repo needs budgets {oracle lexical paths ".fathom/capabilities.cue"}`)
	if suspect == "" {
		t.Error("a mangled block naming oracle keys must lint loud, not vanish (O4)")
	}

	// Prose braces stay prose.
	_, _, suspect = verify.ParseAnnotation(`the shape is {a, b} in the docs`)
	if suspect != "" {
		t.Errorf("prose braces must not be reported as a mangled annotation: %q", suspect)
	}

	// Quoted braces inside a value never terminate the block (O4 proper).
	kv, _, _ = verify.ParseAnnotation(`row {oracle: lexical, pattern: "budgets: {list, map}", min: 1}`)
	if kv["pattern"] != "budgets: {list, map}" || kv["min"] != "1" {
		t.Errorf("quoted braces must survive as value content: %#v", kv)
	}
}

// ─── UpsertAnnotationField (RC-30: `thread promote --route`) ─────────────

func TestUpsertAnnotationField_AddsNewKey(t *testing.T) {
	line := "- [ ] Wire the router {id: RC-30-1, symbol: cmdRoutes, package: cmd/fathom}"
	got, ok := verify.UpsertAnnotationField(line, "ctx", "cmd/fathom.cmdRoutes internal/mcp.registerDiscoveries")
	if !ok {
		t.Fatal("expected ok=true")
	}
	kv, title, suspect := verify.ParseAnnotation(got)
	if suspect != "" {
		t.Fatalf("result is suspect: %q (line: %q)", suspect, got)
	}
	if kv["ctx"] != "cmd/fathom.cmdRoutes internal/mcp.registerDiscoveries" {
		t.Errorf("ctx = %q, got line %q", kv["ctx"], got)
	}
	// Every pre-existing key survives untouched.
	if kv["id"] != "RC-30-1" || kv["symbol"] != "cmdRoutes" || kv["package"] != "cmd/fathom" {
		t.Errorf("existing keys mangled: %#v (line: %q)", kv, got)
	}
	if title != "- [ ] Wire the router" {
		t.Errorf("title = %q", title)
	}
}

func TestUpsertAnnotationField_ReplacesExistingKeyInPlace(t *testing.T) {
	line := `- [ ] Row {id: RC-30-2, ctx: "old.Pointer", package: cmd/fathom}`
	got, ok := verify.UpsertAnnotationField(line, "ctx", "new.Pointer another.Pointer")
	if !ok {
		t.Fatal("expected ok=true")
	}
	kv, _, suspect := verify.ParseAnnotation(got)
	if suspect != "" {
		t.Fatalf("result is suspect: %q (line: %q)", suspect, got)
	}
	if kv["ctx"] != "new.Pointer another.Pointer" {
		t.Errorf("ctx = %q, want the replaced value (line: %q)", kv["ctx"], got)
	}
	if kv["id"] != "RC-30-2" || kv["package"] != "cmd/fathom" {
		t.Errorf("other keys mangled: %#v (line: %q)", kv, got)
	}
}

func TestUpsertAnnotationField_QuotesValueContainingCommaOrBrace(t *testing.T) {
	line := "- [ ] Row {id: RC-30-3}"
	got, ok := verify.UpsertAnnotationField(line, "reason", "needs a, comma")
	if !ok {
		t.Fatal("expected ok=true")
	}
	kv, _, _ := verify.ParseAnnotation(got)
	if kv["reason"] != "needs a, comma" {
		t.Errorf("reason = %q, want the comma preserved as value content (line: %q)", kv["reason"], got)
	}
	if !strings.Contains(got, `"needs a, comma"`) {
		t.Errorf("a comma-carrying value must be quoted in the output: %q", got)
	}
}

func TestUpsertAnnotationField_AppendsBlockWhenLineHasNone(t *testing.T) {
	line := "- [ ] A plain row with no annotation yet"
	got, ok := verify.UpsertAnnotationField(line, "ctx", "pkg.Symbol")
	if !ok {
		t.Fatal("expected ok=true")
	}
	kv, title, suspect := verify.ParseAnnotation(got)
	if suspect != "" {
		t.Fatalf("result is suspect: %q (line: %q)", suspect, got)
	}
	if kv["ctx"] != "pkg.Symbol" {
		t.Errorf("ctx = %q (line: %q)", kv["ctx"], got)
	}
	if title != "- [ ] A plain row with no annotation yet" {
		t.Errorf("title = %q", title)
	}
}

func TestUpsertAnnotationField_RefusesAMangledBlock(t *testing.T) {
	// No colons at all — this is exactly the O4 "suspect" shape
	// (TestLintPlan_UnparseableAnnotationIsLoud's fixture): writing into it
	// would stack a second corruption on the first.
	line := "- [ ] Broken block {id T-9 symbol Foo}"
	got, ok := verify.UpsertAnnotationField(line, "ctx", "pkg.Symbol")
	if ok {
		t.Fatalf("expected ok=false for a mangled/suspect block, got %q", got)
	}
	if got != line {
		t.Errorf("a refused write must return the line UNCHANGED, got %q", got)
	}
}

func TestUpsertAnnotationField_RefusesADuplicateKey(t *testing.T) {
	line := "- [ ] Row {id: RC-30-4, ctx: foo, ctx: bar}"
	got, ok := verify.UpsertAnnotationField(line, "ctx", "baz")
	if ok {
		t.Fatalf("expected ok=false for an already-duplicated key, got %q", got)
	}
	if got != line {
		t.Errorf("a refused write must return the line UNCHANGED, got %q", got)
	}
}
