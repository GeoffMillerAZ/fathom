package main

// trace_test.go — CX-16 / CX-17 (design change-and-compliance.md §3): the
// evidence pack is written only on request, carries a sha256 manifest over
// every file, and re-verifies byte-identically at its commit; a tampered
// pack is named by file and cell.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/review"
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
		{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "user.email=t@example.invalid", "-c", "user.name=t", "commit", "-q", "-m", "fixture"},
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

// TOOL08 tests intentionally use only the existing CLI/review APIs. Before the
// implementation they must compile, then fail at runtime on the unknown flag.
// No proposed production type or helper is referenced here.
func traceProjectWrite(t *testing.T, root, name, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func traceProjectCommit(t *testing.T, root string) string {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "user.email=t@example.invalid", "-c", "user.name=t", "commit", "-q", "-m", "project decision sources"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	output, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(output))
}

func newTraceProjectFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root, database := newTraceFixtureRepo(t)
	traceProjectWrite(t, root, "INTENT.md", "# Intent\n\n## Invariants\n\n1. Preserve custody.\n2. Remain honest.\n\n| ID | Statement |\n| --- | --- |\n| I77 | Explicit identity. |\n")
	traceProjectWrite(t, root, "docs/adr/0001.md", "# ADR0001 First decision\n\nAccepted.\n")
	traceProjectWrite(t, root, "docs/adr/spaced.md", "# ADR 0002 Second decision\n\nAccepted.\n")
	traceProjectWrite(t, root, "docs/adr/dashed.md", "## ADR-003 Third decision\n\nAccepted.\n")
	traceProjectWrite(t, root, "docs/adr/not-markdown.txt", "# ADR-999 Not a Markdown input\n")
	traceProjectWrite(t, root, "docs/adr/examples.md", "# Examples\n\n```markdown\n# ADR9999 Not a declaration\n| I88 | Example only |\n```\n\nOrdinary ADR7777 prose is not a declaration.\n")
	traceProjectWrite(t, root, "docs/plans/project.md", "# Project\n\n"+
		"- [ ] Compact (ADR0001) {id: PR-1, symbol: DoThing, package: thing}\n"+
		"- [ ] Spaced (ADR 0002) {id: PR-2, symbol: DoThing, package: thing}\n"+
		"- [ ] Dashed (ADR-003) {id: PR-3, symbol: DoThing, package: thing}\n"+
		"- [ ] Declared invariant (I77) {id: PR-4, symbol: DoThing, package: thing}\n"+
		"- [ ] Numbered prose is no ID (I1) {id: PR-5, symbol: DoThing, package: thing}\n"+
		"- [ ] Different spelling (ADR-0001) {id: PR-6, symbol: DoThing, package: thing}\n"+
		"- [ ] Fenced heading (ADR9999) {id: PR-7, symbol: DoThing, package: thing}\n"+
		"- [ ] Fenced invariant (I88) {id: PR-8, symbol: DoThing, package: thing}\n")
	return root, database, traceProjectCommit(t, root)
}

func traceProjectExport(t *testing.T, root, database, commit string, selectors ...string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "pack")
	args := []string{"trace", "export", "--standard", "example", "--out", out, "--service", "tracesvc", "--store", database, "--commit", commit}
	for _, selector := range selectors {
		args = append(args, "--decision-source", selector)
	}
	if code, stdout, stderr := runInDir(t, root, args); code != 0 {
		t.Fatalf("explicit export exit %d\n%s\n%s", code, stdout, stderr)
	}
	return out
}

