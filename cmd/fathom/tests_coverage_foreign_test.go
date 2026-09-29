package main

// tests_coverage_foreign_test.go — TQ-14w (docs/plans/test-intelligence.md):
// `fathom tests coverage --foreign <format>=<file>` joins a non-Go coverage
// report to the service's indexed symbols — per symbol where the file's
// symbols carry line ranges, per file (named as file-level) where they do
// not.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// foreignCoverageFixture ingests a two-file Go service: pkg/a.go, whose func
// A carries a real line range, and pkg/consts.go, which holds only a const
// and a type — no function-like symbol, so a block there can only be joined
// at file level. It returns the store path, the service id and the report
// files (an lcov tracefile naming absolute CI paths, a coverage.py JSON, and
// an lcov report for a file the index never saw).
func foreignCoverageFixture(t *testing.T) (storePath, service, lcov, covpy string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"pkg/a.go":      reviewFixtureSrc,
		"pkg/consts.go": "package pkg\n\nconst Limit = 3\n\ntype Box struct{}\n",
	}
	for rel, body := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	storePath = filepath.Join(t.TempDir(), "s.sqlite")
	service = "foreign-svc"
	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	dir := t.TempDir()
	lcov = filepath.Join(dir, "cov.info")
	// func A spans lines 3-5: line 3 ran, line 4 did not. The SF path is the
	// CI machine's absolute path, joined by suffix. The second record names
	// a file the index never saw.
	lcovBody := "TN:\nSF:/ci/work/pkg/a.go\nDA:3,1\nDA:4,0\nend_of_record\nSF:/ci/work/vendor/ghost.go\nDA:1,1\nend_of_record\n"
	if err := os.WriteFile(lcov, []byte(lcovBody), 0o644); err != nil {
		t.Fatal(err)
	}
	covpy = filepath.Join(dir, "cov.json")
	covpyBody := `{"meta": {"format": 3}, "files": {"pkg/consts.go": {"executed_lines": [3], "missing_lines": [5]}}, "totals": {}}`
	if err := os.WriteFile(covpy, []byte(covpyBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return storePath, service, lcov, covpy
}

// TestTestsCoverageJoinsForeignBlocks is TQ-14w: two --foreign reports of
// different formats join to the indexed symbols with no --profile at all —
// func A per symbol (50%), pkg/consts.go per file and named file-level, the
// report's unknown file named as unmatched, never guessed at — and --json
// carries both views.
func TestTestsCoverageJoinsForeignBlocks(t *testing.T) {
	storePath, service, lcov, covpy := foreignCoverageFixture(t)
	base := []string{"--service", service, "--store", storePath, "--foreign", "lcov=" + lcov, "--foreign", "coverage-py=" + covpy}

	var out, errBuf bytes.Buffer
	if code := runTestsCoverage(append(append([]string{}, base...), "--json"), fakeGitRunner(nil), &out, &errBuf); code != 0 {
		t.Fatalf("--foreign without --profile: exit %d, stderr: %s", code, errBuf.String())
	}
	var payload struct {
		Symbols []any `json:"symbols"`
		Foreign struct {
			Inputs []struct {
				Format string `json:"format"`
				File   string `json:"file"`
				Blocks int    `json:"blocks"`
			} `json:"inputs"`
			BySymbol []struct {
				Symbol     string  `json:"symbol"`
				File       string  `json:"file"`
				Level      string  `json:"level"`
				Statements int     `json:"statements"`
				Covered    int     `json:"covered"`
				Percent    float64 `json:"percent"`
			} `json:"by_symbol"`
			ByFile []struct {
				File       string  `json:"file"`
				Level      string  `json:"level"`
				Statements int     `json:"statements"`
				Covered    int     `json:"covered"`
				Percent    float64 `json:"percent"`
			} `json:"by_file"`
			Unmatched []string `json:"unmatched_files"`
		} `json:"foreign"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal %q: %v", out.String(), err)
	}
	f := payload.Foreign
	if len(f.Inputs) != 2 || f.Inputs[0].Format != "lcov" || f.Inputs[0].Blocks != 3 || f.Inputs[1].Format != "coverage-py" || f.Inputs[1].Blocks != 2 {
		t.Errorf("inputs = %+v, want lcov (3 blocks) and coverage-py (2 blocks)", f.Inputs)
	}
	if len(f.BySymbol) != 1 {
		t.Fatalf("by_symbol = %+v, want func A alone (consts.go has no function-like symbol)", f.BySymbol)
	}
	a := f.BySymbol[0]
	if !strings.Contains(a.Symbol, "A") || a.File != "pkg/a.go" || a.Level != "symbol" || a.Statements != 2 || a.Covered != 1 || a.Percent != 50 {
		t.Errorf("func A = %+v, want pkg/a.go at symbol level, 1 of 2 statements (50%%)", a)
	}
	if len(f.ByFile) != 1 {
		t.Fatalf("by_file = %+v, want pkg/consts.go alone", f.ByFile)
	}
	c := f.ByFile[0]
	if c.File != "pkg/consts.go" || c.Level != "file" || c.Statements != 2 || c.Covered != 1 || c.Percent != 50 {
		t.Errorf("consts.go = %+v, want a file-level row, 1 of 2 statements (50%%)", c)
	}
	if len(f.Unmatched) != 1 || f.Unmatched[0] != "/ci/work/vendor/ghost.go" {
		t.Errorf("unmatched_files = %v, want the report's unindexed file named", f.Unmatched)
	}
	if len(payload.Symbols) != 0 {
		t.Errorf("symbols = %v, want none: no Go --profile was given", payload.Symbols)
	}

	// The table names each granularity.
	out.Reset()
	errBuf.Reset()
	if code := runTestsCoverage(base, fakeGitRunner(nil), &out, &errBuf); code != 0 {
		t.Fatalf("table exit %d, stderr: %s", code, errBuf.String())
	}
	table := out.String()
	for _, want := range []string{"FOREIGN COVERAGE", "pkg/a.go", "50.0%", "FILE-LEVEL", "pkg/consts.go", "/ci/work/vendor/ghost.go"} {
		if !strings.Contains(table, want) {
			t.Errorf("table lacks %q:\n%s", want, table)
		}
	}

	// A Go --profile beside --foreign keeps its own view.
	_, _, fn := testsCoverageFixture(t)
	profile := writeCoverProfile(t, fn, 1)
	out.Reset()
	errBuf.Reset()
	if code := runTestsCoverage(append(append([]string{}, base...), "--profile", profile, "--json"), fakeGitRunner(nil), &out, &errBuf); code != 0 {
		t.Fatalf("--profile with --foreign: exit %d, stderr: %s", code, errBuf.String())
	}
	var both map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &both); err != nil {
		t.Fatal(err)
	}
	if _, ok := both["symbols"]; !ok {
		t.Error("--profile's symbols view is missing beside --foreign")
	}
	if _, ok := both["foreign"]; !ok {
		t.Error("the foreign view is missing beside --profile")
	}

	// Refusals: a format the ingest verb does not accept, a bare value, an
	// unreadable report, and --diff (it narrows the Go view only).
	for name, args := range map[string][]string{
		"unknown format": {"--foreign", "gcov=" + lcov},
		"no format":      {"--foreign", lcov},
		"missing file":   {"--foreign", "lcov=" + filepath.Join(t.TempDir(), "absent.info")},
		"diff":           {"--foreign", "lcov=" + lcov, "--diff", "HEAD~1"},
	} {
		out.Reset()
		errBuf.Reset()
		code := runTestsCoverage(append([]string{"--service", service, "--store", storePath}, args...), fakeGitRunner(nil), &out, &errBuf)
		if code == 0 || out.Len() != 0 || !strings.Contains(errBuf.String(), "error:") {
			t.Errorf("%s: exit %d, stdout %q, stderr %q, want a named refusal", name, code, out.String(), errBuf.String())
		}
	}
	if code := runTestsCoverage([]string{"--service", service, "--store", storePath}, fakeGitRunner(nil), &out, &errBuf); code == 0 {
		t.Error("neither --profile nor --foreign was accepted")
	}
}
