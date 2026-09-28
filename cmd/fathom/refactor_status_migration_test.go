package main

// refactor_status_migration_test.go — CX-13 (design §4, U9): `fathom
// refactor status --migration "<kind> <from> -> <to>"` builds the
// fleet-wide migration view (CX-10's review.MigrationSites) from
// per-service typed (internal/analysis/callgraph) and lexical
// (internal/analysis/references) reference queries, run through
// *pkg/client.Client one service at a time (never the whole fleet's
// sources loaded together) — and is read-only throughout: no migration
// site is ever rewritten (the Moderne non-goal, CX-X4 refused).

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newMigrationFixtureRepo writes a tiny two-package Go module at dir/name:
// pkgDir declares funcName, caller/ imports it and calls it once — a real,
// typed cross-package reference (mirrors newRefactorFixtureRepo's own
// shape in refactor_plan_test.go).
func newMigrationFixtureRepo(t *testing.T, dir, name, pkgDir, funcName string) string {
	t.Helper()
	root := filepath.Join(dir, name)
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
	write("go.mod", "module "+name+"\n\ngo 1.21\n")
	write(pkgDir+"/def.go", "package "+pkgDir+"\n\nfunc "+funcName+"() {}\n")
	write("caller/user.go", "package caller\n\nimport \""+name+"/"+pkgDir+"\"\n\nfunc use() {\n\t"+pkgDir+"."+funcName+"()\n}\n")
	return root
}

// hashTreeForTest is a content+layout fingerprint of every file under
// root — the "never rewrites" proof compares this before and after the
// command runs, rather than trusting a mocked writer.
func hashTreeForTest(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	var paths []string
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	for _, p := range paths {
		rel, _ := filepath.Rel(root, p)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		h.Write([]byte(rel))
		h.Write([]byte{0})
		h.Write(b)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TestRefactorStatusMigrationNeverRewrites: two services in one shared
// store — svc-a still calls OldFunc (a remaining, typed site), svc-b
// already calls NewFunc (a done, typed site). The fleet view reports both,
// grouped per repository with their tier — and every file on disk in both
// service roots is byte-identical before and after the call.
func TestRefactorStatusMigrationNeverRewrites(t *testing.T) {
	dir := t.TempDir()
	svcARoot := newMigrationFixtureRepo(t, dir, "svc-a", "oldpkg", "OldFunc")
	svcBRoot := newMigrationFixtureRepo(t, dir, "svc-b", "newpkg", "NewFunc")

	storePath := filepath.Join(t.TempDir(), "fleet.sqlite")
	for svc, root := range map[string]string{"svc-a": svcARoot, "svc-b": svcBRoot} {
		var ing bytes.Buffer
		if code := run([]string{
			"ingest", "--service", svc, "--path", root, "--store", storePath,
		}, &ing, &ing); code != 0 {
			t.Fatalf("ingest %s failed (%d): %s", svc, code, ing.String())
		}
	}

	beforeA, beforeB := hashTreeForTest(t, svcARoot), hashTreeForTest(t, svcBRoot)

	code, out, errBuf := runInDir(t, dir, []string{
		"refactor", "status",
		"--migration", "symbol OldFunc -> NewFunc",
		"--store", storePath, "--json",
	})
	if code != 0 {
		t.Fatalf("refactor status --migration failed (%d)\nstdout=%s\nstderr=%s", code, out, errBuf)
	}

	afterA, afterB := hashTreeForTest(t, svcARoot), hashTreeForTest(t, svcBRoot)
	if beforeA != afterA || beforeB != afterB {
		t.Fatalf("refactor status --migration modified a service's working tree — it must only ever read (CX-X4 refused: never rewrite a site)")
	}

	if !strings.Contains(out, `"repository": "svc-a"`) {
		t.Errorf("missing svc-a in the fleet view:\n%s", out)
	}
	if !strings.Contains(out, `"repository": "svc-b"`) {
		t.Errorf("missing svc-b in the fleet view:\n%s", out)
	}
	if !strings.Contains(out, `"tier": "typed"`) {
		t.Errorf("expected at least one typed-tier site (both fixtures have a real typed caller):\n%s", out)
	}
	if !strings.Contains(out, `"file": "caller/user.go"`) {
		t.Errorf("expected caller/user.go as a site in both services:\n%s", out)
	}
}

// TestRefactorStatusMigration_RejectsBadDeclaration: an unparsable
// --migration value is refused with a named reason (CX-9's own contract),
// never silently treated as an empty migration.
func TestRefactorStatusMigration_RejectsBadDeclaration(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "empty.sqlite")

	code, _, errBuf := runInDir(t, dir, []string{
		"refactor", "status", "--migration", "not a valid declaration", "--store", storePath,
	})
	if code == 0 {
		t.Fatalf("want a non-zero exit for an unparsable --migration value; stderr=%s", errBuf)
	}
	if !strings.Contains(errBuf, "migration declaration") {
		t.Errorf("expected the parse error to name the bad declaration, got: %s", errBuf)
	}
}