func traceProjectRead(t *testing.T, directory, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(directory, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func traceProjectMatrix(t *testing.T, directory string) map[string]review.TraceRow {
	t.Helper()
	var pack review.TracePack
	if err := json.Unmarshal(traceProjectRead(t, directory, "matrix.json"), &pack); err != nil {
		t.Fatal(err)
	}
	rows := map[string]review.TraceRow{}
	for _, row := range pack.Matrix.Rows {
		rows[row.Requirement.ID] = row
	}
	return rows
}

func TestCmdTraceExplicitDecisionSourcesUseExactIDsAndRealMarkdown(t *testing.T) {
	root, database, commit := newTraceProjectFixture(t)
	before := hashTreeForTest(t, root)
	out := traceProjectExport(t, root, database, commit, "INTENT.md", "docs/adr")
	if hashTreeForTest(t, root) != before {
		t.Fatal("explicit export mutated source repository")
	}
	rows := traceProjectMatrix(t, out)
	for id, exact := range map[string]string{"PR-1": "ADR0001", "PR-2": "ADR 0002", "PR-3": "ADR-003", "PR-4": "I77"} {
		cell := rows[id].Decision
		if cell.State != "cited" || cell.Reading != exact {
			t.Errorf("%s decision = %+v, want exact cited %q", id, cell, exact)
		}
	}
	for _, id := range []string{"PR-5", "PR-6", "PR-7", "PR-8"} {
		if rows[id].Decision.State != "unproven" {
			t.Errorf("%s fabricated declaration: %+v", id, rows[id].Decision)
		}
	}
	var readings struct {
		DecisionSources []string `json:"decision_sources"`
	}
	if err := json.Unmarshal(traceProjectRead(t, out, "readings.json"), &readings); err != nil {
		t.Fatal(err)
	}
	if strings.Join(readings.DecisionSources, ",") != "INTENT.md,docs/adr" {
		t.Fatalf("selectors not pinned canonically: %+v", readings)
	}
	digests := string(traceProjectRead(t, out, "digests.json"))
	for _, path := range []string{"INTENT.md", "docs/adr/0001.md", "docs/adr/spaced.md", "docs/adr/dashed.md", "docs/adr/examples.md"} {
		if !strings.Contains(digests, `"path": "`+path+`"`) {
			t.Errorf("committed source digest missing: %s", path)
		}
	}
	if strings.Contains(digests, "not-markdown.txt") || strings.Contains(digests, `"path": "ADR.md"`) {
		t.Errorf("explicit selection widened to default/non-Markdown inputs: %s", digests)
	}
}

func TestCmdTraceExplicitDecisionSourcesAreCommittedDeterministicAndVerifyPinnedSelectors(t *testing.T) {
	root, database, commit := newTraceProjectFixture(t)
	a := traceProjectExport(t, root, database, commit, "docs/adr/", "./INTENT.md", "docs/adr", "docs/adr/0001.md")
	b := traceProjectExport(t, root, database, commit, "docs/adr/0001.md", "INTENT.md", "docs/adr")
	for _, name := range append(append([]string{}, tracePackFiles...), review.ManifestName) {
		if !bytes.Equal(traceProjectRead(t, a, name), traceProjectRead(t, b, name)) {
			t.Errorf("selector order/duplicates changed %s", name)
		}
	}
	traceProjectWrite(t, root, "docs/adr/0001.md", "# ADR0009 Dirty replacement\n")
	traceProjectWrite(t, root, "docs/adr/new.md", "# ADR7777 Untracked\n")
	before := hashTreeForTest(t, root)
	c := traceProjectExport(t, root, database, commit, "INTENT.md", "docs/adr", "docs/adr/0001.md")
	if hashTreeForTest(t, root) != before {
		t.Fatal("dirty-tree export mutated source repository")
	}
	for _, name := range tracePackFiles {
		if !bytes.Equal(traceProjectRead(t, a, name), traceProjectRead(t, c, name)) {
			t.Errorf("working tree became authoritative for %s", name)
		}
	}
	traceProjectCommit(t, root)
	traceProjectWrite(t, root, "INTENT.md", "# Dirty intent\n")
	if code, stdout, stderr := runInDir(t, root, []string{"trace", "verify", a, "--store", database}); code != 0 {
		t.Fatalf("verify lost pinned commit/selectors: %d %s %s", code, stdout, stderr)
	}
}

func TestCmdTraceDefaultDecisionSourcesPreserveLegacyPackAndMissingDisclosure(t *testing.T) {
	root, database, commit := newTraceProjectFixture(t)
	legacy, err := buildTracePack(context.Background(), root, commit, "tracesvc", database, "example")
	if err != nil {
		t.Fatal(err)
	}
	out := traceProjectExport(t, root, database, commit)
	for name, data := range legacy {
		if !bytes.Equal(data, traceProjectRead(t, out, name)) {
			t.Errorf("default pack changed %s", name)
		}
	}
	readings := string(traceProjectRead(t, out, "readings.json"))
	if strings.Contains(readings, "decision_sources") || !strings.Contains(readings, "docs/intent/invariants.md") {
		t.Fatalf("default optional field/unread contract changed: %s", readings)
	}
	rows := traceProjectMatrix(t, out)
	if rows["PR-1"].Decision.State != "none" || rows["PR-2"].Decision.State != "none" || rows["TH-1"].Decision.Reading != "ADR-001" {
		t.Fatalf("default citation grammar changed: %+v", rows)
	}
}

func TestCmdTraceExplicitDecisionSourcesTamperingIsNamedEvenWithRehashedManifest(t *testing.T) {
	root, database, commit := newTraceProjectFixture(t)
	out := traceProjectExport(t, root, database, commit, "INTENT.md", "docs/adr")
	var readings map[string]any
	if err := json.Unmarshal(traceProjectRead(t, out, "readings.json"), &readings); err != nil {
		t.Fatal(err)
	}
	readings["decision_sources"] = []string{"INTENT.md"}
	traceProjectWrite(t, out, "readings.json", string(traceJSON(readings)))
	files := map[string][]byte{}
	for _, name := range tracePackFiles {
		files[name] = traceProjectRead(t, out, name)
	}
	traceProjectWrite(t, out, review.ManifestName, string(review.RenderManifest(files)))
	if code, stdout, stderr := runInDir(t, root, []string{"trace", "verify", out, "--store", database}); code != 1 || !strings.Contains(stdout, "matrix.json") {
		t.Fatalf("changed selectors not named by recomputed matrix: %d %s %s", code, stdout, stderr)
	}
}

func TestCmdTraceExplicitDecisionSourcesUseNulTreeListingForQuotedMarkdownNames(t *testing.T) {
	root, database, _ := newTraceProjectFixture(t)
	name := "docs/adr/quoted \"policy\".md"
	traceProjectWrite(t, root, name, "# ADR0042 Quoted path\n")
	commit := traceProjectCommit(t, root)
	out := traceProjectExport(t, root, database, commit, name)
	var digests []traceDigest
	if err := json.Unmarshal(traceProjectRead(t, out, "digests.json"), &digests); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, digest := range digests {
		if digest.Path == name {
			found = true
		}
	}
	if !found {
		t.Fatalf("quoted git path not read exactly: %+v", digests)
	}
}

func TestCmdTraceExplicitDecisionSourcesRefuseBadPathsAndMissingInputsBeforeWritingPack(t *testing.T) {
	root, database, commit := newTraceProjectFixture(t)
	for _, selector := range []string{"", "/tmp/outside.md", "../outside.md", "docs/../ADR.md", "docs\\adr", "docs/adr\n", "docs/adr\t", "missing.md", "docs/adr/not-markdown.txt"} {
		t.Run(strings.ReplaceAll(selector, "/", "_"), func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "must-not-exist")
			args := []string{"trace", "export", "--standard", "example", "--out", out, "--service", "tracesvc", "--store", database, "--commit", commit, "--decision-source", selector}
			code, _, stderr := runInDir(t, root, args)
			if code == 0 || !strings.Contains(stderr, "decision") || strings.Contains(stderr, "flag provided but not defined") {
				t.Fatalf("selector not explicitly refused: %q => %d %s", selector, code, stderr)
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatalf("refused source wrote a partial pack: %s, %v", out, err)
			}
		})
	}
}

