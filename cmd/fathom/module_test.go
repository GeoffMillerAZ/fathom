package main

// module_test.go — coverage for the `fathom module` verb family (Phase J,
// docs/vision/13-expert-suite.md §4/§9 row J): init, sources, map, compile,
// eval, doctor, pack. Every test uses t.TempDir() (never the repo's own
// tree) and the mock LLM lane for compile/eval — no network, no key,
// matching the phase's cold-run mandate.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/modspec"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func runModule(t *testing.T, args ...string) (stdout, stderr string, rc int) {
	t.Helper()
	var out, errOut bytes.Buffer
	rc = cmdModule(args, &out, &errOut)
	return out.String(), errOut.String(), rc
}

// ─── init ───────────────────────────────────────────────────────────────

func TestModuleInit_ScaffoldsLoadableManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, rc := runModule(
		t, "init",
		"--dir", dir,
		"--name", "my-module",
		"--description", "A test module.",
		"--include", "README.md",
	)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr)
	}
	if !strings.Contains(stdout, "wrote") {
		t.Errorf("stdout = %q", stdout)
	}

	path := filepath.Join(dir, "module.cue")
	if err := modspec.ValidateStrict(path); err != nil {
		t.Fatalf("ValidateStrict: %v", err)
	}
	m, err := modspec.LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Name != "my-module" {
		t.Errorf("Name = %q", m.Name)
	}
	if m.Watermark == "" {
		t.Error("Watermark must not be empty")
	}
}

func TestModuleInit_RefusesOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	_, _, rc := runModule(t, "init", "--dir", dir, "--name", "m1", "--description", "d", "--include", "*.md")
	if rc != 0 {
		t.Fatalf("first init rc=%d", rc)
	}
	_, stderr, rc := runModule(t, "init", "--dir", dir, "--name", "m2", "--description", "d", "--include", "*.md")
	if rc == 0 {
		t.Fatal("expected the second init to refuse without --force")
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("stderr = %q", stderr)
	}
	// --force overwrites.
	_, stderr, rc = runModule(t, "init", "--dir", dir, "--name", "m2", "--description", "d", "--include", "*.md", "--force")
	if rc != 0 {
		t.Fatalf("forced init rc=%d stderr=%s", rc, stderr)
	}
	m, err := modspec.LoadManifest(filepath.Join(dir, "module.cue"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "m2" {
		t.Errorf("Name = %q, want m2 (force should have overwritten)", m.Name)
	}
}

// ─── sources ────────────────────────────────────────────────────────────

func TestModuleSources_FlagsZeroMatchSelector(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "module.cue")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("m1", []string{"README.md", "does-not-exist/**"}, testWatermark(t, dir, "README.md"))), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, rc := runModule(t, "sources", "--manifest", manifestPath, "--json")
	if rc != 1 {
		t.Fatalf("rc=%d, want 1 (zero-match selector present); stdout=%s", rc, stdout)
	}
	var inv sourceInventory
	if err := json.Unmarshal([]byte(stdout), &inv); err != nil {
		t.Fatalf("parse JSON: %v\n%s", err, stdout)
	}
	if len(inv.ZeroMatch) != 1 || inv.ZeroMatch[0] != "does-not-exist/**" {
		t.Errorf("ZeroMatch = %v", inv.ZeroMatch)
	}
	if inv.MatchedBySelect["README.md"] != 1 {
		t.Errorf("MatchedBySelect[README.md] = %d, want 1", inv.MatchedBySelect["README.md"])
	}
}

func TestModuleSources_NoZeroMatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "module.cue")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("m1", []string{"README.md"}, testWatermark(t, dir, "README.md"))), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, rc := runModule(t, "sources", "--manifest", manifestPath)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr)
	}
}

// ─── map ────────────────────────────────────────────────────────────────

func TestModuleMap_NoStoreGivenIsHonestNote(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "module.cue")
	wm := testWatermark(t, dir, "README.md")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("m1", []string{"README.md"}, wm)), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, rc := runModule(t, "map", "--manifest", manifestPath, "--json")
	if rc != 1 {
		t.Fatalf("rc=%d, want 1 (the store_not_provided gap always fires without --store); stdout=%s", rc, stdout)
	}
	var report MapReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("parse JSON: %v\n%s", err, stdout)
	}
	found := false
	for _, g := range report.Gaps {
		if g.Code == GapStoreNotProvided {
			found = true
		}
	}
	if !found {
		t.Errorf("Gaps = %+v, want a %s gap", report.Gaps, GapStoreNotProvided)
	}
}

func TestModuleMap_WatermarkDriftDetected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "module.cue")
	// A bogus but well-formed watermark: guaranteed to mismatch the corpus's
	// real current watermark.
	bogus := "manifest:0000000000000000000000000000000000000000000000000000000000000000"
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("m1", []string{"README.md"}, bogus)), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, rc := runModule(t, "map", "--manifest", manifestPath, "--json")
	if rc != 1 {
		t.Fatalf("rc=%d stdout=%s", rc, stdout)
	}
	var report MapReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("parse JSON: %v\n%s", err, stdout)
	}
	found := false
	for _, g := range report.Gaps {
		if g.Code == GapWatermarkDrift {
			found = true
		}
	}
	if !found {
		t.Errorf("Gaps = %+v, want a %s gap", report.Gaps, GapWatermarkDrift)
	}
}

