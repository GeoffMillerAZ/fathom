package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestTestsCoverage_RequiresProfile(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := runTestsCoverage([]string{"--service", "svc"}, fakeGitRunner(nil), &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --profile is missing")
	}
}

func TestParseProfileArg(t *testing.T) {
	cases := []struct {
		raw, wantLayer, wantPath string
	}{
		{"workspace/cov/unit.out", "unit", "workspace/cov/unit.out"},
		{"unit=workspace/cov/unit.out", "unit", "workspace/cov/unit.out"},
		{"integration=workspace/cov/integration.out", "integration", "workspace/cov/integration.out"},
		// A Windows drive-letter path must never be misread as a label.
		{`C:\cov.out`, "unit", `C:\cov.out`},
	}
	for _, tc := range cases {
		layer, path := parseProfileArg(tc.raw)
		if layer != tc.wantLayer || path != tc.wantPath {
			t.Errorf("parseProfileArg(%q) = (%q, %q), want (%q, %q)", tc.raw, layer, path, tc.wantLayer, tc.wantPath)
		}
	}
}

// testsCoverageFixture ingests reviewFixtureSrc (shared with review_test.go)
// into a fresh store and returns its store path plus func A's real
// [Line, EndLine] span, so callers can build a coverage profile block that
// reliably lands inside it regardless of the extractor's exact convention.
func testsCoverageFixture(t *testing.T) (storePath, service string, fn core.SymbolNode) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte(reviewFixtureSrc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	storePath = filepath.Join(t.TempDir(), "s.sqlite")
	service = "cov-svc"

	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	nodes, err := c.ListAllSymbols(context.Background(), service)
	if err != nil {
		t.Fatalf("list symbols: %v", err)
	}
	for _, n := range nodes {
		if n.Name == "A" {
			fn = n
		}
	}
	if fn.Name == "" {
		t.Fatalf("func A not found among indexed symbols: %+v", nodes)
	}
	return storePath, service, fn
}

func writeCoverProfile(t *testing.T, fn core.SymbolNode, count int) string {
	t.Helper()
	body := fmt.Sprintf("mode: set\npkg/a.go:%d.1,%d.1 1 %d\n", fn.Line, fn.EndLine, count)
	path := filepath.Join(t.TempDir(), "cover.out")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return path
}

func TestTestsCoverage_JSONReportsJoinedSymbol_BarePathDefaultsUnit(t *testing.T) {
	storePath, service, fn := testsCoverageFixture(t)
	profilePath := writeCoverProfile(t, fn, 1)

	var out, errBuf bytes.Buffer
	code := runTestsCoverage(
		// A bare (unlabeled) --profile: defaults to layer "unit".
		[]string{"--profile", profilePath, "--service", service, "--store", storePath, "--json"},
		fakeGitRunner(nil), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal %q: %v", out.String(), err)
	}
	syms, _ := payload["symbols"].([]any)
	if len(syms) != 1 {
		t.Fatalf("symbols = %+v, want exactly func A", payload["symbols"])
	}
	entry := syms[0].(map[string]any)
	if !strings.Contains(entry["symbol"].(string), "A") {
		t.Errorf("symbol = %v", entry["symbol"])
	}
	layers, _ := entry["layers"].([]any)
	if len(layers) != 1 || layers[0] != "unit" {
		t.Errorf("layers = %v, want [unit] (bare path default)", entry["layers"])
	}
	anyStat, _ := entry["any"].(map[string]any)
	if anyStat["covered"].(float64) == 0 {
		t.Errorf("expected any.covered > 0, got %+v", entry)
	}
	byLayer, _ := entry["by_layer"].(map[string]any)
	if _, ok := byLayer["unit"]; !ok {
		t.Errorf("by_layer should carry a \"unit\" entry: %+v", byLayer)
	}
}

func TestTestsCoverage_TwoLayeredProfiles(t *testing.T) {
	storePath, service, fn := testsCoverageFixture(t)
	unitPath := writeCoverProfile(t, fn, 0) // unit never executed it
	intPath := writeCoverProfile(t, fn, 1)  // integration did

	var out, errBuf bytes.Buffer
	code := runTestsCoverage(
		[]string{
			"--profile", "unit=" + unitPath,
			"--profile", "integration=" + intPath,
			"--service", service, "--store", storePath, "--json",
		},
		fakeGitRunner(nil), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal %q: %v", out.String(), err)
	}
	modes, _ := payload["modes"].([]any)
	if len(modes) != 1 || modes[0] != "set" {
		t.Errorf("modes = %v", payload["modes"])
	}
	syms, _ := payload["symbols"].([]any)
	if len(syms) != 1 {
		t.Fatalf("symbols = %+v", payload["symbols"])
	}
	entry := syms[0].(map[string]any)
	layers, _ := entry["layers"].([]any)
	if len(layers) != 1 || layers[0] != "integration" {
		t.Errorf("layers = %v, want [integration] (only that layer covers it)", entry["layers"])
	}
	uncoveredAt, _ := entry["uncovered_at"].([]any)
	if len(uncoveredAt) != 1 || uncoveredAt[0] != "unit" {
		t.Errorf("uncovered_at = %v, want [unit]", entry["uncovered_at"])
	}
	anyStat, _ := entry["any"].(map[string]any)
	if anyStat["covered"].(float64) == 0 {
		t.Errorf("any.covered should be >0 (union covered by integration): %+v", anyStat)
	}
}

