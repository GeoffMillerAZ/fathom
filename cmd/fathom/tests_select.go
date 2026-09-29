package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/coverage"
	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/testlayer"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// cmdTestsSelect implements "fathom tests select" (VB-33, docs/vision/29-
// verbs-not-lookups.md §3 wave J, docs/plans/verbs.md): which Test*
// functions to actually run for a diff, ranked, plus the changed symbols
// no test reaches.
//
// Reuses the SAME diff→changed-symbols composition `fathom review` and
// review_diff's test_impact dimension already use (impact.TestGap over
// impact.TestGapResult — the git-diff-to-changed-files step mirrors
// cmd/fathom/review.go's runReviewBody exactly): every AffectedTest is a
// Test* function the call graph (syntax edges, plus typed edges when the
// index has them) says statically reaches at least one changed symbol;
// every Gap entry is a changed symbol no test reaches at all.
//
// Ranking (rankTestSelection) orders AffectedTests by, in priority order:
//
//  1. direct reach before a reach that crossed a typed interface-dispatch
//     edge (ADR-021 §3) — a dispatch-mediated reach is real evidence, but
//     weaker: the test's concrete call site does not literally name the
//     changed symbol.
//  2. WHEN --profile is supplied: descending mean statement-coverage
//     percent (coverage.JoinLayeredSymbols' Any view) over the changed
//     symbols the test reaches — a test whose reach is corroborated by a
//     real, executed run outranks one that is only a static, possibly dead,
//     edge. Silently skipped (no ranking effect) when no profile is given:
//     a standard Go cover profile carries no per-test attribution (see
//     internal/analysis/coverage's own doc comments), so this is a
//     symbol-level corroboration signal, never a per-test one.
//  3. test layer, unit before integration before e2e
//     (internal/testlayer.Classify) — run the cheaper tests first.
//  4. descending count of changed symbols reached (breadth).
//  5. test name, for a total deterministic order.
//
// Store-direct, mirroring cmdReview's and cmdTestsCoverage's own
// works-without-the-daemon discipline.
//
// In front of that reach ranking sits the certifying-first selection (T4,
// tests_select_certifying.go): the minimal set of tests that re-proves every
// certificate the diff touched, what it skipped, what no live test certifies,
// and the CI time it saves from attested run durations (recorded for
// economics). --safe adds every test that statically reaches a changed
// symbol to that selection.
//
//	fathom tests select --since <ref> [--safe] [--profile [layer=]path]... [--service S] [--store DB] [--json]
func cmdTestsSelect(args []string, stdout, stderr io.Writer) int {
	return runTestsSelect(args, defaultGitRunner, stdout, stderr)
}

func runTestsSelect(args []string, gitRunner func(args ...string) ([]byte, error), stdout, stderr io.Writer) int {
	fs := newFlagSet("tests select", stderr)
	since := fs.String("since", "", "git ref: rank tests for the diff since this ref (required)")
	var profileFlags profileFlagValues
	fs.Var(&profileFlags, "profile", "Go cover profile from `go test -coverprofile`, as `[layer=]path` (repeatable); when supplied, corroborating statement coverage over a test's reached symbols is used as a ranking tie-break (see cmdTestsSelect's doc comment)")
	service := fs.String("service", "", "service ID (default: resolved from the repo directory name, the same guess the Claude Code hooks use)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a table")
	safe := fs.Bool("safe", false, "also select every test that statically reaches a changed symbol, beside the minimal certifying-first set")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *since == "" {
		fmt.Fprintln(stderr, "error: tests select requires --since")
		fs.Usage()
		return 1
	}

	var profiles []coverage.LabeledProfile
	if len(profileFlags) > 0 {
		var err error
		profiles, err = loadLabeledProfiles(profileFlags)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}

	svc := *service
	if svc == "" {
		svc = defaultTestsCoverageService()
		if svc == "" {
			fmt.Fprintln(stderr, "error: tests select requires --service (could not guess one from the current directory)")
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

	out, gerr := gitRunner("diff", "--name-only", *since)
	if gerr != nil {
		fmt.Fprintf(stderr, "error: git diff --name-only %s: %v\n", *since, gerr)
		return 1
	}
	var changedFiles []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			changedFiles = append(changedFiles, line)
		}
	}

	gap := impact.TestGap(nodes, changedFiles, typed)

	var coverageSyms []coverage.SymbolLayerCoverage
	if len(profiles) > 0 {
		coverageSyms = review.ResolveCoverage(profiles, nodes)
	}

	ranked := rankTestSelection(nodes, gap.AffectedTests, coverageSyms, len(profiles) > 0)
	// TC-23 (design doc §5's "Ordering"): a test that already certifies a
	// target in the changed files ranks first, ahead of the reach ranking
	// above — best effort (an absent/unreadable certifications layer
	// degrades to no reordering at all, never an error: this is an
	// advisory tie-break, not the selection itself).
	claims, _ := c.ClaimsForService(ctx, svc, certify.Layer)
	ranked = rankCertifyingFirst(ranked, certifyingTestKeys(nodes, claims, changedFiles))
	invocations := groupTestInvocations(nodes, ranked)
	replay := loadTestSelectReplay(workspacePath("artifacts/tests-select/replay.jsonl"))

	// T4: the certifying-first selection, its time saved (TQ-16w), and the
	// record of that time for economics — a known estimate only, never a
	// fabricated zero.
	sel := selectCertifyingFirst(nodes, typed, claims, changedFiles, *safe)
	view := buildCertifyingSelectionView(ctx, st, svc, nodes, sel, *safe)
	if view.SavedKnown {
		row := timeSavedRow{
			At: testsSelectNow().UTC().Format(time.RFC3339), Service: svc, Since: *since, Safe: *safe,
			Selected: len(sel.Tests), Skipped: len(sel.Skipped),
			SelectedMS: view.Saved.SelectedMs, SkippedMS: view.Saved.SkippedMs, TotalMS: view.Saved.TotalMs,
			Percent: view.Saved.Percent, Missing: len(view.Saved.Missing),
		}
		if rerr := recordTimeSaved(testsSelectEconomicsPath(), row); rerr != nil {
			fmt.Fprintf(stderr, "warning: tests select: could not record the time saved for economics: %v\n", rerr)
		}
	}

	if *jsonOut {
		return renderTestsSelectJSON(stdout, *since, ranked, invocations, gap.Gap, replay, len(profiles) > 0, view)
	}
	renderCertifyingSelectionText(stdout, *since, view)
	renderTestsSelectTable(stdout, *since, ranked, invocations, gap.Gap, replay, len(profiles) > 0)
	return 0
}

