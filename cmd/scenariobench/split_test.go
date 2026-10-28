package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/benchprep"
)

const (
	splitDevURL   = "https://example.org/acme/dev-go"
	splitHeldURL  = "https://example.org/acme/held-go"
	splitNoneURL  = "https://example.org/acme/no-split"
	splitOtherURL = "https://example.org/acme/no-manifest"
)

// splitFixture is a self-contained root: three manifests (dev, held-out and
// no split), a context-bench task file declared dev by the record, the record
// itself and a scenario corpus whose tasks cover dev by derivation, dev by id,
// held-out, post-cutoff, unassigned, undated and dropped.
type splitFixture struct {
	root, corpus, record, devFile string
	devSHA                        string
}

// splitTask is one corpus task of the fixture; merged and disposition are optional.
func splitTask(id, repo, derivation, merged, disposition string) map[string]any {
	task := map[string]any{
		"id":          id,
		"category":    "bug_fix",
		"prompt":      "fix it",
		"derivation":  derivation,
		"subject":     map[string]any{"repo_url": repo, "base_commit": "deadbeef"},
		"sessions":    map[string]any{"mode": "single", "count": 1, "max_turns": 5},
		"oracle":      map[string]any{"fail_to_pass": []string{"TestPass"}},
		"predictions": map[string]any{"control": "no-effect (this is the control)"},
	}
	if merged != "" {
		task["merged_at"] = merged
	}
	if disposition != "" {
		task["disposition"] = disposition
		task["reason"] = "oracle cannot be won from the task text"
	}
	return task
}

func writeSplitJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeSplitManifest writes a minimal valid #Subject manifest (JSON text, as
// calibration/bench.d does) named name, with the given url, split ("" for
// none) and fresh_after.
func writeSplitManifest(t *testing.T, dir, name, url, split, freshAfter string) {
	t.Helper()
	bench := map[string]any{
		"toolchain": map[string]any{
			"versions":        map[string]string{"go": "1.26.2"},
			"lockfile_sha256": strings.Repeat("0123456789abcdef", 4),
		},
		"setup":         []string{"go mod download"},
		"test":          []string{"go test ./..."},
		"suite_seconds": 12.5,
		"flake_runs":    3,
		"l0_reading":    "workspace/bench/l0/" + name + ".json",
		"task_sources":  []map[string]string{{"dataset": "synthetic-fixes", "revision": "v1", "licence": "CC0-1.0"}},
		"contamination": map[string]any{"age": 365, "popularity_band": "low"},
	}
	if freshAfter != "" {
		bench["contamination"].(map[string]any)["fresh_after"] = freshAfter
	}
	if split != "" {
		bench["split"] = split
	}
	manifest := map[string]any{
		"name": name, "url": url, "ref": "v1.2.3",
		"sha":     "0123456789abcdef0123456789abcdef01234567",
		"licence": "MIT", "subpath": "lib", "bench": bench,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append([]byte("# fixture manifest; comment lines are stripped\n"), data...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func newSplitFixture(t *testing.T, mutate func(tasks []map[string]any)) splitFixture {
	t.Helper()
	root := t.TempDir()
	benchD := filepath.Join(root, "calibration", "bench.d")
	writeSplitManifest(t, benchD, "dev-go", splitDevURL, "dev", "2026-01-01")
	writeSplitManifest(t, benchD, "held-go", splitHeldURL, "held-out", "2026-01-01")
	writeSplitManifest(t, benchD, "no-split", splitNoneURL, "", "2026-01-01")

	devRel := "testdata/contextbench/tasks-dev.json"
	devFile := filepath.Join(root, filepath.FromSlash(devRel))
	devTasks := []map[string]any{}
	for _, id := range []string{"X1", "X2"} {
		devTasks = append(devTasks, map[string]any{"id": id, "prompt": "where is it", "ground_truth": []string{"x"}, "scoring_rule": "all-of"})
	}
	writeSplitJSON(t, devFile, map[string]any{"version": 1, "tasks": devTasks})
	devSHA, err := benchprep.SplitSHA256([]string{"X1", "X2"})
	if err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(root, "docs", "bench", "heldout-reads.md")
	if err := os.MkdirAll(filepath.Dir(record), 0o755); err != nil {
		t.Fatal(err)
	}
	recordText := "# Held-out reads\n\nprose line\n\nDev split sha256: " + devSHA + " " + devRel + " (2 tasks)\n"
	if err := os.WriteFile(record, []byte(recordText), 0o644); err != nil {
		t.Fatal(err)
	}

	tasks := []map[string]any{
		splitTask("S-D1", splitOtherURL, "Copied verbatim from "+devRel+" id X1", "2025-10-01T00:00:00Z", ""),
		splitTask("X2", splitDevURL, "declared dev by id", "2026-06-01T00:00:00Z", ""),
		splitTask("H1", splitHeldURL, "held-out task", "2025-12-01T10:00:00Z", ""),
		splitTask("P1", splitHeldURL, "post-cutoff task", "2026-06-01T10:00:00Z", ""),
		splitTask("N1", splitHeldURL, "undated task", "", ""),
		splitTask("U1", splitNoneURL, "repository has no split", "2025-10-02T00:00:00Z", ""),
		splitTask("DR1", splitHeldURL, "dropped task", "", "dropped"),
	}
	if mutate != nil {
		mutate(tasks)
	}
	corpusPath := filepath.Join(t.TempDir(), "split-corpus.json")
	writeSplitJSON(t, corpusPath, map[string]any{
		"arms":      map[string]any{"control": map[string]any{"wired": false, "instructions": "none", "surface": ""}},
		"tasks":     tasks,
		"criterion": map[string]any{"id": "C1", "treatment": "control", "control": "control", "min_trials": 1},
	})
	return splitFixture{root: root, corpus: corpusPath, record: record, devFile: devFile, devSHA: devSHA}
}

func runSplit(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	stderr = captureStderr(t, func() {
		stdout = captureStdout(t, func() {
			code = run(append([]string{"split"}, args...))
		})
	})
	return code, stdout, stderr
}

func mustSplitSHA(t *testing.T, ids ...string) string {
	t.Helper()
	sha, err := benchprep.SplitSHA256(ids)
	if err != nil {
		t.Fatal(err)
	}
	return sha
}

func readSplitResult(t *testing.T, path string) benchprep.SplitResult {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result benchprep.SplitResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("%s does not unmarshal into SplitResult: %v", path, err)
	}
	return result
}

func splitIDs(result benchprep.SplitResult, split string) []string {
	for _, s := range result.Splits {
		if s.Split == split {
			return s.IDs
		}
	}
	return nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCmdSplitHappyPath(t *testing.T) {
	fx := newSplitFixture(t, nil)
	recordBefore, err := os.ReadFile(fx.record)
	if err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	want := []string{
		"dev sha256: " + mustSplitSHA(t, "S-D1", "X2") + " (2 tasks)",
		"held-out sha256: " + mustSplitSHA(t, "H1", "N1") + " (2 tasks)",
		"post-cutoff sha256: " + mustSplitSHA(t, "P1") + " (1 tasks)",
		"unassigned: 1 tasks (U1 manifest has no split)",
		"undated: 1 tasks (N1)",
		"dropped: 1 tasks (DR1)",
		"shortfall: go: 1 of 3 dev repositories",
		"shortfall: go: 1 of 3 held-out repositories",
		"shortfall: python: 0 of 3 dev repositories",
		"shortfall: python: 0 of 3 held-out repositories",
		"shortfall: typescript: 0 of 3 dev repositories",
		"shortfall: typescript: 0 of 3 held-out repositories",
		"wrote workspace/bench/splits/split-corpus.json",
	}
	if !equalStrings(lines, want) {
		t.Fatalf("stdout lines:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}

	result := readSplitResult(t, filepath.Join(fx.root, "workspace", "bench", "splits", "split-corpus.json"))
	if got := splitIDs(result, "dev"); !equalStrings(got, []string{"S-D1", "X2"}) {
		t.Fatalf("dev ids = %v", got)
	}
	if got := splitIDs(result, "held-out"); !equalStrings(got, []string{"H1", "N1"}) {
		t.Fatalf("held-out ids = %v", got)
	}
	if got := splitIDs(result, "post-cutoff"); !equalStrings(got, []string{"P1"}) {
		t.Fatalf("post-cutoff ids = %v", got)
	}
	if result.PostCutoffPending {
		t.Fatal("post_cutoff_pending is true although every manifest carries fresh_after")
	}
	raw, err := os.ReadFile(filepath.Join(fx.root, "workspace", "bench", "splits", "split-corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(raw, []byte("}\n")) || !bytes.Contains(raw, []byte("\n  \"splits\"")) {
		t.Fatalf("out file is not indented JSON with a trailing newline: %q", raw)
	}
	recordAfter, err := os.ReadFile(fx.record)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(recordBefore, recordAfter) {
		t.Fatal("the record changed: split must never write it")
	}
}

func TestCmdSplitPendingAndEmptySplitLines(t *testing.T) {
	fx := newSplitFixture(t, func(tasks []map[string]any) {
		// every task is dev or unassigned, so held-out and post-cutoff are empty
		for _, task := range tasks {
			switch task["id"] {
			case "H1", "P1", "N1", "DR1":
				task["subject"] = map[string]any{"repo_url": splitOtherURL, "base_commit": "deadbeef"}
			}
		}
	})
	// no manifest carries fresh_after, so the post-cutoff slice is pending
	benchD := filepath.Join(fx.root, "calibration", "bench.d")
	writeSplitManifest(t, benchD, "dev-go", splitDevURL, "dev", "")
	writeSplitManifest(t, benchD, "held-go", splitHeldURL, "held-out", "")
	writeSplitManifest(t, benchD, "no-split", splitNoneURL, "", "")
	code, out, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
	}
	for _, want := range []string{
		"held-out sha256: none (0 tasks)\n",
		"post-cutoff sha256: none (0 tasks): pending Q2\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "unassigned: 4 tasks (") {
		t.Fatalf("stdout lacks the unassigned line:\n%s", out)
	}
}

func TestCmdSplitPrintsZeroCountLines(t *testing.T) {
	fx := newSplitFixture(t, func(tasks []map[string]any) {
		for _, task := range tasks {
			switch task["id"] {
			case "N1":
				task["merged_at"] = "2025-11-01T00:00:00Z"
			case "U1":
				task["subject"] = map[string]any{"repo_url": splitHeldURL, "base_commit": "deadbeef"}
				task["merged_at"] = "2025-11-02T00:00:00Z"
			case "DR1":
				delete(task, "disposition")
				delete(task, "reason")
				task["merged_at"] = "2025-11-03T00:00:00Z"
			}
		}
	})
	code, out, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
	}
	for _, want := range []string{"unassigned: 0 tasks\n", "dropped: 0 tasks\n", "undated: 0 tasks\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out)
		}
	}
}

func TestCmdSplitRunsFromAnotherCwd(t *testing.T) {
	fx := newSplitFixture(t, nil)
	t.Chdir(t.TempDir())
	code, out, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(fx.root, "workspace", "bench", "splits", "split-corpus.json")); err != nil {
		t.Fatalf("out file is not under the root: %v", err)
	}
	if _, err := os.Stat(filepath.Join("workspace")); err == nil {
		t.Fatal("something was written under the cwd")
	}
}

func TestCmdSplitRelativePathsResolveAgainstRoot(t *testing.T) {
	fx := newSplitFixture(t, nil)
	cwd := t.TempDir()
	t.Chdir(cwd)
	// the manifests and record live under the root under non-default names
	if err := os.Rename(filepath.Join(fx.root, "calibration", "bench.d"), filepath.Join(fx.root, "alt-manifests")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(fx.record, filepath.Join(fx.root, "alt-record.md")); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root,
		"-manifests", "alt-manifests", "-record", "alt-record.md", "-out", filepath.Join("scratch", "out.json"))
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
	}
	if !strings.HasSuffix(out, "wrote scratch/out.json\n") {
		t.Fatalf("last line is not the repo-relative out path:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(fx.root, "scratch", "out.json")); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(cwd); len(entries) != 0 {
		t.Fatalf("the cwd was written to: %v", entries)
	}
}

func TestCmdSplitAbsoluteOutIsUsedAsIs(t *testing.T) {
	fx := newSplitFixture(t, nil)
	outPath := filepath.Join(t.TempDir(), "nested", "out.json")
	code, out, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root, "-out", outPath)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
	}
	if !strings.HasSuffix(out, "wrote "+outPath+"\n") {
		t.Fatalf("last line is not the absolute out path:\n%s", out)
	}
	readSplitResult(t, outPath)
	if _, err := os.Stat(filepath.Join(fx.root, "workspace")); err == nil {
		t.Fatal("the default out directory was created although -out was absolute")
	}
}

