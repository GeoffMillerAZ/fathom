package semantic

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeFixturePack writes a minimal, valid pack directory at dir: layer.cue
// (name=name, a corpus, a prompt_file pointing at dir's own prompt.md),
// prompt.md, and (unless omitEval) a non-empty eval/ fixture.
func writeFixturePack(t *testing.T, dir, name string, omitEval bool) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
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
	if err := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte("You are a fixture layer. Emit {\"claims\": []}."), 0o644); err != nil {
		t.Fatalf("write prompt.md: %v", err)
	}
	if !omitEval {
		if err := os.MkdirAll(filepath.Join(dir, "eval"), 0o755); err != nil {
			t.Fatalf("MkdirAll eval: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "eval", "labels.json"), []byte(`{"probes":[],"rules":[]}`), 0o644); err != nil {
			t.Fatalf("write eval fixture: %v", err)
		}
	}
}

// ─── LoadLayerIn / ListLayersIn: the union ──────────────────────────────────

func TestLoadLayerIn_EmptyRootIsEmbeddedOnly(t *testing.T) {
	def, err := LoadLayerIn("", LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayerIn(\"\", obligations): %v", err)
	}
	want, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer(obligations): %v", err)
	}
	if !reflect.DeepEqual(def, want) {
		t.Errorf("LoadLayerIn(\"\", ...) = %+v, want exactly LoadLayer's result %+v", def, want)
	}
}

// numEmbeddedLayers returns how many layers ship embedded — tests assert
// against this instead of a hardcoded count, so a future pack landing
// (LC-9's docs aside, a new built-in) doesn't spuriously break the union
// tests below.
func numEmbeddedLayers(t *testing.T) int {
	t.Helper()
	names, err := LayerNames()
	if err != nil {
		t.Fatalf("LayerNames: %v", err)
	}
	return len(names)
}

func TestLoadLayerIn_NoProjectDirIsEmbeddedOnly(t *testing.T) {
	root := t.TempDir() // no .fathom/layers/ at all
	names, err := ListLayersIn(root)
	if err != nil {
		t.Fatalf("ListLayersIn: %v", err)
	}
	want := numEmbeddedLayers(t)
	if len(names) != want {
		t.Fatalf("ListLayersIn(no .fathom/layers) = %d entries, want %d (embedded only): %+v", len(names), want, names)
	}
	for _, n := range names {
		if n.Source != SourceEmbedded {
			t.Errorf("layer %q source = %q, want embedded", n.Name, n.Source)
		}
	}
}

func TestLoadLayerIn_ProjectPackAddsNewLayer(t *testing.T) {
	root := t.TempDir()
	packDir := filepath.Join(root, ProjectLayersDir, "custom-pack")
	writeFixturePack(t, packDir, "custom-pack", false)

	def, err := LoadLayerIn(root, "custom-pack")
	if err != nil {
		t.Fatalf("LoadLayerIn(root, custom-pack): %v", err)
	}
	if def.Name != "custom-pack" || def.Kind != "custom-kind" {
		t.Errorf("def = %+v", def)
	}
	if def.PromptText == "" {
		t.Errorf("PromptText not resolved for project pack")
	}

	// promptForLayer must resolve it too (the actual compile-time use).
	text, version, perr := promptForLayer(def)
	if perr != nil {
		t.Fatalf("promptForLayer(project pack): %v", perr)
	}
	if !strings.Contains(text, "fixture layer") || !strings.HasPrefix(version, "custom-pack/") {
		t.Errorf("promptForLayer = %q, %q", text, version)
	}

	names, err := ListLayersIn(root)
	if err != nil {
		t.Fatalf("ListLayersIn: %v", err)
	}
	want := numEmbeddedLayers(t) + 1
	if len(names) != want {
		t.Fatalf("ListLayersIn = %d entries, want %d (embedded + 1 project): %+v", len(names), want, names)
	}
}

func TestLoadLayerIn_ProjectPackOverridesEmbedded(t *testing.T) {
	root := t.TempDir()
	packDir := filepath.Join(root, ProjectLayersDir, LayerObligations)
	writeFixturePack(t, packDir, LayerObligations, false)

	def, err := LoadLayerIn(root, LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayerIn: %v", err)
	}
	if def.Kind != "custom-kind" {
		t.Errorf("Kind = %q, want the PROJECT pack's override (custom-kind), not the embedded obligation kind", def.Kind)
	}

	infos, err := ListLayersIn(root)
	if err != nil {
		t.Fatalf("ListLayersIn: %v", err)
	}
	found := false
	for _, info := range infos {
		if info.Name == LayerObligations {
			found = true
			if info.Source != SourceProject || !info.Overrides {
				t.Errorf("obligations entry = %+v, want Source=project Overrides=true", info)
			}
		}
	}
	if !found {
		t.Fatalf("obligations missing from ListLayersIn: %+v", infos)
	}
	// Still exactly the embedded count total — the project pack REPLACES,
	// not adds beside, the embedded one of the same name.
	want := numEmbeddedLayers(t)
	if len(infos) != want {
		t.Errorf("ListLayersIn = %d entries, want %d (override, not addition): %+v", len(infos), want, infos)
	}
}

