package main

// Tests for kit-new's -package mode: golden files for the scaffold and its
// report, ledger application and its idempotence, the refuse-to-overwrite
// and refuse-before-writing rules, TRACKING.md section resolution, the
// constitution checks the scaffold must pass (gofmt, doc-coverage,
// concurrency-contract, schema-presence, cue vet), and a gated integration
// test that `go vet`s and `go test -race`s the scaffold as a real package.
//
// Every run scaffolds into a copy of testdata/fixture-repo inside
// t.TempDir(): the fixture ledgers mirror the real ones' shapes (numbered
// TRACKING sections with multi-token titles, the sorted compat table, an
// [Unreleased] › Added block, a practices table) without being them.

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// copyFixtureRepo copies testdata/fixture-repo into a fresh temp dir.
func copyFixtureRepo(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	src := filepath.Join("testdata", "fixture-repo")
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture repo: %v", err)
	}
	return dst
}

// scaffoldArgs are the flags every package-mode test starts from.
func scaffoldArgs(repo string, extra ...string) []string {
	return append([]string{
		"-package", "foundations/alpha/widget-gizmo",
		"-repo", repo,
		"-practice", "1",
		"-consumer", "kits/beta",
	}, extra...)
}

// plainPackageFiles is the six-file shape plus the commented negative
// fixture, as written for a package without -interface.
var plainPackageFiles = []goldenFile{
	{generated: "schema.cue", golden: "schema.cue"},
	{generated: "types.go", golden: "types.go"},
	{generated: "doc.go", golden: "doc.go"},
	{generated: "widget-gizmo.go", golden: "widget-gizmo.go"},
	{generated: "widget-gizmo_test.go", golden: "widget-gizmo_test.go"},
	{generated: filepath.Join("testdata", "valid.cue"), golden: filepath.Join("testdata", "valid.cue")},
	{generated: filepath.Join("testdata", "invalid.cue"), golden: filepath.Join("testdata", "invalid.cue")},
}