func TestCmdSplitDefaultRootIsTheGitTopLevel(t *testing.T) {
	fx := newSplitFixture(t, nil)
	if out, err := exec.Command("git", "init", "-q", fx.root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	sub := filepath.Join(fx.root, "some", "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	code, out, errOut := runSplit(t, "-corpus", fx.corpus)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
	}
	if !strings.HasSuffix(out, "wrote workspace/bench/splits/split-corpus.json\n") {
		t.Fatalf("stdout:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(sub, "workspace")); err == nil {
		t.Fatal("the out file went under the cwd, not the git top level")
	}
}

func TestCmdSplitNoRootAndNoGitExits2(t *testing.T) {
	fx := newSplitFixture(t, nil)
	cwd := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(cwd))
	t.Chdir(cwd)
	code, out, errOut := runSplit(t, "-corpus", fx.corpus)
	if code != 2 || out != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
	}
	if !strings.HasPrefix(errOut, "scenariobench split: ") || !strings.Contains(errOut, "-root") {
		t.Fatalf("stderr does not name the missing root: %q", errOut)
	}
}

func TestCmdSplitManifestRefusals(t *testing.T) {
	t.Run("absent directory", func(t *testing.T) {
		fx := newSplitFixture(t, nil)
		if err := os.RemoveAll(filepath.Join(fx.root, "calibration", "bench.d")); err != nil {
			t.Fatal(err)
		}
		code, out, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root)
		if code != 2 || out != "" || !strings.HasPrefix(errOut, "scenariobench split: ") {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
		}
	})
	t.Run("empty directory", func(t *testing.T) {
		fx := newSplitFixture(t, nil)
		dir := filepath.Join(fx.root, "calibration", "bench.d")
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				t.Fatal(err)
			}
		}
		code, out, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root)
		if code != 2 || out != "" || !strings.HasPrefix(errOut, "scenariobench split: ") {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
		}
	})
	t.Run("unreadable directory", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads a chmod 000 directory; the unreadable case cannot be built as uid 0")
		}
		fx := newSplitFixture(t, nil)
		dir := filepath.Join(fx.root, "calibration", "bench.d")
		if err := os.Chmod(dir, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(dir, 0o755) })
		code, out, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root)
		if code != 2 || out != "" || !strings.HasPrefix(errOut, "scenariobench split: ") {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
		}
		if _, err := os.Stat(filepath.Join(fx.root, "workspace")); err == nil {
			t.Fatal("an out file was written despite the refusal")
		}
	})
}