func TestTestsCoverage_TableOutput_MultiLayerColumns(t *testing.T) {
	storePath, service, fn := testsCoverageFixture(t)
	unitPath := writeCoverProfile(t, fn, 1)
	intPath := writeCoverProfile(t, fn, 0)

	var out, errBuf bytes.Buffer
	code := runTestsCoverage(
		[]string{"--profile", "unit=" + unitPath, "--profile", "integration=" + intPath, "--service", service, "--store", storePath},
		fakeGitRunner(nil), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	header := strings.ToUpper(strings.SplitN(out.String(), "\n", 2)[0])
	for _, want := range []string{"UNIT", "INTEGRATION", "ANY"} {
		if !strings.Contains(header, want) {
			t.Errorf("table header %q missing column %q", header, want)
		}
	}
	if !strings.Contains(out.String(), "A") {
		t.Errorf("table output = %q", out.String())
	}
}

func TestTestsCoverage_DiffNamesUncoveredChangedSymbol(t *testing.T) {
	storePath, service, fn := testsCoverageFixture(t)
	// count=0: the profile HAS data for func A, but it was never executed.
	profilePath := writeCoverProfile(t, fn, 0)

	var out, errBuf bytes.Buffer
	code := runTestsCoverage(
		[]string{"--profile", profilePath, "--service", service, "--store", storePath, "--diff", "origin/main", "--json"},
		fakeGitRunner([]string{"pkg/a.go"}), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal %q: %v", out.String(), err)
	}
	gap, _ := payload["gap"].([]any)
	if len(gap) != 1 {
		t.Fatalf("gap = %+v, want exactly func A (0%% covered)", payload["gap"])
	}
	entry := gap[0].(map[string]any)
	if !strings.Contains(entry["symbol"].(string), "A") {
		t.Errorf("gap symbol = %v", entry["symbol"])
	}
	if entry["in_profile"].(bool) != true {
		t.Errorf("expected in_profile=true (profile has data, just 0%% covered), got %+v", entry)
	}
	uncoveredAt, _ := entry["uncovered_at"].([]any)
	if len(uncoveredAt) != 1 || uncoveredAt[0] != "unit" {
		t.Errorf("gap uncovered_at = %v, want [unit] (bare path default layer)", entry["uncovered_at"])
	}
}

// writeMutationReport writes fathom's own JSON mutation-report shape with
// one mutant inside fn's [Line, EndLine] span.
func writeMutationReport(t *testing.T, fn core.SymbolNode, killed bool) string {
	t.Helper()
	body := fmt.Sprintf(`{"mutants":[{"file":"pkg/a.go","line":%d,"killed":%t}]}`, fn.Line, killed)
	path := filepath.Join(t.TempDir(), "mutation.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write mutation report: %v", err)
	}
	return path
}

func TestTestsCoverage_MutationJSON_SurvivedScoreZero(t *testing.T) {
	storePath, service, fn := testsCoverageFixture(t)
	profilePath := writeCoverProfile(t, fn, 1)
	mutationPath := writeMutationReport(t, fn, false)

	var out, errBuf bytes.Buffer
	code := runTestsCoverage(
		[]string{"--profile", profilePath, "--mutation", mutationPath, "--service", service, "--store", storePath, "--json"},
		fakeGitRunner(nil), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal %q: %v", out.String(), err)
	}
	syms, _ := payload["symbols"].([]any)
	if len(syms) != 1 {
		t.Fatalf("symbols = %+v", payload["symbols"])
	}
	entry := syms[0].(map[string]any)
	mut, ok := entry["mutation"].(map[string]any)
	if !ok {
		t.Fatalf("entry missing mutation field: %+v", entry)
	}
	if mut["mutants"].(float64) != 1 || mut["killed"].(float64) != 0 || mut["survived"].(float64) != 1 || mut["score"].(float64) != 0 {
		t.Errorf("mutation = %+v, want {mutants:1 killed:0 survived:1 score:0}", mut)
	}
}

func TestTestsCoverage_MutationTable_AddsColumn(t *testing.T) {
	storePath, service, fn := testsCoverageFixture(t)
	profilePath := writeCoverProfile(t, fn, 1)
	mutationPath := writeMutationReport(t, fn, true)

	var out, errBuf bytes.Buffer
	code := runTestsCoverage(
		[]string{"--profile", profilePath, "--mutation", mutationPath, "--service", service, "--store", storePath},
		fakeGitRunner(nil), &out, &errBuf,
	)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	header := strings.ToUpper(strings.SplitN(out.String(), "\n", 2)[0])
	if !strings.Contains(header, "MUTATION") {
		t.Errorf("table header %q missing MUTATION column", header)
	}
	if !strings.Contains(out.String(), "1.00") {
		t.Errorf("table output missing the score 1.00 (all mutants killed): %q", out.String())
	}
}

func TestTestsCoverage_MutationBadPath(t *testing.T) {
	storePath, service, fn := testsCoverageFixture(t)
	profilePath := writeCoverProfile(t, fn, 1)

	var out, errBuf bytes.Buffer
	code := runTestsCoverage(
		[]string{"--profile", profilePath, "--mutation", filepath.Join(t.TempDir(), "missing.json"), "--service", service, "--store", storePath},
		fakeGitRunner(nil), &out, &errBuf,
	)
	if code == 0 {
		t.Fatal("expected nonzero exit for an unreadable --mutation path")
	}
}
