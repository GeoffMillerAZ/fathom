package verify

// refactorgrammar_test.go — CV-25 (ADR-044): the refactor-conservation plan
// grammar's new row keys (gone, tests, retires, surface_change, max, min,
// eq) and the plan-level "Baseline: <sha>" header line parse into typed
// fields, exactly like every other v2 annotation key.

import "testing"

const refactorGrammarMD = `# refactor plan
Baseline: 8c1e4f2a9b3d7e6501234567890abcdef123456

- [ ] Import gone {id: RG-1, gone: "import github.com/foo/bar"}
- [ ] Callers gone, tests allowed {id: RG-2, gone: callers OldFunc, package: internal/old, tests: allowed}
- [ ] Retires one key {id: RG-3, retires: "api:foo.Bar"}
- [ ] Retires many keys {id: RG-4, retires: "api:foo.Bar, api:foo.Baz*"}
- [ ] Surface change {id: RG-5, surface_change: "json:foo.Thing.field"}
- [ ] Bounded pattern {id: RG-6, oracle: lexical, path: notes.txt, pattern: "x", min: 2, max: 5}
- [ ] Exact bound {id: RG-7, oracle: lexical, path: notes.txt, pattern: "x", eq: 3}
`

func TestParseMarkdownPlan_RefactorGrammarKeys(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte(refactorGrammarMD))
	if p.Baseline != "8c1e4f2a9b3d7e6501234567890abcdef123456" {
		t.Fatalf("Plan.Baseline = %q, want the header sha", p.Baseline)
	}

	byID := map[string]Task{}
	for _, tk := range p.Tasks {
		byID[tk.ID] = tk
	}

	if g := byID["RG-1"].Gone; g != `import github.com/foo/bar` {
		t.Errorf("RG-1 Gone = %q", g)
	}
	rg2 := byID["RG-2"]
	if rg2.Gone != "callers OldFunc" || rg2.Expect.Package != "internal/old" || rg2.Tests != "allowed" {
		t.Errorf("RG-2 parsed wrong: %+v", rg2)
	}
	if got := byID["RG-3"].Retires; len(got) != 1 || got[0] != "api:foo.Bar" {
		t.Errorf("RG-3 Retires = %v", got)
	}
	if got := byID["RG-4"].Retires; len(got) != 2 || got[0] != "api:foo.Bar" || got[1] != "api:foo.Baz*" {
		t.Errorf("RG-4 Retires = %v", got)
	}
	if got := byID["RG-5"].SurfaceChange; len(got) != 1 || got[0] != "json:foo.Thing.field" {
		t.Errorf("RG-5 SurfaceChange = %v", got)
	}
	rg6 := byID["RG-6"]
	if rg6.Min == nil || *rg6.Min != 2 || rg6.Max == nil || *rg6.Max != 5 {
		t.Errorf("RG-6 Min/Max = %v/%v, want 2/5", rg6.Min, rg6.Max)
	}
	rg7 := byID["RG-7"]
	if rg7.Eq == nil || *rg7.Eq != 3 {
		t.Errorf("RG-7 Eq = %v, want 3", rg7.Eq)
	}

	// The new keys must be recognised — a row using ONLY them must never
	// register as a suspect annotation (O4).
	for _, id := range []string{"RG-1", "RG-2", "RG-3", "RG-4", "RG-5", "RG-6", "RG-7"} {
		if byID[id].AnnotationSuspect != "" {
			t.Errorf("%s: unexpected AnnotationSuspect %q", id, byID[id].AnnotationSuspect)
		}
	}
}

func TestParseMarkdownPlan_NoBaselineHeaderLeavesPlanBaselineEmpty(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte("# plan\n- [ ] Row {id: A}\n"))
	if p.Baseline != "" {
		t.Errorf("Baseline = %q, want empty (no header line)", p.Baseline)
	}
}