// TestPackageGolden pins every scaffolded file and the printed report for a
// package without -interface against testdata/golden/package/plain/.
func TestPackageGolden(t *testing.T) {
	repo := copyFixtureRepo(t)
	code, stdout, stderr := runNew(t, scaffoldArgs(repo)...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	pkgDir := filepath.Join(repo, "foundations", "alpha", "widget-gizmo")
	if _, err := os.Stat(filepath.Join(pkgDir, "mock.go")); err == nil {
		t.Error("mock.go written without -interface")
	}
	goldenDir := filepath.Join("testdata", "golden", "package", "plain")
	checkGoldenFiles(t, pkgDir, goldenDir, plainPackageFiles)

	// The report names only repo-relative paths, so it is golden-able too.
	if err := os.WriteFile(filepath.Join(repo, "stdout.txt"), []byte(stdout), 0o644); err != nil {
		t.Fatal(err)
	}
	checkGoldenFiles(t, repo, goldenDir, []goldenFile{{generated: "stdout.txt", golden: "stdout.txt"}})
}

// TestPackageGoldenInterface pins the files -interface changes or adds.
func TestPackageGoldenInterface(t *testing.T) {
	repo := copyFixtureRepo(t)
	code, _, stderr := runNew(t, scaffoldArgs(repo, "-interface", "Stamper")...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	pkgDir := filepath.Join(repo, "foundations", "alpha", "widget-gizmo")
	checkGoldenFiles(t, pkgDir, filepath.Join("testdata", "golden", "package", "iface"), []goldenFile{
		{generated: "types.go", golden: "types.go"},
		{generated: "widget-gizmo.go", golden: "widget-gizmo.go"},
		{generated: "widget-gizmo_test.go", golden: "widget-gizmo_test.go"},
		{generated: "mock.go", golden: "mock.go"},
	})
	// Files -interface does not touch are byte-identical to the plain scaffold.
	checkGoldenFiles(t, pkgDir, filepath.Join("testdata", "golden", "package", "plain"), []goldenFile{
		{generated: "schema.cue", golden: "schema.cue"},
		{generated: "doc.go", golden: "doc.go"},
	})
}

// ledgerFiles are the ledgers -apply rewrites, as (repo-relative, golden name).
var ledgerFiles = []goldenFile{
	{generated: "TRACKING.md", golden: "TRACKING.md"},
	{generated: filepath.Join("docs", "compat.md"), golden: "compat.md"},
	{generated: "CHANGELOG.md", golden: "CHANGELOG.md"},
	{generated: filepath.Join("docs", "practices.md"), golden: "practices.md"},
	{generated: "README.md", golden: "README.md"},
}

// TestPackageApplyLedgers pins the five ledgers after -apply, then proves
// idempotence: a second -apply (after the package dir is cleared, since the
// scaffold itself refuses to re-run) reports every line already present and
// the README count already correct, and rewrites nothing.
func TestPackageApplyLedgers(t *testing.T) {
	repo := copyFixtureRepo(t)
	code, stdout, stderr := runNew(t, scaffoldArgs(repo, "-apply")...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, want := range []string{"[applied]\n- [ ] `widget-gizmo`", "[applied]\n| `foundations/alpha/widget-gizmo` | experimental", "[applied]\n- **`foundations/alpha/widget-gizmo`**", "§1 table [applied]", "[applied]\n**0 Go packages → **1 Go packages"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("apply report missing %q:\n%s", want, stdout)
		}
	}
	checkGoldenFiles(t, repo, filepath.Join("testdata", "golden", "package", "apply"), ledgerFiles)

	before := snapshot(t, repo, ledgerFiles)
	if err := os.RemoveAll(filepath.Join(repo, "foundations")); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = runNew(t, scaffoldArgs(repo, "-apply")...)
	if code != 0 {
		t.Fatalf("second apply: exit %d: %s", code, stderr)
	}
	if n := strings.Count(stdout, "[already present]"); n != 4 {
		t.Errorf("second apply: %d ledgers reported already present, want 4:\n%s", n, stdout)
	}
	if !strings.Contains(stdout, "[already correct]") {
		t.Errorf("second apply: README count not reported already correct:\n%s", stdout)
	}
	after := snapshot(t, repo, ledgerFiles)
	for name := range before {
		if !bytes.Equal(before[name], after[name]) {
			t.Errorf("second apply rewrote %s", name)
		}
	}
}

// snapshot reads the ledgers by golden name.
func snapshot(t *testing.T, repo string, files []goldenFile) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(repo, f.generated))
		if err != nil {
			t.Fatal(err)
		}
		out[f.golden] = data
	}
	return out
}

