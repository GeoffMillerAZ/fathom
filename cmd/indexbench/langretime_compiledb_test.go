package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// cudaCompDBFixture is a git checkout whose src/a.cu includes "k.cuh" from
// inc/, found only through the gitignored build/ database's -I inc.
func cudaCompDBFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	put := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put("src/a.cu", "#include \"k.cuh\"\n__global__ void run() { helper(); }\n")
	put("inc/k.cuh", "#pragma once\n__device__ void helper() {}\n")
	put(".gitignore", "build/\n")
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}} {
		c := exec.Command("git", args...)
		c.Dir = root
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git: %v: %s", err, out)
		}
	}
	return root
}

func cudaMeasure(t *testing.T, root string) langbench.Subject {
	t.Helper()
	spec, source, err := resolveBenchSpec("cuda", "")
	if err != nil {
		t.Fatal(err)
	}
	with, closeExtractor, err := langOracleExtractor(spec.Manifest, source)
	if err != nil {
		t.Fatal(err)
	}
	defer closeExtractor()
	m, err := measureSubject(t.Context(), spec, root, "fixture", oracleOpts{noOracles: true, retimeImports: true}, with)
	if err != nil {
		t.Fatal(err)
	}
	return m.subject
}

func installDB(src, root string) error {
	_, err := retimeInstallCompileDB(context.Background(), src, root, "fixture")
	return err
}

