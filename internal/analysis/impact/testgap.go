package impact

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestReach is one Test* function that statically reaches at least one
// changed symbol, with file:line evidence (BD-4, docs/plans/blast-depth.md).
type TestReach struct {
	Test     string   // canonical key of the test function
	File     string   // repo-relative file the test lives in
	Line     int      // 1-based definition line (0 when the index has none)
	Citation string   // core.CiteRef form "path#Lstart-Lend", or the bare path
	Reaches  []string // sorted canonical keys of the changed symbols it reaches
	// ViaDispatch is true when the shortest path to at least one reached
	// symbol crossed an interface-dispatch edge (callgraph.ReachDetail).
	ViaDispatch bool
}

// SymbolGap is one changed symbol that no test statically reaches.
type SymbolGap struct {
	Symbol   string
	File     string
	Line     int
	Citation string
}

// TestGapResult is the test-impact composition review_diff and `fathom
// review` report: which tests to run for a change, and which changed
// symbols are unprotected. Checked is false only when the service has no
// indexed symbols (the same honesty rule review's impact dimension uses);
// an indexed change with an empty gap is a real clean pass.
type TestGapResult struct {
	Checked       bool
	Reason        string
	AffectedTests []TestReach
	Gap           []SymbolGap
}

// TestGap composes callgraph.TestImpactDetail over every symbol defined in
// changedFiles (following typed edges like BlastRadiusWithTypedEdges):
// AffectedTests is every Test* function reaching at least one changed
// symbol, Gap every changed symbol no test reaches. Test functions
// themselves are never "gap" (a test that reaches nothing is not
// unprotected code). Pure and deterministic: every slice is sorted.
func TestGap(nodes []core.SymbolNode, changedFiles []string, typed []core.Edge) TestGapResult {
	if len(nodes) == 0 {
		return TestGapResult{Reason: "no symbols indexed for this service — ingest it before reviewing", AffectedTests: []TestReach{}, Gap: []SymbolGap{}}
	}
	changed := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = true
	}
	g := callgraph.Build(nodes)
	if len(typed) > 0 {
		callgraph.AddTypedEdges(g, typed)
	}
	byKey := make(map[string]core.SymbolNode, len(nodes))
	var seeds []string
	for _, n := range nodes {
		byKey[n.Key()] = n
		if changed[n.FilePath] && !strings.HasPrefix(n.Name, "Test") {
			seeds = append(seeds, n.Key())
		}
	}
	sort.Strings(seeds)

	reached := map[string]*TestReach{}
	res := TestGapResult{Checked: true, AffectedTests: []TestReach{}, Gap: []SymbolGap{}}
	for _, seed := range seeds {
		details := callgraph.TestImpactDetail(g, seed)
		if len(details) == 0 {
			n := byKey[seed]
			res.Gap = append(res.Gap, SymbolGap{Symbol: seed, File: n.FilePath, Line: n.Line, Citation: cite(n.FilePath, n.Line)})
			continue
		}
		for _, d := range details {
			tr := reached[d.Key]
			if tr == nil {
				n := byKey[d.Key]
				tr = &TestReach{Test: d.Key, File: n.FilePath, Line: n.Line, Citation: cite(n.FilePath, n.Line)}
				reached[d.Key] = tr
			}
			tr.Reaches = append(tr.Reaches, seed)
			tr.ViaDispatch = tr.ViaDispatch || d.ViaDispatch
		}
	}
	for _, tr := range reached {
		sort.Strings(tr.Reaches)
		res.AffectedTests = append(res.AffectedTests, *tr)
	}
	sort.Slice(res.AffectedTests, func(i, j int) bool { return res.AffectedTests[i].Test < res.AffectedTests[j].Test })
	sort.Slice(res.Gap, func(i, j int) bool { return res.Gap[i].Symbol < res.Gap[j].Symbol })
	return res
}

// Payload renders the result as the ONE JSON shape both review_diff's
// `test_impact` field and `fathom review --json` emit, so the two surfaces
// never drift (the same discipline as review.Verdict.Payload). Arrays are
// never null.
func (r TestGapResult) Payload() map[string]any {
	tests := make([]any, len(r.AffectedTests))
	for i, t := range r.AffectedTests {
		reaches := make([]any, len(t.Reaches))
		for j, s := range t.Reaches {
			reaches[j] = s
		}
		m := map[string]any{"test": t.Test, "file": t.File, "citation": t.Citation, "reaches": reaches}
		if t.Line > 0 {
			m["line"] = t.Line
		}
		if t.ViaDispatch {
			m["via_dispatch"] = true
		}
		tests[i] = m
	}
	gap := make([]any, len(r.Gap))
	for i, s := range r.Gap {
		m := map[string]any{"symbol": s.Symbol, "file": s.File, "citation": s.Citation}
		if s.Line > 0 {
			m["line"] = s.Line
		}
		gap[i] = m
	}
	out := map[string]any{"checked": r.Checked, "affected_tests": tests, "test_gap": gap}
	if !r.Checked {
		out["reason"] = r.Reason
	}
	return out
}

func cite(file string, line int) string {
	if file == "" {
		return ""
	}
	if line < 1 {
		return file
	}
	return core.CiteRef(file, line, line)
}