// ─── compile / eval (mock lane) ─────────────────────────────────────────

// writeTestLayerPack scaffolds a minimal project-local layer pack at
// <root>/.fathom/layers/<name>/{layer.cue,prompt.md} — the prerequisite
// `module compile`/`module eval` document (a module's compile identity is
// its same-named layer pack, LC-3's LoadLayerIn).
func writeTestLayerPack(t *testing.T, root, name string, include []string) {
	t.Helper()
	dir := filepath.Join(root, ".fathom", "layers", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	incs := make([]string, len(include))
	for i, s := range include {
		incs[i] = `"` + s + `"`
	}
	layerCUE := "name: \"" + name + "\"\n" +
		"version: \"v1\"\n" +
		"kind: \"obligation\"\n" +
		"corpus: include: [" + strings.Join(incs, ", ") + "]\n" +
		"prompt_version: \"" + name + "/content-addressed\"\n" +
		"prompt_file: \".fathom/layers/" + name + "/prompt.md\"\n" +
		"tier: \"doc-claimed\"\n" +
		"stratum: \"platform\"\n" +
		"evaled: true\n" +
		"budgets: {\n\tmax_tokens_per_run: 200000\n\tmax_usd_per_run: 1.0\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "layer.cue"), []byte(layerCUE), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte("Compile governing rules from the given section.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestModuleCompile_MockStoreDirect(t *testing.T) {
	root := t.TempDir()
	doc := "## Sorting\n\nAlways sort output before returning it.\n"
	if err := os.WriteFile(filepath.Join(root, "RULES.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestLayerPack(t, root, "compile-test-module", []string{"RULES.md"})

	manifestPath := filepath.Join(root, "module.cue")
	wm := testWatermark(t, root, "RULES.md")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("compile-test-module", []string{"RULES.md"}, wm)), 0o644); err != nil {
		t.Fatal(err)
	}

	storePath := filepath.Join(t.TempDir(), "module.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	// Seed the store as if `fathom ingest` had already run for this service —
	// module compile reads the service's corpus via pkg/client.Sources,
	// which reads from the connector directly (not from stored
	// observations), so no observation rows are required here; only
	// ServiceMeta (for the idempotency guard's root lookup) matters.
	if err := st.SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: "compile-test-module", Path: root, IndexedCommit: wm}); err != nil {
		t.Fatal(err)
	}
	st.Close()

	stdout, stderr, rc := runModule(
		t, "compile",
		"--manifest", manifestPath,
		"--store", storePath,
		"--dry-run=true",
	)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s stdout=%s", rc, stderr, stdout)
	}
	if !strings.Contains(stdout, "compile-test-module") {
		t.Errorf("stdout = %q", stdout)
	}
}

// TestModuleCompile_MockLaneSpends proves the --mock lane (moduleCompileMock)
// actually runs a real (non-dry-run) compile through the SAME production
// entry point (pkg/client.Client.CompileLayerWithEscape) with a scripted
// MockClient — no network, no key, matching scripts/module-cold-run.sh's
// requirement. Claims must actually land in the store.
func TestModuleCompile_MockLaneSpends(t *testing.T) {
	root := t.TempDir()
	doc := "## Sorting\n\nAlways sort output before returning it.\n"
	if err := os.WriteFile(filepath.Join(root, "RULES.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestLayerPack(t, root, "mock-compile-module", []string{"RULES.md"})

	manifestPath := filepath.Join(root, "module.cue")
	wm := testWatermark(t, root, "RULES.md")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("mock-compile-module", []string{"RULES.md"}, wm)), 0o644); err != nil {
		t.Fatal(err)
	}

	storePath := filepath.Join(t.TempDir(), "module.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	// A REAL ingest (not just a hand-seeded ServiceMeta row): verifyDraft's
	// scope check (internal/semantic/compile.go) rejects a claim whose scope
	// glob matches zero INDEXED files, and the indexed-file list comes from
	// stored observations (pkg/client.CompileLayerWithEscape reads them via
	// QueryObservations) — a service with ServiceMeta but no observations
	// looks like an empty index and every claim is rejected.
	cl := client.New(connector.NewLocalConnector(), st)
	if err := cl.IngestServiceAt(context.Background(), "mock-compile-module", root); err != nil {
		t.Fatalf("IngestServiceAt: %v", err)
	}
	cl.Close()
	st.Close()

	// The mock-entries file lives OUTSIDE root: dropping it inside the
	// corpus root would change root's content-addressed watermark
	// (manifest: prefix, non-git corpus) AFTER wm was already computed and
	// baked into module.cue, producing a spurious watermark-drift gap that
	// has nothing to do with the module's actual corpus.
	entriesPath := filepath.Join(t.TempDir(), "mock-entries.json")
	entriesJSON := `["{\"claims\":[{\"rule\":\"Always sort output before returning it.\",\"scope\":[\"RULES.md\"],\"cite_start\":3,\"cite_end\":3}]}"]`
	if err := os.WriteFile(entriesPath, []byte(entriesJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, rc := runModule(
		t, "compile",
		"--manifest", manifestPath,
		"--store", storePath,
		"--dry-run=false",
		"--mock", "--mock-entries-file", entriesPath,
		"--json",
	)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s stdout=%s", rc, stderr, stdout)
	}
	var run map[string]any
	if err := json.Unmarshal([]byte(stdout), &run); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout)
	}
	if n, _ := run["ClaimsEmitted"].(float64); n != 1 {
		t.Errorf("ClaimsEmitted = %v, want 1: %v", run["ClaimsEmitted"], run)
	}

	st2, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	claims, err := st2.ClaimsForService(context.Background(), "mock-compile-module", "mock-compile-module")
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 1 {
		t.Errorf("stored claims = %d, want 1", len(claims))
	}

	// Regression (found by scripts/module-cold-run.sh's real end-to-end
	// run): `module map`'s anchor-health check used a FRESH
	// connector.NewLocalConnector() and called Fetch directly without a
	// preceding Discover on that same instance — LocalConnector.Fetch
	// requires Discover to have recorded the service's root first, so
	// every citation looked like a broken anchor ("no longer exists"),
	// even though the corpus was untouched. A real, unbroken anchor must
	// report ZERO gaps here.
	stdout2, stderr2, rc2 := runModule(t, "map", "--manifest", manifestPath, "--store", storePath, "--json")
	if rc2 != 0 {
		t.Fatalf("map rc=%d stderr=%s stdout=%s", rc2, stderr2, stdout2)
	}
	var mapReport MapReport
	if err := json.Unmarshal([]byte(stdout2), &mapReport); err != nil {
		t.Fatalf("parse map JSON: %v\n%s", err, stdout2)
	}
	if len(mapReport.Gaps) != 0 {
		t.Errorf("Gaps = %+v, want none (the corpus has not changed since compile)", mapReport.Gaps)
	}
}

