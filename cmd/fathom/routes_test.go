package main

// routes_test.go — RC-30 (docs/vision/28-receipts.md §4) cmdRoutes
// coverage, plus the MAJOR fix (finding 2, discoveries.go/routes.go): an
// unreachable index must never render as "no routes yet" — that line is
// the positive claim "nothing routes here", which an index that was never
// consulted never made.

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPrimeRoutesFlagDelegatesToCmdRoutes is SR-29's proof (surface
// reckoning F-47, design §4) that `routes` "becomes prime --routes": the
// flag reaches the exact same cmdRoutes render a bare `fathom routes`
// invocation used to, byte for byte, and every other prime flag
// (`--url`/`--dir`) is left for cmdPrime's own parsing untouched.
func TestPrimeRoutesFlagDelegatesToCmdRoutes(t *testing.T) {
	threadEnv(t)
	root := t.TempDir()
	now := time.Now().UTC()
	windowsPath := filepath.Join(root, "sessions.jsonl")
	t.Setenv("FATHOM_SESSIONS_LOG", windowsPath)
	recordSessionWindow(root, sessionWindowRow{SchemaVersion: 1, SessionID: "s1", Cwd: root, Service: "fathom", Started: now.Add(-2 * time.Hour).Format(time.RFC3339)})
	recordSessionWindow(root, sessionWindowRow{SchemaVersion: 1, SessionID: "s2", Cwd: root, Service: "fathom", Started: now.Add(-time.Hour).Format(time.RFC3339)})
	logPath := filepath.Join(root, "calls.jsonl")
	writeDiscoCallLogFixture(t, logPath, []string{
		`{"ts":"` + now.Add(-90*time.Minute).Format(time.RFC3339Nano) + `","service":"fathom","pointers":["pkg.Routed"]}`,
		`{"ts":"` + now.Add(-30*time.Minute).Format(time.RFC3339Nano) + `","service":"fathom","pointers":["pkg.Routed"]}`,
	})
	routesArgs := []string{
		"--root", root, "--service", "fathom", "--call-log", logPath,
		"--url", "http://127.0.0.1:1", // never listening: index unreachable
	}

	var wantOut, wantErr bytes.Buffer
	wantCode := cmdRoutes(routesArgs, &wantOut, &wantErr)

	var gotOut, gotErr bytes.Buffer
	primeArgs := append([]string{"--routes"}, routesArgs...)
	gotCode := cmdPrime(primeArgs, &gotOut, &gotErr)

	if gotCode != wantCode {
		t.Fatalf("cmdPrime(--routes) exit = %d, want cmdRoutes' own %d", gotCode, wantCode)
	}
	if gotOut.String() != wantOut.String() {
		t.Errorf("cmdPrime(--routes) output = %q, want the identical cmdRoutes render %q", gotOut.String(), wantOut.String())
	}

	// The flag may appear anywhere in the argument list, not only first.
	var reorderedOut bytes.Buffer
	reordered := append(append([]string{}, routesArgs...), "--routes")
	if code := cmdPrime(reordered, &reorderedOut, &bytes.Buffer{}); code != wantCode || reorderedOut.String() != wantOut.String() {
		t.Errorf("cmdPrime with --routes last = (%d, %q), want (%d, %q)", code, reorderedOut.String(), wantCode, wantOut.String())
	}
}

func TestCmdRoutes_ReportsUnreachableIndexInsteadOfEmptyList(t *testing.T) {
	threadEnv(t)
	root := t.TempDir()
	now := time.Now().UTC()
	windowsPath := filepath.Join(root, "sessions.jsonl")
	t.Setenv("FATHOM_SESSIONS_LOG", windowsPath)
	recordSessionWindow(root, sessionWindowRow{SchemaVersion: 1, SessionID: "s1", Cwd: root, Service: "fathom", Started: now.Add(-2 * time.Hour).Format(time.RFC3339)})
	recordSessionWindow(root, sessionWindowRow{SchemaVersion: 1, SessionID: "s2", Cwd: root, Service: "fathom", Started: now.Add(-time.Hour).Format(time.RFC3339)})
	logPath := filepath.Join(root, "calls.jsonl")
	writeDiscoCallLogFixture(t, logPath, []string{
		`{"ts":"` + now.Add(-90*time.Minute).Format(time.RFC3339Nano) + `","service":"fathom","pointers":["pkg.Routed"]}`,
		`{"ts":"` + now.Add(-30*time.Minute).Format(time.RFC3339Nano) + `","service":"fathom","pointers":["pkg.Routed"]}`,
	})

	var out, errb bytes.Buffer
	code := cmdRoutes([]string{
		"--root", root, "--service", "fathom", "--call-log", logPath,
		"--url", "http://127.0.0.1:1", // never listening: index unreachable
	}, &out, &errb)
	if code != 0 {
		t.Fatalf("cmdRoutes exit %d: %s", code, errb.String())
	}
	if strings.Contains(out.String(), "no routes yet") {
		t.Errorf("cmdRoutes with an unreachable index = %q, must not claim \"no routes yet\" (a routed candidate exists)", out.String())
	}
	if !strings.Contains(out.String(), "unreachable") {
		t.Errorf("cmdRoutes with an unreachable index = %q, want it to name the index as unreachable", out.String())
	}
}

