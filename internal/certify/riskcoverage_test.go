package certify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// coverageRepo writes files under a fresh repository root.
func coverageRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// coverageStore seeds a store with symbols (two ranged Go functions, one
// unranged JS function) and their stored centrality.
func coverageStore(t *testing.T) store.Store {
	t.Helper()
	ctx := context.Background()
	st := store.NewMemoryStore()
	obs := []core.Observation{
		{ServiceID: "svc", Package: "p", Symbol: "Hot", SourceKind: core.KindGoFunc, SourceRef: "p/hot.go", Line: 3, EndLine: 9, EvidenceTier: core.TierVerified},
		{ServiceID: "svc", Package: "p", Symbol: "Cold", SourceKind: core.KindGoFunc, SourceRef: "p/hot.go", Line: 11, EndLine: 15, EvidenceTier: core.TierVerified},
		{ServiceID: "svc", Package: "web", Symbol: "render", SourceKind: core.SourceKind("JSFunction"), SourceRef: "web/app.js", Line: 1, EvidenceTier: core.TierVerified},
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSymbolCentrality(ctx, "svc", "wm1", []store.SymbolCentrality{
		{Symbol: "svc::p/p.Hot", File: "p/hot.go", Callers: 4, Centrality: 1},
		{Symbol: "svc::p/p.Cold", File: "p/hot.go", Callers: 2, Centrality: 0.5},
		{Symbol: "svc::web/web.render", File: "web/app.js", Callers: 1, Centrality: 0.25},
	}); err != nil {
		t.Fatal(err)
	}
	return st
}

func factor(t *testing.T, rep RiskReport, symbol string) RiskFactor {
	t.Helper()
	for _, r := range rep.Rows {
		if r.Symbol == symbol {
			return r.Factors[0]
		}
	}
	t.Fatalf("no row for %s in %+v", symbol, rep.Rows)
	return RiskFactor{}
}

// TestCoverageForRiskReadsNamedSources proves the risk rank's coverage
// join (TQ-26; ADR-051 decision 1): the sources a repository names under
// [tests] results_dir — the attested Go profile pair and a report of a
// known name — are read as files, parsed and joined by joinForeignBlocks
// (by line range where symbols carry one, else at file level, named), and
// the answer names what it read. An unusable pair is named, never used.
func TestCoverageForRiskReadsNamedSources(t *testing.T) {
	ctx := context.Background()
	const sha = "0123456789abcdef0123456789abcdef01234567"
	files := map[string]string{
		".fathom/config.toml":    "[tests]\nresults_dir = \"out\"\n",
		"out/coverage.meta.json": `{"schema": "fathom.coverage/v1", "commit": "` + sha + `", "dirty": false, "race": false, "profile": "coverage.out", "passed": true}`,
		// Hot: 3 of 4 statements ran (one block duplicated by -coverpkg,
		// merged by its maximum count); Cold: none ran.
		"out/coverage.out": "mode: set\n" +
			"example.com/m/p/hot.go:4.2,5.10 2 1\n" +
			"example.com/m/p/hot.go:6.2,7.10 1 1\n" +
			"example.com/m/p/hot.go:8.2,8.10 1 0\n" +
			"example.com/m/p/hot.go:4.2,5.10 2 0\n" +
			"example.com/m/p/hot.go:12.2,14.10 3 0\n",
		"out/lcov.info": "SF:/ci/checkout/web/app.js\nDA:1,1\nDA:2,0\nDA:3,1\nDA:4,1\nend_of_record\n",
	}
	root := coverageRepo(t, files)
	st := coverageStore(t)

	sources := CoverageSources(root)
	if len(sources) != 2 || sources[0].String() != "go-profile=out/coverage.meta.json" || sources[1].String() != "lcov=out/lcov.info" {
		t.Fatalf("CoverageSources = %+v, want the Go pair then lcov.info", sources)
	}
	rep, err := RiskWeightedCoverage(ctx, st, "svc", root, RiskOptions{Sources: sources})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.CoverageSupplied || rep.CoverageSource != "go-profile=out/coverage.meta.json (commit 0123456789ab), lcov=out/lcov.info" || len(rep.CoverageUnusable) != 0 {
		t.Fatalf("report coverage = supplied %v source %q unusable %v", rep.CoverageSupplied, rep.CoverageSource, rep.CoverageUnusable)
	}
	if f := factor(t, rep, "svc::p/p.Hot"); f.Value != 0.25 || !strings.Contains(f.Cite, "go-profile=out/coverage.meta.json") || !strings.Contains(f.Cite, "75%") {
		t.Errorf("Hot uncovered_share = %+v, want 0.25 citing the Go pair at 75%%", f)
	}
	if f := factor(t, rep, "svc::p/p.Cold"); f.Value != 1 || !strings.Contains(f.Cite, "0%") {
		t.Errorf("Cold uncovered_share = %+v, want 1 at 0%%", f)
	}
	if f := factor(t, rep, "svc::web/web.render"); f.Value != 0.25 || !strings.Contains(f.Cite, "lcov=out/lcov.info") || !strings.Contains(f.Cite, "file-level") {
		t.Errorf("render uncovered_share = %+v, want 0.25 at file level citing lcov", f)
	}

	// The portfolio caller reads the same named sources and says so.
	p, err := ClassifyTests(ctx, st, "svc", root)
	if err != nil {
		t.Fatal(err)
	}
	if !p.CoverageSupplied || p.CoverageSource != rep.CoverageSource {
		t.Errorf("portfolio coverage = %v %q, want the rank's source", p.CoverageSupplied, p.CoverageSource)
	}

	// A pair whose run failed is named and never used; the rank says so.
	bad := coverageRepo(t, map[string]string{
		".fathom/config.toml":    "[tests]\nresults_dir = \"out\"\n",
		"out/coverage.meta.json": `{"schema": "fathom.coverage/v1", "commit": "` + sha + `", "profile": "coverage.out", "passed": false}`,
		"out/coverage.out":       "mode: set\nexample.com/m/p/hot.go:4.2,5.10 2 1\n",
	})
	rep, err = RiskWeightedCoverage(ctx, st, "svc", bad, RiskOptions{Sources: CoverageSources(bad)})
	if err != nil {
		t.Fatal(err)
	}
	if rep.CoverageSupplied || len(rep.CoverageUnusable) != 1 || !strings.Contains(rep.CoverageUnusable[0], "did not pass") {
		t.Errorf("failed-run pair: supplied %v unusable %v, want unsupplied naming the failed run", rep.CoverageSupplied, rep.CoverageUnusable)
	}
	if f := factor(t, rep, "svc::p/p.Hot"); f.Value != 1 || !strings.Contains(f.Cite, "no coverage supplied") {
		t.Errorf("Hot with an unusable pair = %+v, want treated as uncovered", f)
	}

	// No source named: as today.
	none := coverageRepo(t, map[string]string{"README.md": "x\n"})
	if got := CoverageSources(none); len(got) != 0 {
		t.Errorf("a repository naming nothing: %+v", got)
	}
	// A results_dir that leaves the repository is refused by name.
	esc := coverageRepo(t, map[string]string{".fathom/config.toml": "[tests]\nresults_dir = \"../elsewhere\"\n"})
	if got := CoverageSources(esc); len(got) != 1 || got[0].Problem == "" {
		t.Errorf("an escaping results_dir: %+v, want one named problem", got)
	}
}
