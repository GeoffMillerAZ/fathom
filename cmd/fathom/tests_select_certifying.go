package main

// tests_select_certifying.go — `fathom tests select`'s certifying-first
// selection (TQ-15w, SR-50; docs/design/test-intelligence.md §3 T4) and the
// CI time it saves (TQ-16w). The reach ranking cmdTestsSelect documents
// stays: it is every test that statically reaches the diff, the set --safe
// selects. Certifying-first is the smaller set in front of it: the minimal
// tests that re-prove every certificate the diff touched.
//
// Selection is advice to CI and never marks a certificate live (I43, TQ-X2):
// only an attested passing run does that.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// selectCertifyingFirstOn computes the T4 selection for a diff over the
// service's symbols and certifications-layer claims. Pure and
// deterministic.
//
// A candidate is a test carrying at least one LIVE certificate (a claim
// whose Meta state is live; flaky, broken, unproven and unreachable ones
// are not re-proofs), or a test that statically reaches a changed symbol —
// the latter only ever enters the selection under --safe. A certificate's
// target owns the non-test symbols of the files its claim's Scope names
// (the convention certifyingTestKeys, TC-23, already reads the Scope by);
// it is touched when one of them changed, or when a test certifying it
// reaches a changed symbol. certify.SelectCertifyingFirst does the greedy
// set cover with its deterministic tie-break.
//
// seeds are the changed symbols by key (TQ-29 decides which, plus the
// //go:embed and source-read seeds of TQ-21 and TQ-24; a test key or one no
// node carries is ignored); affected are the tests
// impact.TestGapSelectOn confirms reach them under rapid type analysis
// (TQ-30), which the caller already holds.
func selectCertifyingFirstOn(nodes []core.SymbolNode, claims []core.SemanticClaim, seeds []string, affected []impact.TestReach, safe bool) certify.Selection {
	extraKey := make(map[string]bool, len(seeds))
	for _, k := range seeds {
		extraKey[k] = true
	}
	var changed []string
	symbolsByFile := map[string][]string{} // non-test symbol keys defined in a file
	testKeysByName := map[string][]string{}
	for _, n := range nodes {
		if strings.HasPrefix(n.Name, "Test") {
			testKeysByName[n.Name] = append(testKeysByName[n.Name], n.Key())
			continue
		}
		symbolsByFile[n.FilePath] = append(symbolsByFile[n.FilePath], n.Key())
		if extraKey[n.Key()] {
			changed = append(changed, n.Key())
		}
	}
	sort.Strings(changed)

	liveTargets := map[string]map[string]bool{} // test key -> live certificate targets
	targetFiles := map[string]map[string]bool{} // target -> files its claims' scopes name
	for _, c := range claims {
		if c.Layer != certify.Layer || len(c.Meta) == 0 {
			continue
		}
		var m certify.Meta
		if json.Unmarshal(c.Meta, &m) != nil || certify.State(m.State) != certify.StateLive {
			continue
		}
		testKeys := testKeysByName[m.Test]
		if m.TestSymbolKey != "" {
			testKeys = []string{m.TestSymbolKey}
		}
		for _, k := range testKeys {
			if liveTargets[k] == nil {
				liveTargets[k] = map[string]bool{}
			}
			liveTargets[k][c.Subject] = true
		}
		if targetFiles[c.Subject] == nil {
			targetFiles[c.Subject] = map[string]bool{}
		}
		for _, f := range c.Scope {
			targetFiles[c.Subject][f] = true
		}
	}
	targetSymbols := make(map[string][]string, len(targetFiles))
	for target, files := range targetFiles {
		set := map[string]bool{}
		for f := range files {
			for _, k := range symbolsByFile[f] {
				set[k] = true
			}
		}
		syms := make([]string, 0, len(set))
		for k := range set {
			syms = append(syms, k)
		}
		sort.Strings(syms)
		targetSymbols[target] = syms
	}

	candidateKeys := map[string]bool{}
	for k := range liveTargets {
		candidateKeys[k] = true
	}
	// A candidate's Reach is read only against the changed symbols
	// (certify.SelectCertifyingFirst), so the confirmed changed symbols it
	// reaches stand for its whole reach; a certifying test none of them
	// confirms reaches no changed symbol.
	reach := map[string][]string{}
	for _, t := range affected {
		candidateKeys[t.Test] = true
		reach[t.Test] = t.Reaches
	}
	keys := make([]string, 0, len(candidateKeys))
	for k := range candidateKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	candidates := make([]certify.CertifyingTest, 0, len(keys))
	for _, k := range keys {
		targets := make([]string, 0, len(liveTargets[k]))
		for tgt := range liveTargets[k] {
			targets = append(targets, tgt)
		}
		sort.Strings(targets)
		candidates = append(candidates, certify.CertifyingTest{Test: k, Targets: targets, Reach: reach[k]})
	}
	return certify.SelectCertifyingFirst(changed, candidates, targetSymbols, safe)
}