func TestCmdRoutes_JSONNamesUnreachableIndex(t *testing.T) {
	threadEnv(t)
	root := t.TempDir()
	now := time.Now().UTC()
	t.Setenv("FATHOM_SESSIONS_LOG", filepath.Join(root, "sessions.jsonl"))
	recordSessionWindow(root, sessionWindowRow{SchemaVersion: 1, SessionID: "s1", Cwd: root, Service: "fathom", Started: now.Add(-2 * time.Hour).Format(time.RFC3339)})
	recordSessionWindow(root, sessionWindowRow{SchemaVersion: 1, SessionID: "s2", Cwd: root, Service: "fathom", Started: now.Add(-time.Hour).Format(time.RFC3339)})
	logPath := filepath.Join(root, "calls.jsonl")
	writeDiscoCallLogFixture(t, logPath, []string{
		`{"ts":"` + now.Add(-90*time.Minute).Format(time.RFC3339Nano) + `","service":"fathom","pointers":["pkg.Routed"]}`,
		`{"ts":"` + now.Add(-30*time.Minute).Format(time.RFC3339Nano) + `","service":"fathom","pointers":["pkg.Routed"]}`,
	})

	var out, errb bytes.Buffer
	code := cmdRoutes([]string{
		"--root", root, "--service", "fathom", "--call-log", logPath,
		"--url", "http://127.0.0.1:1", "--json",
	}, &out, &errb)
	if code != 0 {
		t.Fatalf("cmdRoutes --json exit %d: %s", code, errb.String())
	}
	var payload struct {
		IndexUnreachable string `json:"index_unreachable"`
		Routes           []struct {
			Line     string `json:"line"`
			Resolved bool   `json:"resolved"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v (%s)", err, out.String())
	}
	if payload.IndexUnreachable == "" {
		t.Errorf("cmdRoutes --json with an unreachable index = %+v, want index_unreachable set", payload)
	}
	if len(payload.Routes) != 1 || payload.Routes[0].Resolved {
		t.Errorf("cmdRoutes --json routes = %+v, want one unresolved candidate kept (not dropped)", payload.Routes)
	}
}

func TestCmdRoutes_ResolvesAgainstFixtureStore(t *testing.T) {
	threadEnv(t)
	storePath := seedPlansFixtureStore(t)
	root := t.TempDir()
	now := time.Now().UTC()
	t.Setenv("FATHOM_SESSIONS_LOG", filepath.Join(root, "sessions.jsonl"))
	recordSessionWindow(root, sessionWindowRow{SchemaVersion: 1, SessionID: "s1", Cwd: root, Service: "payment-service", Started: now.Add(-2 * time.Hour).Format(time.RFC3339)})
	recordSessionWindow(root, sessionWindowRow{SchemaVersion: 1, SessionID: "s2", Cwd: root, Service: "payment-service", Started: now.Add(-time.Hour).Format(time.RFC3339)})
	logPath := filepath.Join(root, "calls.jsonl")
	writeDiscoCallLogFixture(t, logPath, []string{
		`{"ts":"` + now.Add(-90*time.Minute).Format(time.RFC3339Nano) + `","service":"payment-service","pointers":["Charge"]}`,
		`{"ts":"` + now.Add(-30*time.Minute).Format(time.RFC3339Nano) + `","service":"payment-service","pointers":["Charge"]}`,
	})

	var out, errb bytes.Buffer
	code := cmdRoutes([]string{
		"--root", root, "--service", "payment-service", "--call-log", logPath, "--store", storePath,
	}, &out, &errb)
	if code != 0 {
		t.Fatalf("cmdRoutes exit %d: %s", code, errb.String())
	}
	if strings.Contains(out.String(), "unreachable") {
		t.Errorf("cmdRoutes against a real store = %q, must not claim the index is unreachable", out.String())
	}
	if !strings.Contains(out.String(), "symbol Charge") {
		t.Errorf("cmdRoutes against a real store = %q, want the resolved \"symbol Charge → ...\" line", out.String())
	}
}
