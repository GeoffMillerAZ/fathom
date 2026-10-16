package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// selectRepo is a git work tree holding a Go module, for the tests that
// read --since through git (TQ-29, TQ-30).
type selectRepo struct {
	t    *testing.T
	root string
}

func newSelectRepo(t *testing.T, files map[string]string) *selectRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	r := &selectRepo{t: t, root: t.TempDir()}
	for rel, body := range files {
		r.write(rel, body)
	}
	r.git("init", "-q")
	r.commit()
	return r
}

func (r *selectRepo) write(rel, body string) {
	r.t.Helper()
	p := filepath.Join(r.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *selectRepo) git(args ...string) {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = r.root
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func (r *selectRepo) commit() {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-q", "-m", "c")
}

// selectSafe ingests the working tree and returns the --safe selection's
// tests (bare names) for changed files since HEAD.
func (r *selectRepo) selectSafe(service string, changed ...string) []string {
	r.t.Helper()
	storePath := filepath.Join(r.t.TempDir(), "s.sqlite")
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", r.root, "--store", storePath}, &ing, &ing); code != 0 {
		r.t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	old := testsSelectRoot
	testsSelectRoot = r.root
	r.t.Cleanup(func() { testsSelectRoot = old })
	setEconomicsLedger(r.t, filepath.Join(r.t.TempDir(), "econ.jsonl"))
	var out, errBuf bytes.Buffer
	args := []string{"--since", "HEAD", "--service", service, "--store", storePath, "--safe", "--json"}
	if code := runTestsSelect(args, fakeGitRunner(changed), &out, &errBuf); code != 0 {
		r.t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	var payload struct {
		Selection struct {
			Tests []string `json:"tests"`
		} `json:"selection"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		r.t.Fatalf("unmarshal: %v\n%s", err, out.String())
	}
	var names []string
	for _, k := range payload.Selection.Tests {
		names = append(names, k[strings.LastIndex(k, ".")+1:])
	}
	sort.Strings(names)
	return names
}

const bodyLibV1 = `package lib

func Stable() int { return 1 }

func Other() int { return 2 }
`

// TestTestsSelectSafeIgnoresMovedSymbol is TQ-29: lines inserted above an
// unchanged function leave it out of the selection; a one-byte edit of its
// body puts it in. A symbol new since --since counts as changed.
func TestTestsSelectSafeIgnoresMovedSymbol(t *testing.T) {
	r := newSelectRepo(t, map[string]string{
		"go.mod":     "module example.com/body\n\ngo 1.22\n",
		"lib/lib.go": bodyLibV1,
		"lib/lib_test.go": `package lib

import "testing"

func TestStable(t *testing.T) { _ = Stable() }

func TestOther(t *testing.T) { _ = Other() }

func TestAdded(t *testing.T) { _ = Added() }
`,
		"lib/added.go": "package lib\n\nfunc Added() int { return 0 }\n",
	})

	// Lines inserted above both functions: nothing they declare changed.
	r.write("lib/lib.go", "package lib\n\n// A comment and a constant above.\nconst k = 7\n\n"+strings.TrimPrefix(bodyLibV1, "package lib\n\n"))
	if got := r.selectSafe("body-moved", "lib/lib.go"); len(got) != 0 {
		t.Fatalf("moved-only change selected %v, want none", got)
	}

	// One byte of Stable's body.
	r.write("lib/lib.go", strings.Replace(bodyLibV1, "return 1", "return 3", 1))
	if got := r.selectSafe("body-edit", "lib/lib.go"); strings.Join(got, ",") != "TestStable" {
		t.Fatalf("one-byte body edit selected %v, want [TestStable]", got)
	}

	// A file absent at --since: everything it declares is new, so changed.
	r.write("lib/lib.go", bodyLibV1)
	r.git("rm", "-q", "--cached", "lib/added.go")
	r.git("commit", "-q", "-m", "drop added.go from the index")
	if got := r.selectSafe("body-new", "lib/added.go"); strings.Join(got, ",") != "TestAdded" {
		t.Fatalf("new file selected %v, want [TestAdded]", got)
	}
}

// TestTestsSelectSafeDispatchNeedsConstruction is TQ-30: a new error type
// built only in package p selects only the tests that reach p's
// constructor, though every test's err.Error() call dispatches to it; an
// existing type's Error body edit still selects the tests that build it.
func TestTestsSelectSafeDispatchNeedsConstruction(t *testing.T) {
	files := map[string]string{
		"go.mod":     "module example.com/rta\n\ngo 1.22\n",
		"use/use.go": "package use\n\nfunc Describe(err error) string { return err.Error() }\n",
		"p/p.go":     "package p\n\ntype PErr struct{}\n\nfunc (*PErr) Error() string { return \"p\" }\n\nfunc NewP() error { return &PErr{} }\n",
		"q/q.go":     "package q\n\ntype QErr struct{}\n\nfunc (*QErr) Error() string { return \"q\" }\n\nfunc NewQ() error { return &QErr{} }\n",
		"p/p_test.go": `package p_test

import (
	"testing"

	"example.com/rta/p"
	"example.com/rta/use"
)

func TestP(t *testing.T) { _ = use.Describe(p.NewP()) }
`,
		"q/q_test.go": `package q_test

import (
	"testing"

	"example.com/rta/q"
	"example.com/rta/use"
)

func TestQ(t *testing.T) { _ = use.Describe(q.NewQ()) }
`,
	}
	r := newSelectRepo(t, files)

	r.write("p/n.go", "package p\n\ntype NErr struct{}\n\nfunc (*NErr) Error() string { return \"n\" }\n\nfunc NewN() error { return &NErr{} }\n")
	r.write("p/n_test.go", `package p_test

import (
	"testing"

	"example.com/rta/p"
	"example.com/rta/use"
)

func TestN(t *testing.T) { _ = use.Describe(p.NewN()) }
`)
	if got := r.selectSafe("rta-new", "p/n.go", "p/n_test.go"); strings.Join(got, ",") != "TestN" {
		t.Fatalf("a new error type selected %v, want only [TestN] (TestP and TestQ never build NErr)", got)
	}

	r.commit()
	r.write("q/q.go", strings.Replace(files["q/q.go"], `return "q"`, `return "Q"`, 1))
	if got := r.selectSafe("rta-edit", "q/q.go"); strings.Join(got, ",") != "TestQ" {
		t.Fatalf("an Error body edit selected %v, want [TestQ] (the test that builds QErr)", got)
	}
}

// TestGoDeclUnits pins the text rule: a moved declaration reads the same, a
// const in an iota group reads its whole group, and a name declared twice
// (two init functions) reads as unknown.
func TestGoDeclUnits(t *testing.T) {
	a := "package x\n\nconst (\n\tA = iota\n\tB\n)\n\nfunc init() {}\nfunc init() {}\n\nfunc (s *S[T]) M() int { return 1 }\n"
	b := "package x\n\n// moved\nvar extra = 1\n\nconst (\n\tA = iota\n\tN\n\tB\n)\n\nfunc init() {}\nfunc init() {}\n\nfunc (s *S[T]) M() int { return 1 }\n"
	da, err := goDeclUnits([]byte(a))
	if err != nil {
		t.Fatal(err)
	}
	db, err := goDeclUnits([]byte(b))
	if err != nil {
		t.Fatal(err)
	}
	ua, ub := da.units, db.units
	if !bytes.Equal(da.header, db.header) {
		t.Errorf("headers differ: %q vs %q", da.header, db.header)
	}
	if !bytes.Equal(ua["S.M"], ub["S.M"]) || ua["S.M"] == nil {
		t.Errorf("moved method S.M must read the same: %q vs %q", ua["S.M"], ub["S.M"])
	}
	if bytes.Equal(ua["B"], ub["B"]) {
		t.Errorf("B's iota value changed; its text must too: %q", ub["B"])
	}
	if v, ok := ua["init"]; !ok || v != nil {
		t.Errorf("two init functions must read as unknown, got %q (present %v)", v, ok)
	}
	if _, err := goDeclUnits([]byte("package x\nfunc {")); err == nil {
		t.Error("a parse error must be reported")
	}
	imp := func(src string) map[string]bool {
		d, err := goDeclUnits([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		return d.imports
	}
	v2 := imp("package x\nimport \"gopkg.in/yaml.v2\"\n")
	v3 := imp("package x\nimport \"gopkg.in/yaml.v3\"\n")
	both := imp("package x\nimport (\n\"gopkg.in/yaml.v2\"\n\"strings\"\n)\n")
	if !importsRebound(v2, v3) {
		t.Error("an import swapped for another can rebind a name: file level")
	}
	if importsRebound(v2, both) || importsRebound(both, v2) {
		t.Error("an import only added or only removed rebinds nothing")
	}
	ctx := context.Background()
	if _, _, ok := goUnitsBothSides(ctx, t.TempDir(), "HEAD", "README.md"); ok {
		t.Error("a non-Go file must read at file level")
	}
}