// TestModuleRuns_ListsCompileHistory pins the fix for dogfood gap (c): only
// the just-triggered `module compile` result was visible to a verb-JSON-only
// consumer before this verb existed — every EARLIER compile_runs row was
// invisible outside a direct SQLite query. `module runs --json` reuses the
// same store read module doctor's loadStoreState already makes
// (Store.CompileRuns), which is already newest-first and deterministic.
func TestModuleRuns_ListsCompileHistory(t *testing.T) {
	root := t.TempDir()
	doc := "## Sorting\n\nAlways sort output before returning it.\n"
	if err := os.WriteFile(filepath.Join(root, "RULES.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestLayerPack(t, root, "runs-module", []string{"RULES.md"})

	manifestPath := filepath.Join(root, "module.cue")
	wm := testWatermark(t, root, "RULES.md")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("runs-module", []string{"RULES.md"}, wm)), 0o644); err != nil {
		t.Fatal(err)
	}

	storePath := filepath.Join(t.TempDir(), "module.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	cl := client.New(connector.NewLocalConnector(), st)
	if err := cl.IngestServiceAt(context.Background(), "runs-module", root); err != nil {
		t.Fatalf("IngestServiceAt: %v", err)
	}
	cl.Close()
	st.Close()

	// Before any compile: an empty, non-error runs report.
	stdout, stderr, rc := runModule(t, "runs", "--manifest", manifestPath, "--store", storePath, "--json")
	if rc != 0 {
		t.Fatalf("runs (pre-compile) rc=%d stderr=%s stdout=%s", rc, stderr, stdout)
	}
	var pre RunsReport
	if err := json.Unmarshal([]byte(stdout), &pre); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout)
	}
	if len(pre.Runs) != 0 {
		t.Errorf("pre-compile Runs = %+v, want none", pre.Runs)
	}
	if pre.Module != "runs-module" || pre.Layer != "runs-module" {
		t.Errorf("Module/Layer = %q/%q, want runs-module/runs-module", pre.Module, pre.Layer)
	}

	entriesPath := filepath.Join(t.TempDir(), "mock-entries.json")
	entriesJSON := `["{\"claims\":[{\"rule\":\"Always sort output before returning it.\",\"scope\":[\"RULES.md\"],\"cite_start\":3,\"cite_end\":3}]}"]`
	if err := os.WriteFile(entriesPath, []byte(entriesJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, rc = runModule(
		t, "compile",
		"--manifest", manifestPath, "--store", storePath,
		"--dry-run=false", "--mock", "--mock-entries-file", entriesPath,
	)
	if rc != 0 {
		t.Fatalf("compile rc=%d stderr=%s", rc, stderr)
	}

	stdout, stderr, rc = runModule(t, "runs", "--manifest", manifestPath, "--store", storePath, "--json")
	if rc != 0 {
		t.Fatalf("runs rc=%d stderr=%s stdout=%s", rc, stderr, stdout)
	}
	var report RunsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout)
	}
	if len(report.Runs) != 1 {
		t.Fatalf("Runs = %+v, want exactly 1", report.Runs)
	}
	run := report.Runs[0]
	if run.ID == "" {
		t.Error("run.ID is empty")
	}
	if run.StartedAt == "" {
		t.Error("run.StartedAt is empty")
	}
	if run.Lane != "mock" {
		t.Errorf("run.Lane = %q, want %q (a --mock compile)", run.Lane, "mock")
	}
	if run.Model != mockModelName {
		t.Errorf("run.Model = %q, want %q", run.Model, mockModelName)
	}
	if run.Watermark != wm {
		t.Errorf("run.Watermark = %q, want %q (the manifest's own watermark — the mock compile's corpus was untouched)", run.Watermark, wm)
	}
	if run.ClaimsEmitted != 1 {
		t.Errorf("run.ClaimsEmitted = %d, want 1", run.ClaimsEmitted)
	}

	// --limit truncates without disturbing newest-first ordering (a single
	// run here, so this just proves --limit 0 still returns it and doesn't
	// error).
	stdout, stderr, rc = runModule(t, "runs", "--manifest", manifestPath, "--store", storePath, "--limit", "0", "--json")
	if rc != 0 {
		t.Fatalf("runs --limit 0 rc=%d stderr=%s stdout=%s", rc, stderr, stdout)
	}
	var unlimited RunsReport
	if err := json.Unmarshal([]byte(stdout), &unlimited); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout)
	}
	if len(unlimited.Runs) != 1 {
		t.Errorf("--limit 0 Runs = %+v, want exactly 1 (unlimited, not zero)", unlimited.Runs)
	}
}