// TestPackageRefusesOverwrite proves the idempotence rule: a second run on
// an existing scaffold exits 1 naming the files, changes none of them, and
// — even with -apply — touches no ledger.
func TestPackageRefusesOverwrite(t *testing.T) {
	repo := copyFixtureRepo(t)
	if code, _, stderr := runNew(t, scaffoldArgs(repo)...); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	pkgDir := filepath.Join(repo, "foundations", "alpha", "widget-gizmo")
	// Make one file distinguishable so an overwrite would show.
	if err := os.WriteFile(filepath.Join(pkgDir, "doc.go"), []byte("// edited\npackage widgetgizmo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, repo, ledgerFiles)
	code, _, stderr := runNew(t, scaffoldArgs(repo, "-apply")...)
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "refusing to overwrite") || !strings.Contains(stderr, "foundations/alpha/widget-gizmo/doc.go") {
		t.Errorf("stderr does not name the refusal: %s", stderr)
	}
	got, _ := os.ReadFile(filepath.Join(pkgDir, "doc.go"))
	if string(got) != "// edited\npackage widgetgizmo\n" {
		t.Error("existing doc.go was overwritten")
	}
	after := snapshot(t, repo, ledgerFiles)
	for name := range before {
		if !bytes.Equal(before[name], after[name]) {
			t.Errorf("refused run still rewrote %s", name)
		}
	}
}

// TestPackageApplyMissingAnchorWritesNothing: with -apply, a ledger anchor
// that cannot be found (here a practices section that does not exist) is
// refused BEFORE any file is written — no package dir, no ledger change.
func TestPackageApplyMissingAnchorWritesNothing(t *testing.T) {
	repo := copyFixtureRepo(t)
	before := snapshot(t, repo, ledgerFiles)
	code, _, stderr := runNew(t, "-package", "foundations/alpha/widget-gizmo", "-repo", repo, "-practice", "9", "-apply")
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "docs/practices.md") || !strings.Contains(stderr, "nothing written") {
		t.Errorf("stderr does not explain the refusal: %s", stderr)
	}
	if _, err := os.Stat(filepath.Join(repo, "foundations")); !os.IsNotExist(err) {
		t.Error("package dir was created despite the refusal")
	}
	after := snapshot(t, repo, ledgerFiles)
	for name := range before {
		if !bytes.Equal(before[name], after[name]) {
			t.Errorf("refused apply rewrote %s", name)
		}
	}

	// In print mode the same gap is reported, not fatal.
	code, stdout, stderr := runNew(t, "-package", "foundations/alpha/widget-gizmo", "-repo", repo, "-practice", "9")
	if code != 0 {
		t.Fatalf("print mode: exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "docs/practices.md — no section for this package in docs/practices.md — add by hand [anchor not found]") {
		t.Errorf("print mode did not report the missing anchor:\n%s", stdout)
	}
}

// TestPackageTrackingAnchors pins TRACKING.md section resolution against the
// fixture's real-shaped headers: a category is matched as a whole title token
// ("data" must not match "core data-access layer"), multi-token titles carry
// several categories, and non-foundation roots resolve to their "## N." header.
func TestPackageTrackingAnchors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "fixture-repo", "TRACKING.md"))
	if err != nil {
		t.Fatal(err)
	}
	lines := splitLines(string(data))
	tests := []struct {
		pkg  string
		want string // "" = not found
	}{
		{"foundations/alpha/widget-gizmo", "### 1.1 alpha ✅"},
		{"foundations/data/zeta", "### 1.2 data"},
		{"foundations/evidence/omega", "### 1.3 evidence — core data-access layer"},
		{"foundations/cache/omega", "### 1.4 cost · cache · persistence · prompt ✅"},
		{"foundations/prompt/omega", "### 1.4 cost · cache · persistence · prompt ✅"},
		{"foundations/reasoning/theta", "### 1.5 provenance + reasoning ✅"},
		{"kits/delta-kit", "## 2. Kits (compositions)"},
		{"runtimes/rho", "## 3. Runtimes (1 active)"},
		{"foundations/nope/x", ""},
		{"surfaces/sigma", ""},
	}
	for _, tc := range tests {
		t.Run(tc.pkg, func(t *testing.T) {
			d, err := parsePackagePath(tc.pkg)
			if err != nil {
				t.Fatal(err)
			}
			_, _, header, ok := trackingSection(lines, d.Root, d.Category)
			if ok != (tc.want != "") || header != tc.want {
				t.Errorf("trackingSection = %q (found=%v), want %q", header, ok, tc.want)
			}
		})
	}
}

// TestPackageApplyEmptySection covers insertion into a TRACKING section that
// has no rows yet and into the Kits section, whose rows end at a "### Wave"
// sub-header: the row lands after the last existing row, never inside the
// sub-section, and compat rows stay sorted by path.
func TestPackageApplyEmptySection(t *testing.T) {
	repo := copyFixtureRepo(t)
	for _, pkg := range []string{"foundations/reasoning/theta", "kits/delta-kit"} {
		if code, _, stderr := runNew(t, "-package", pkg, "-repo", repo, "-apply"); code != 0 {
			t.Fatalf("%s: exit %d: %s", pkg, code, stderr)
		}
	}
	tracking, _ := os.ReadFile(filepath.Join(repo, "TRACKING.md"))
	for _, want := range []string{
		"### 1.5 provenance + reasoning ✅\n\n- [ ] `theta` (`foundations/reasoning/theta`)",
		"- [x] `beta` (`kits/beta`) — existing row\n- [ ] `delta-kit` (`kits/delta-kit`)",
	} {
		if !strings.Contains(string(tracking), want) {
			t.Errorf("TRACKING.md missing %q:\n%s", want, tracking)
		}
	}
	compat, _ := os.ReadFile(filepath.Join(repo, "docs", "compat.md"))
	if !strings.Contains(string(compat), "| `foundations/data/store` | stable | existing |\n| `foundations/reasoning/theta` | experimental |") ||
		!strings.Contains(string(compat), "| `kits/beta` | stable | existing |\n| `kits/delta-kit` | experimental |") {
		t.Errorf("compat rows not in sorted position:\n%s", compat)
	}
}

