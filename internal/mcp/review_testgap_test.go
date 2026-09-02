package mcp_test

import (
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// BD-4 (docs/plans/blast-depth.md): review_diff composes test_impact —
// affected_tests (Test* functions statically reaching a changed symbol,
// with file:line) and test_gap (changed symbols no test reaches) — in the
// same verdict. Fixture: TestFoo → A → B → C; Orphan is unreached.
func TestReviewDiff_TestImpactComposed(t *testing.T) {
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}},
	}
	srv := mcp.NewServer(fake)
	call := func(files []string) map[string]any {
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 3, Method: "tools/call",
			Params: map[string]any{"name": "review_diff", "arguments": map[string]any{"service": "svc", "changed_files": files}},
		})
		p := extToolPayload(t, resp)
		ti, ok := p["test_impact"].(map[string]any)
		if !ok {
			t.Fatalf("test_impact missing from review_diff payload: %v", p)
		}
		return ti
	}

	ti := call([]string{"pkg/a.go", "pkg/orphan.go"})
	if ti["checked"] != true {
		t.Fatalf("test_impact.checked = %v", ti["checked"])
	}
	tests := ti["affected_tests"].([]any)
	if len(tests) != 1 {
		t.Fatalf("affected_tests = %v, want TestFoo", tests)
	}
	tf := tests[0].(map[string]any)
	if tf["test"] != "svc::pkg.TestFoo" || tf["citation"] != "pkg/a_test.go#L1-L1" || tf["line"] != float64(1) {
		t.Errorf("affected test = %v: want key + file:line citation", tf)
	}
	if r := tf["reaches"].([]any); len(r) != 2 || r[0] != "svc::pkg.A" || r[1] != "svc::pkg.B" {
		t.Errorf("reaches = %v, want [A, B] sorted", r)
	}
	gap := ti["test_gap"].([]any)
	if len(gap) != 1 || gap[0].(map[string]any)["symbol"] != "svc::pkg.Orphan" || gap[0].(map[string]any)["citation"] != "pkg/orphan.go#L1-L1" {
		t.Errorf("test_gap = %v, want Orphan with its citation", gap)
	}

	// A clean change (every symbol reached) reports an empty gap, never null.
	clean := call([]string{"pkg/c.go"})
	if g, ok := clean["test_gap"].([]any); !ok || len(g) != 0 {
		t.Errorf("clean test_gap = %v, want []", clean["test_gap"])
	}

	// Determinism.
	a, _ := json.Marshal(call([]string{"pkg/a.go", "pkg/orphan.go"}))
	b, _ := json.Marshal(ti)
	if string(a) != string(b) {
		t.Fatalf("test_impact not deterministic:\n%s\n%s", a, b)
	}
}