// writeBrokenPack writes a pack directory whose layer.cue cannot decode —
// the "half-authored pack" case the union loader must survive (critic
// finding 1: one broken project pack must never take down drift tracking,
// or anything else, for every OTHER layer).
func writeBrokenPack(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "layer.cue"), []byte("this is { not CUE ]"), 0o644); err != nil {
		t.Fatalf("write broken layer.cue: %v", err)
	}
}

// TestLoadLayerIn_BrokenProjectPackDoesNotBreakUnion: a project pack with an
// undecodable layer.cue must not fail the whole union — every embedded
// layer still resolves; only asking for the broken pack BY NAME errors; and
// ListLayersIn surfaces the breakage via LoadError instead of hiding it.
func TestLoadLayerIn_BrokenProjectPackDoesNotBreakUnion(t *testing.T) {
	root := t.TempDir()
	writeBrokenPack(t, filepath.Join(root, ProjectLayersDir, "half-authored"))

	// Embedded layers resolve exactly as if the broken pack were absent.
	def, err := LoadLayerIn(root, LayerFindings)
	if err != nil {
		t.Fatalf("LoadLayerIn(findings) with a broken sibling pack: %v", err)
	}
	if !def.Serving.DriftTracked {
		t.Errorf("findings lost drift_tracked because of an unrelated broken pack")
	}

	// The broken pack itself errors loudly by name.
	if _, err := LoadLayerIn(root, "half-authored"); err == nil {
		t.Fatal("LoadLayerIn(half-authored): want a loud error for the broken pack itself")
	}

	// ListLayersIn shows it as broken, never silently drops it.
	infos, err := ListLayersIn(root)
	if err != nil {
		t.Fatalf("ListLayersIn: %v", err)
	}
	found := false
	for _, info := range infos {
		if info.Name == "half-authored" {
			found = true
			if info.LoadError == "" {
				t.Errorf("broken pack listed without LoadError: %+v", info)
			}
		}
	}
	if !found {
		t.Errorf("broken pack missing from ListLayersIn (silently dropped): %+v", infos)
	}
}

// TestLoadLayerIn_BrokenPackShadowingEmbeddedFallsBack: a broken project
// pack whose name matches an embedded pack must NOT take the built-in down
// with it — the embedded definition stays live, with the breakage recorded.
func TestLoadLayerIn_BrokenPackShadowingEmbeddedFallsBack(t *testing.T) {
	root := t.TempDir()
	writeBrokenPack(t, filepath.Join(root, ProjectLayersDir, LayerFindings))

	def, err := LoadLayerIn(root, LayerFindings)
	if err != nil {
		t.Fatalf("LoadLayerIn(findings) with a broken project override: %v", err)
	}
	if def.Kind != "finding" || !def.Serving.DriftTracked {
		t.Errorf("embedded findings definition not preserved: %+v", def)
	}

	infos, err := ListLayersIn(root)
	if err != nil {
		t.Fatalf("ListLayersIn: %v", err)
	}
	for _, info := range infos {
		if info.Name == LayerFindings {
			if info.Source != SourceEmbedded {
				t.Errorf("Source = %q, want embedded (the broken override is ignored)", info.Source)
			}
			if info.LoadError == "" {
				t.Errorf("breakage not recorded on the surviving embedded entry: %+v", info)
			}
		}
	}
}

// TestLoadLayerIn_ProjectPackEvaledLieIsOverridden replicates the D3-1
// bounce finding-2 executed attack exactly: a project pack whose layer.cue
// declares `evaled: true` but has NO eval/ directory at all walked straight
// through the compile eval-gate (exit 0, empty escape_note). The
// self-declared field is a necessity only for embedded packs (their eval/
// is not compiled into the binary); a project pack's eval/ is on disk, so a
// declaration with no readable eval/RESULTS.md behind it must load as
// Evaled=false.
func TestLoadLayerIn_ProjectPackEvaledLieIsOverridden(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ProjectLayersDir, "bogus")
	writeFixturePack(t, dir, "bogus", true /* omitEval */)
	// Append the lie.
	f, err := os.OpenFile(filepath.Join(dir, "layer.cue"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open layer.cue: %v", err)
	}
	if _, err := f.WriteString("evaled: true\n"); err != nil {
		t.Fatalf("append evaled: %v", err)
	}
	f.Close()

	def, err := LoadLayerIn(root, "bogus")
	if err != nil {
		t.Fatalf("LoadLayerIn(bogus): %v", err)
	}
	if def.Evaled {
		t.Fatal("Evaled = true for a project pack with no eval/RESULTS.md — the compile eval-gate would pass a one-line lie")
	}
}