func TestCmdSplitRecordRefusals(t *testing.T) {
	t.Run("absent record", func(t *testing.T) {
		fx := newSplitFixture(t, nil)
		if err := os.Remove(fx.record); err != nil {
			t.Fatal(err)
		}
		code, out, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root)
		if code != 2 || out != "" {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
		}
		if !strings.Contains(errOut, "record is absent") || !strings.Contains(errOut, "heldout-reads.md") {
			t.Fatalf("stderr does not name the absent record: %q", errOut)
		}
		if _, err := os.Stat(filepath.Join(fx.root, "workspace")); err == nil {
			t.Fatal("an out file was written despite the refusal")
		}
	})
	t.Run("drifted dev line", func(t *testing.T) {
		fx := newSplitFixture(t, nil)
		extra := []map[string]any{
			{"id": "X1", "prompt": "where is it", "ground_truth": []string{"x"}, "scoring_rule": "all-of"},
			{"id": "X3", "prompt": "where is it", "ground_truth": []string{"x"}, "scoring_rule": "all-of"},
		}
		writeSplitJSON(t, fx.devFile, map[string]any{"version": 1, "tasks": extra})
		code, out, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root)
		if code != 2 || out != "" {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
		}
		if strings.Contains(errOut, "is absent") || !strings.Contains(errOut, "drifted") {
			t.Fatalf("a drifted record must not read as absent: %q", errOut)
		}
	})
	t.Run("unreadable record is not absent", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads a chmod 000 file")
		}
		fx := newSplitFixture(t, nil)
		if err := os.Chmod(fx.record, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(fx.record, 0o644) })
		code, out, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root)
		if code != 2 || out != "" || strings.Contains(errOut, "is absent") || !strings.Contains(errOut, "permission denied") {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
		}
	})
}

