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
	// obligations row should show a claim count of 1, followed by the
	// LAST_COMPILE column (A4-1: "never", since no compile_runs exist here).
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "obligations") {
			fields := strings.Fields(line)
			if len(fields) < 2 || fields[len(fields)-2] != "1" || fields[len(fields)-1] != "never" {
				t.Errorf("obligations row = %q, want a claim count of 1 followed by LAST_COMPILE=never", line)
			}
		}
	}
}

// TestLayers_List_SurfacesOrphanedLayers is critic finding 2's contract:
// removing a pack directory does NOT stop its stored claims serving —
// `layers list --store` must surface such orphaned layers so injected rules
// are always traceable (see docs/plans/layer-catalog.md LC-13 for the real
// uninstall verb).
func TestLayers_List_SurfacesOrphanedLayers(t *testing.T) {
	root := t.TempDir() // no project packs
	storePath := filepath.Join(t.TempDir(), "s.duckdb")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	claim := core.SemanticClaim{
		ClaimID: "cl-ghost", ServiceID: "svc",
		Layer: "ghost-layer", LayerVersion: "v1", Kind: "ghost-kind",
		Subject: "gone/pack.md#x", Rule: "A rule from an uninstalled pack.",
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
		t.Fatalf("layers list exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "orphaned layers") {
		t.Errorf("output missing the orphaned-layers section:\n%s", got)
	}
	if !strings.Contains(got, "ghost-layer") {
		t.Errorf("output missing the orphaned layer name:\n%s", got)
	}
}

// TestLayers_List_ShowsBrokenPack: a half-authored project pack must be
// VISIBLE in list output (critic finding 1's surfacing half), never silently
// dropped, while the built-ins still tabulate normally.
func TestLayers_List_ShowsBrokenPack(t *testing.T) {
	root := t.TempDir()
	brokenDir := filepath.Join(root, ".fathom", "layers", "half-done")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, "layer.cue"), []byte("not [ cue"), 0o644); err != nil {
		t.Fatalf("write broken layer.cue: %v", err)
	}

	var out, errBuf bytes.Buffer
	code := run([]string{"layers", "list", "--root", root}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("layers list exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "broken project pack") || !strings.Contains(got, "half-done") {
		t.Errorf("output missing the broken-pack notice:\n%s", got)
	}
	if !strings.Contains(got, "obligations") {
		t.Errorf("built-ins must still tabulate beside a broken pack:\n%s", got)
	}
}

