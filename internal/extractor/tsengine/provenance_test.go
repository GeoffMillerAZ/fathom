package tsengine

import (
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
)

var updateProvenance = flag.Bool("update-provenance", false, "rewrite grammars/PROVENANCE.md's generated block from grammars.lock.json")

// TestProvenanceRenderedFromLock is LB-F28 (design §8.1): PROVENANCE.md's
// source and digest tables are a rendering of the lock, never hand-kept.
// Regenerate with -update-provenance after a lock change.
func TestProvenanceRenderedFromLock(t *testing.T) {
	lock, err := LoadGrammarLock()
	if err != nil {
		t.Fatal(err)
	}
	const path = "grammars/PROVENANCE.md"
	doc, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReplaceProvenanceBlock(string(doc), RenderProvenanceTables(lock))
	if err != nil {
		t.Fatal(err)
	}
	if got == string(doc) {
		return
	}
	if *updateProvenance {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("%s's generated block is stale; run: go test ./internal/extractor/tsengine -run TestProvenanceRenderedFromLock -update-provenance", path)
}

func TestRenderProvenanceTables(t *testing.T) {
	lock, _ := fixtureLock(t)
	out := RenderProvenanceTables(lock)
	for _, want := range []string{
		"| core | tree-sitter/tree-sitter | v0.26.11 | `" + strings.Repeat("a", 40) + "` | MIT |",
		"| `demo` | example/tree-sitter-demo | v1.0.0 | `" + strings.Repeat("c", 40) + "` | MIT | `tree_sitter_demo` |",
		"zig 0.16.0, wasm-opt 129",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendering lacks %q:\n%s", want, out)
		}
	}
	if RenderProvenanceTables(lock) != out {
		t.Fatal("rendering is not deterministic")
	}
	if _, err := ReplaceProvenanceBlock("no markers here", out); err == nil {
		t.Fatal("a document without the generated-block markers must be an error")
	}
}

// TestBuildScript (LB-F28): build.sh parses and is driven by the lock.
func TestBuildScript(t *testing.T) {
	b, err := os.ReadFile("grammars/build.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"grammars.lock.json", "tarball_sha256", "get-tar-commit-id", "check_licence", "wasm-opt -Oz", "gzip -9 -n"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("build.sh lacks %q", want)
		}
	}
	if out, err := exec.Command("bash", "-n", "grammars/build.sh").CombinedOutput(); err != nil {
		t.Fatalf("bash -n build.sh: %v\n%s", err, out)
	}
}

// TestProvenanceTablesCarryNoTotalRow: the rendering has no aggregate row, so
// two branches that each add a grammar cannot conflict on a shared sum.
func TestProvenanceTablesCarryNoTotalRow(t *testing.T) {
	lock, _ := fixtureLock(t)
	if out := RenderProvenanceTables(lock); strings.Contains(out, "**total**") {
		t.Fatalf("rendering carries a total row:\n%s", out)
	}
}