func TestCmdTraceExplicitDecisionSourcesRefuseCommittedSymlinksAndControlPaths(t *testing.T) {
	for _, control := range []bool{false, true} {
		t.Run(map[bool]string{false: "symlink", true: "control-name"}[control], func(t *testing.T) {
			root, database, _ := newTraceProjectFixture(t)
			if control {
				traceProjectWrite(t, root, "docs/adr/bad\nname.md", "# ADR0044\n")
			} else {
				outside := filepath.Join(t.TempDir(), "outside.md")
				traceProjectWrite(t, filepath.Dir(outside), filepath.Base(outside), "# ADR0044\n")
				if err := os.Symlink(outside, filepath.Join(root, "docs/adr/linked.md")); err != nil {
					t.Fatal(err)
				}
			}
			commit := traceProjectCommit(t, root)
			out := filepath.Join(t.TempDir(), "must-not-exist")
			code, _, stderr := runInDir(t, root, []string{"trace", "export", "--standard", "example", "--out", out, "--service", "tracesvc", "--store", database, "--commit", commit, "--decision-source", "docs/adr"})
			if code == 0 || strings.Contains(stderr, "flag provided but not defined") {
				t.Fatalf("unsafe committed entry not refused: %d %s", code, stderr)
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatalf("unsafe entry wrote pack: %v", err)
			}
		})
	}
}

