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
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
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

func TestModuleDoctor_LadderProgression(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "RULES.md"), []byte("## R\n\nRule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wm := testWatermark(t, root, "RULES.md")
	manifestPath := filepath.Join(root, "module.cue")

	// Stage 1: draft only (no eval evidence, no store, default license).
	if err := os.WriteFile(manifestPath, []byte(testManifestCUE("doctor-module", []string{"RULES.md"}, wm)), 0o644); err != nil {
		t.Fatal(err)
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

	// Stage 2: eval-passed (evidence file exists on disk, evaled:true).
	if err := os.WriteFile(filepath.Join(root, "RESULTS.md"), []byte("precision 1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	evalManifest := strings.Replace(testManifestCUE("doctor-module", []string{"RULES.md"}, wm), "eval_evidence: evaled: false", "eval_evidence: {\n\tevaled: true\n\tpath: \"RESULTS.md\"\n}", 1)
	if err := os.WriteFile(manifestPath, []byte(evalManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, rc = runModule(t, "doctor", "--manifest", manifestPath, "--json")
	if rc != 0 {
		t.Fatalf("doctor rc=%d stdout=%s", rc, stdout)
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.Verdict != LevelEvalPassed {
		t.Errorf("verdict = %q, want %q; criteria=%+v", report.Verdict, LevelEvalPassed, report.Criteria)
	}

	// Stage 3: servable (a conforming, fresh-watermark store; empty claims
	// for this layer is fine — computeMap reports "layer not compiled yet"
	// as a store-note, which zeroBrokenAnchors treats as NOT servable, so
	// we must actually have zero broken anchors AND no store-note — i.e.
	// verify the "no claims at all" case does NOT falsely claim servable).
	storePath := filepath.Join(t.TempDir(), "module.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	stdout, _, rc = runModule(t, "doctor", "--manifest", manifestPath, "--store", storePath, "--json")
	if rc != 0 {
		t.Fatalf("doctor rc=%d stdout=%s", rc, stdout)
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.Verdict != LevelEvalPassed {
		t.Errorf("verdict = %q, want %q (an empty store has no compiled claims yet, so servable's anchor criterion must NOT pass); criteria=%+v", report.Verdict, LevelEvalPassed, report.Criteria)
	}
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
