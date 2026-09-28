package main

// trace_test.go — CX-16 / CX-17 (design change-and-compliance.md §3): the
// evidence pack is written only on request, carries a sha256 manifest over
// every file, and re-verifies byte-identically at its commit; a tampered
// pack is named by file and cell.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func newTraceFixtureRepo(t *testing.T) (root, storePath string) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "tracesvc")
	write := func(rel, content string) {
		t.Helper()
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module tracesvc\n\ngo 1.21\n")
	write("thing/thing.go", "package thing\n\n// DoThing does the thing.\nfunc DoThing() int { return 1 }\n")
	write("thing/thing_test.go", "package thing\n\nimport \"testing\"\n\nfunc TestDoThing(t *testing.T) {\n\tif DoThing() != 1 {\n\t\tt.Fatal(\"no\")\n\t}\n}\n")
	write("ADR.md", "# ADRs\n\n## ADR-001 — Things are done\n\nYes.\n")
	write("docs/plans/things.md", "# Things\n\n- [x] Do the thing (ADR-001) {id: TH-1, symbol: DoThing, package: thing, reachable_from: TestDoThing, accept: \"the thing is done\"}\n- [ ] Not yet {id: TH-2, symbol: Missing, package: thing}\n")
	write(".fathom/trace/example.cue", "standard: \"example\"\nclauses: [\n\t{id: \"C-1\", requirements: [\"TH-1\", \"TH-1/accept\"]},\n\t{id: \"C-2\", patterns: [\"^TH-2$\"]},\n]\n")
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@example.invalid", "-c", "user.name=t", "add", "-A"},
		{"-c", "user.email=t@example.invalid", "-c", "user.name=t", "commit", "-q", "-m", "fixture"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	storePath = filepath.Join(t.TempDir(), "trace.sqlite")
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", "tracesvc", "--path", root, "--store", storePath}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	return root, storePath
}

func TestCmdTraceExportManifest(t *testing.T) {
	root, storePath := newTraceFixtureRepo(t)
	out := filepath.Join(t.TempDir(), "pack")
	before := hashTreeForTest(t, root)

	code, stdout, stderr := runInDir(t, root, []string{"trace", "export", "--standard", "example", "--out", out, "--service", "tracesvc", "--store", storePath})
	if code != 0 {
		t.Fatalf("trace export exit %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if hashTreeForTest(t, root) != before {
		t.Fatal("trace export wrote into the repository; the pack goes only to --out")
	}

	manifest, err := os.ReadFile(filepath.Join(out, "MANIFEST.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(manifest)), "\n")
	want := []string{"COMMIT", "digests.json", "matrix.json", "matrix.md", "readings.json"}
	if len(lines) != len(want) {
		t.Fatalf("manifest has %d lines, want %d:\n%s", len(lines), len(want), manifest)
	}
	for i, line := range lines {
		sum, name, ok := strings.Cut(line, "  ")
		if !ok || name != want[i] {
			t.Fatalf("manifest line %q, want file %s", line, want[i])
		}
		b, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		got := sha256.Sum256(b)
		if hex.EncodeToString(got[:]) != sum {
			t.Errorf("manifest sha256 for %s does not match the file", name)
		}
	}

	head, _ := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	commitFile, _ := os.ReadFile(filepath.Join(out, "COMMIT"))
	if strings.TrimSpace(string(commitFile)) != strings.TrimSpace(string(head)) {
		t.Errorf("COMMIT = %q, want HEAD %q", commitFile, head)
	}
	matrix, _ := os.ReadFile(filepath.Join(out, "matrix.json"))
	for _, s := range []string{`"id": "TH-1"`, `"id": "TH-1/accept"`, `"state": "proven"`, `"state": "reaches"`, `"reading": "ADR-001"`, `"id": "C-1"`} {
		if !strings.Contains(string(matrix), s) {
			t.Errorf("matrix.json lacks %s:\n%s", s, matrix)
		}
	}
	digests, _ := os.ReadFile(filepath.Join(out, "digests.json"))
	for _, p := range []string{".fathom/trace/example.cue", "docs/plans/things.md", "ADR.md"} {
		if !strings.Contains(string(digests), p) {
			t.Errorf("digests.json lacks %s", p)
		}
	}

	// A standard the repository never mapped is refused, never invented.
	code, _, stderr = runInDir(t, root, []string{"trace", "export", "--standard", "nope", "--out", filepath.Join(t.TempDir(), "x"), "--service", "tracesvc", "--store", storePath})
	if code == 0 || !strings.Contains(stderr, ".fathom/trace/nope.cue") {
		t.Errorf("unmapped standard: exit %d, stderr %s", code, stderr)
	}
}

func TestCmdTraceVerifyByteIdentical(t *testing.T) {
	root, storePath := newTraceFixtureRepo(t)
	out := filepath.Join(t.TempDir(), "pack")
	if code, so, se := runInDir(t, root, []string{"trace", "export", "--standard", "example", "--out", out, "--service", "tracesvc", "--store", storePath}); code != 0 {
		t.Fatalf("export: %d %s %s", code, so, se)
	}
	code, stdout, stderr := runInDir(t, root, []string{"trace", "verify", out, "--store", storePath})
	if code != 0 || !strings.Contains(stdout, "byte-identical") {
		t.Fatalf("verify of an untouched pack: exit %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}

	// Recompute is at the PACK's commit: a later commit changing the plan
	// does not change the recomputation.
	if err := os.WriteFile(filepath.Join(root, "docs/plans/things.md"), []byte("# Things\n\n- [ ] gone {id: TH-9, symbol: X}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"-c", "user.email=t@example.invalid", "-c", "user.name=t", "commit", "-q", "-m", "later"}} {
		if o, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, o)
		}
	}
	if code, so, se := runInDir(t, root, []string{"trace", "verify", out, "--store", storePath}); code != 0 {
		t.Fatalf("verify after a later commit: exit %d\n%s\n%s", code, so, se)
	}

	// Tamper with one cell: the difference is named by file and cell.
	mp := filepath.Join(out, "matrix.json")
	b, _ := os.ReadFile(mp)
	tampered := strings.Replace(string(b), `"state": "reaches"`, `"state": "unreached"`, 1)
	if tampered == string(b) {
		t.Fatal("fixture: no reaches cell to tamper with")
	}
	if err := os.WriteFile(mp, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = runInDir(t, root, []string{"trace", "verify", out, "--store", storePath})
	if code != 1 {
		t.Fatalf("tampered pack: exit %d, want 1\n%s", code, stdout)
	}
	if !strings.Contains(stdout, "matrix.json: docs/plans/things.md#TH-1.test") {
		t.Errorf("difference not named by file and cell:\n%s", stdout)
	}
	if !strings.Contains(stdout, "MANIFEST.sha256") {
		t.Errorf("manifest mismatch not named:\n%s", stdout)
	}
}