// TestLoadLayerIn_ProjectPackEvaledWithResultsSurvives is the positive
// control: a project pack that declares evaled: true AND ships an
// eval/RESULTS.md keeps its declaration.
func TestLoadLayerIn_ProjectPackEvaledWithResultsSurvives(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ProjectLayersDir, "gated")
	writeFixturePack(t, dir, "gated", false /* omitEval */)
	if err := os.WriteFile(filepath.Join(dir, "eval", "RESULTS.md"), []byte("# gated — eval results\n\nPASS.\n"), 0o644); err != nil {
		t.Fatalf("write RESULTS.md: %v", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "layer.cue"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open layer.cue: %v", err)
	}
	if _, err := f.WriteString("evaled: true\n"); err != nil {
		t.Fatalf("append evaled: %v", err)
	}
	f.Close()

	def, err := LoadLayerIn(root, "gated")
	if err != nil {
		t.Fatalf("LoadLayerIn(gated): %v", err)
	}
	if !def.Evaled {
		t.Fatal("Evaled = false for a project pack WITH eval/RESULTS.md — the honesty override is over-firing")
	}
}

func TestLoadLayerIn_UnknownLayerListsKnownNames(t *testing.T) {
	root := t.TempDir()
	writeFixturePack(t, filepath.Join(root, ProjectLayersDir, "custom-pack"), "custom-pack", false)

	_, err := LoadLayerIn(root, "nope")
	if err == nil {
		t.Fatal("LoadLayerIn(nope): want error")
	}
	for _, want := range []string{"obligations", "findings", "custom-pack"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %v missing known layer %q", err, want)
		}
	}
}

// ─── ValidatePack / InstallPack ─────────────────────────────────────────────

func TestValidatePack_Clean(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "clean-pack")
	writeFixturePack(t, packDir, "clean-pack", false)

	def, warnings, err := ValidatePack(packDir)
	if err != nil {
		t.Fatalf("ValidatePack: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none for a clean pack", warnings)
	}
	if def.Name != "clean-pack" {
		t.Errorf("def.Name = %q", def.Name)
	}
}

func TestValidatePack_WarnsOnMissingEval(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "no-eval-pack")
	writeFixturePack(t, packDir, "no-eval-pack", true)

	_, warnings, err := ValidatePack(packDir)
	if err != nil {
		t.Fatalf("ValidatePack: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "eval/") {
		t.Errorf("warnings = %v, want exactly one eval/ warning", warnings)
	}
}

func TestValidatePack_WarnsOnMissingPrompt(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "no-prompt-pack")
	writeFixturePack(t, packDir, "no-prompt-pack", false)
	if err := os.Remove(filepath.Join(packDir, "prompt.md")); err != nil {
		t.Fatalf("remove prompt.md: %v", err)
	}

	_, warnings, err := ValidatePack(packDir)
	if err != nil {
		t.Fatalf("ValidatePack: %v", err)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "prompt.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("warnings = %v, want a prompt.md warning", warnings)
	}
}

func TestValidatePack_RejectsNameMismatch(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "outer-dir-name")
	writeFixturePack(t, packDir, "inner-declared-name", false)

	if _, _, err := ValidatePack(packDir); err == nil {
		t.Fatal("ValidatePack: want error when directory name and layer.cue's name field disagree")
	}
}