func TestCmdTraceExplicitDecisionSourcesRefuseWorkingTreeOnlySelector(t *testing.T) {
	root, database, commit := newTraceProjectFixture(t)
	traceProjectWrite(t, root, "dirty-only.md", "# ADR0045 Not committed\n")
	out := filepath.Join(t.TempDir(), "must-not-exist")
	code, _, stderr := runInDir(t, root, []string{"trace", "export", "--standard", "example", "--out", out, "--service", "tracesvc", "--store", database, "--commit", commit, "--decision-source", "dirty-only.md"})
	if code == 0 || !strings.Contains(stderr, "dirty-only.md") || strings.Contains(stderr, "flag provided but not defined") {
		t.Fatalf("uncommitted selector became authoritative: %d %s", code, stderr)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("missing committed selector wrote pack: %v", err)
	}
}

func TestCmdTraceVerifyRefusesDecisionSourceOverride(t *testing.T) {
	root, database, commit := newTraceProjectFixture(t)
	out := traceProjectExport(t, root, database, commit)
	code, _, stderr := runInDir(t, root, []string{"trace", "verify", out, "--store", database, "--decision-source", "INTENT.md"})
	if code != 2 || !strings.Contains(stderr, "decision-source") {
		t.Fatalf("verify accepted an invocation override of pinned sources: %d %s", code, stderr)
	}
}

func TestCmdTraceExplicitDecisionSourcesRefuseBoundOverflowBeforeWritingPack(t *testing.T) {
	for _, kind := range []string{"selectors", "files", "file-bytes", "total-bytes"} {
		t.Run(kind, func(t *testing.T) {
			root, database, _ := newTraceProjectFixture(t)
			var selectors []string
			switch kind {
			case "selectors":
				for i := 0; i < 17; i++ {
					name := fmt.Sprintf("decisions/%02d.md", i)
					traceProjectWrite(t, root, name, "# ADR0050\n")
					selectors = append(selectors, name)
				}
			case "files":
				for i := 0; i < 257; i++ {
					traceProjectWrite(t, root, fmt.Sprintf("many/%03d.md", i), "# ADR0050\n")
				}
				selectors = []string{"many"}
			case "file-bytes":
				traceProjectWrite(t, root, "large.md", strings.Repeat("x", (1<<20)+1))
				selectors = []string{"large.md"}
			case "total-bytes":
				for i := 0; i < 9; i++ {
					traceProjectWrite(t, root, fmt.Sprintf("total/%02d.md", i), strings.Repeat("x", 1<<20))
				}
				selectors = []string{"total"}
			}
			commit := traceProjectCommit(t, root)
			out := filepath.Join(t.TempDir(), "must-not-exist")
			args := []string{"trace", "export", "--standard", "example", "--out", out, "--service", "tracesvc", "--store", database, "--commit", commit}
			for _, selector := range selectors {
				args = append(args, "--decision-source", selector)
			}
			code, _, stderr := runInDir(t, root, args)
			if code == 0 || strings.Contains(stderr, "flag provided but not defined") || !strings.Contains(strings.ToLower(stderr), "limit") {
				t.Fatalf("%s overflow not explicitly bounded: %d %s", kind, code, stderr)
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatalf("overflow wrote partial pack: %v", err)
			}
		})
	}
}

// Fixture setup must not leave a detached Git writer racing the read-only
// export's full-tree assertion. Check native child-process events, rather than
// waiting for or excluding a maintenance lock that can exist for milliseconds.
func TestTraceFixtureCommitsDoNotLaunchAutoMaintenance(t *testing.T) {
	tracePath := filepath.Join(t.TempDir(), "git-trace2.jsonl")
	t.Setenv("GIT_TRACE2_EVENT", tracePath)
	newTraceProjectFixture(t)

	raw, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	type traceEvent struct {
		Event string   `json:"event"`
		SID   string   `json:"sid"`
		Argv  []string `json:"argv"`
	}
	var events []traceEvent
	commits := map[string]bool{}
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event traceEvent
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatalf("native Git Trace2 event: %v", err)
		}
		events = append(events, event)
		if event.Event == "start" {
			for _, arg := range event.Argv {
				if arg == "commit" {
					commits[event.SID] = true
				}
			}
		}
	}
	if len(commits) != 2 {
		t.Fatalf("native Trace2 recorded %d fixture commits, want both 2", len(commits))
	}
	for _, event := range events {
		if event.Event != "child_start" || !commits[event.SID] {
			continue
		}
		automatic, maintenance := false, false
		for _, arg := range event.Argv {
			automatic = automatic || arg == "--auto"
			maintenance = maintenance || arg == "maintenance" || arg == "gc"
		}
		if automatic && maintenance {
			t.Errorf("fixture commit launched automatic maintenance: %s", strings.Join(event.Argv, " "))
		}
	}
}
