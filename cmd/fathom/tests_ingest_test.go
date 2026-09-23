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