// rankedTest is one AffectedTest with its ranking inputs resolved —
// rankTestSelection's output row.
type rankedTest struct {
	Test         string
	Package      string // directory the test file lives in ("go test ./<Package>")
	File         string
	Line         int
	Citation     string
	Layer        string
	LayerRank    int
	ViaDispatch  bool
	Reaches      []string
	CoverageMean float64 // mean Any.Percent over Reaches; meaningful only when haveCoverage
	haveCoverage bool
}

// testLayerOrder is the fixed run-cost ordering VB-33 ranks by: run the
// cheap layer first. A layer testlayer.Classify never returns by name here
// (a repo-configured custom layer beyond the built-in unit/integration/e2e
// trio) sorts after all three, alphabetically among themselves — see
// layerRankOf.
//
//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
var testLayerOrder = map[string]int{"unit": 0, "integration": 1, "e2e": 2}

func layerRankOf(layer string) int {
	if r, ok := testLayerOrder[layer]; ok {
		return r
	}
	return len(testLayerOrder)
}

// rankTestSelection turns impact.TestGap's AffectedTests into ranked,
// sorted rows — see cmdTestsSelect's doc comment for the exact ranking
// rule. Pure and deterministic. coverageSyms may be nil (no --profile);
// haveCoverage says whether the caller supplied at least one profile at
// all, distinct from coverageSyms being empty (a profile with no data for
// any reached symbol) — only the former disables the coverage ranking key
// entirely, matching the DIFFERENCE-BLIND-VS-NO-DATA distinction the rest
// of internal/analysis/coverage keeps.
func rankTestSelection(nodes []core.SymbolNode, affected []impact.TestReach, coverageSyms []coverage.SymbolLayerCoverage, haveCoverage bool) []rankedTest {
	byKey := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byKey[n.Key()] = n
	}
	covByKey := make(map[string]coverage.SymbolLayerCoverage, len(coverageSyms))
	for _, s := range coverageSyms {
		covByKey[s.Symbol] = s
	}

	out := make([]rankedTest, 0, len(affected))
	for _, t := range affected {
		layer := testlayer.Classify(t.File, nil)
		rt := rankedTest{
			Test: t.Test, File: t.File, Line: t.Line, Citation: t.Citation,
			Layer: layer, LayerRank: layerRankOf(layer),
			ViaDispatch: t.ViaDispatch, Reaches: append([]string(nil), t.Reaches...),
			haveCoverage: haveCoverage,
		}
		if n, ok := byKey[t.Test]; ok {
			rt.Package = path.Dir(n.FilePath)
		} else {
			rt.Package = path.Dir(t.File)
		}
		if haveCoverage {
			var sum float64
			for _, r := range t.Reaches {
				if cs, ok := covByKey[r]; ok {
					sum += cs.Any.Percent
				}
			}
			if len(t.Reaches) > 0 {
				rt.CoverageMean = sum / float64(len(t.Reaches))
			}
		}
		out = append(out, rt)
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.ViaDispatch != b.ViaDispatch {
			return !a.ViaDispatch // direct reach first
		}
		if haveCoverage && a.CoverageMean != b.CoverageMean {
			return a.CoverageMean > b.CoverageMean
		}
		if a.LayerRank != b.LayerRank {
			return a.LayerRank < b.LayerRank
		}
		if len(a.Reaches) != len(b.Reaches) {
			return len(a.Reaches) > len(b.Reaches)
		}
		return a.Test < b.Test
	})
	return out
}