func TestCmdSplitRefusesDeclaredDevTaskOnHeldOutRepository(t *testing.T) {
	for _, tc := range []struct{ name, id, derivation string }{
		{"by id", "X2", "declared dev by id"},
		{"by derivation", "S-D9", "Copied verbatim from testdata/contextbench/tasks-dev.json id X1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newSplitFixture(t, func(tasks []map[string]any) {
				tasks[0] = splitTask(tc.id, splitHeldURL, tc.derivation, "", "")
			})
			code, out, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root)
			if code != 2 || out != "" {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
			}
			if !strings.HasPrefix(errOut, "scenariobench split: ") || !strings.Contains(errOut, tc.id) {
				t.Fatalf("stderr does not name the task: %q", errOut)
			}
			if _, err := os.Stat(filepath.Join(fx.root, "workspace")); err == nil {
				t.Fatal("an out file was written despite the refusal")
			}
		})
	}
}

func TestCmdSplitJSON(t *testing.T) {
	fx := newSplitFixture(t, nil)
	code, out, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root, "-json")
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
	}
	var result benchprep.SplitResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("stdout is not a SplitResult: %v\n%s", err, out)
	}
	if got := splitIDs(result, "dev"); !equalStrings(got, []string{"S-D1", "X2"}) {
		t.Fatalf("dev ids = %v", got)
	}
	if strings.Contains(out, "wrote ") || strings.Contains(out, " sha256: ") {
		t.Fatalf("-json printed text lines:\n%s", out)
	}
	// the file is still written, and equals what was printed
	onDisk := readSplitResult(t, filepath.Join(fx.root, "workspace", "bench", "splits", "split-corpus.json"))
	a, _ := json.Marshal(result)
	b, _ := json.Marshal(onDisk)
	if !bytes.Equal(a, b) {
		t.Fatalf("stdout and out file differ:\n%s\n%s", a, b)
	}
}

func TestCmdSplitUsage(t *testing.T) {
	t.Run("missing corpus", func(t *testing.T) {
		code, out, errOut := runSplit(t)
		if code != 2 || out != "" || !strings.Contains(errOut, "scenariobench split: -corpus is required") {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
		}
	})
	t.Run("unexpected argument", func(t *testing.T) {
		code, _, errOut := runSplit(t, "-corpus", "x.json", "extra")
		if code != 2 || !strings.Contains(errOut, `unexpected argument "extra"`) {
			t.Fatalf("exit=%d stderr=%q", code, errOut)
		}
	})
	t.Run("corpus that does not load", func(t *testing.T) {
		fx := newSplitFixture(t, nil)
		code, out, errOut := runSplit(t, "-corpus", filepath.Join(t.TempDir(), "absent.json"), "-root", fx.root)
		if code != 2 || out != "" || !strings.HasPrefix(errOut, "scenariobench split: ") {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
		}
	})
	t.Run("usage lists split", func(t *testing.T) {
		var code int
		errOut := captureStderr(t, func() { code = run(nil) })
		if code != 2 || !strings.Contains(errOut, "split") {
			t.Fatalf("exit=%d usage=%q", code, errOut)
		}
		names := commandUsage()
		if !strings.Contains("|"+names+"|", "|split|") {
			t.Fatalf("commandUsage() = %q", names)
		}
	})
	t.Run("help lists the flags", func(t *testing.T) {
		var code int
		errOut := captureStderr(t, func() {
			captureStdout(t, func() { code = run([]string{"split", "-help"}) })
		})
		if code != 2 {
			t.Fatalf("exit=%d", code)
		}
		for _, flagName := range []string{"-corpus", "-manifests", "-record", "-root", "-out", "-json"} {
			if !strings.Contains(errOut, flagName) {
				t.Fatalf("-help does not list %s:\n%s", flagName, errOut)
			}
		}
	})
}

// splitRepoRoot walks up from the test's cwd to the directory holding go.mod.
func splitRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test's cwd")
		}
		dir = parent
	}
}