// attestedDurations reads each test's last attested run duration in
// milliseconds (the newest run that stated one). A test with no attested run,
// or one that stated no duration, is absent: certify.SelectionTimeSaved names
// it, never a guessed average. A read error stops the read and is returned so
// the report says the estimate is unavailable rather than showing a partial
// one as whole.
func attestedDurations(ctx context.Context, st store.Store, service string, tests []string) (map[string]int64, error) {
	out := make(map[string]int64, len(tests))
	for _, key := range tests {
		runs, err := st.TestCaseRunsForSymbol(ctx, service, key, 1)
		if err != nil {
			return nil, fmt.Errorf("attested run of %s: %w", key, err)
		}
		if len(runs) > 0 && runs[0].DurationMS > 0 {
			out[key] = runs[0].DurationMS
		}
	}
	return out, nil
}

// testsSelectEconomicsPath is the ledger `tests select` appends each known
// time saved to, one JSON line per run, under the repository's workspace/
// (or ~/.fathom outside one). A variable so a test can point it at a temp
// file.
var testsSelectEconomicsPath = func() string {
	return workspacePath("artifacts/tests-select/time-saved.jsonl")
}

// testsSelectNow is the ledger row's clock (injected in tests).
var testsSelectNow = time.Now

// timeSavedRow is one line of the economics ledger.
type timeSavedRow struct {
	At         string  `json:"at"`
	Service    string  `json:"service"`
	Since      string  `json:"since"`
	Safe       bool    `json:"safe"`
	Selected   int     `json:"selected"`
	Skipped    int     `json:"skipped"`
	SelectedMS int64   `json:"selected_ms"`
	SkippedMS  int64   `json:"skipped_ms"`
	TotalMS    int64   `json:"total_ms"`
	Percent    float64 `json:"percent"`
	Missing    int     `json:"missing"`
}

// recordTimeSaved appends row to the ledger at path.
func recordTimeSaved(path string, row timeSavedRow) error {
	b, err := json.Marshal(row)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// certifyingSelectionView is what the two renderers print: the selection,
// its go test commands and its estimated CI time saved.
type certifyingSelectionView struct {
	Safe        bool
	Selection   certify.Selection
	Invocations []testInvocation
	Saved       certify.TimeSaved
	// SavedKnown is false when no selected or skipped test has an attested
	// duration on record (or reading them failed: SavedErr), so no estimate
	// exists. Percent is never shown as 0 in that case.
	SavedKnown bool
	SavedErr   string
}

// buildCertifyingSelectionView assembles the selection's view: the commands
// for its tests and the time saved from their attested durations.
func buildCertifyingSelectionView(ctx context.Context, st store.Store, service string, nodes []core.SymbolNode, sel certify.Selection, safe bool) certifyingSelectionView {
	v := certifyingSelectionView{Safe: safe, Selection: sel}
	byKey := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byKey[n.Key()] = n
	}
	rows := make([]rankedTest, 0, len(sel.Tests))
	for _, k := range sel.Tests {
		pkg := "."
		if n, ok := byKey[k]; ok {
			pkg = path.Dir(n.FilePath)
		}
		rows = append(rows, rankedTest{Test: k, Package: pkg})
	}
	v.Invocations = groupTestInvocations(nodes, rows)

	durations, err := attestedDurations(ctx, st, service, append(append([]string(nil), sel.Tests...), sel.Skipped...))
	if err != nil {
		v.SavedErr = err.Error()
		return v
	}
	v.Saved = certify.SelectionTimeSaved(sel, durations)
	v.SavedKnown = v.Saved.TotalMs > 0
	return v
}