func TestModuleEval_MockLane(t *testing.T) {
	root := t.TempDir()
	writeTestLayerPack(t, root, "eval-test-module", []string{"doc.md"})

	manifestPath := filepath.Join(root, "module.cue")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("eval-test-module", []string{"doc.md"}, testWatermark(t, root, "doc.md"))), 0o644); err != nil {
		t.Fatal(err)
	}

	evalDir := filepath.Join(root, "eval")
	if err := os.MkdirAll(evalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evalDir, "doc.md"), []byte("## Sorting\n\nAlways sort output before returning it.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	labels := `{"docs":["doc.md"],"probes":["a.go","b.go"],"rules":[{"key":"sort","cite_substr":"Always sort output","governs":["a.go","b.go"]}]}`
	if err := os.WriteFile(filepath.Join(evalDir, "labels.json"), []byte(labels), 0o644); err != nil {
		t.Fatal(err)
	}

	entriesPath := filepath.Join(root, "mock-entries.json")
	entriesJSON := `["{\"claims\":[{\"rule\":\"Always sort output before returning it.\",\"scope\":[\"a.go\",\"b.go\"],\"cite_start\":3,\"cite_end\":3}]}"]`
	if err := os.WriteFile(entriesPath, []byte(entriesJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, rc := runModule(
		t, "eval",
		"--manifest", manifestPath,
		"--mock", "--mock-entries-file", entriesPath,
		"--json",
	)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s stdout=%s", rc, stderr, stdout)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\n%s", err, stdout)
	}
	if result["passes_gate"] != true {
		t.Errorf("passes_gate = %v, want true; full result: %v", result["passes_gate"], result)
	}
}

func TestModuleEval_RequiresProfileOrMock(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "module.cue")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("x", []string{"README.md"}, testWatermark(t, dir, "README.md"))), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, rc := runModule(t, "eval", "--manifest", manifestPath)
	if rc == 0 {
		t.Fatal("expected an error when neither --profile nor --mock is given")
	}
	if !strings.Contains(stderr, "--profile") || !strings.Contains(stderr, "--mock") {
		t.Errorf("stderr = %q", stderr)
	}
}

// ─── doctor ─────────────────────────────────────────────────────────────

