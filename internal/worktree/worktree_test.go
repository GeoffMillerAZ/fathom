package worktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRepoWithWorktree initialises a repository with one commit and one
// linked worktree, returning both canonical roots.
func gitRepoWithWorktree(t *testing.T) (repo, wt string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	base := t.TempDir()
	repo = filepath.Join(base, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(repo, "init", "-q")
	run(repo, "commit", "-q", "--allow-empty", "-m", "init")
	wt = filepath.Join(base, "wt")
	run(repo, "worktree", "add", "-q", wt)
	return canonical(repo), canonical(wt)
}

func TestLinkedByMarker(t *testing.T) {
	base := t.TempDir()
	wt := filepath.Join(base, "repo", ".claude", "worktrees", "wf_1")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	main, ok := Linked(wt)
	if !ok {
		t.Fatal("a path under .claude/worktrees/ is a worktree even without git")
	}
	if want := filepath.Join(base, "repo"); main != want {
		t.Errorf("main root = %q, want %q", main, want)
	}
	if _, ok := Linked(filepath.Join(base, "repo")); ok {
		t.Error("the main checkout is not linked")
	}
}

func TestLinkedByGit(t *testing.T) {
	repo, wt := gitRepoWithWorktree(t)
	main, ok := Linked(wt)
	if !ok {
		t.Fatal("git worktree add must be detected as linked")
	}
	if main != repo {
		t.Errorf("main root = %q, want %q", main, repo)
	}
	if _, ok := Linked(repo); ok {
		t.Error("the main checkout must not read as linked")
	}
	if _, ok := Linked(t.TempDir()); ok {
		t.Error("a non-repository directory must not read as linked")
	}
	c1, ok1 := CommonDir(repo)
	c2, ok2 := CommonDir(wt)
	if !ok1 || !ok2 || c1 != c2 {
		t.Errorf("common dir must be shared: %q (%v) vs %q (%v)", c1, ok1, c2, ok2)
	}
}

func TestCheckReroot(t *testing.T) {
	repo, wt := gitRepoWithWorktree(t)
	other := t.TempDir()

	t.Run("new_service_is_free", func(t *testing.T) {
		if err := CheckReroot("svc", "", wt); err != nil {
			t.Errorf("no recorded root: got %v", err)
		}
	})
	t.Run("same_root_is_a_refresh", func(t *testing.T) {
		if err := CheckReroot("svc", repo, repo); err != nil {
			t.Errorf("same path: got %v", err)
		}
	})
	t.Run("worktree_of_recorded_root_refused", func(t *testing.T) {
		err := CheckReroot("svc", repo, wt)
		var re *RerootError
		if !errors.As(err, &re) {
			t.Fatalf("want *RerootError, got %v", err)
		}
		if re.Recorded != repo || re.Requested != wt || re.MainRoot != repo {
			t.Errorf("error paths: %+v", re)
		}
		msg := err.Error()
		for _, want := range []string{"svc", repo, wt, "refused"} {
			if !strings.Contains(msg, want) {
				t.Errorf("message must carry %q: %s", want, msg)
			}
		}
	})
	t.Run("back_to_main_checkout_allowed", func(t *testing.T) {
		if err := CheckReroot("svc", wt, repo); err != nil {
			t.Errorf("moving a service back to its main checkout is the O22 fix, got %v", err)
		}
	})
	t.Run("worktree_of_another_repo_allowed", func(t *testing.T) {
		if err := CheckReroot("svc", other, wt); err != nil {
			t.Errorf("a worktree of a different repository is a genuine re-root, got %v", err)
		}
	})
	t.Run("marker_path_refused_without_git", func(t *testing.T) {
		marker := filepath.Join(other, ".claude", "worktrees", "wf_9")
		if err := os.MkdirAll(marker, 0o755); err != nil {
			t.Fatal(err)
		}
		err := CheckReroot("svc", repo, marker)
		var re *RerootError
		if !errors.As(err, &re) {
			t.Fatalf("any path under .claude/worktrees/ is refused for an existing service, got %v", err)
		}
		if !strings.Contains(re.Reason, ".claude/worktrees") {
			t.Errorf("reason should name the marker: %q", re.Reason)
		}
	})
}

// TestRepositoryIdentityIsSharedByCloneAndWorktree (EW-19): a clone, a linked
// worktree and the original name one repository; another repository, a
// plain directory and a path that does not exist do not.
func TestRepositoryIdentityIsSharedByCloneAndWorktree(t *testing.T) {
	repo, wt := gitRepoWithWorktree(t)
	ctx := context.Background()
	id, ok := RepositoryIdentity(ctx, repo)
	if !ok || id == "" {
		t.Fatalf("RepositoryIdentity(repo) = %q, %v", id, ok)
	}
	if got, ok := RepositoryIdentity(ctx, wt); !ok || got != id {
		t.Errorf("a linked worktree's identity = %q, %v; want %q", got, ok, id)
	}
	clone := filepath.Join(t.TempDir(), "clone")
	if out, err := exec.Command("git", "clone", "-q", repo, clone).CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	if got, ok := RepositoryIdentity(ctx, clone); !ok || got != id {
		t.Errorf("a clone's identity = %q, %v; want %q", got, ok, id)
	}
	other := filepath.Join(t.TempDir(), "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"commit", "-q", "--allow-empty", "-m", "another root"}} {
		if out, err := exec.Command("git", append([]string{"-C", other, "-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if got, ok := RepositoryIdentity(ctx, other); !ok || got == id {
		t.Errorf("another repository's identity = %q, %v; must differ from %q", got, ok, id)
	}
	if _, ok := RepositoryIdentity(ctx, t.TempDir()); ok {
		t.Error("a plain directory has no repository identity")
	}
	if _, ok := RepositoryIdentity(ctx, filepath.Join(t.TempDir(), "absent")); ok {
		t.Error("a path that does not exist has no repository identity")
	}
}
