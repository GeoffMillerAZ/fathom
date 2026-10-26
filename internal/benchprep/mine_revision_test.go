package benchprep

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGoTestRunnerPackageSelections(t *testing.T) {
	dir := t.TempDir()
	mineWrite(t, dir, "go.mod", "module example.invalid/fixture\n\ngo 1.26.0\n")
	for _, pkg := range []string{"a", "b"} {
		sub := filepath.Join(dir, pkg)
		if err := os.MkdirAll(sub, 0o700); err != nil {
			t.Fatal(err)
		}
		mineWrite(t, sub, "answer_test.go", "package fixture\nimport \"testing\"\nfunc TestAnswer(t *testing.T) {}\nfunc TestUnselected(t *testing.T) { t.Fatal(\"must not run\") }\n")
	}
	selection := []string{"example.invalid/fixture/a.TestAnswer", "example.invalid/fixture/b.TestAnswer"}
	got, err := GoTestRunner(t.Context(), dir, "fixture", selection)
	want := map[string]string{selection[0]: "pass", selection[1]: "pass"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("selection outcomes = %v, %v; want %v", got, err, want)
	}
	got, err = GoTestRunner(t.Context(), dir, "fixture", selection[:1])
	if err != nil || !reflect.DeepEqual(got, map[string]string{selection[0]: "pass"}) {
		t.Fatalf("package selection ran unrelated tests: %v, %v", got, err)
	}
}

func TestGoTestRunnerSelectionWithTestLikeSubtest(t *testing.T) {
	dir := t.TempDir()
	mineWrite(t, dir, "go.mod", "module example.invalid/fixture\n\ngo 1.26.0\n")
	mineWrite(t, dir, "answer_test.go", `package fixture
import "testing"
func TestA(t *testing.T) {
	t.Run("case.TestB", func(t *testing.T) {})
	t.Run("case.ExampleB", func(t *testing.T) {})
	t.Run("case.FuzzB", func(t *testing.T) {})
	t.Run("unselected", func(t *testing.T) { t.Fatal("must not run") })
}

func TestUnselected(t *testing.T) { t.Fatal("must not run") }
`)
	for _, name := range []string{"case.TestB", "case.ExampleB", "case.FuzzB"} {
		t.Run(name, func(t *testing.T) {
			parent := "example.invalid/fixture.TestA"
			id := parent + "/" + name
			got, err := GoTestRunner(t.Context(), dir, "fixture", []string{id})
			want := map[string]string{parent: "pass", id: "pass"}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("subtest selection outcomes = %v, %v; want %v", got, err, want)
			}
		})
	}
}

func TestGoTestRunnerSelectionWithDottedPackage(t *testing.T) {
	dir := t.TempDir()
	mineWrite(t, dir, "go.mod", "module example.invalid/fixture.TestA\n\ngo 1.26.0\n")
	mineWrite(t, dir, "answer_test.go", "package fixture\nimport \"testing\"\nfunc TestB(t *testing.T) {}\nfunc TestUnselected(t *testing.T) { t.Fatal(\"must not run\") }\n")
	id := "example.invalid/fixture.TestA.TestB"
	got, err := GoTestRunner(t.Context(), dir, "fixture", []string{id})
	want := map[string]string{id: "pass"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("dotted package selection outcomes = %v, %v; want %v", got, err, want)
	}
}

func TestGoTestRunnerSelectionWithTestLikePackageDirectory(t *testing.T) {
	dir := t.TempDir()
	mineWrite(t, dir, "go.mod", "module example.invalid/fixture\n\ngo 1.26.0\n")
	sub := filepath.Join(dir, "foo.TestA", "bar")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	mineWrite(t, sub, "answer_test.go", "package fixture\nimport \"testing\"\nfunc TestB(t *testing.T) {}\nfunc TestUnselected(t *testing.T) { t.Fatal(\"must not run\") }\n")
	id := "example.invalid/fixture/foo.TestA/bar.TestB"
	got, err := GoTestRunner(t.Context(), dir, "fixture", []string{id})
	want := map[string]string{id: "pass"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("test-like package directory outcomes = %v, %v; want %v", got, err, want)
	}
}