// TestModuleDoctor_LadderProgression is the real, store-backed progression
// (Phase J critic round: doctor's criteria are checked against the STORE,
// never author-typed text) — draft, then eval-passed via a genuine
// EvalArtifact cross-checked against the store's own compiled
// prompt_version, then servable via a real (mock-lane) compile whose
// recorded watermark is directly comparable to module.cue's own (both
// module-scoped, via scopedWatermarkConnector — see module_compileeval.go).
// It also proves the mock lane, having reached servable honestly, is
// EXPLICITLY refused at published (finding 2) — the ladder never silently
// launders a scripted/dev compile into a production verdict.
func TestModuleDoctor_LadderProgression(t *testing.T) {
	root := t.TempDir()
	doc := "## Sorting\n\nAlways sort output before returning it.\n"
	if err := os.WriteFile(filepath.Join(root, "RULES.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestLayerPack(t, root, "doctor-module", []string{"RULES.md"})
	manifestPath := filepath.Join(root, "module.cue")

	// Stage 1: draft only (module init scaffolds a real, scoped-watermark
	// manifest; no store, no eval evidence, default license).
	_, stderr, rc := runModule(
		t, "init",
		"--dir", root, "--name", "doctor-module", "--description", "d",
		"--include", "RULES.md", "--license", "Apache-2.0",
	)
	if rc != 0 {
		t.Fatalf("init rc=%d stderr=%s", rc, stderr)
	}
	stdout, _, rc := runModule(t, "doctor", "--manifest", manifestPath, "--json")
	if rc != 0 {
		t.Fatalf("doctor rc=%d stdout=%s", rc, stdout)
	}
	var report DoctorReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout)
	}
	if report.Verdict != LevelDraft {
		t.Errorf("verdict = %q, want %q; criteria=%+v", report.Verdict, LevelDraft, report.Criteria)
	}
	// Dogfood gap (a): the manifest summary — name/version/description/
	// license/tier_profile/provenance/corpus selector count/watermark — is
	// on DoctorReport so a verb-JSON-only consumer never needs a separate
	// manifest dump.
	if report.Manifest == nil {
		t.Fatal("Manifest is nil, want a summary once the manifest loads")
	}
	if report.Manifest.Name != "doctor-module" {
		t.Errorf("Manifest.Name = %q, want %q", report.Manifest.Name, "doctor-module")
	}
	if report.Manifest.License != "Apache-2.0" {
		t.Errorf("Manifest.License = %q, want %q", report.Manifest.License, "Apache-2.0")
	}
	if report.Manifest.CorpusSelectorCount != 1 {
		t.Errorf("Manifest.CorpusSelectorCount = %d, want 1 (one --include glob, no --exclude)", report.Manifest.CorpusSelectorCount)
	}
	if report.Manifest.Watermark == "" {
		t.Error("Manifest.Watermark is empty")
	}

	// Stage 2/3 setup: a real mock-lane compile — scopedWatermarkConnector
	// makes its recorded compile_runs.CorpusWatermark module-scoped, the
	// SAME algorithm module.cue's own watermark field uses, so they stay
	// comparable even after module.cue is edited again below (finding 3's
	// fix; RULES.md's scoped hash never depends on module.cue's own bytes).
	storePath := filepath.Join(t.TempDir(), "module.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	// A real ingest (not just a hand-seeded ServiceMeta row): the layer-pack
	// resolution inside CompileLayerWithEscape reads the service's root from
	// ServiceMeta, and the scope-verification pass needs RULES.md to appear
	// as an indexed file (see TestModuleCompile_MockLaneSpends's identical
	// note above).
	seedClient := client.New(connector.NewLocalConnector(), st)
	if err := seedClient.IngestServiceAt(context.Background(), "doctor-module", root); err != nil {
		t.Fatalf("IngestServiceAt: %v", err)
	}
	seedClient.Close()
	st.Close()
	compileEntries := filepath.Join(t.TempDir(), "compile-entries.json")
	if err := os.WriteFile(compileEntries, []byte(`["{\"claims\":[{\"rule\":\"Always sort output before returning it.\",\"scope\":[\"RULES.md\"],\"cite_start\":3,\"cite_end\":3}]}"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, rc = runModule(
		t, "compile",
		"--manifest", manifestPath, "--store", storePath,
		"--dry-run=false", "--mock", "--mock-entries-file", compileEntries,
	)
	if rc != 0 {
		t.Fatalf("compile rc=%d stderr=%s", rc, stderr)
	}

	// Eval, --record'ing a real EvalArtifact — its prompt_version comes
	// from RunEval's own compile of the SAME layer pack, so it matches the
	// store's compiled claims' PromptVersion exactly.
	evalDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(evalDir, "doc.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	labels := `{"docs":["doc.md"],"probes":["a.go","b.go"],"rules":[{"key":"sort","cite_substr":"Always sort output","governs":["a.go","b.go"]}]}`
	if err := os.WriteFile(filepath.Join(evalDir, "labels.json"), []byte(labels), 0o644); err != nil {
		t.Fatal(err)
	}
	evalEntries := filepath.Join(t.TempDir(), "eval-entries.json")
	if err := os.WriteFile(evalEntries, []byte(`["{\"claims\":[{\"rule\":\"Always sort output before returning it.\",\"scope\":[\"a.go\",\"b.go\"],\"cite_start\":3,\"cite_end\":3}]}"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(t.TempDir(), "eval-artifact.json")
	_, stderr, rc = runModule(
		t, "eval",
		"--manifest", manifestPath, "--fixtures", evalDir,
		"--mock", "--mock-entries-file", evalEntries,
		"--record", artifactPath,
	)
	if rc != 0 {
		t.Fatalf("eval rc=%d stderr=%s", rc, stderr)
	}

	// Point eval_evidence at the external artifact (outside root, so it
	// never perturbs RULES.md's scoped watermark) — this IS the documented
	// hand-edit workflow (docs/plans/expert-suite-j.md's deferred-items
	// note: no verb writes eval_evidence back automatically).
	manifestSrc, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(manifestSrc), "eval_evidence: evaled: false", "eval_evidence: {\n\tevaled: true\n\tpath: \""+artifactPath+"\"\n}", 1)
	if edited == string(manifestSrc) {
		t.Fatal("eval_evidence replacement did not match — Template's output shape changed?")
	}
	if err := os.WriteFile(manifestPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// Stage 2: eval-passed.
	stdout, _, rc = runModule(t, "doctor", "--manifest", manifestPath, "--store", storePath, "--json")
	if rc != 0 {
		t.Fatalf("doctor rc=%d stdout=%s", rc, stdout)
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	evalCriterion := findCriterion(report, LevelEvalPassed, "eval evidence present, gate-passing, and prompt_version-fresh (store-checked)")
	if evalCriterion == nil || !evalCriterion.Pass {
		t.Fatalf("eval-passed criterion did not pass: %+v; full report=%+v", evalCriterion, report)
	}
	// Dogfood gap (b): typed facts alongside the prose — precision/recall/
	// lane/prompt_version were previously readable only by parsing
	// Evidence's sentence.
	if evalCriterion.Facts == nil {
		t.Fatal("eval-passed criterion has no Facts, want precision/recall/lane/prompt_version")
	}
	if evalCriterion.Facts["lane"] != "mock" {
		t.Errorf("eval Facts[lane] = %q, want %q (a --mock eval)", evalCriterion.Facts["lane"], "mock")
	}
	if evalCriterion.Facts["prompt_version"] == "" {
		t.Error("eval Facts[prompt_version] is empty")
	}
	if _, err := strconv.ParseFloat(evalCriterion.Facts["precision"], 64); err != nil {
		t.Errorf("eval Facts[precision] = %q is not a float: %v", evalCriterion.Facts["precision"], err)
	}
	watermarkCriterion := findCriterion(report, LevelServable, "watermark fresh: manifest == store's recorded compile == current corpus")
	if watermarkCriterion == nil || watermarkCriterion.Facts["manifest_watermark"] == "" {
		t.Errorf("watermark criterion Facts[manifest_watermark] missing: %+v", watermarkCriterion)
	}

	// Stage 3: servable — the mock compile's scoped watermark matches
	// module.cue's own, and RULES.md's citation is intact.
	if report.Verdict != LevelServable {
		t.Errorf("verdict = %q, want %q; criteria=%+v", report.Verdict, LevelServable, report.Criteria)
	}

	// Stage 4: published is explicitly REFUSED — the store's claims were
	// compiled under mockModelName, and init's default provenance.compiled_by
	// is "unspecified"; either alone would refuse, and both fire.
	packDir := filepath.Join(t.TempDir(), "packed")
	_, stderr, rc = runModule(t, "pack", "--manifest", manifestPath, "--store", storePath, "--out", packDir)
	if rc != 0 {
		t.Fatalf("pack rc=%d stderr=%s", rc, stderr)
	}
	stdout, _, rc = runModule(t, "doctor", "--manifest", manifestPath, "--store", storePath, "--pack-dir", packDir, "--json")
	if rc != 0 {
		t.Fatalf("doctor rc=%d stdout=%s", rc, stdout)
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.Verdict != LevelServable {
		t.Errorf("verdict = %q, want %q (a mock-lane compile must never reach published); criteria=%+v", report.Verdict, LevelServable, report.Criteria)
	}
	laneCriterion := findCriterion(report, LevelPublished, "compile lane is real (not mock/dev) and matches provenance.compiled_by")
	if laneCriterion == nil || laneCriterion.Pass {
		t.Fatalf("compile-lane criterion should FAIL for a mock compile: %+v", laneCriterion)
	}
	if !strings.Contains(laneCriterion.Evidence, mockModelName) {
		t.Errorf("compile-lane evidence = %q, want it to name %q", laneCriterion.Evidence, mockModelName)
	}
	if laneCriterion.Facts["lane"] != "mock" || laneCriterion.Facts["model"] != mockModelName {
		t.Errorf("compile-lane Facts = %+v, want lane=mock model=%q", laneCriterion.Facts, mockModelName)
	}
}

// findCriterion returns the first matching Criterion, or nil.
func findCriterion(report DoctorReport, level, name string) *Criterion {
	for i := range report.Criteria {
		if report.Criteria[i].Level == level && report.Criteria[i].Name == name {
			return &report.Criteria[i]
		}
	}
	return nil
}

func TestModuleDoctor_ManifestDoesNotLoad(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "module.cue")
	if err := os.WriteFile(manifestPath, []byte("not valid cue {{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, rc := runModule(t, "doctor", "--manifest", manifestPath, "--json")
	if rc != 0 {
		t.Fatalf("doctor rc=%d stdout=%s", rc, stdout)
	}
	var report DoctorReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.Verdict == LevelDraft {
		t.Error("an unloadable manifest must not reach draft")
	}
}

// ─── pack ───────────────────────────────────────────────────────────────

func TestModulePack_RoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "RULES.md"), []byte("## R\n\nRule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "module.cue")
	wm := testWatermark(t, root, "RULES.md")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("pack-module", []string{"RULES.md"}, wm)), 0o644); err != nil {
		t.Fatal(err)
	}

	storePath := filepath.Join(t.TempDir(), "module.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	st.Close()

	outDir := filepath.Join(t.TempDir(), "packed")
	stdout, stderr, rc := runModule(
		t, "pack",
		"--manifest", manifestPath,
		"--store", storePath,
		"--out", outDir,
		"--json",
	)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s stdout=%s", rc, stderr, stdout)
	}
	var report PackReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout)
	}
	if report.Module != "pack-module" {
		t.Errorf("Module = %q", report.Module)
	}
	if report.StoreBytes == 0 {
		t.Error("StoreBytes = 0")
	}

	for _, want := range []string{"module.sqlite", "module.cue", "README.md"} {
		if _, err := os.Stat(filepath.Join(outDir, want)); err != nil {
			t.Errorf("expected %s to exist: %v", want, err)
		}
	}
	mounted, err := store.MountReadOnly(filepath.Join(outDir, "module.sqlite"), store.LevelL0)
	if err != nil {
		t.Fatalf("the packed copy failed conformance: %v", err)
	}
	mounted.Close()
}

func TestModulePack_RefusesNonEmptyOutWithoutForce(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "RULES.md"), []byte("## R\n\nRule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "module.cue")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("pack-module-2", []string{"RULES.md"}, testWatermark(t, root, "RULES.md"))), 0o644); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(t.TempDir(), "module.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	st.Close()

	outDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outDir, "stray.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, rc := runModule(t, "pack", "--manifest", manifestPath, "--store", storePath, "--out", outDir)
	if rc == 0 {
		t.Fatal("expected refusal on a non-empty --out without --force")
	}
	if !strings.Contains(stderr, "--force") {
		t.Errorf("stderr = %q", stderr)
	}
}

// TestModulePack_NoWALSidecars proves the VACUUM INTO fix (finding 6): the
// packed copy carries no -wal/-shm sidecar files — the exact artifact the
// old checkpoint+copy implementation could leak, both from a busy
// checkpoint (frames silently dropped, not reproducible here without a
// concurrent writer) and from the LevelWriter conformance probe's leaked
// handle (fully reproducible: MountReadOnly opens in WAL-capable mode, and
// a real leaked handle leaves its own -wal/-shm behind).
func TestModulePack_NoWALSidecars(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "RULES.md"), []byte("## R\n\nRule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "module.cue")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("pack-nowal", []string{"RULES.md"}, testWatermark(t, root, "RULES.md"))), 0o644); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(t.TempDir(), "module.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	st.Close()

	outDir := filepath.Join(t.TempDir(), "packed")
	_, stderr, rc := runModule(t, "pack", "--manifest", manifestPath, "--store", storePath, "--out", outDir)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr)
	}
	for _, sidecar := range []string{"module.sqlite-wal", "module.sqlite-shm"} {
		if _, err := os.Stat(filepath.Join(outDir, sidecar)); err == nil {
			t.Errorf("packed artifact carries a %s sidecar — VACUUM INTO should never produce one", sidecar)
		}
	}
}