// The committed manifests, record and corpus: every lookups-vs task derives
// from the home corpus the record declares dev, so it lands in dev; the P1
// fleet is far from three and three per language, which is only reported.
func TestCmdSplitRealCommittedData(t *testing.T) {
	root := splitRepoRoot(t)
	recordPath := filepath.Join(root, "docs", "bench", "heldout-reads.md")
	recordBefore, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	_, existed := os.Stat(filepath.Join(root, "workspace", "bench", "splits"))
	outPath := filepath.Join(t.TempDir(), "lookups-vs.json")
	corpusPath := filepath.Join(root, "testdata", "scenariobench", "lookups-vs.json")
	code, out, errOut := runSplit(t, "-root", root, "-corpus", corpusPath, "-out", outPath)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
	}
	corpus, _, err := loadCorpus(corpusPath)
	if err != nil {
		t.Fatal(err)
	}
	result := readSplitResult(t, outPath)
	dev := map[string]bool{}
	for _, id := range splitIDs(result, "dev") {
		dev[id] = true
	}
	for _, task := range corpus.Tasks {
		if task.Disposition == "dropped" {
			continue
		}
		if strings.Contains(task.Derivation, "testdata/contextbench/tasks-home.json") && !dev[task.ID] {
			t.Errorf("task %s derives from the home corpus but is not dev", task.ID)
		}
	}
	for _, id := range splitIDs(result, "dev") {
		if !strings.HasPrefix(id, "H1-T") {
			t.Errorf("unexpected dev id %s", id)
		}
	}
	if len(dev) == 0 {
		t.Fatal("no dev tasks")
	}
	for _, want := range []string{
		"shortfall: go: ", "shortfall: python: ", "shortfall: typescript: ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks a %q line:\n%s", want, out)
		}
	}
	if !strings.HasSuffix(out, "wrote "+outPath+"\n") {
		t.Errorf("last line is not the absolute out path:\n%s", out)
	}
	recordAfter, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(recordBefore, recordAfter) {
		t.Fatal("the committed record changed")
	}
	if _, nowExists := os.Stat(filepath.Join(root, "workspace", "bench", "splits")); (existed == nil) != (nowExists == nil) {
		t.Fatal("the test created or removed workspace/bench/splits in the repository")
	}
}

// split never writes the record (BX-11 is the controller's): an -out that
// names the record, relatively, absolutely or through a symlink, exits 2 and
// leaves the record byte-identical.
func TestCmdSplitRefusesOutOnTheRecord(t *testing.T) {
	fx := newSplitFixture(t, nil)
	before, err := os.ReadFile(fx.record)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "record-link.md")
	if err := os.Symlink(fx.record, link); err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{filepath.Join("docs", "bench", "heldout-reads.md"), fx.record, link} {
		code, stdout, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root, "-out", out)
		if code != 2 || !strings.Contains(errOut, "record") {
			t.Fatalf("-out %s: exit=%d stdout=%q stderr=%q, want 2 naming the record", out, code, stdout, errOut)
		}
		after, err := os.ReadFile(fx.record)
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != string(before) {
			t.Fatalf("-out %s rewrote the record", out)
		}
	}
}

// split never overwrites its inputs: an -out on the corpus, or inside the
// manifest directory, exits 2 and leaves the corpus byte-identical.
func TestCmdSplitRefusesOutOnItsInputs(t *testing.T) {
	fx := newSplitFixture(t, nil)
	before, err := os.ReadFile(fx.corpus)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ out, want string }{
		{fx.corpus, "corpus"},
		{filepath.Join("calibration", "bench.d", "split.json"), "manifest directory"},
	} {
		code, stdout, errOut := runSplit(t, "-corpus", fx.corpus, "-root", fx.root, "-out", tc.out)
		if code != 2 || !strings.Contains(errOut, tc.want) {
			t.Fatalf("-out %s: exit=%d stdout=%q stderr=%q, want 2 naming the %s", tc.out, code, stdout, errOut, tc.want)
		}
	}
	after, err := os.ReadFile(fx.corpus)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("an -out refusal rewrote the corpus")
	}
	if _, err := os.Stat(filepath.Join(fx.root, "calibration", "bench.d", "split.json")); !os.IsNotExist(err) {
		t.Fatalf("split wrote inside the manifest directory: %v", err)
	}
}
