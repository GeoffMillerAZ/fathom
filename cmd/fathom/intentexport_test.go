package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/intent"
)

// handGuardCue is a guard.cue an adopter wrote by hand: fathom must leave
// every byte of it alone when it writes its own entries (P15, law 15).
const handGuardCue = `// hand-written header comment
guards: [
	{
		// a human's own guard
		scopes: ["cmd/**"]
		note:   "keep cmd thin"
	},
]
`

func runIntentExport(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code := cmdIntent(append([]string{"export"}, args...), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

// IV-3a (ADR-074 decision 6): the router's sensitive-surface table is the
// one map. `--write` generates guard.cue's intent entries from it inside
// fathom's own markers, leaving the hand-written entries byte-identical;
// a second write is a no-op; `--check` exits 1 naming the first entry that
// drifted from the router, and runs in `task validate`.
func TestIntentExportGuard_CheckFailsOnDriftWriteIsIdempotent(t *testing.T) {
	root := t.TempDir()
	writeCorpusFixture(t, root)
	writeGuardCue(t, root, handGuardCue)
	guardPath := filepath.Join(root, ".fathom", "guard.cue")

	// No generated region yet: the map and the file disagree.
	if code, _, stderr := runIntentExport(t, "--root", root, "--guard", "--check"); code != 1 {
		t.Fatalf("--check before any write: exit %d, want 1 (stderr %q)", code, stderr)
	}

	code, stdout, stderr := runIntentExport(t, "--root", root, "--guard", "--write")
	if code != 0 {
		t.Fatalf("--write: exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "1 intent entr") {
		t.Errorf("--write must leave a receipt naming what it wrote, got %q", stdout)
	}
	first, err := os.ReadFile(guardPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(first)

	// Hand-written entries untouched: the hand file's lines all survive, in order.
	handHead := strings.TrimSuffix(handGuardCue, "]\n")
	if !strings.HasPrefix(got, handHead) {
		t.Fatalf("hand-written entries must be untouched; file now:\n%s", got)
	}
	if !strings.Contains(got, intentGuardBegin) || !strings.Contains(got, intentGuardEnd) {
		t.Fatalf("generated entries must sit inside fathom's own markers:\n%s", got)
	}

	// Provenance: each generated note names its glob and the row's stamp.
	c, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	stamp := intent.SurfaceStamp(c.Surfaces[0])
	if !strings.Contains(got, `"intent: internal/store/** `) || !strings.Contains(got, "@ "+stamp) {
		t.Fatalf("generated note must carry `intent: <glob> … @ <stamp>` provenance:\n%s", got)
	}
	// The ids a row names, each with its rung; never an id the row does not name.
	if !strings.Contains(got, "I1 [test]") || strings.Contains(got, "I2 [") {
		t.Fatalf("generated note must carry exactly the row's invariant ids with rungs:\n%s", got)
	}

	// The written file loads as guard.cue (hand entry + generated entry).
	gs, err := guardspec.ParseGuards(guardPath, first)
	if err != nil {
		t.Fatalf("written guard.cue does not load: %v\n%s", err, got)
	}
	if len(gs.Guards) != 2 {
		t.Fatalf("want 2 guards (1 hand + 1 generated), got %d", len(gs.Guards))
	}

	// Idempotent: a second write changes nothing.
	if code, _, stderr := runIntentExport(t, "--root", root, "--guard", "--write"); code != 0 {
		t.Fatalf("second --write: exit %d: %s", code, stderr)
	}
	second, _ := os.ReadFile(guardPath)
	if !bytes.Equal(first, second) {
		t.Fatalf("--write is not idempotent:\n--- first\n%s\n--- second\n%s", first, second)
	}
	if code, _, stderr := runIntentExport(t, "--root", root, "--guard", "--check"); code != 0 {
		t.Fatalf("--check after --write: exit %d: %s", code, stderr)
	}

	// The router row changes: --check names the drifted entry by its globs.
	routerPath := filepath.Join(root, "docs", "intent", "README.md")
	router, _ := os.ReadFile(routerPath)
	moved := strings.Replace(string(router), "store purity (I1)", "store purity (I1, I2)", 1)
	if err := os.WriteFile(routerPath, []byte(moved), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = runIntentExport(t, "--root", root, "--guard", "--check")
	if code != 1 || !strings.Contains(stderr, "internal/store/**") {
		t.Fatalf("--check after a router change: exit %d, stderr %q; want 1 naming internal/store/**", code, stderr)
	}
	// --check never writes.
	if now, _ := os.ReadFile(guardPath); !bytes.Equal(now, second) {
		t.Fatal("--check must never write guard.cue")
	}

	// One map, not two: the check is a step of the gate.
	taskfile, err := os.ReadFile(filepath.Join(repoRootForDocs(t), "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(taskfile), "intent export --guard --check") {
		t.Fatal("Taskfile.yml must run `fathom intent export --guard --check` in task validate")
	}
	if !strings.Contains(validateDepsLine(string(taskfile)), "intent-guard-check") {
		t.Fatal("validate:unleased must depend on intent-guard-check")
	}
}

// validateDepsLine returns validate:unleased's deps line.
func validateDepsLine(taskfile string) string {
	i := strings.Index(taskfile, "validate:unleased:")
	if i < 0 {
		return ""
	}
	rest := taskfile[i:]
	j := strings.Index(rest, "deps: [")
	if j < 0 {
		return ""
	}
	end := strings.IndexByte(rest[j:], '\n')
	if end < 0 {
		return rest[j:]
	}
	return rest[j : j+end]
}

// P15: fathom writes only inside its own markers and never resolves a
// conflict. A human edit inside the generated region stops the write and
// is reported; an entry pasted from the old printed proposal (it carries
// fathom's stamp outside the markers) stops it too, instead of being
// silently duplicated or deleted.
func TestIntentExportGuard_WriteStopsOnHandEditedRegionOrPastedEntries(t *testing.T) {
	root := t.TempDir()
	writeCorpusFixture(t, root)
	writeGuardCue(t, root, handGuardCue)
	guardPath := filepath.Join(root, ".fathom", "guard.cue")
	if code, _, stderr := runIntentExport(t, "--root", root, "--guard", "--write"); code != 0 {
		t.Fatalf("--write: exit %d: %s", code, stderr)
	}
	b, _ := os.ReadFile(guardPath)
	edited := strings.Replace(string(b), "max_rules: 2", "max_rules: 1", 1)
	if edited == string(b) {
		t.Fatalf("fixture: generated entry carries no max_rules: 2:\n%s", b)
	}
	if err := os.WriteFile(guardPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runIntentExport(t, "--root", root, "--guard", "--write")
	if code != 1 || !strings.Contains(stderr, "by hand") {
		t.Fatalf("hand-edited region: exit %d, stderr %q; want 1 naming the hand edit", code, stderr)
	}
	if now, _ := os.ReadFile(guardPath); string(now) != edited {
		t.Fatal("a refused write must leave the file as it was")
	}

	// The old printed proposal, pasted by hand outside any markers.
	root2 := t.TempDir()
	writeCorpusFixture(t, root2)
	_, proposal, _ := runIntentExport(t, "--root", root2, "--guard")
	pasted := proposal[strings.Index(proposal, "guards: ["):]
	writeGuardCue(t, root2, pasted)
	code, _, stderr = runIntentExport(t, "--root", root2, "--guard", "--write")
	if code != 1 || !strings.Contains(stderr, "outside") {
		t.Fatalf("pasted stamped entries: exit %d, stderr %q; want 1 naming entries outside the markers", code, stderr)
	}
}