// TestPackageUsageErrors pins the flag grammar.
func TestPackageUsageErrors(t *testing.T) {
	repo := copyFixtureRepo(t)
	tests := []struct {
		name string
		args []string
	}{
		{"two-segment foundations path", []string{"-package", "foundations/widget", "-repo", repo}},
		{"four segments", []string{"-package", "foundations/a/b/c", "-repo", repo}},
		{"cmd root", []string{"-package", "cmd/tool", "-repo", repo}},
		{"uppercase segment", []string{"-package", "foundations/alpha/Widget", "-repo", repo}},
		{"underscore segment", []string{"-package", "foundations/alpha/widget_gizmo", "-repo", repo}},
		{"trailing slash", []string{"-package", "foundations/alpha/widget/", "-repo", repo}},
		{"lowercase interface", []string{"-package", "foundations/alpha/widget", "-repo", repo, "-interface", "stamper"}},
		{"negative practice", []string{"-package", "foundations/alpha/widget", "-repo", repo, "-practice", "-1"}},
		{"mixed with -name", []string{"-package", "foundations/alpha/widget", "-repo", repo, "-name", "x"}},
		{"mixed with -minimal", []string{"-package", "foundations/alpha/widget", "-repo", repo, "-minimal"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := runNew(t, tc.args...)
			if code != 2 {
				t.Errorf("exit %d, want 2; stderr: %s", code, stderr)
			}
			if _, err := os.Stat(filepath.Join(repo, "foundations")); !os.IsNotExist(err) {
				t.Error("usage error still wrote files")
			}
		})
	}
	t.Run("repo without go.mod", func(t *testing.T) {
		code, _, stderr := runNew(t, "-package", "foundations/alpha/widget", "-repo", t.TempDir())
		if code != 1 || !strings.Contains(stderr, "go.mod") {
			t.Errorf("exit %d, want 1 naming go.mod; stderr: %s", code, stderr)
		}
	})
}

// TestPackageScaffoldPassesConstitutionChecks runs, over the scaffold, the
// constitution-lint checks that apply to a single package — gofmt-clean
// (fmt-check), a package doc that states its concurrency contract
// (concurrency-contract) and lives in doc.go only (so the catalog synopsis
// is the intended one), every exported identifier documented name-first
// (doc-coverage), schema.cue present (schema-presence), no bare time.Now()
// (I12) — and, when the cue binary is on PATH, `cue vet` over the schema
// and its testdata exactly as `task cue-vet` does.
func TestPackageScaffoldPassesConstitutionChecks(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra []string
	}{
		{"plain", nil},
		{"interface", []string{"-interface", "Stamper"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := copyFixtureRepo(t)
			if code, _, stderr := runNew(t, scaffoldArgs(repo, tc.extra...)...); code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			pkgDir := filepath.Join(repo, "foundations", "alpha", "widget-gizmo")
			checkScaffoldConstitution(t, pkgDir)
		})
	}
}

