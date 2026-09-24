package main

// certify_orphans.go — "fathom certify orphans" (TC-22, design doc §5): a
// list, never an action (test-kit's own rule is that a test you stop
// running is deleted, not hidden — this command only surfaces the
// candidates for a human to look at). A Test* symbol is an orphan when
// ALL THREE hold:
//
//  1. it certifies nothing — no compiled certifications-layer claim names
//     it at all (per-test marker OR suite-level #Certifies, either origin
//     — see certifyingTestSymbolKeys' own doc comment);
//  2. it reaches no symbol whose file changed in the last 180 days (a
//     single callgraph.ReachableAny walk per candidate against the whole
//     changed-symbol set, not one Reachable call per changed symbol);
//  3. it never failed or flaked in any ingested run (TestCaseRunsForSymbol
//     with no limit — a test with NO runs at all vacuously satisfies this;
//     "no evidence of failure" is not the same claim as "always passed",
//     and orphans never overstates it into one).
//
// Store-direct, mirroring cmdCertifyGaps'/cmdTestsSelect's own
// works-without-the-daemon discipline. The clock is injected
// (runCertifyOrphans's now parameter) so the 180-day cutoff is
// deterministic in tests — repo convention: never time.Now() in the
// logic a test needs to control.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func cmdCertifyOrphans(args []string, stdout, stderr io.Writer) int {
	return runCertifyOrphans(args, defaultGitRunner, time.Now, stdout, stderr)
}

// certifyOrphan is one reported orphan candidate.
type certifyOrphan struct {
	Test string
	File string
	Line int
}

func runCertifyOrphans(args []string, gitRunner func(args ...string) ([]byte, error), now func() time.Time, stdout, stderr io.Writer) int {
	fs := newFlagSet("certify orphans", stderr)
	service := fs.String("service", "", "service ID (default: resolved from the repo directory name)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a table")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	svc := *service
	if svc == "" {
		svc = defaultTestsCoverageService()
		if svc == "" {
			fmt.Fprintln(stderr, "error: certify orphans requires --service (could not guess one from the current directory)")
			return 1
		}
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	ctx := context.Background()

	nodes, err := c.ListAllSymbols(ctx, svc)
	if err != nil {
		fmt.Fprintf(stderr, "error: list symbols for service %q: %v\n", svc, err)
		return 1
	}
	typed, _ := c.TypedEdges(ctx, svc) // best effort: absent for a fixture/module with no typed pass

	claims, err := c.ClaimsForService(ctx, svc, certify.Layer)
	if err != nil {
		fmt.Fprintf(stderr, "error: certifications layer claims for %q: %v\n", svc, err)
		return 1
	}
	certifying := certifyingTestSymbolKeys(claims)

	out, gerr := gitRunner("log", "--since", now().AddDate(0, 0, -orphanWindowDays).Format(time.RFC3339), "--name-only", "--pretty=format:")
	if gerr != nil {
		fmt.Fprintf(stderr, "error: git log --since: %v\n", gerr)
		return 1
	}
	changedFiles := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			changedFiles[line] = true
		}
	}
	var recentSymbols []string
	for _, n := range nodes {
		if changedFiles[n.FilePath] {
			recentSymbols = append(recentSymbols, n.Key())
		}
	}

	g := callgraph.Build(nodes)
	callgraph.AddTypedEdges(g, typed)

	var orphans []certifyOrphan
	for _, n := range nodes {
		if n.Kind != core.KindGoFunc || !strings.HasPrefix(n.Name, "Test") {
			continue
		}
		key := n.Key()
		if certifying[key] {
			continue // condition 1 fails: it certifies something
		}
		runs, rerr := st.TestCaseRunsForSymbol(ctx, svc, key, 0)
		if rerr != nil {
			fmt.Fprintf(stderr, "error: test case runs for %q: %v\n", key, rerr)
			return 1
		}
		if everFailedOrFlaked(runs) {
			continue // condition 3 fails
		}
		if callgraph.ReachableAny(g, key, recentSymbols) {
			continue // condition 2 fails: reaches a recently changed symbol
		}
		orphans = append(orphans, certifyOrphan{Test: n.Name, File: n.FilePath, Line: n.Line})
	}
	sort.Slice(orphans, func(i, j int) bool {
		if orphans[i].Test != orphans[j].Test {
			return orphans[i].Test < orphans[j].Test
		}
		return orphans[i].File < orphans[j].File
	})

	if *jsonOut {
		return renderCertifyOrphansJSON(stdout, orphans)
	}
	renderCertifyOrphansTable(stdout, orphans)
	return 0
}

// orphanWindowDays is design doc §5's fixed "changed in the last 180
// days" window.
const orphanWindowDays = 180

// certifyingTestSymbolKeys returns the set of TestSymbolKeys (core.
// SymbolNode.Key() format) that certify AT LEAST ONE target, from either
// origin CompileCertifications considers (a per-test marker, or a
// suite-level #Certifies entry resolved to a symbol key at compile time —
// compile.go's own declaredEntry). certify.Record (evidence.go) does not
// carry the symbol key, only the test's bare name (ambiguous across
// packages), so this reads the compiled claim's own Meta.TestSymbolKey
// directly via the exported certify.Meta shape instead of going through
// Records — the SAME claim rows, read for the one field Records leaves
// out. A claim with no or unparseable Meta contributes nothing (the same
// silent-exclusion discipline certify.Records/ByTest already apply).
func certifyingTestSymbolKeys(claims []core.SemanticClaim) map[string]bool {
	out := map[string]bool{}
	for _, cl := range claims {
		if len(cl.Meta) == 0 {
			continue
		}
		var m certify.Meta
		if err := json.Unmarshal(cl.Meta, &m); err != nil {
			continue
		}
		if m.TestSymbolKey != "" {
			out[m.TestSymbolKey] = true
		}
	}
	return out
}

// everFailedOrFlaked reports whether any of runs (every attested run ever
// ingested for one test symbol, TestCaseRunsForSymbol with limit 0) has
// status "fail" or "flake". An empty runs (never ingested at all)
// VACUOUSLY satisfies "never failed" — orphans never claims "always
// passed" for a test with zero evidence, only "no evidence of failure".
func everFailedOrFlaked(runs []store.TestCaseRun) bool {
	for _, r := range runs {
		if r.Status == "fail" || r.Status == "flake" {
			return true
		}
	}
	return false
}

func renderCertifyOrphansTable(w io.Writer, orphans []certifyOrphan) {
	if len(orphans) == 0 {
		fmt.Fprintln(w, "certify orphans: none found")
		return
	}
	fmt.Fprintf(w, "certify orphans: %d found (certifies nothing, reaches no symbol changed in %d days, never failed/flaked)\n", len(orphans), orphanWindowDays)
	for _, o := range orphans {
		fmt.Fprintf(w, "  %s (%s:%d)\n", o.Test, o.File, o.Line)
	}
}

func renderCertifyOrphansJSON(w io.Writer, orphans []certifyOrphan) int {
	items := make([]map[string]any, 0, len(orphans))
	for _, o := range orphans {
		items = append(items, map[string]any{"test": o.Test, "file": o.File, "line": o.Line})
	}
	payload := map[string]any{"orphans": items, "count": len(orphans)}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return 1
	}
	return 0
}