// TestModuleDoctor_PackTamperingRefused proves finding 4: `module doctor`'s
// pack criterion is a real conformance + identity + claim-set check, not
// two filename Stats. A packed copy whose module.cue was hand-edited after
// packing (a different watermark than the manifest under review) is
// refused, even though both files still exist and the store still mounts.
func TestModuleDoctor_PackTamperingRefused(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "RULES.md"), []byte("## R\n\nRule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "module.cue")
	wm := testWatermark(t, root, "RULES.md")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("pack-tamper", []string{"RULES.md"}, wm)), 0o644); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(t.TempDir(), "module.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	st.Close()

	packDir := filepath.Join(t.TempDir(), "packed")
	_, stderr, rc := runModule(t, "pack", "--manifest", manifestPath, "--store", storePath, "--out", packDir)
	if rc != 0 {
		t.Fatalf("pack rc=%d stderr=%s", rc, stderr)
	}

	// Tamper: rewrite the PACKED module.cue with a different watermark —
	// two Stats would still see both files present; the real check must
	// catch the identity mismatch.
	packedManifestPath := filepath.Join(packDir, "module.cue")
	tampered := strings.Replace(testManifestCUE("pack-tamper", []string{"RULES.md"}, wm), wm, "manifest:1111111111111111111111111111111111111111111111111111111111111111", 1)
	if err := os.WriteFile(packedManifestPath, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, rc := runModule(t, "doctor", "--manifest", manifestPath, "--pack-dir", packDir, "--json")
	if rc != 0 {
		t.Fatalf("doctor rc=%d stdout=%s", rc, stdout)
	}
	var report DoctorReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	packCriterion := findCriterion(report, LevelPublished, "pack artifact conforms and matches this manifest")
	if packCriterion == nil || packCriterion.Pass {
		t.Fatalf("expected the pack criterion to FAIL on a tampered packed manifest: %+v", packCriterion)
	}
	if !strings.Contains(packCriterion.Evidence, "does not match") {
		t.Errorf("evidence = %q, want it to name the mismatch", packCriterion.Evidence)
	}
}

// TestModuleDoctor_RequireLevel proves finding 9: doctor's exit code is
// meaningful when --require-level is given, instead of always 0.
func TestModuleDoctor_RequireLevel(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "module.cue")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("require-level-module", []string{"README.md"}, testWatermark(t, dir, "README.md"))), 0o644); err != nil {
		t.Fatal(err)
	}

	// draft is reached: --require-level draft passes.
	_, _, rc := runModule(t, "doctor", "--manifest", manifestPath, "--require-level", "draft")
	if rc != 0 {
		t.Errorf("--require-level draft: rc=%d, want 0 (draft is reached)", rc)
	}

	// servable is NOT reached (no store, no eval): --require-level servable fails.
	_, stderr, rc := runModule(t, "doctor", "--manifest", manifestPath, "--require-level", "servable")
	if rc == 0 {
		t.Error("--require-level servable: rc=0, want non-zero (servable is not reached)")
	}
	if !strings.Contains(stderr, "servable") {
		t.Errorf("stderr = %q, want it to name the required level", stderr)
	}

	// An unknown level is a usage error, not silently accepted.
	_, _, rc = runModule(t, "doctor", "--manifest", manifestPath, "--require-level", "not-a-level")
	if rc == 0 {
		t.Error("expected --require-level not-a-level to be refused")
	}
}

