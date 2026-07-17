package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// writeTestPack writes a minimal, valid pack directory at dir (LC-7's CLI
// tests exercise the same pack shape internal/semantic/packs_test.go's
// writeFixturePack does, kept local since cmd/fathom black-box tests the CLI
// surface, not internal/semantic's loader directly).
func writeTestPack(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "eval"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	layerCUE := `name:    "` + name + `"
version: "v1"
kind:    "custom-kind"
corpus: include: ["docs/**"]
prompt_version: "` + name + `/fixture"
prompt_file:    "` + name + `/prompt.md"
tier:           "doc-claimed"
budgets: {
	max_tokens_per_run: 1000
	max_usd_per_run:    0.1
}
`
	if err := os.WriteFile(filepath.Join(dir, "layer.cue"), []byte(layerCUE), 0o644); err != nil {
		t.Fatalf("write layer.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte("Test prompt."), 0o644); err != nil {
		t.Fatalf("write prompt.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "eval", "labels.json"), []byte(`{"probes":[],"rules":[]}`), 0o644); err != nil {
		t.Fatalf("write eval fixture: %v", err)
	}
}

func TestLayers_List_EmbeddedOnly(t *testing.T) {
	root := t.TempDir()
	var out, errBuf bytes.Buffer
	code := run([]string{"layers", "list", "--root", root}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("layers list exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "obligations") || !strings.Contains(got, "findings") {
		t.Errorf("output missing built-in layers:\n%s", got)
	}
	if !strings.Contains(got, "embedded") {
		t.Errorf("output missing source column:\n%s", got)
	}
}

func TestLayers_List_UnionsProjectPack(t *testing.T) {
	root := t.TempDir()
	writeTestPack(t, filepath.Join(root, ".fathom", "layers", "custom"), "custom")

	var out, errBuf bytes.Buffer
	code := run([]string{"layers", "list", "--root", root}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("layers list exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "custom") || !strings.Contains(got, "project") {
		t.Errorf("output missing the project pack:\n%s", got)
	}
}

func TestLayers_Show_KnownLayer(t *testing.T) {
	root := t.TempDir()
	var out, errBuf bytes.Buffer
	code := run([]string{"layers", "show", "--root", root, "obligations"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("layers show exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "obligations/c02b1aa1") {
		t.Errorf("show output missing the pinned prompt identity:\n%s", got)
	}
	if !strings.Contains(got, "doc-claimed") {
		t.Errorf("show output missing tier:\n%s", got)
	}
}

func TestLayers_Show_UnknownLayer(t *testing.T) {
	root := t.TempDir()
	var out, errBuf bytes.Buffer
	code := run([]string{"layers", "show", "--root", root, "nonexistent"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("layers show nonexistent: want non-zero exit")
	}
	if !strings.Contains(errBuf.String(), "obligations") {
		t.Errorf("error should list known layers: %s", errBuf.String())
	}
}

func TestLayers_Install_CopiesAndListable(t *testing.T) {
	srcDir := filepath.Join(t.TempDir(), "adr-rules")
	writeTestPack(t, srcDir, "adr-rules")

	root := t.TempDir()
	var out, errBuf bytes.Buffer
	code := run([]string{"layers", "install", "--root", root, srcDir}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("layers install exit = %d, stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "installed: adr-rules") {
		t.Errorf("install output = %q", out.String())
	}

	destLayerCUE := filepath.Join(root, ".fathom", "layers", "adr-rules", "layer.cue")
	if _, err := os.Stat(destLayerCUE); err != nil {
		t.Errorf("installed layer.cue missing: %v", err)
	}

	// The installed pack now shows up in `layers list`.
	out.Reset()
	errBuf.Reset()
	code = run([]string{"layers", "list", "--root", root}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("layers list after install: exit = %d, stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "adr-rules") {
		t.Errorf("adr-rules missing from list after install:\n%s", out.String())
	}
}

func TestLayers_Install_RejectsInvalidPack(t *testing.T) {
	srcDir := t.TempDir() // no layer.cue at all
	root := t.TempDir()

	var out, errBuf bytes.Buffer
	code := run([]string{"layers", "install", "--root", root, srcDir}, &out, &errBuf)
	if code == 0 {
		t.Fatal("layers install with no layer.cue: want non-zero exit")
	}
}

func TestLayers_List_WithStoreShowsClaimCounts(t *testing.T) {
	root := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "s.duckdb")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	claim := core.SemanticClaim{
		ClaimID: "cl-1", ServiceID: "svc",
		Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
		Subject: "CLAUDE.md#x", Rule: "Do the thing.",
		Scope: []string{"**/*.go"}, Tier: core.TierDocClaimed, TierProfile: "engineering",
		Status: core.ClaimFresh, CompiledAt: time.Unix(1, 0).UTC(),
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, nil); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}
	st.Close()

	var out, errBuf bytes.Buffer
	code := run([]string{"layers", "list", "--root", root, "--store", storePath, "--service", "svc"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("layers list --store exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "CLAIMS") {
		t.Errorf("output missing CLAIMS column header:\n%s", got)
	}
	// obligations row should show a count of 1.
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "obligations") {
			if !strings.HasSuffix(strings.TrimSpace(line), "1") {
				t.Errorf("obligations row = %q, want a trailing claim count of 1", line)
			}
		}
	}
}

func TestLayers_List_StoreRequiresService(t *testing.T) {
	root := t.TempDir()
	var out, errBuf bytes.Buffer
	code := run([]string{"layers", "list", "--root", root, "--store", "workspace/db/fathom.duckdb"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("layers list --store without --service: want non-zero exit")
	}
}

func TestLayers_NoVerb(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"layers"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("layers with no verb: want non-zero exit")
	}
}

func TestLayers_UnknownVerb(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"layers", "bogus"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("layers bogus: want non-zero exit")
	}
}
