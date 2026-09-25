package conserve

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// inputTestRepo is a throwaway git work tree for NewInput's git-integration
// tests — mirrors cmd/fathom/selfskip_test.go's selfSkipGitRepo exactly
// (this package cannot import cmd/fathom's test helpers), never the real
// project repo.
type inputTestRepo struct {
	t    *testing.T
	root string
}

func newInputTestRepo(t *testing.T) *inputTestRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	r := &inputTestRepo{t: t, root: t.TempDir()}
	r.run("init", "-q")
	r.run("config", "commit.gpgsign", "false")
	r.run("config", "user.name", "Test")
	r.run("config", "user.email", "test@example.com")
	return r
}

func (r *inputTestRepo) run(args ...string) {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.root
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out.String())
	}
}

func (r *inputTestRepo) write(rel, content string) {
	r.t.Helper()
	full := filepath.Join(r.root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *inputTestRepo) commitAll(msg string) {
	r.t.Helper()
	r.run("add", "-A")
	r.run("commit", "-q", "-m", msg)
}

func (r *inputTestRepo) headSHA() string {
	r.t.Helper()
	out, err := exec.Command("git", "-C", r.root, "rev-parse", "HEAD").Output()
	if err != nil {
		r.t.Fatal(err)
	}
	return string(bytes.TrimSpace(out))
}

func TestNewInput_ResolvesBaseChangedFilesAndDeclarations(t *testing.T) {
	repo := newInputTestRepo(t)
	repo.write("a.go", "package a\n")
	repo.write("docs/plans/p.md", "- [ ] Row {id: R-1, retires: \"api:foo.Old\"}\n")
	repo.commitAll("base commit")
	base := repo.headSHA()

	// A tracked change, plus an untracked new file — NewInput must see both.
	repo.write("a.go", "package a\n\nfunc B() {}\n")
	repo.write("b.go", "package a\n\nfunc NewOne() {}\n") // untracked, never added
	repo.write("docs/plans/p.md",
		"- [x] Row {id: R-1, retires: \"api:foo.Old\", surface_change: \"json:foo.New.tag\"}\n")

	in, err := NewInput(context.Background(), repo.root, base, nil, "svc", []string{"docs/plans/p.md"})
	if err != nil {
		t.Fatalf("NewInput: %v", err)
	}
	if in.Base != base {
		t.Errorf("Base = %q, want %q", in.Base, base)
	}
	if in.Head != HeadWorktree {
		t.Errorf("Head = %q, want %q", in.Head, HeadWorktree)
	}
	if in.Root != repo.root {
		t.Errorf("Root = %q, want %q", in.Root, repo.root)
	}

	wantChanged := []string{"a.go", "b.go", "docs/plans/p.md"}
	gotChanged := append([]string(nil), in.ChangedFiles...)
	sort.Strings(gotChanged)
	if len(gotChanged) != len(wantChanged) {
		t.Fatalf("ChangedFiles = %v, want %v", gotChanged, wantChanged)
	}
	for i, w := range wantChanged {
		if gotChanged[i] != w {
			t.Errorf("ChangedFiles[%d] = %q, want %q (full: %v)", i, gotChanged[i], w, gotChanged)
		}
	}

	// BaseFile: a.go existed at base with the ORIGINAL content.
	baseContent, ok, err := in.BaseFile("a.go")
	if err != nil || !ok {
		t.Fatalf("BaseFile(a.go) = ok:%v err:%v", ok, err)
	}
	if string(baseContent) != "package a\n" {
		t.Errorf("BaseFile(a.go) = %q, want the base version", baseContent)
	}
	// BaseFile: b.go did not exist at base at all.
	if _, ok, err := in.BaseFile("b.go"); ok || err != nil {
		t.Errorf("BaseFile(b.go) = ok:%v err:%v, want ok:false err:nil (absent at base)", ok, err)
	}

	// HeadFile: b.go exists in the worktree even though it was never added.
	headContent, ok, err := in.HeadFile("b.go")
	if err != nil || !ok || string(headContent) == "" {
		t.Fatalf("HeadFile(b.go) = %q ok:%v err:%v", headContent, ok, err)
	}

	// Declarations: retires "api:foo.Old" was present at base -> prior;
	// surface_change "json:foo.New.tag" only appears at head -> self.
	if got := in.Declarations.Declared("api:foo.Old"); got != DeclaredPrior {
		t.Errorf("Declared(api:foo.Old) = %q, want prior", got)
	}
	if got := in.Declarations.Declared("json:foo.New.tag"); got != DeclaredSelf {
		t.Errorf("Declared(json:foo.New.tag) = %q, want self", got)
	}
}

// TestNewInput_RetiredCombinesBaseAndHeadUnlikeDeclarations pins Input.
// Retired's own contract (ADR-044 §3.3, resurrection honesty): a
// retires: key gained only at HEAD still counts as a claim here — unlike
// Declarations.Declared, which would call the exact same key "self" and
// exclude it from excusing a loss.
func TestNewInput_RetiredCombinesBaseAndHeadUnlikeDeclarations(t *testing.T) {
	repo := newInputTestRepo(t)
	repo.write("a.go", "package a\n")
	repo.write("docs/plans/p.md", "- [ ] Row {id: R-1, retires: \"api:foo.Old\"}\n")
	repo.commitAll("base commit")
	base := repo.headSHA()

	repo.write("docs/plans/p.md",
		"- [x] Row {id: R-1, retires: \"api:foo.Old\"}\n- [x] Row2 {id: R-2, gone: import example.com/x/legacy}\n")

	in, err := NewInput(context.Background(), repo.root, base, nil, "svc", []string{"docs/plans/p.md"})
	if err != nil {
		t.Fatalf("NewInput: %v", err)
	}
	if !in.Retired.ClaimsKey("api:foo.Old") {
		t.Error("Retired.ClaimsKey(api:foo.Old) = false, want true (declared at base)")
	}
	if !in.Retired.ClaimsImport("example.com/x/legacy") {
		t.Error("Retired.ClaimsImport(example.com/x/legacy) = false, want true — a gone: import row added ONLY at head still counts as a claim here")
	}
	if in.Retired.ClaimsImport("example.com/x/something-else") {
		t.Error("Retired.ClaimsImport matched an unrelated import path")
	}
}

func TestNewInput_BadRefIsAnError(t *testing.T) {
	repo := newInputTestRepo(t)
	repo.write("a.go", "package a\n")
	repo.commitAll("only commit")

	if _, err := NewInput(context.Background(), repo.root, "not-a-real-ref", nil, "svc", nil); err == nil {
		t.Fatal("want an error resolving a nonexistent ref, got nil")
	}
}

// TestNewInput_RenamedFileDoesNotMisreadAsAdded pins that a pure
// same-directory `git mv`, with no other edit, never turns the moved
// file's functions into "added" residue items: --no-renames in
// changedFilesSinceBase guarantees BOTH the old and new paths appear in
// ChangedFiles (so ChangedFunctions can pool the old path's base content),
// whatever the caller's own diff.renames git config says.
func TestNewInput_RenamedFileDoesNotMisreadAsAdded(t *testing.T) {
	repo := newInputTestRepo(t)
	repo.write("pkg/a.go", "package pkg\n\nfunc Keep() {}\n\nfunc Other() {}\n")
	repo.commitAll("base commit")
	base := repo.headSHA()

	repo.run("mv", "pkg/a.go", "pkg/b.go")
	repo.commitAll("pure rename")

	in, err := NewInput(context.Background(), repo.root, base, nil, "svc", nil)
	if err != nil {
		t.Fatalf("NewInput: %v", err)
	}

	var sawOld, sawNew bool
	for _, f := range in.ChangedFiles {
		switch f {
		case "pkg/a.go":
			sawOld = true
		case "pkg/b.go":
			sawNew = true
		}
	}
	if !sawOld || !sawNew {
		t.Fatalf("ChangedFiles = %v, want both pkg/a.go and pkg/b.go (--no-renames)", in.ChangedFiles)
	}

	funcs, unanalyzed, missing := ChangedFunctions(in)
	if len(missing) != 0 || len(unanalyzed) != 0 {
		t.Fatalf("missing=%v unanalyzed=%v, want none", missing, unanalyzed)
	}
	if len(funcs) != 0 {
		t.Errorf("funcs = %+v, want none (a pure rename is neither added nor modified)", funcs)
	}
}

// TestNewInput_SubdirectoryRootChangedFilesAreRelative pins that a service
// rooted at a subdirectory of the git repo gets ChangedFiles relative to
// THAT root — matching every index FilePath (ingested relative to the
// service's own root) — never relative to the git toplevel.
func TestNewInput_SubdirectoryRootChangedFilesAreRelative(t *testing.T) {
	repo := newInputTestRepo(t)
	repo.write("svc/pkg/a.go", "package pkg\n\nfunc F() int { return 1 }\n")
	repo.write("other/x.txt", "x\n")
	repo.commitAll("base commit")
	base := repo.headSHA()
	repo.write("svc/pkg/a.go", "package pkg\n\nfunc F() int { return 2 }\n")

	svcRoot := repo.root + "/svc"
	in, err := NewInput(context.Background(), svcRoot, base, nil, "svc", nil)
	if err != nil {
		t.Fatalf("NewInput: %v", err)
	}
	if len(in.ChangedFiles) != 1 || in.ChangedFiles[0] != "pkg/a.go" {
		t.Fatalf("ChangedFiles = %v, want exactly [pkg/a.go] (relative to the service root, not the git toplevel)", in.ChangedFiles)
	}

	funcs, _, missing := ChangedFunctions(in)
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	if len(funcs) != 1 || funcs[0].Symbol != "F" || funcs[0].Change != "modified" {
		t.Errorf("funcs = %+v, want exactly [{F modified}]", funcs)
	}
}

// TestGitShowFile_UnreadableBlobIsAnError pins I38: an unreadable base
// object (a permission error, a corrupt loose object — never the ordinary
// "this path did not exist at base" shape) must come back as a real
// error, never silently read as "did not exist".
func TestGitShowFile_UnreadableBlobIsAnError(t *testing.T) {
	repo := newInputTestRepo(t)
	repo.write("pkg/a.go", "package pkg\n\nfunc F() int { return 1 }\n")
	repo.commitAll("base commit")
	base := repo.headSHA()

	blob, err := exec.Command("git", "-C", repo.root, "rev-parse", base+":pkg/a.go").Output()
	if err != nil {
		t.Fatalf("rev-parse blob: %v", err)
	}
	sha := strings.TrimSpace(string(blob))
	objPath := filepath.Join(repo.root, ".git", "objects", sha[:2], sha[2:])
	if err := os.Chmod(objPath, 0o000); err != nil {
		t.Fatalf("chmod 000 %s: %v", objPath, err)
	}
	t.Cleanup(func() { _ = os.Chmod(objPath, 0o644) })

	// Control: an ordinary absent-at-base path still reads ok=false, err=nil.
	if _, ok, err := gitShowFile(repo.root, base, "pkg/never-existed.go"); ok || err != nil {
		t.Errorf("gitShowFile(never-existed) = ok:%v err:%v, want ok:false err:nil", ok, err)
	}

	content, ok, err := gitShowFile(repo.root, base, "pkg/a.go")
	if err == nil {
		t.Fatalf("gitShowFile(unreadable blob) = content:%q ok:%v err:nil, want a non-nil error", content, ok)
	}
	if ok {
		t.Errorf("gitShowFile(unreadable blob) ok = true, want false")
	}
}

func TestNewInput_PlanAbsentAtBaseIsEveryHeadDeclarationSelf(t *testing.T) {
	repo := newInputTestRepo(t)
	repo.write("a.go", "package a\n")
	repo.commitAll("base commit, no plan file yet")
	base := repo.headSHA()

	repo.write("docs/plans/p.md", "- [x] Row {id: R-1, retires: \"api:foo.Brand.New\"}\n")

	in, err := NewInput(context.Background(), repo.root, base, nil, "svc", []string{"docs/plans/p.md"})
	if err != nil {
		t.Fatalf("NewInput: %v", err)
	}
	if got := in.Declarations.Declared("api:foo.Brand.New"); got != DeclaredSelf {
		t.Errorf("Declared(api:foo.Brand.New) = %q, want self (the plan itself did not exist at base)", got)
	}
}