// renderCertifyingSelectionText prints the certifying-first block that leads
// `tests select`'s text output.
func renderCertifyingSelectionText(w io.Writer, since string, v certifyingSelectionView) {
	sel := v.Selection
	mode := ""
	if v.Safe {
		mode = " (--safe: plus every test that reaches a changed symbol)"
	}
	fmt.Fprintf(w, "certifying-first selection --since %s%s: %d test(s) selected, %d touched certificate(s), %d with no certifying test\n",
		since, mode, len(sel.Tests), len(sel.TouchedTargets), len(sel.Uncovered))
	for i, t := range sel.Tests {
		if re := sel.Reproves[t]; len(re) > 0 {
			fmt.Fprintf(w, "  %d. %s re-proves %s\n", i+1, t, strings.Join(re, ", "))
		} else {
			fmt.Fprintf(w, "  %d. %s reaches a changed symbol, certifies no touched certificate (--safe)\n", i+1, t)
		}
	}
	if len(sel.Skipped) > 0 {
		fmt.Fprintf(w, "skipped (certifying tests this diff does not need: their certificates are re-proved by a selected test or untouched): %s\n", strings.Join(sel.Skipped, ", "))
	}
	if len(sel.Uncovered) > 0 {
		fmt.Fprintf(w, "uncovered (touched, no live test certifies them): %s\n", strings.Join(sel.Uncovered, ", "))
	}
	switch {
	case v.SavedErr != "":
		fmt.Fprintf(w, "time saved: unavailable — %s\n", v.SavedErr)
	case v.SavedKnown:
		miss := ""
		if n := len(v.Saved.Missing); n > 0 {
			miss = fmt.Sprintf("; %d test(s) with no attested duration count as 0", n)
		}
		fmt.Fprintf(w, "time saved: %.0f%% (skipped %d ms of %d ms, from attested run durations%s)\n",
			v.Saved.Percent, v.Saved.SkippedMs, v.Saved.TotalMs, miss)
	default:
		fmt.Fprintf(w, "time saved: unknown — no selected or skipped test has an attested run duration on record\n")
	}
	if len(v.Invocations) > 0 {
		fmt.Fprintln(w, "selection invocations:")
		for _, inv := range v.Invocations {
			fmt.Fprintf(w, "  %s\n", inv.Command)
		}
	}
}

// certifyingSelectionJSON is the `selection` object of the --json payload.
func certifyingSelectionJSON(v certifyingSelectionView) map[string]any {
	sel := v.Selection
	reproves := map[string]any{}
	for t, tgts := range sel.Reproves {
		reproves[t] = nonNilStrList(tgts)
	}
	invs := make([]any, len(v.Invocations))
	for i, inv := range v.Invocations {
		invs[i] = map[string]any{"package": inv.Package, "tests": nonNilStrList(inv.Tests), "command": inv.Command}
	}
	saved := map[string]any{"known": v.SavedKnown}
	switch {
	case v.SavedErr != "":
		saved["error"] = v.SavedErr
	case v.SavedKnown:
		saved["selected_ms"] = v.Saved.SelectedMs
		saved["skipped_ms"] = v.Saved.SkippedMs
		saved["total_ms"] = v.Saved.TotalMs
		saved["percent"] = v.Saved.Percent
		saved["missing"] = nonNilStrList(v.Saved.Missing)
	}
	return map[string]any{
		"safe":            v.Safe,
		"tests":           nonNilStrList(sel.Tests),
		"reproves":        reproves,
		"touched_targets": nonNilStrList(sel.TouchedTargets),
		"uncovered":       nonNilStrList(sel.Uncovered),
		"skipped":         nonNilStrList(sel.Skipped),
		"invocations":     invs,
		"time_saved":      saved,
	}
}
