package testevidence

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const bareEnvelope = `{
  "suite": {"component": "cart", "name": "unit", "class": "unit"},
  "key": "abc123",
  "verdict": "pass",
  "cases": [
    {"id": "1", "name": "TestAdd", "status": "pass", "duration": "1ms", "attempts": 1},
    {"id": "2", "name": "TestAdd/sub", "status": "pass", "duration": "1ms", "attempts": 1}
  ],
  "totals": {"run": 2, "passed": 2, "failed": 0, "skipped": 0, "flaked": 0},
  "timing": {"startedAt": "2026-09-23T00:00:00Z", "wallClock": "10ms"},
  "env": {"toolchainDigest": "x", "runnerClass": "y", "arch": "amd64", "os": "darwin"},
  "artifacts": []
}`

func TestLoadEnvelopes_BareSingleObject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run.json")
	if err := os.WriteFile(path, []byte(bareEnvelope), 0o644); err != nil {
		t.Fatal(err)
	}
	runs, err := LoadEnvelopes(path)
	if err != nil {
		t.Fatalf("LoadEnvelopes: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("want 1 run, got %d", len(runs))
	}
	r := runs[0]
	if r.Suite.Component != "cart" || r.Verdict != "pass" || len(r.Cases) != 2 {
		t.Errorf("run = %+v", r)
	}
	if r.StartedAt != "2026-09-23T00:00:00Z" {
		t.Errorf("StartedAt = %q", r.StartedAt)
	}
	if r.SignerTier != "" {
		t.Errorf("SignerTier = %q, want empty for a bare envelope", r.SignerTier)
	}
	if !strings.HasPrefix(r.Digest, "sha256:") {
		t.Errorf("Digest = %q, want sha256: prefix", r.Digest)
	}
}

func TestLoadEnvelopes_NDJSONStream(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runs.json")
	content := bareEnvelope + "\n" + bareEnvelope
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runs, err := LoadEnvelopes(path)
	if err != nil {
		t.Fatalf("LoadEnvelopes: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("want 2 runs from NDJSON, got %d", len(runs))
	}
}

func TestLoadEnvelopes_Directory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.json"), []byte(bareEnvelope), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.json"), []byte(bareEnvelope), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	runs, err := LoadEnvelopes(dir)
	if err != nil {
		t.Fatalf("LoadEnvelopes: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("want 2 runs from the directory, got %d", len(runs))
	}
}

func TestLoadEnvelopes_DSSEWrapped(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte(`{
		"_type": "https://in-toto.io/Statement/v1",
		"subject": [{"name": "x", "digest": {"sha256": "` + strings.Repeat("a", 64) + `"}}],
		"predicateType": "https://in-toto.io/attestation/test-result/v0.1",
		"predicate": ` + bareEnvelope + `
	}`))
	env := map[string]any{
		"payloadType": "application/vnd.in-toto+json",
		"payload":     payload,
		"signatures":  []any{},
		"tier":        "unsigned",
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "attested.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	runs, err := LoadEnvelopes(path)
	if err != nil {
		t.Fatalf("LoadEnvelopes: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("want 1 run, got %d", len(runs))
	}
	r := runs[0]
	if r.SignerTier != "unsigned" {
		t.Errorf("SignerTier = %q, want unsigned", r.SignerTier)
	}
	if r.Suite.Component != "cart" {
		t.Errorf("Suite.Component = %q, want cart (decoded from the DSSE payload)", r.Suite.Component)
	}
	if !strings.HasPrefix(r.Digest, "sha256:") {
		t.Errorf("Digest = %q, want sha256: prefix", r.Digest)
	}
}
