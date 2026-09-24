package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testsIngestFixtureManifest = `
component: "cart"
suites: unit: {
	class:  "unit"
	runner: "go-test"
	inputs: paths: ["."]
	needs: []
	runtime: {}
	flake: {}
	tier: "blocking"
}
`

const testsIngestFixtureSrc = `package cart

// certifies: invariant I1 (asserts)
func TestAdd(t *T) {}
type T struct{}
`

func testsIngestFixtureEnvelope(startedAt string) string {
	return `{
  "suite": {"component": "cart", "name": "unit", "class": "unit"},
  "key": "k1",
  "verdict": "pass",
  "cases": [
    {"id": "1", "name": "TestAdd", "status": "pass", "duration": "1ms", "attempts": 1},
    {"id": "2", "name": "TestNeverIndexed", "status": "fail", "duration": "1ms", "attempts": 1}
  ],
  "totals": {"run": 2, "passed": 1, "failed": 1, "skipped": 0, "flaked": 0},
  "timing": {"startedAt": "` + startedAt + `", "wallClock": "2ms"},
  "env": {"toolchainDigest": "x", "runnerClass": "y", "arch": "amd64", "os": "darwin"},
  "artifacts": []
}`
}

// setUpTestsIngestFixture builds a repo root with one tests/cart.cue
// manifest and one Go source file carrying a certifies marker, ingests it
// with plain `fathom ingest`, and returns (root, storePath, service).
func setUpTestsIngestFixture(t *testing.T) (root, storePath, service string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tests", "cart.cue"), []byte(testsIngestFixtureManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cart_test.go"), []byte(testsIngestFixtureSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	service = "cart-svc"
	storePath = filepath.Join(t.TempDir(), "s.sqlite")
	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}
	return root, storePath, service
}

func TestCmdTestsIngest_ManifestAndResults(t *testing.T) {
	root, storePath, service := setUpTestsIngestFixture(t)

	resultsPath := filepath.Join(t.TempDir(), "run.json")
	if err := os.WriteFile(resultsPath, []byte(testsIngestFixtureEnvelope("2026-09-23T00:00:00Z")), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	code := cmdTestsIngest([]string{"--service", service, "--store", storePath, "--manifest", root, "--results", resultsPath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("tests ingest exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "1 suite(s), 1 run(s), 2 case(s) (1 joined), 1 declared certification(s)") {
		t.Fatalf("tests ingest summary = %q", got)
	}
	if strings.Contains(got, "no manifests found") || strings.Contains(got, "no runs ingested") {
		t.Fatalf("a real manifest+run must never print the skip lines: %q", got)
	}
}

// TestPlainIngest_PicksUpConfiguredResultsDir proves the [tests]
// results_dir half of TC-2: a PLAIN `fathom ingest` (never `fathom tests
// ingest`) also joins attested envelopes when configured, and — the
// specific bug this guards against — never wipes out a suite a prior
// `fathom tests ingest --manifest` call declared.
func TestPlainIngest_PicksUpConfiguredResultsDir(t *testing.T) {
	root, storePath, service := setUpTestsIngestFixture(t)

	// Declare the suite via a real `tests ingest --manifest` first.
	var tiOut, tiErr bytes.Buffer
	if code := cmdTestsIngest([]string{"--service", service, "--store", storePath, "--manifest", root}, &tiOut, &tiErr); code != 0 {
		t.Fatalf("tests ingest exit = %d, stderr: %s", code, tiErr.String())
	}

	// [tests] results_dir, project-scoped: config.DefaultProjectPath()
	// resolves against the process's cwd, so chdir into root for this test
	// (the same "run fathom ingest from inside the service's own checkout"
	// shape the whole config-loading convention assumes elsewhere).
	if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "results"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "results", "run.json"), []byte(testsIngestFixtureEnvelope("2026-09-23T00:00:00Z")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".fathom", "config.toml"), []byte("[tests]\nresults_dir = \"results\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	var out, errBuf bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", ".", "--store", storePath}, &out, &errBuf); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "tests: 1 run(s) picked up from [tests] results_dir") {
		t.Fatalf("plain ingest did not report the configured envelope pickup: %q", out.String())
	}

	var statusOut, statusErr bytes.Buffer
	if code := cmdTestsStatus([]string{"--service", service, "--store", storePath}, &statusOut, &statusErr); code != 0 {
		t.Fatalf("tests status exit = %d, stderr: %s", code, statusErr.String())
	}
	got := statusOut.String()
	if !strings.Contains(got, "cart/unit") {
		t.Fatalf("plain ingest's envelope pickup wiped the declared suite: %q", got)
	}
	if !strings.Contains(got, "pass") {
		t.Fatalf("plain ingest's envelope pickup did not write the run: %q", got)
	}
}

func TestCmdTestsIngest_NoManifestsAndNoRuns(t *testing.T) {
	root := t.TempDir() // no tests/ dir at all
	storePath := filepath.Join(t.TempDir(), "s.sqlite")

	var out, errBuf bytes.Buffer
	code := cmdTestsIngest([]string{"--service", "empty-svc", "--store", storePath, "--manifest", root}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("tests ingest exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "no manifests found") {
		t.Errorf("want a named 'no manifests found' skip, got %q", got)
	}
	if !strings.Contains(got, "no runs ingested") {
		t.Errorf("want a named 'no runs ingested' skip, got %q", got)
	}
}
