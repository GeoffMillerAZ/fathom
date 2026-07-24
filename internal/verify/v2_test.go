package verify

import (
	"reflect"
	"strings"
	"testing"
)

const v2MD = `# Wave plan
- [x] Grammar lands [FR-001] {id: G2-1, symbol: ParseMarkdownPlan, package: internal/verify}
- [~] Long haul {id: W-2, reason: "kit-side done, donor NOT deleted"}
- [-] Later {id: W-3, disposition: deferred, reason: "waiting on O6", trigger: "skip share <30%"}
- [ ] Open item {id: W-4, symbol: GateRequireAll, skip_test: true, skip: "cmd oracle pending"}
- [x] Positional stays
`

func TestParseMarkdownPlan_V2(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte(v2MD))
	if len(p.Tasks) != 5 {
		t.Fatalf("got %d tasks, want 5", len(p.Tasks))
	}

	g := p.Tasks[0]
	if g.ID != "G2-1" || !g.Claimed || g.Expect.Symbol != "ParseMarkdownPlan" || g.Expect.Package != "internal/verify" {
		t.Errorf("G2-1 parsed wrong: %+v", g)
	}
	if !reflect.DeepEqual(g.SpecIDs, []string{"[FR-001]"}) {
		t.Errorf("spec IDs = %v, want [[FR-001]]", g.SpecIDs)
	}

	w2 := p.Tasks[1]
	if !w2.InProgress || w2.Claimed || w2.Skipped {
		t.Errorf("[~] must be in-progress only: %+v", w2)
	}
	if w2.Reason != "kit-side done, donor NOT deleted" {
		t.Errorf("quoted reason with comma mangled: %q", w2.Reason)
	}

	w3 := p.Tasks[2]
	if !w3.Skipped || w3.Disposition != "deferred" || w3.Trigger != "skip share <30%" {
		t.Errorf("deferred disposition parsed wrong: %+v", w3)
	}

	w4 := p.Tasks[3]
	if !w4.Expect.SkipTest || w4.Expect.SkipReason != "cmd oracle pending" {
		t.Errorf("skip justification parsed wrong: %+v", w4.Expect)
	}

	if p.Tasks[4].ID != "md-005" {
		t.Errorf("positional ID = %q, want md-005 (numbering counts all lines)", p.Tasks[4].ID)
	}
}

func TestParseAnnotation_QuotedValues(t *testing.T) {
	m, title := parseAnnotation(`Do it {reason: "a, b, c", trigger: plain}`)
	if title != "Do it" {
		t.Errorf("title = %q", title)
	}
	if m["reason"] != "a, b, c" {
		t.Errorf("quoted value = %q, want %q", m["reason"], "a, b, c")
	}
	if m["trigger"] != "plain" {
		t.Errorf("unquoted value = %q", m["trigger"])
	}
}

func TestLintPlan(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte(`# L
- [x] A {id: T-1, symbol: X, skip_test: true}
- [x] B {id: T-1, symbol: Y, skip_test: true, skip: "cited"}
- [x] C {id: T-3, disposition: refused}
- [ ] D {id: T-4, trigger: "later"}
- [-] E {id: T-5, disposition: sideways, reason: ok}
`))
	got := LintPlan(p)
	want := []string{
		"T-1: skip_test without a skip: justification",
		"T-1: duplicate task id (first used by \"A\")",
		"T-3: disposition \"refused\" requires a [-] checkbox",
		"T-3: disposition \"refused\" requires a reason",
		"T-4: trigger only applies to disposition: deferred",
		"T-5: unknown disposition \"sideways\" (want refused|deferred)",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d findings %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// Determinism.
	again := LintPlan(p)
	if !reflect.DeepEqual(got, again) {
		t.Error("LintPlan is not deterministic")
	}
}

func TestGateRequireAll(t *testing.T) {
	rep := Report{Tasks: []TaskVerdict{
		{TaskID: "a", Title: "done", Status: Pass, Claimed: true},
		{TaskID: "b", Title: "skipped", Status: Pass, Skipped: true},
		{TaskID: "c", Title: "wip", Status: Pass, InProgress: true},
		{TaskID: "d", Title: "open", Status: Pass},
	}}
	// Normal gate passes: nothing claimed is failing.
	if g := Gate(rep, false); !g.Pass {
		t.Fatalf("Gate should pass: %v", g.Failures)
	}
	g := GateRequireAll(rep, false)
	if g.Pass {
		t.Fatal("GateRequireAll must fail with open/in-progress tasks")
	}
	if len(g.Failures) != 2 {
		t.Fatalf("failures = %v, want 2 (c and d)", g.Failures)
	}
	// A claimed failure still fails too.
	rep.Tasks[0].Status = Fail
	rep.Tasks[0].Criteria = []Criterion{{Name: "symbol_present", Status: Fail, Detail: "gone"}}
	if g := GateRequireAll(rep, false); g.Pass || len(g.Failures) != 3 {
		t.Fatalf("claimed failure must also gate: %v", g.Failures)
	}
	// Complete plan passes.
	all := Report{Tasks: []TaskVerdict{
		{TaskID: "a", Status: Pass, Claimed: true},
		{TaskID: "b", Status: Pass, Skipped: true},
	}}
	if g := GateRequireAll(all, false); !g.Pass {
		t.Fatalf("complete plan must pass: %v", g.Failures)
	}
}

func TestParseMarkdownPlan_V1Unchanged(t *testing.T) {
	// v1 plans keep parsing identically: no v2 keys, positional IDs.
	p := ParseMarkdownPlan("svc", []byte("- [x] Old {symbol: Foo, skip_test: true}\n- [-] Skip me\n"))
	if p.Tasks[0].ID != "md-001" || !p.Tasks[0].Claimed || p.Tasks[0].Expect.Symbol != "Foo" {
		t.Errorf("v1 claimed line changed: %+v", p.Tasks[0])
	}
	if !p.Tasks[1].Skipped || p.Tasks[1].Disposition != "" {
		t.Errorf("v1 skip line changed: %+v", p.Tasks[1])
	}
}

func TestParseAnnotation_TrailingBlockRule(t *testing.T) {
	// GI-7: braces in the title (code literals, prose annotation examples)
	// must not hijack the contract — the annotation is the TRAILING block.
	m, title := parseAnnotation("JSONL beats (`Beat{Channel, Kind, At, Meta}`, injected clock) {id: FA-4, symbol: CheckLiveness}")
	if m["id"] != "FA-4" || m["symbol"] != "CheckLiveness" {
		t.Fatalf("trailing block not parsed: %v", m)
	}
	if !strings.Contains(title, "Beat{Channel, Kind, At, Meta}") {
		t.Errorf("title lost its code literal: %q", title)
	}
	// A prose example annotation earlier in the line stays prose.
	m2, _ := parseAnnotation("Prove `{symbol: SQLiteStore.PurgeSource}` flips {id: X-1, symbol: Real}")
	if m2["symbol"] != "Real" || m2["id"] != "X-1" {
		t.Errorf("prose example hijacked the contract: %v", m2)
	}
	// No annotation at all still returns nil.
	if m3, _ := parseAnnotation("plain title"); m3 != nil {
		t.Errorf("want nil map, got %v", m3)
	}
}