// TestModuleMap_ClaimsWithNoCitationsIsNotAVacuousPass proves finding 10:
// a store with compiled claims but ZERO "cites" links must NOT report "0
// broken anchors" — that would be a vacuous pass on exactly the
// hand-built/foreign-store case packaging invites.
func TestModuleMap_ClaimsWithNoCitationsIsNotAVacuousPass(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "RULES.md"), []byte("## R\n\nRule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "module.cue")
	wm := testWatermark(t, root, "RULES.md")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("no-citations-module", []string{"RULES.md"}, wm)), 0o644); err != nil {
		t.Fatal(err)
	}

	storePath := filepath.Join(t.TempDir(), "module.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	// Hand-build a claim with NO links at all — a store this phase's own
	// `module compile` would never produce (verifyDraft always attaches a
	// cites link), but exactly the shape an arbitrary hand-built or foreign
	// store could carry.
	claim := core.SemanticClaim{
		ClaimID:     "deadbeef",
		ServiceID:   "no-citations-module",
		Layer:       "no-citations-module",
		Kind:        "obligation",
		Subject:     "RULES.md#r",
		Rule:        "A rule with no citation.",
		Scope:       []string{"RULES.md"},
		Status:      core.ClaimFresh,
		ReproKey:    "x",
		InputDigest: "x",
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, nil); err != nil {
		t.Fatal(err)
	}
	st.Close()

	stdout, _, rc := runModule(t, "map", "--manifest", manifestPath, "--store", storePath, "--json")
	if rc != 1 {
		t.Fatalf("rc=%d, want 1 (a no-citations claim must be a gap); stdout=%s", rc, stdout)
	}
	var report MapReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout)
	}
	found := false
	for _, g := range report.Gaps {
		if g.Code == GapNoCitations {
			found = true
		}
	}
	if !found {
		t.Errorf("Gaps = %+v, want a %s gap", report.Gaps, GapNoCitations)
	}
}