func TestValidatePack_RejectsSchemaViolation(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "bad-tier-pack")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := `name: "bad-tier-pack"
version: "v1"
kind: "x"
corpus: include: ["docs/**"]
prompt_version: "x/v1"
prompt_file: "bad-tier-pack/prompt.md"
tier: "not-a-real-tier"
budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
`
	if err := os.WriteFile(filepath.Join(packDir, "layer.cue"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ValidatePack(packDir); err == nil {
		t.Fatal("ValidatePack: want error for a tier outside #Layer's enum")
	}
}

func TestInstallPack_CopiesAndBecomesLoadable(t *testing.T) {
	srcDir := filepath.Join(t.TempDir(), "adr-rules")
	writeFixturePack(t, srcDir, "adr-rules", false)

	root := t.TempDir()
	def, warnings, err := InstallPack(srcDir, root, false)
	if err != nil {
		t.Fatalf("InstallPack: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if def.Name != "adr-rules" {
		t.Errorf("def.Name = %q", def.Name)
	}

	// The pack now resolves via LoadLayerIn as a PROJECT layer.
	got, err := LoadLayerIn(root, "adr-rules")
	if err != nil {
		t.Fatalf("LoadLayerIn after install: %v", err)
	}
	if got.Kind != "custom-kind" {
		t.Errorf("installed layer Kind = %q", got.Kind)
	}

	// Files landed at <root>/.fathom/layers/adr-rules/.
	destLayerCUE := filepath.Join(root, ProjectLayersDir, "adr-rules", "layer.cue")
	if _, err := os.Stat(destLayerCUE); err != nil {
		t.Errorf("installed layer.cue not found at %s: %v", destLayerCUE, err)
	}
	destEval := filepath.Join(root, ProjectLayersDir, "adr-rules", "eval", "labels.json")
	if _, err := os.Stat(destEval); err != nil {
		t.Errorf("installed eval/ fixture not found at %s: %v", destEval, err)
	}
}

func TestInstallPack_RejectsInvalidSource(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "outer")
	writeFixturePack(t, packDir, "different-inner-name", false)

	root := t.TempDir()
	if _, _, err := InstallPack(packDir, root, false); err == nil {
		t.Fatal("InstallPack: want error for an invalid source pack, and nothing should be copied")
	}
	if _, err := os.Stat(filepath.Join(root, ProjectLayersDir)); !os.IsNotExist(err) {
		t.Errorf(".fathom/layers/ should not exist after a rejected install: stat err = %v", err)
	}
}

// TestInstallPack_RefusesOverwriteWithoutForce_ForceReplacesWholly is critic
// finding 5's contract: a reinstall must never silently overwrite, and a
// forced reinstall REPLACES the whole directory (never merges — files from
// the previous version do not survive).
func TestInstallPack_RefusesOverwriteWithoutForce_ForceReplacesWholly(t *testing.T) {
	srcDir := filepath.Join(t.TempDir(), "re-pack")
	writeFixturePack(t, srcDir, "re-pack", false)
	root := t.TempDir()

	if _, _, err := InstallPack(srcDir, root, false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	// A stray file from "version 1" that the new source does NOT carry.
	stale := filepath.Join(root, ProjectLayersDir, "re-pack", "old-notes.md")
	if err := os.WriteFile(stale, []byte("v1 leftovers"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	// Second install without force: refused, and the installed pack (incl.
	// the stale file) is untouched.
	if _, _, err := InstallPack(srcDir, root, false); err == nil {
		t.Fatal("reinstall without force: want an already-installed refusal")
	} else if !strings.Contains(err.Error(), "already installed") {
		t.Errorf("refusal error = %v, want it to say already installed", err)
	}
	if _, err := os.Stat(stale); err != nil {
		t.Errorf("refused reinstall must not touch the installed pack: %v", err)
	}

	// Forced reinstall: succeeds, and the stale file is GONE (replace, not
	// merge).
	if _, _, err := InstallPack(srcDir, root, true); err != nil {
		t.Fatalf("forced reinstall: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("forced reinstall merged instead of replacing: stale file survives (stat err = %v)", err)
	}
	if _, err := os.Stat(filepath.Join(root, ProjectLayersDir, "re-pack", "layer.cue")); err != nil {
		t.Errorf("forced reinstall lost layer.cue: %v", err)
	}
}

// TestInstallPack_MidCopyFailureLeavesNoDebris: a copy failure mid-install
// must leave NEITHER a half-written destination (a layer.cue-less dir would
// register as a broken pack and pollute every list/compile) NOR a staging
// dir behind.
func TestInstallPack_MidCopyFailureLeavesNoDebris(t *testing.T) {
	srcDir := filepath.Join(t.TempDir(), "fail-pack")
	writeFixturePack(t, srcDir, "fail-pack", false)
	// An unreadable file inside the pack makes copyPackDir fail after
	// layer.cue/prompt.md were already staged.
	blocked := filepath.Join(srcDir, "eval", "blocked.json")
	if err := os.WriteFile(blocked, []byte("x"), 0o000); err != nil {
		t.Fatalf("write blocked file: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o644) })

	root := t.TempDir()
	if _, _, err := InstallPack(srcDir, root, false); err == nil {
		t.Fatal("InstallPack: want a copy error from the unreadable file")
	}

	// No destination dir.
	destDir := filepath.Join(root, ProjectLayersDir, "fail-pack")
	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		t.Errorf("half-written destination left behind at %s (stat err = %v)", destDir, err)
	}
	// No staging debris (and by extension nothing the union loader could
	// mistake for a pack).
	layersDir := filepath.Join(root, ProjectLayersDir)
	if entries, err := os.ReadDir(layersDir); err == nil {
		for _, e := range entries {
			t.Errorf("debris left in %s after failed install: %s", layersDir, e.Name())
		}
	}
	infos, err := ListLayersIn(root)
	if err != nil {
		t.Fatalf("ListLayersIn after failed install: %v", err)
	}
	if len(infos) != numEmbeddedLayers(t) {
		t.Errorf("failed install changed the union: %+v", infos)
	}
}