func TestMineFixCommitsModuleAndTestdataOverlay(t *testing.T) {
	for _, mode := range []string{"root-testdata", "nested-module", "no-root-module"} {
		t.Run(mode, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "workspace", "scenariobench", "subjects", "fixture")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			mineGit(t, dir, "init", "-b", "fixture")
			sub := dir
			module := "example.invalid/fixture"
			if mode != "no-root-module" {
				mineWrite(t, dir, "go.mod", "module "+module+"\n\ngo 1.26.0\n")
			}
			if mode != "root-testdata" {
				sub = filepath.Join(dir, "sub")
				if err := os.MkdirAll(sub, 0o700); err != nil {
					t.Fatal(err)
				}
				module = "example.invalid/nested"
				mineWrite(t, sub, "go.mod", "module "+module+"\n\ngo 1.26.0\n")
			}
			mineWrite(t, sub, "answer.go", "package fixture\nfunc Answer() string { return \"wrong\" }\n")
			mineWrite(t, sub, "answer_test.go", "package fixture\nimport \"testing\"\nfunc TestStable(t *testing.T) {}\n")
			commit := func(message string) string {
				mineGit(t, dir, "add", ".")
				mineGit(t, dir, "-c", "core.hooksPath="+os.DevNull, "-c", "commit.gpgsign=false", "commit", "-m", message)
				return mineGit(t, dir, "rev-parse", "HEAD")
			}
			parent := commit("baseline")
			mineWrite(t, sub, "answer.go", "package fixture\nfunc Answer() string { return \"answer\" }\n")
			if err := os.MkdirAll(filepath.Join(sub, "testdata"), 0o700); err != nil {
				t.Fatal(err)
			}
			mineWrite(t, sub, "testdata/answer.txt", "answer")
			mineWrite(t, sub, "answer_test.go", "package fixture\nimport (\"testing\"; \"os\")\nfunc TestStable(t *testing.T) {}\nfunc TestAnswer(t *testing.T) { b, err := os.ReadFile(\"testdata/answer.txt\"); if err != nil { t.Fatal(err) }; if string(b) != Answer() { t.Fatal(\"answer\") } }\n")
			fix := commit("fix answer")
			got, err := MineFixCommits(t.Context(), dir, nil)
			if err != nil || len(got) != 1 {
				t.Fatalf("mining %s = %+v, %v", mode, got, err)
			}
			r := got[0]
			if r.BaseCommit != parent || r.Commit != fix || !reflect.DeepEqual(r.FailToPass, []string{module + ".TestAnswer"}) || !reflect.DeepEqual(r.PassToPass, []string{module + ".TestStable"}) {
				t.Fatalf("wrong module evidence: %+v", r)
			}
			if !strings.Contains(r.TestPatch, "testdata/answer.txt") || strings.Contains(r.GoldPatch, "testdata/") || strings.Contains(r.RevertPatch, "testdata/") {
				t.Fatalf("testdata leaked into gold: %+v", r)
			}
		})
	}
}

func TestMineFixCommitsTemporaryRoot(t *testing.T) {
	for _, cancelRun := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "cancel"}[cancelRun], func(t *testing.T) {
			dir, _, _, head := mineFixture(t)
			mineWrite(t, dir, "keep.txt", "keep")
			wantRoot := filepath.Join(filepath.Dir(filepath.Dir(dir)), "mine-tmp")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			called := false
			_, err := MineFixCommits(ctx, dir, func(_ context.Context, checkout, _ string, _ []string) (map[string]string, error) {
				called = true
				rel, err := filepath.Rel(wantRoot, checkout)
				if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					t.Fatalf("temporary checkout is outside mine-tmp: %s", checkout)
				}
				if cancelRun {
					cancel()
					return nil, context.Canceled
				}
				return map[string]string{}, nil
			})
			if !called || (cancelRun && !errors.Is(err, context.Canceled)) || (!cancelRun && err != nil) {
				t.Fatalf("runner/cancellation = %v, %v", called, err)
			}
			assertMineClean(t, dir, head)
			entries, err := os.ReadDir(wantRoot)
			if err != nil || len(entries) != 0 {
				t.Fatalf("mine-tmp not cleaned: %v, %v", entries, err)
			}
		})
	}
}

func TestMineFixCommitsPrunesStaleWorktrees(t *testing.T) {
	dir, _, _, head := mineFixture(t)
	mineWrite(t, dir, "keep.txt", "keep")
	stale := filepath.Join(t.TempDir(), "stale")
	mineGit(t, dir, "worktree", "add", "--detach", stale, head)
	if err := os.RemoveAll(stale); err != nil {
		t.Fatal(err)
	}
	called := false
	_, err := MineFixCommits(t.Context(), dir, func(_ context.Context, _, _ string, _ []string) (map[string]string, error) {
		called = true
		if got := mineGit(t, dir, "worktree", "list", "--porcelain"); strings.Contains(got, stale) {
			t.Fatal("stale worktree still registered at first run")
		}
		return map[string]string{}, nil
	})
	if err != nil || !called {
		t.Fatalf("prune/run = %v, %v", called, err)
	}
	assertMineClean(t, dir, head)
}