// TestLayers_Install_RefusesOverwriteWithoutForce mirrors the package-level
// InstallPack contract at the CLI surface, including the --force escape.
func TestLayers_Install_RefusesOverwriteWithoutForce(t *testing.T) {
	srcDir := filepath.Join(t.TempDir(), "twice-pack")
	writeTestPack(t, srcDir, "twice-pack")
	root := t.TempDir()

	var out, errBuf bytes.Buffer
	if code := run([]string{"layers", "install", "--root", root, srcDir}, &out, &errBuf); code != 0 {
		t.Fatalf("first install exit = %d, stderr: %s", code, errBuf.String())
	}

	out.Reset()
	errBuf.Reset()
	if code := run([]string{"layers", "install", "--root", root, srcDir}, &out, &errBuf); code == 0 {
		t.Fatal("second install without --force: want non-zero exit")
	}
	if !strings.Contains(errBuf.String(), "--force") {
		t.Errorf("refusal should mention --force, got %q", errBuf.String())
	}

	out.Reset()
	errBuf.Reset()
	if code := run([]string{"layers", "install", "--root", root, "--force", srcDir}, &out, &errBuf); code != 0 {
		t.Fatalf("install --force exit = %d, stderr: %s", code, errBuf.String())
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

// ─── A4-1 (docs/plans/activation.md): catalog de-theater ──────────────────
//
// The layers CLI verb gains an EVAL column (pass/not-gated, from
// LayerDef.Evaled — schema data, never hand-maintained prose) and, with
// --store/--service, a LAST_COMPILE column read live from compile_runs
// (never hand-maintained) alongside the existing CLAIMS count — "production"
// truth as the row's own claim count + last compile date, both read from the
// store.

func TestLayers_List_ShowsEvalColumnFromManifestData(t *testing.T) {
	root := t.TempDir()
	var out, errBuf bytes.Buffer
	code := run([]string{"layers", "list", "--root", root}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("layers list exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "EVAL") {
		t.Fatalf("output missing EVAL column header:\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		switch {
		case strings.HasPrefix(line, "obligations"), strings.HasPrefix(line, "adr-rules"), strings.HasPrefix(line, "spec-bindings"):
			if !strings.Contains(line, "pass") {
				t.Errorf("evaled layer row should show a passing eval status: %q", line)
			}
		case strings.HasPrefix(line, "findings"):
			// findings ships without eval/ yet (docs/layers.md) — LayerDef.Evaled
			// is false; the CLI must say so, not silently omit the column.
			if !strings.Contains(line, "not-gated") {
				t.Errorf("un-evaled findings row should say not-gated: %q", line)
			}
		}
	}
}

func TestLayers_List_WithStoreShowsLastCompileDate(t *testing.T) {
	root := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "s.duckdb")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ranAt := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	run2 := core.CompileRun{
		RunID: "run-1", ServiceID: "svc", Layer: "obligations",
		CorpusWatermark: "wm1", PromptVersion: "obligations/abc123",
		DryRun: false, RanAt: ranAt, ClaimsEmitted: 1,
	}
	if err := st.RecordCompileRun(context.Background(), run2); err != nil {
		t.Fatalf("RecordCompileRun: %v", err)
	}
	st.Close()

	var out, errBuf bytes.Buffer
	code := run([]string{"layers", "list", "--root", root, "--store", storePath, "--service", "svc"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("layers list --store exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "LAST_COMPILE") {
		t.Fatalf("output missing LAST_COMPILE column header:\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "obligations") {
			if !strings.Contains(line, "2026-07-20") {
				t.Errorf("obligations row should show the recorded compile date, got %q", line)
			}
		}
		if strings.HasPrefix(line, "adr-rules") {
			// adr-rules has claims/compiles for a DIFFERENT service in this
			// test — its own row for "svc" must show "never", not a stale
			// borrowed date.
			if !strings.Contains(line, "never") {
				t.Errorf("adr-rules row with no compile_runs should say never, got %q", line)
			}
		}
	}
}

// ─── ADR-024 (docs/plans/pack-taxonomy-adoption.md TX-2): STRATUM column ──

func TestLayers_List_ShowsStratumColumn(t *testing.T) {
	root := t.TempDir()
	var out, errBuf bytes.Buffer
	code := run([]string{"layers", "list", "--root", root}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("layers list exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "STRATUM") {
		t.Fatalf("output missing STRATUM column header:\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		switch {
		case strings.HasPrefix(line, "obligations"), strings.HasPrefix(line, "findings"),
			strings.HasPrefix(line, "adr-rules"), strings.HasPrefix(line, "spec-bindings"):
			if !strings.Contains(line, "concern") {
				t.Errorf("shipped pack row should show stratum concern: %q", line)
			}
		}
	}
}

func TestLayers_List_ProjectPackDefaultsToConcernStratum(t *testing.T) {
	root := t.TempDir()
	writeTestPack(t, filepath.Join(root, ".fathom", "layers", "custom"), "custom")

	var out, errBuf bytes.Buffer
	code := run([]string{"layers", "list", "--root", root}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("layers list exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "custom") {
			if !strings.Contains(line, "concern") {
				t.Errorf("project pack with no stratum field should default to concern: %q", line)
			}
		}
	}
}

func TestLayers_Show_ShowsStratum(t *testing.T) {
	root := t.TempDir()
	var out, errBuf bytes.Buffer
	code := run([]string{"layers", "show", "--root", root, "obligations"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("layers show exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "stratum:") || !strings.Contains(got, "concern") {
		t.Errorf("show output missing stratum:\n%s", got)
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
