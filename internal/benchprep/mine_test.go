package benchprep

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mineGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
		"GIT_AUTHOR_DATE=2026-01-02T03:04:05Z", "GIT_COMMITTER_DATE=2026-01-02T03:04:05Z")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func mineWrite(t *testing.T, dir, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A baseline followed by three candidate commits: fix, refactor, regression.
func mineFixture(t *testing.T) (dir, parent, fix, broken string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "workspace", "scenariobench", "subjects", "fixture")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	mineGit(t, dir, "init", "-b", "fixture")
	mineWrite(t, dir, "go.mod", "module example.invalid/fixture\n\ngo 1.26.0\n")
	mineWrite(t, dir, "answer.go", "package fixture\nfunc Answer() int { return 0 }\nfunc Other() int { return 0 }\nfunc Stable() int { return 1 }\n")
	mineWrite(t, dir, "answer_test.go", "package fixture\nimport \"testing\"\nfunc TestStable(t *testing.T) { if Stable() != 1 { t.Fatal(\"stable\") } }\n")
	commit := func(message string) string {
		mineGit(t, dir, "add", ".")
		mineGit(t, dir, "-c", "core.hooksPath="+os.DevNull, "-c", "commit.gpgsign=false", "commit", "-m", message)
		return mineGit(t, dir, "rev-parse", "HEAD")
	}
	parent = commit("baseline")
	mineWrite(t, dir, "answer.go", "package fixture\nfunc Answer() int { return 42 }\nfunc Other() int { return 0 }\nfunc Stable() int { return 1 }\n")
	mineWrite(t, dir, "answer_test.go", "package fixture\nimport \"testing\"\nfunc TestStable(t *testing.T) { if Stable() != 1 { t.Fatal(\"stable\") } }\nfunc TestAnswer(t *testing.T) { if Answer() != 42 { t.Fatal(\"answer\") } }\n")
	fix = commit("fix answer")
	mineWrite(t, dir, "answer.go", "package fixture\n// Answer returns the answer.\nfunc Answer() int { return 42 }\nfunc Other() int { return 0 }\nfunc Stable() int { return 1 }\n")
	commit("refactor without test changes")
	mineWrite(t, dir, "answer.go", "package fixture\nfunc Answer() int { return 42 }\nfunc Other() int { return 7 }\nfunc Stable() int { return 0 }\n")
	mineWrite(t, dir, "answer_test.go", "package fixture\nimport \"testing\"\nfunc TestStable(t *testing.T) { if Stable() != 1 { t.Fatal(\"stable\") } }\nfunc TestAnswer(t *testing.T) { if Answer() != 42 { t.Fatal(\"answer\") } }\nfunc TestOther(t *testing.T) { if Other() != 7 { t.Fatal(\"other\") } }\n")
	broken = commit("fix other but break stable")
	return
}

func assertMineClean(t *testing.T, dir, head string) {
	t.Helper()
	if got := mineGit(t, dir, "rev-parse", "HEAD"); got != head {
		t.Fatalf("subject HEAD changed: %s", got)
	}
	if got := mineGit(t, dir, "status", "--porcelain"); got != "?? keep.txt" {
		t.Fatalf("subject contents changed: %q", got)
	}
	if got := mineGit(t, dir, "worktree", "list", "--porcelain"); strings.Count(got, "worktree ") != 1 {
		t.Fatalf("temporary worktrees leaked: %s", got)
	}
	entries, err := os.ReadDir(filepath.Dir(dir))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary directories leaked: %v, %v", entries, err)
	}
}

func TestMineFixCommitsFakeRunner(t *testing.T) {
	dir, parent, fix, head := mineFixture(t)
	mineWrite(t, dir, "keep.txt", "preserve untracked contents")
	var calls int
	runner := func(ctx context.Context, checkout, commit string, selection []string) (map[string]string, error) {
		calls++
		if checkout == dir || mineGit(t, checkout, "rev-parse", "HEAD") != commit {
			t.Fatal("runner did not receive a detached checkout at the requested commit")
		}
		if commit == parent {
			contents, _ := os.ReadFile(filepath.Join(checkout, "answer_test.go"))
			if !strings.Contains(string(contents), "TestAnswer") || len(selection) != 0 {
				t.Fatalf("parent lacks regression test or selection: %s, %v", contents, selection)
			}
			return map[string]string{"example.invalid/fixture.TestAnswer": "fail", "example.invalid/fixture.TestStable": "pass"}, nil
		}
		if commit == fix {
			if len(selection) != 0 {
				t.Fatal("commit must run the complete suite")
			}
			return map[string]string{"example.invalid/fixture.TestAnswer": "pass", "example.invalid/fixture.TestStable": "pass"}, nil
		}
		return map[string]string{"example.invalid/fixture.TestOther": "pass", "example.invalid/fixture.TestStable": "fail"}, nil
	}
	got, err := MineFixCommits(t.Context(), dir, runner)
	if err != nil || len(got) != 1 {
		t.Fatalf("MineFixCommits = %+v, %v", got, err)
	}
	r := got[0]
	if r.Commit != fix || r.Parent != parent || r.CreatedAt != "2026-01-02T03:04:05Z" || r.Shape != "revert-and-redo" || r.BaseCommit != fix {
		t.Fatalf("wrong provenance/task shape: %+v", r)
	}
	if !reflect.DeepEqual(r.FailToPass, []string{"example.invalid/fixture.TestAnswer"}) || !reflect.DeepEqual(r.PassToPass, []string{"example.invalid/fixture.TestStable"}) {
		t.Fatalf("wrong test evidence: %+v", r)
	}
	if strings.Contains(r.GoldPatch, "answer_test.go") || !strings.Contains(r.TestPatch, "TestAnswer") || !strings.Contains(r.RevertPatch, "-func Answer() int { return 42 }") {
		t.Fatalf("incorrect split/revert patches: %+v", r)
	}
	if calls < 2 {
		t.Fatalf("missing runner calls: %d", calls)
	}
	assertMineClean(t, dir, head)
}