// TestModuleMap_NeverWritesToStore proves finding 5: `module map` (and by
// extension `module doctor`, which delegates to the same computeMap)
// mounts the store READ-ONLY — MountReadOnly refuses a path that does not
// exist rather than creating one, unlike store.Open (which would run
// CREATE TABLE and silently "succeed" against an empty file it just
// stamped fathom's own schema onto).
func TestModuleMap_NeverWritesToStore(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "RULES.md"), []byte("## R\n\nRule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "module.cue")
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("readonly-module", []string{"RULES.md"}, testWatermark(t, root, "RULES.md"))), 0o644); err != nil {
		t.Fatal(err)
	}

	nonexistent := filepath.Join(t.TempDir(), "does-not-exist.sqlite")
	_, stderr, rc := runModule(t, "map", "--manifest", manifestPath, "--store", nonexistent)
	if rc == 0 {
		t.Fatal("expected `module map` against a nonexistent store to error, not silently create one")
	}
	if _, err := os.Stat(nonexistent); err == nil {
		t.Error("`module map` created a store file at a path that did not exist — store.Open's CREATE TABLE path was used, not MountReadOnly")
	}
	_ = stderr
}

// TestModuleInit_WarnsOnSelectorMatchingModuleCue proves finding 7's
// warning half: a selector broad enough to catch module.cue itself gets an
// explicit warning naming the moving-target risk.
func TestModuleInit_WarnsOnSelectorMatchingModuleCue(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "RULES.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, rc := runModule(
		t, "init",
		"--dir", dir, "--name", "broad-selector-module", "--description", "d",
		"--include", "*",
	)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr)
	}
	if !strings.Contains(stderr, "moving target") {
		t.Errorf("stderr = %q, want a moving-target warning for a selector matching module.cue itself", stderr)
	}
}

// ─── shared test fixtures ───────────────────────────────────────────────

// testManifestCUE renders a minimal, valid module.cue body for tests —
// deliberately hand-written rather than via modspec.Template, so the two
// code paths (the scaffold Template and the test fixture) can never mask
// the same bug.
func testManifestCUE(name string, include []string, watermark string) string {
	incs := make([]string, len(include))
	for i, s := range include {
		incs[i] = `"` + s + `"`
	}
	return "name: \"" + name + "\"\n" +
		"version: \"v1\"\n" +
		"description: \"A test module.\"\n" +
		"corpus: include: [" + strings.Join(incs, ", ") + "]\n" +
		"watermark: \"" + watermark + "\"\n" +
		"eval_evidence: evaled: false\n" +
		"recompile: allowed: false\n" +
		"provenance: {\n\tcompiled_by: \"deterministic\"\n\tcompiled_at: \"2026-07-28T00:00:00Z\"\n}\n"
}

// testWatermark computes the real, CORPUS-SCOPED watermark for dir (via
// moduleWatermark — the same function every `module` verb uses, not the
// raw whole-directory connector.Watermark), so fixtures start with a
// watermark that genuinely matches what `module map`/`doctor` will later
// recompute (tests that want DRIFT construct a bogus one explicitly
// instead). include must match the manifest's own corpus.include — a
// mismatch here would silently produce a watermark scoped to the WRONG
// selector set.
func testWatermark(t *testing.T, dir string, include ...string) string {
	t.Helper()
	wm, err := moduleWatermark(context.Background(), dir, modspec.Manifest{
		Name:   "test",
		Corpus: modspec.Corpus{Include: include},
	})
	if err != nil {
		t.Fatalf("compute test watermark: %v", err)
	}
	return wm
}
