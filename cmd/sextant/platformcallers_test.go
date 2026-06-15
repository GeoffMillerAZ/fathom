package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/store"
)

// fleetRoot returns the absolute path to testdata/fleet, resolved relative to
// this source file so the test is working-directory independent.
func fleetRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file is .../cmd/sextant/platformcallers_test.go
	// testdata/fleet is two levels up from cmd/sextant
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "fleet")
}

// ─── flag validation ─────────────────────────────────────────────────────────

// TestPlatformCallers_StoreDefaultEmpty documents the new contract: --repos is
// no longer required. With no --repos the command reads the store (the
// language-agnostic source) and, when empty, reports no callers and exits 0. We
// point --store at a temp path so the test never touches the real workspace.
func TestPlatformCallers_StoreDefaultEmpty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty.duckdb")
	var out, errBuf bytes.Buffer
	code := run([]string{
		"platform-callers",
		"--store", dbPath,
		"--symbol", "Greet",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("expected exit 0 against an empty store; stderr=%s", errBuf.String())
	}
	if !strings.Contains(out.String(), "no cross-repo callers") {
		t.Errorf("expected 'no cross-repo callers'; got:\n%s", out.String())
	}
}

// TestPlatformCallers_StorePath proves the language-agnostic CLI path: seed a
// store with a cross-service Python pair (snake_case, no Go AST), then resolve
// the caller via --store.
func TestPlatformCallers_StorePath(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fleet.duckdb")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	if err := st.PutObservations(context.Background(), []core.Observation{
		{ServiceID: "py-lib", SourceRef: "helpers.py", Symbol: "validate", Package: "helpers", EvidenceTier: core.TierVerified},
		{ServiceID: "py-api", SourceRef: "app.py", Symbol: "handler", Package: "app", Calls: []string{"helpers.validate"}, EvidenceTier: core.TierVerified},
	}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	// Release the single-writer lock before the CLI reopens the same store.
	if err := st.Close(); err != nil {
		t.Fatalf("store.Close: %v", err)
	}

	var out, errBuf bytes.Buffer
	code := run([]string{
		"platform-callers",
		"--store", dbPath,
		"--symbol", "validate",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("platform-callers exited %d; stderr=%s", code, errBuf.String())
	}
	outStr := out.String()
	for _, want := range []string{"py-api", "py-lib", "validate", "total:"} {
		if !strings.Contains(outStr, want) {
			t.Errorf("output missing %q; got:\n%s", want, outStr)
		}
	}
}

func TestPlatformCallersMissingSymbol(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"platform-callers", "--repos", "svc=/tmp"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --symbol is missing")
	}
	if errBuf.String() == "" {
		t.Fatal("expected non-empty stderr")
	}
}

func TestPlatformCallersMalformedRepos(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{
		"platform-callers",
		"--repos", "no-equals-sign",
		"--symbol", "Foo",
	}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit for malformed --repos")
	}
}

// ─── end-to-end: fleet fixture ───────────────────────────────────────────────

func TestPlatformCallers_FleetFixture(t *testing.T) {
	root := fleetRoot(t)
	svcA := filepath.Join(root, "service-a")
	svcB := filepath.Join(root, "service-b")

	// Verify fixture paths exist; skip if not (CI might not have them).
	for _, p := range []string{svcA, svcB} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("fleet fixture not found at %s: %v", p, err)
		}
	}

	reposArg := "service-a=" + svcA + ",service-b=" + svcB

	var out, errBuf bytes.Buffer
	code := run([]string{
		"platform-callers",
		"--repos", reposArg,
		"--symbol", "Greet",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("platform-callers exited %d\nstdout=%s\nstderr=%s",
			code, out.String(), errBuf.String())
	}

	outStr := out.String()
	// Must mention the calling service and called service.
	if !strings.Contains(outStr, "service-b") {
		t.Errorf("output should mention service-b (caller); got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "service-a") {
		t.Errorf("output should mention service-a (callee); got:\n%s", outStr)
	}
	// Must include "Greet" in the header.
	if !strings.Contains(outStr, "Greet") {
		t.Errorf("output should mention symbol name 'Greet'; got:\n%s", outStr)
	}
	// Must include a "total:" summary line.
	if !strings.Contains(outStr, "total:") {
		t.Errorf("output should include totals line; got:\n%s", outStr)
	}
}

func TestPlatformCallers_NoCallers(t *testing.T) {
	root := fleetRoot(t)
	svcA := filepath.Join(root, "service-a")
	svcB := filepath.Join(root, "service-b")

	for _, p := range []string{svcA, svcB} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("fleet fixture not found at %s: %v", p, err)
		}
	}

	reposArg := "service-a=" + svcA + ",service-b=" + svcB

	var out, errBuf bytes.Buffer
	code := run([]string{
		"platform-callers",
		"--repos", reposArg,
		"--symbol", "SymbolThatDoesNotExist",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("platform-callers should exit 0 when no callers found; stderr=%s", errBuf.String())
	}
	if !strings.Contains(out.String(), "no cross-repo callers") {
		t.Errorf("expected 'no cross-repo callers' message; got:\n%s", out.String())
	}
}

// ─── parseReposFlag unit tests ────────────────────────────────────────────────

func TestParseReposFlag_Valid(t *testing.T) {
	repos, err := parseReposFlag("svc-a=/path/a,svc-b=/path/b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repos["svc-a"] != "/path/a" {
		t.Errorf("svc-a: got %q, want /path/a", repos["svc-a"])
	}
	if repos["svc-b"] != "/path/b" {
		t.Errorf("svc-b: got %q, want /path/b", repos["svc-b"])
	}
}

func TestParseReposFlag_Single(t *testing.T) {
	repos, err := parseReposFlag("only=/tmp/only")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repos["only"] != "/tmp/only" {
		t.Errorf("only: got %q, want /tmp/only", repos["only"])
	}
}

func TestParseReposFlag_Malformed(t *testing.T) {
	cases := []string{
		"noequalssign",
		"=emptyid",
		"emptypath=",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			_, err := parseReposFlag(c)
			if err == nil {
				t.Errorf("expected error for malformed pair %q", c)
			}
		})
	}
}

func TestParseReposFlag_TrailingComma(t *testing.T) {
	// Trailing comma should not cause an error — it produces an empty segment
	// that is skipped.
	repos, err := parseReposFlag("svc-a=/tmp/a,")
	if err != nil {
		t.Fatalf("unexpected error for trailing comma: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(repos))
	}
}
