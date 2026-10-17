package mcp

import (
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// testimpact_rank_test.go — O1128: test_impact orders its tests by hops
// from the symbol, then package proximity, then name, so a test that calls
// the symbol directly is never the one the budget cuts while a deeper one
// is shown; the elision line names the count and the exact offset argument
// that lists the rest, and paging with it lists them all.

func rankFixtureServer(farTests int) *Server {
	x := core.SymbolNode{Service: "svc", Package: "pkg", Name: "X", Kind: core.KindGoFunc, FilePath: "pkg/x.go", Line: 1}
	h := core.SymbolNode{Service: "svc", Package: "pkg", Name: "H", Kind: core.KindGoFunc, FilePath: "pkg/h.go", Line: 1, Calls: []string{"X"}}
	// A direct test whose key sorts after every far one ("pkg/" > "far/").
	direct := core.SymbolNode{Service: "svc", Package: "pkg", Name: "TestZDirect", Kind: core.KindGoFunc, FilePath: "pkg/x_test.go", Line: 5, Calls: []string{"X"}}
	// A two-hop test in the symbol's own package.
	near := core.SymbolNode{Service: "svc", Package: "pkg", Name: "TestYNear", Kind: core.KindGoFunc, FilePath: "pkg/x_test.go", Line: 9, Calls: []string{"pkg.H"}}
	nodes := []core.SymbolNode{x, h, direct, near}
	for i := 0; i < farTests; i++ {
		nodes = append(nodes, core.SymbolNode{
			Service: "svc", Package: "away", Name: fmt.Sprintf("TestA%04d", i), Kind: core.KindGoFunc,
			FilePath: "far/away/away_test.go", Line: i + 1, Calls: []string{"pkg.H"},
		})
	}
	return NewServer(&multiSvcBackend{svcs: map[string]*svcFixture{"svc": {nodes: nodes, indexCommit: "c1", path: "/repo"}}})
}

func testsOf(t *testing.T, payload map[string]any) []string {
	t.Helper()
	raw, ok := payload["tests"].([]any)
	if !ok {
		t.Fatalf("tests missing or wrong type: %v", payload)
	}
	out := make([]string, len(raw))
	for i, r := range raw {
		out[i] = r.(string)
	}
	return out
}

func TestTestImpact_RanksByDistanceAndPagesTheRest(t *testing.T) {
	const far = 1200
	srv := rankFixtureServer(far)
	first := callTool(t, srv, "test_impact", map[string]any{"service": "svc", "symbol": "X"})
	page := testsOf(t, first)
	if len(page) < 2 || len(page) >= far+2 {
		t.Fatalf("first page holds %d tests, want a budgeted page of the %d", len(page), far+2)
	}
	if page[0] != "svc::pkg/pkg.TestZDirect" {
		t.Errorf("tests[0] = %q, want the direct test first (one hop), whatever its name", page[0])
	}
	if page[1] != "svc::pkg/pkg.TestYNear" {
		t.Errorf("tests[1] = %q, want the two-hop test in the symbol's own package before the far ones", page[1])
	}
	if page[2] != "svc::far/away/away.TestA0000" {
		t.Errorf("tests[2] = %q, want the far two-hop tests by name", page[2])
	}

	line, _ := first["elision"].(string)
	rest := far + 2 - len(page)
	wantCount := fmt.Sprintf("elided: %d more", rest)
	wantArg := fmt.Sprintf("offset: %d", len(page))
	if !strings.Contains(line, wantCount) || !strings.Contains(line, wantArg) {
		t.Errorf("elision = %q, want the count (%q) and the exact argument for the rest (%q)", line, wantCount, wantArg)
	}

	// Paging with the named argument lists every test exactly once.
	all := append([]string(nil), page...)
	seen := map[string]bool{}
	for _, k := range all {
		seen[k] = true
	}
	for calls := 0; len(all) < far+2 && calls < 20; calls++ {
		next := callTool(t, srv, "test_impact", map[string]any{"service": "svc", "symbol": "X", "offset": len(all)})
		p := testsOf(t, next)
		if len(p) == 0 {
			t.Fatalf("offset %d returned no tests with %d of %d listed", len(all), len(all), far+2)
		}
		for _, k := range p {
			if seen[k] {
				t.Fatalf("offset %d repeats %s", len(all), k)
			}
			seen[k] = true
		}
		all = append(all, p...)
	}
	if len(all) != far+2 {
		t.Errorf("paging listed %d tests, want all %d", len(all), far+2)
	}

	// Deterministic: the same call answers the same bytes.
	again := callTool(t, srv, "test_impact", map[string]any{"service": "svc", "symbol": "X"})
	if fmt.Sprint(testsOf(t, again)) != fmt.Sprint(page) || again["elision"] != first["elision"] {
		t.Error("two identical test_impact calls differ")
	}
}