// certifyingTestKeys resolves design doc §5's "tests that certify a
// target in the changed files": every certifications-layer claim
// (certify.Records) whose Scope intersects changedFiles names a
// certifying test by its BARE name (Record carries no symbol key — see
// certify.Record's own doc comment); this resolves each matching bare
// name back to every symbol key nodes gives it (ambiguous names resolve
// to more than one key, matching this advisory tie-break's own tolerance
// for approximation — a wrong extra promotion costs nothing but ordering,
// unlike a gate). claims or changedFiles empty yields an empty set, never
// an error: rankCertifyingFirst then reorders nothing.
func certifyingTestKeys(nodes []core.SymbolNode, claims []core.SemanticClaim, changedFiles []string) map[string]bool {
	changed := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = true
	}
	certifyingNames := map[string]bool{}
	for _, r := range certify.Records(claims) {
		for _, f := range r.Scope {
			if changed[f] {
				certifyingNames[r.Test] = true
				break
			}
		}
	}
	if len(certifyingNames) == 0 {
		return nil
	}
	out := map[string]bool{}
	for _, n := range nodes {
		if certifyingNames[n.Name] {
			out[n.Key()] = true
		}
	}
	return out
}

// rankCertifyingFirst (TC-23, design doc §5's "Ordering") stable-
// partitions ranked into two groups — tests in certifying (keyed by
// rankedTest.Test, i.e. a symbol key) first, everyone else after —
// preserving rankTestSelection's own relative order WITHIN each group
// (sort.SliceStable): "ranks tests that certify a target in the changed
// files first, THEN the existing reach ranking." Pure, advisory only: it
// never changes WHICH tests are selected, only their order. A nil/empty
// certifying set is a no-op copy of ranked in its original order.
func rankCertifyingFirst(ranked []rankedTest, certifying map[string]bool) []rankedTest {
	out := append([]rankedTest(nil), ranked...)
	if len(certifying) == 0 {
		return out
	}
	sort.SliceStable(out, func(i, j int) bool {
		return certifying[out[i].Test] && !certifying[out[j].Test]
	})
	return out
}

// testInvocation is one `go test` command line for a single package
// directory — VB-33's "emit a go test invocation (grouped per package)".
type testInvocation struct {
	Package string
	Tests   []string // bare Go test function names, sorted
	Command string   // "go test ./<Package> -run '^(TestA|TestB)$'"
}

// groupTestInvocations groups ranked's tests by their package directory,
// preserving each group's ranked order internally but emitting groups
// sorted by package path for a deterministic overall command list — the
// ranking answers "which tests, in what order of confidence"; grouping
// answers "how to actually invoke them," and the two are independent
// concerns; sorting Tests alphabetically within a group keeps `-run`'s
// regex stable regardless of ranking-order jitter within a tie.
func groupTestInvocations(nodes []core.SymbolNode, ranked []rankedTest) []testInvocation {
	byKey := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byKey[n.Key()] = n
	}
	byPkg := make(map[string][]string)
	var pkgs []string
	for _, rt := range ranked {
		name := rt.Test
		if n, ok := byKey[rt.Test]; ok {
			name = n.Name
		}
		if _, seen := byPkg[rt.Package]; !seen {
			pkgs = append(pkgs, rt.Package)
		}
		byPkg[rt.Package] = append(byPkg[rt.Package], name)
	}
	sort.Strings(pkgs)

	out := make([]testInvocation, 0, len(pkgs))
	for _, pkg := range pkgs {
		names := dedupSorted(byPkg[pkg])
		pkgArg := pkg
		if pkgArg == "." || pkgArg == "" {
			pkgArg = "."
		}
		quoted := make([]string, len(names))
		for i, n := range names {
			// A Go identifier never contains a regex metacharacter, but
			// quoting is what makes that a guarantee of the -run pattern
			// rather than an assumption about what a test can be named.
			quoted[i] = regexp.QuoteMeta(n)
		}
		out = append(out, testInvocation{
			Package: pkg,
			Tests:   names,
			Command: fmt.Sprintf("go test ./%s -run '^(%s)$'", pkgArg, strings.Join(quoted, "|")),
		})
	}
	return out
}