func TestMineFixCommitsRefusesUnprovenTests(t *testing.T) {
	for _, mode := range []string{"no-parent-failure", "missing-at-commit", "skipped-at-commit", "suite-failure", "runner-error"} {
		t.Run(mode, func(t *testing.T) {
			dir, parent, fix, head := mineFixture(t)
			mineWrite(t, dir, "keep.txt", "keep")
			runner := func(_ context.Context, _ string, commit string, _ []string) (map[string]string, error) {
				if mode == "runner-error" {
					return nil, errors.New("runner unavailable")
				}
				if commit == parent {
					status := "fail"
					if mode == "no-parent-failure" {
						status = "pass"
					}
					return map[string]string{"pkg.TestAnswer": status}, nil
				}
				if commit == fix {
					switch mode {
					case "missing-at-commit":
						return map[string]string{"pkg.TestStable": "pass"}, nil
					case "skipped-at-commit":
						return map[string]string{"pkg.TestAnswer": "skip"}, nil
					case "suite-failure":
						return map[string]string{"pkg.TestAnswer": "pass", "pkg.TestStable": "fail"}, nil
					}
				}
				return map[string]string{"pkg.TestOther": "pass"}, nil
			}
			got, err := MineFixCommits(t.Context(), dir, runner)
			if mode == "runner-error" {
				if err == nil || !strings.Contains(err.Error(), "runner unavailable") {
					t.Fatalf("missing error: %v", err)
				}
			} else if err != nil || len(got) != 0 {
				t.Fatalf("kept unproven fix: %+v, %v", got, err)
			}
			assertMineClean(t, dir, head)
		})
	}
}

func TestMineFixCommitsAcceptance(t *testing.T) {
	dir, _, fix, head := mineFixture(t)
	mineWrite(t, dir, "keep.txt", "keep")
	got, err := MineFixCommits(t.Context(), dir, nil)
	if err != nil || len(got) != 1 || got[0].Commit != fix {
		t.Fatalf("real go test miner = %+v, %v; want exactly %s", got, err, fix)
	}
	// Applying the reverse patch at the fix preserves the tests, which must
	// fail, and applying the gold patch again makes the whole suite green.
	checkout := filepath.Join(t.TempDir(), "redo")
	mineGit(t, dir, "worktree", "add", "--detach", checkout, fix)
	defer func() {
		if _, err := os.Stat(checkout); err == nil {
			mineGit(t, dir, "worktree", "remove", "--force", checkout)
		}
	}()
	for i, patch := range []string{got[0].RevertPatch, got[0].GoldPatch} {
		cmd := exec.CommandContext(t.Context(), "git", "apply", "-")
		cmd.Dir, cmd.Stdin = checkout, strings.NewReader(patch)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("apply: %v\n%s", err, out)
		}
		outcomes, err := GoTestRunner(t.Context(), checkout, fix, nil)
		if err != nil {
			t.Fatal(err)
		}
		if (outcomes["example.invalid/fixture.TestAnswer"] == "pass") != (i == 1) {
			t.Fatalf("redo %d: %v", i, outcomes)
		}
	}
	mineGit(t, dir, "worktree", "remove", "--force", checkout)
	assertMineClean(t, dir, head)
}

func TestMineFixCommitsRejectsShallowClone(t *testing.T) {
	dir, parent, _, _ := mineFixture(t)
	mineWrite(t, filepath.Join(dir, ".git"), "shallow", parent+"\n")
	if _, err := MineFixCommits(t.Context(), dir, nil); err == nil || !strings.Contains(err.Error(), "full clone") {
		t.Fatalf("shallow clone accepted: %v", err)
	}
}

func TestMineFixCommitsRequiresMatchingPackageAndParentPassEvidence(t *testing.T) {
	for _, mode := range []string{"wrong-package", "unproven-pass-to-pass"} {
		t.Run(mode, func(t *testing.T) {
			dir, parent, fix, _ := mineFixture(t)
			runner := func(_ context.Context, _ string, commit string, _ []string) (map[string]string, error) {
				if commit == parent {
					if mode == "wrong-package" {
						return map[string]string{"unrelated.TestAnswer": "fail", "example.invalid/fixture.TestAnswer": "pass"}, nil
					}
					return map[string]string{"example.invalid/fixture.TestAnswer": "fail"}, nil
				}
				if commit == fix {
					return map[string]string{"unrelated.TestAnswer": "pass", "example.invalid/fixture.TestAnswer": "pass", "example.invalid/fixture.TestNew": "pass"}, nil
				}
				return map[string]string{"example.invalid/fixture.TestOther": "pass"}, nil
			}
			got, err := MineFixCommits(t.Context(), dir, runner)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "wrong-package" {
				if len(got) != 0 {
					t.Fatalf("unrelated test qualified a fix: %+v", got)
				}
			} else if len(got) != 1 || len(got[0].PassToPass) != 0 {
				t.Fatalf("invented parent pass evidence: %+v", got)
			}
		})
	}
}
