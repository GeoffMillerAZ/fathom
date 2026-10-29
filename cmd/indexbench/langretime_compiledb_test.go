package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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

	if err := retimeInstallCompileDB(companion, root); err != nil {
		t.Fatal(err)
	}
	if err := retimeInstallCompileDB(companion, root); err != nil {
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
	if err := retimeInstallCompileDB(companion, other); err != nil {
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
	if err := retimeInstallCompileDB(companion, root); err != nil {
		t.Fatal(err)
	}
	if diff := retimeIdentityDiff(recorded, cudaMeasure(t, root), nil, false); len(diff) != 0 {
		t.Fatalf("after install: %v", diff)
	}

	// A different database already in place is refused.
	if err := os.WriteFile(filepath.Join(other, "build", "compile_commands.json"), []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := retimeInstallCompileDB(companion, other); err == nil || !strings.Contains(err.Error(), "different compile database") {
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