// checkScaffoldConstitution is the per-package subset of constitution-lint.
func checkScaffoldConstitution(t *testing.T, pkgDir string) {
	t.Helper()
	for _, must := range []string{"schema.cue", filepath.Join("testdata", "valid.cue")} {
		if _, err := os.Stat(filepath.Join(pkgDir, must)); err != nil {
			t.Errorf("schema-presence: %s missing", must)
		}
	}
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	total, documented := 0, 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(pkgDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if formatted, err := format.Source(src); err != nil {
			t.Errorf("fmt-check: %s: %v", name, err)
		} else if !bytes.Equal(formatted, src) {
			t.Errorf("fmt-check: %s is not gofmt-clean", name)
		}
		f, err := parser.ParseFile(fset, name, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if !strings.HasSuffix(name, "_test.go") && strings.Contains(string(src), "time.Now()") {
			t.Errorf("I12: %s calls bare time.Now()", name)
		}
		if name == "doc.go" {
			if f.Doc == nil {
				t.Error("concurrency-contract: doc.go has no package doc")
			} else if l := strings.ToLower(f.Doc.Text()); !strings.Contains(l, "concurren") {
				t.Error("concurrency-contract: package doc does not state one")
			} else if !strings.Contains(l, "non-guarantees") || !strings.Contains(l, "verified against") {
				t.Error("doc.go lacks the non-guarantees or Verified-against line")
			}
		} else if f.Doc != nil {
			t.Errorf("%s carries a package doc; only doc.go may (the catalog takes the first in filename order)", name)
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		tot, doc := docCoverage(f)
		total += tot
		documented += doc
		if tot != doc {
			t.Errorf("doc-coverage: %s: %d/%d exported identifiers documented name-first", name, doc, tot)
		}
	}
	if total == 0 {
		t.Error("doc-coverage: no exported identifiers found — the scaffold is empty")
	}

	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Log("cue not installed — cue vet skipped (task cue-vet skips the same way)")
		return
	}
	for _, target := range []string{"./...", "./testdata/"} {
		cmd := exec.Command(cue, "vet", target)
		cmd.Dir = pkgDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("cue vet %s failed:\n%s", target, out)
		}
	}
}

// docCoverage counts exported package-level identifiers (funcs, methods on
// exported types, types, consts, vars) and how many have a doc comment
// starting with their own name — constitution-lint's doc-coverage rule.
func docCoverage(f *ast.File) (total, documented int) {
	valid := func(doc *ast.CommentGroup, name string) bool {
		return doc != nil && strings.HasPrefix(strings.TrimSpace(doc.Text()), name)
	}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil {
				recv := d.Recv.List[0].Type
				if star, ok := recv.(*ast.StarExpr); ok {
					recv = star.X
				}
				if id, ok := recv.(*ast.Ident); !ok || !ast.IsExported(id.Name) {
					continue
				}
			}
			if !ast.IsExported(d.Name.Name) {
				continue
			}
			total++
			if valid(d.Doc, d.Name.Name) {
				documented++
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if !ast.IsExported(s.Name.Name) {
						continue
					}
					total++
					doc := s.Doc
					if doc == nil && len(d.Specs) == 1 {
						doc = d.Doc
					}
					if valid(doc, s.Name.Name) {
						documented++
					}
				case *ast.ValueSpec:
					doc := s.Doc
					if doc == nil && len(d.Specs) == 1 {
						doc = d.Doc
					}
					for _, id := range s.Names {
						if id.Name == "_" || !ast.IsExported(id.Name) {
							continue
						}
						total++
						if valid(doc, id.Name) {
							documented++
						}
					}
				}
			}
		}
	}
	return total, documented
}

// TestPackageScaffoldIntegrationGoTest scaffolds into the fixture module and
// runs `go vet` and `go test -race` on the result: the stub must be a real,
// green package on day one. Shells to the go tool (skipped in -short), with
// GOWORK=off and GOTOOLCHAIN=local; no network — the scaffold imports only
// the standard library.
func TestPackageScaffoldIntegrationGoTest(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: skipping go test of the scaffold in short mode")
	}
	repo := copyFixtureRepo(t)
	if code, _, stderr := runNew(t, scaffoldArgs(repo, "-interface", "Stamper")...); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	env := append(filterEnv(os.Environ(), "GOWORK", "GOFLAGS"), "GOWORK=off", "GOTOOLCHAIN=local")
	for _, args := range [][]string{{"vet", "./..."}, {"test", "-race", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = repo
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %s failed:\n%s", strings.Join(args, " "), out)
		}
	}
}