func TestRetimeCompileDB(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	root := cudaCompDBFixture(t)
	companion := filepath.Join(t.TempDir(), "compile_commands.json")
	if err := os.WriteFile(companion, []byte(`[{"directory":".","command":"nvcc -I inc -c src/a.cu","file":"src/a.cu"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	other := cudaCompDBFixture(t)

	// Without a database: nothing recorded.
	bare := cudaMeasure(t, root)
	if bare.Tenancy.CompileDB == nil || bare.Tenancy.CompileDB.Databases != 0 || bare.Tenancy.CompileDB.Fingerprint != "" {
		t.Fatalf("bare identity = %+v", bare.Tenancy.CompileDB)
	}

	if err := installDB(companion, root); err != nil {
		t.Fatal(err)
	}
	if err := installDB(companion, root); err != nil {
		t.Fatalf("same database twice: %v", err)
	}
	recorded := cudaMeasure(t, root)
	if recorded.Tenancy.CompileDB.Databases != 1 || recorded.Tenancy.CompileDB.Fingerprint == "" {
		t.Fatalf("recorded identity = %+v", recorded.Tenancy.CompileDB)
	}
	if recorded.Counts[langbench.CntImportEdgesFathom] == bare.Counts[langbench.CntImportEdgesFathom] {
		t.Fatal("fixture import edges do not depend on the database")
	}

	// One database under a second absolute root: same fingerprint.
	if err := installDB(companion, other); err != nil {
		t.Fatal(err)
	}
	if got := cudaMeasure(t, other).Tenancy.CompileDB; *got != *recorded.Tenancy.CompileDB {
		t.Fatalf("two roots differ: %+v vs %+v", got, recorded.Tenancy.CompileDB)
	}
	if diff := retimeIdentityDiff(recorded, cudaMeasure(t, other), nil, false); len(diff) != 0 {
		t.Fatalf("equal identity refused: %v", diff)
	}

	// A re-time without the database refuses first, naming the cause.
	if err := os.RemoveAll(filepath.Join(root, "build")); err != nil {
		t.Fatal(err)
	}
	diff := retimeIdentityDiff(recorded, cudaMeasure(t, root), nil, false)
	if len(diff) == 0 || !strings.HasPrefix(diff[0], "compile_db: recorded "+recorded.Tenancy.CompileDB.Fingerprint[:12]+" (1 database(s)) -> measured none (0 database(s))") {
		t.Fatalf("diff = %v", diff)
	}

	// -compile-db restores it.
	if err := installDB(companion, root); err != nil {
		t.Fatal(err)
	}
	if diff := retimeIdentityDiff(recorded, cudaMeasure(t, root), nil, false); len(diff) != 0 {
		t.Fatalf("after install: %v", diff)
	}

	// A different database already in place is refused.
	if err := os.WriteFile(filepath.Join(other, "build", "compile_commands.json"), []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installDB(companion, other); err == nil || !strings.Contains(err.Error(), "different compile database") {
		t.Fatalf("overwrite not refused: %v", err)
	}

	// Legacy reading: no compile_db, a note that says 1 database.
	legacy := recorded
	legacyTenancy := *recorded.Tenancy
	legacyTenancy.CompileDB = nil
	legacy.Tenancy = &legacyTenancy
	notes := []string{"fixture: include-path resolution read 1 compile database(s); 0 quoted include(s) resolved only rooted at an ancestor directory of the includer; header-prototype binding bound 0 call(s)"}
	if err := os.RemoveAll(filepath.Join(root, "build")); err != nil {
		t.Fatal(err)
	}
	diff = retimeIdentityDiff(legacy, cudaMeasure(t, root), notes, false)
	if len(diff) == 0 || diff[0] != "compile_db: reading's note records 1 database(s), measured 0" {
		t.Fatalf("legacy diff = %v", diff)
	}
	// Legacy without a note: no compile_db line (the counts still differ).
	for _, line := range retimeIdentityDiff(legacy, cudaMeasure(t, root), nil, false) {
		if strings.HasPrefix(line, "compile_db:") {
			t.Fatalf("legacy no note: %s", line)
		}
	}
}

// commitCudaFixture is cudaCompDBFixture with a real commit (no hooks); the
// .gitignore is dropped when ignore is false.
func commitCudaFixture(t *testing.T, ignore bool) string {
	t.Helper()
	root := cudaCompDBFixture(t)
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	if !ignore {
		git("rm", "-q", "--cached", ".gitignore")
		os.Remove(filepath.Join(root, ".gitignore"))
	}
	tree := git("write-tree")
	c := exec.Command("git", "commit-tree", tree, "-m", "fixture")
	c.Dir = root
	c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	sha, err := c.Output()
	if err != nil {
		t.Fatal(err)
	}
	git("update-ref", "HEAD", strings.TrimSpace(string(sha)))
	return root
}

// TestCmdLangRetimeCompileDB runs lang-retime end to end: a reading recorded
// with a gitignored build database refuses without it, passes with
// -compile-db name=file, leaves the checkout clean, and refuses when build/
// is not ignored.
func TestCmdLangRetimeCompileDB(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	root := commitCudaFixture(t, true)
	companion := filepath.Join(t.TempDir(), "compile_commands.json")
	if err := os.WriteFile(companion, []byte(`[{"directory":".","command":"nvcc -I inc -c src/a.cu","file":"src/a.cu"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installDB(companion, root); err != nil {
		t.Fatal(err)
	}
	ctags := fakeCtags(t, t.TempDir(), `{"_type":"tag","name":"run","path":"src/a.cu","line":2,"end":2,"kind":"function","signature":"()"}`+"\n")
	stubFileClocks(t, time.Second, time.Second, true)
	path := filepath.Join(t.TempDir(), "cuda.json")
	var out, errs bytes.Buffer
	code := cmdLangOracle([]string{"-dialect", "cuda", "-manifest", "internal/extractor/dialects/cuda/dialect.json", "-subject", root, "-subject-name", "fixture", "-ctags", ctags, "-out", path, "-commit", "original"}, &out, &errs)
	if code != 0 {
		t.Fatalf("oracle exit %d: %s", code, errs.String())
	}
	if r := readReading(t, path); r.Subjects[0].Tenancy.CompileDB == nil || r.Subjects[0].Tenancy.CompileDB.Databases != 1 {
		t.Fatalf("reading lacks compile_db: %+v", r.Subjects[0].Tenancy)
	}
	if err := os.RemoveAll(filepath.Join(root, "build")); err != nil {
		t.Fatal(err)
	}
	args := []string{"lang-retime", "-reading", path, "-subject", "fixture=" + root}
	stubFileClocks(t, 11*time.Millisecond, time.Millisecond, true)
	out.Reset()
	errs.Reset()
	if code := mainRun(args, &out, &errs); code != 1 || !strings.Contains(errs.String(), "compile_db: recorded ") || !strings.Contains(errs.String(), "-> measured none (0 database(s))") {
		t.Fatalf("without the database: exit %d: %s", code, errs.String())
	}
	out.Reset()
	errs.Reset()
	if code := mainRun(append(append([]string{}, args...), "-compile-db", "fixture="+companion), &out, &errs); code != 0 {
		t.Fatalf("with -compile-db: exit %d: %s", code, errs.String())
	}
	status := exec.Command("git", "status", "--porcelain")
	status.Dir = root
	if b, err := status.Output(); err != nil || len(b) != 0 {
		t.Fatalf("checkout not clean: %q %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(root, "build", "compile_commands.json")); !os.IsNotExist(err) {
		t.Fatalf("installed database left behind: %v", err)
	}
	// Unknown and repeated subjects.
	for _, bad := range [][]string{{"-compile-db", "nope=" + companion}, {"-compile-db", "fixture=" + companion, "-compile-db", "fixture=" + companion}} {
		errs.Reset()
		if code := mainRun(append(append([]string{}, args...), bad...), &out, &errs); code != 1 || !strings.Contains(errs.String(), "-compile-db") {
			t.Fatalf("%v: exit %d: %s", bad, code, errs.String())
		}
	}
	// build/ not ignored: refused, nothing left behind.
	open := commitCudaFixture(t, false)
	errs.Reset()
	bad := []string{"lang-retime", "-reading", path, "-subject", "fixture=" + open, "-compile-db", "fixture=" + companion}
	if code := mainRun(bad, &out, &errs); code != 1 || !strings.Contains(errs.String(), "build/compile_commands.json is not gitignored in fixture") {
		t.Fatalf("unignored build: exit %d: %s", code, errs.String())
	}
	if _, err := os.Stat(filepath.Join(open, "build")); !os.IsNotExist(err) {
		t.Fatalf("refused install left build/: %v", err)
	}
	// An absolute companion directory is refused.
	abs := filepath.Join(t.TempDir(), "abs.json")
	os.WriteFile(abs, []byte(`[{"directory":"/x","command":"nvcc -c src/a.cu","file":"src/a.cu"}]`), 0o644)
	errs.Reset()
	if code := mainRun(append(append([]string{}, args...), "-compile-db", "fixture="+abs), &out, &errs); code != 1 || !strings.Contains(errs.String(), "entry 1 has an absolute directory; publish the database subject-relative") {
		t.Fatalf("absolute directory: exit %d: %s", code, errs.String())
	}
}
