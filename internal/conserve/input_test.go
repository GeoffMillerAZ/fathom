package conserve

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

func TestNewInput_BadRefIsAnError(t *testing.T) {
	repo := newInputTestRepo(t)
	repo.write("a.go", "package a\n")
	repo.commitAll("only commit")

	if _, err := NewInput(context.Background(), repo.root, "not-a-real-ref", nil, "svc", nil); err == nil {
		t.Fatal("want an error resolving a nonexistent ref, got nil")
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