// fragmentsPointer is the static line a fragments repo keeps in [Unreleased].
const fragmentsPointer = "Unreleased changes wait as fragments under docs/changelog/unreleased/; the release ritual folds them here.\n"

// TestPackageApplyWritesChangeNoteFragment: in a fragments repo kit-new -apply
// writes a change-note fragment and leaves CHANGELOG.md alone; a second apply
// reports it already present.
func TestPackageApplyWritesChangeNoteFragment(t *testing.T) {
	repo := copyFixtureRepo(t)
	cl := "# Changelog\n\n## [Unreleased]\n\n" + fragmentsPointer + "\n## [0.1.0] — earlier\n\n### Added\n\n- old.\n"
	if err := os.WriteFile(filepath.Join(repo, "CHANGELOG.md"), []byte(cl), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runNew(t, scaffoldArgs(repo, "-apply")...); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	frag, err := os.ReadFile(filepath.Join(repo, "docs", "changelog", "unreleased", "kit-new-widget-gizmo.md"))
	if err != nil {
		t.Fatalf("no fragment written: %v", err)
	}
	if !strings.HasPrefix(string(frag), "### Added\n\n- **`foundations/alpha/widget-gizmo`**") || strings.Count(string(frag), "###") != 1 {
		t.Errorf("fragment shape wrong:\n%s", frag)
	}
	if got, _ := os.ReadFile(filepath.Join(repo, "CHANGELOG.md")); string(got) != cl {
		t.Errorf("CHANGELOG.md was edited in a fragments repo:\n%s", got)
	}
	if err := os.RemoveAll(filepath.Join(repo, "foundations")); err != nil {
		t.Fatal(err)
	}
	_, stdout, _ := runNew(t, scaffoldArgs(repo, "-apply")...)
	if n := strings.Count(stdout, "[already present]"); n != 4 {
		t.Errorf("second apply: %d already present, want 4:\n%s", n, stdout)
	}
}

// TestPackageApplyPlainRepoKeepsUnreleasedLine: no pointer line, no fragment.
func TestPackageApplyPlainRepoKeepsUnreleasedLine(t *testing.T) {
	repo := copyFixtureRepo(t)
	if code, _, stderr := runNew(t, scaffoldArgs(repo, "-apply")...); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(repo, "docs", "changelog")); err == nil {
		t.Error("a plain repo got a fragments directory")
	}
	got, _ := os.ReadFile(filepath.Join(repo, "CHANGELOG.md"))
	if !strings.Contains(string(got), "### Added\n\n- **`foundations/alpha/widget-gizmo`**") {
		t.Errorf("plain repo lost the [Unreleased] > Added line:\n%s", got)
	}
}

// TestFragmentsDirMatchesReleaseNotes: kit-new reads the pointer line the way
// release-notes does, so a dotted directory name is kept whole.
func TestFragmentsDirMatchesReleaseNotes(t *testing.T) {
	for pointer, want := range map[string]string{
		fragmentsPointer: "docs/changelog/unreleased",
		"Unreleased changes wait as fragments under docs/v1.2/unreleased/; the release ritual folds them here.\n": "docs/v1.2/unreleased",
		"Unreleased changes wait as fragments under changes\n":                                                    "changes",
		"Unreleased changes wait in CHANGELOG lines.\n":                                                           "",
	} {
		repo := t.TempDir()
		cl := "# Changelog\n\n## [Unreleased]\n\n" + pointer + "\n## [0.1.0] — earlier\n"
		if err := os.WriteFile(filepath.Join(repo, "CHANGELOG.md"), []byte(cl), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := fragmentsDir(repo); got != want {
			t.Errorf("pointer %q: fragmentsDir = %q, want %q", pointer, got, want)
		}
	}
}