func dedupSorted(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// replayStats is the read-side of workspace/artifacts/tests-select/
// replay.jsonl (VB-35's future producer; never fabricated here): one line
// per historical replay, {"commit":"<sha>","selected":<n>,"total":<n>,
// "caught":<bool>} — caught is whether that commit's selection would have
// included a test that actually failed. PredictedCatchRate is Caught/Total
// replays; Total==0 means no ledger was found (or it was empty/unreadable)
// — the CLI omits the line entirely rather than printing a fabricated 0%
// or 100%.
type replayStats struct {
	Caught int
	Total  int
}

func (r replayStats) rate() float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.Caught) / float64(r.Total)
}

type replayLine struct {
	Commit   string `json:"commit"`
	Selected int    `json:"selected"`
	Total    int    `json:"total"`
	Caught   bool   `json:"caught"`
}

// loadTestSelectReplay reads path as newline-delimited JSON replayLine
// records — best effort: a missing file returns a zero replayStats (never
// an error; VB-35's replay ledger may simply not exist yet), a present but
// malformed line is skipped rather than aborting the whole read, so one bad
// line never hides every other one's evidence.
func loadTestSelectReplay(path string) replayStats {
	data, err := os.ReadFile(path)
	if err != nil {
		return replayStats{}
	}
	var stats replayStats
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rl replayLine
		if err := json.Unmarshal([]byte(line), &rl); err != nil {
			continue
		}
		stats.Total++
		if rl.Caught {
			stats.Caught++
		}
	}
	return stats
}

func renderTestsSelectTable(w io.Writer, since string, ranked []rankedTest, invocations []testInvocation, gap []impact.SymbolGap, replay replayStats, haveCoverage bool) {
	fmt.Fprintf(w, "tests select --since %s: %d test(s) ranked, %d changed symbol(s) with no test\n", since, len(ranked), len(gap))
	for i, rt := range ranked {
		via := ""
		if rt.ViaDispatch {
			via = " via interface dispatch"
		}
		cov := ""
		if haveCoverage {
			cov = fmt.Sprintf(" coverage=%.1f%%", rt.CoverageMean)
		}
		fmt.Fprintf(w, "  %d. %s (%s) [%s]%s reaches %s%s\n", i+1, rt.Test, rt.Citation, rt.Layer, cov, strings.Join(rt.Reaches, ", "), via)
	}
	fmt.Fprintln(w, "invocations:")
	for _, inv := range invocations {
		fmt.Fprintf(w, "  %s\n", inv.Command)
	}
	if len(gap) > 0 {
		fmt.Fprintln(w, "test_gap:")
		for _, g := range gap {
			fmt.Fprintf(w, "  - %s (%s): no test reaches it\n", g.Symbol, g.Citation)
		}
	}
	if replay.Total > 0 {
		fmt.Fprintf(w, "predicted catch-rate: %.0f%% (%d/%d historical replays)\n", replay.rate()*100, replay.Caught, replay.Total)
	}
}

func renderTestsSelectJSON(w io.Writer, since string, ranked []rankedTest, invocations []testInvocation, gap []impact.SymbolGap, replay replayStats, haveCoverage bool, view certifyingSelectionView) int {
	rankedItems := make([]any, len(ranked))
	for i, rt := range ranked {
		item := map[string]any{
			"test":     rt.Test,
			"file":     rt.File,
			"citation": rt.Citation,
			"layer":    rt.Layer,
			"reaches":  nonNilStrList(rt.Reaches),
		}
		if rt.Line > 0 {
			item["line"] = rt.Line
		}
		if rt.ViaDispatch {
			item["via_dispatch"] = true
		}
		if haveCoverage {
			item["coverage_percent"] = rt.CoverageMean
		}
		rankedItems[i] = item
	}
	invItems := make([]any, len(invocations))
	for i, inv := range invocations {
		invItems[i] = map[string]any{
			"package": inv.Package,
			"tests":   nonNilStrList(inv.Tests),
			"command": inv.Command,
		}
	}
	gapItems := make([]any, len(gap))
	for i, g := range gap {
		gapItems[i] = map[string]any{"symbol": g.Symbol, "file": g.File, "citation": g.Citation}
	}
	payload := map[string]any{
		"since":        since,
		"selection":    certifyingSelectionJSON(view),
		"ranked_tests": rankedItems,
		"invocations":  invItems,
		"test_gap":     gapItems,
	}
	if replay.Total > 0 {
		payload["replay"] = map[string]any{
			"caught":               replay.Caught,
			"total":                replay.Total,
			"predicted_catch_rate": replay.rate(),
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return 1
	}
	return 0
}
