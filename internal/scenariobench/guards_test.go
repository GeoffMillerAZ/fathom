package scenariobench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

// guards_test.go is SB-24's anchor: TestGuardsCorpusDetectorsExist proves
// testdata/scenariobench/guards-fathom.json is not just prose — every
// detector command it names is something this tree can actually run, and
// every rule citation actually points at the invariant it claims to.

const guardsCorpusPath = "../../testdata/scenariobench/guards-fathom.json"

// moduleRoot is the repo root from this package's location
// (internal/scenariobench), for detectors that walk the tree.
const moduleRoot = "../.."

func loadGuardsCorpus(t *testing.T) Corpus {
	t.Helper()
	return loadGuardsCorpusAt(t, guardsCorpusPath)
}

// guardsCorpora are every committed guards corpus; a second one
// (guards-adr.json, SB-49) must pass the same detector gate as the first.
func guardsCorpora(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob("../../testdata/scenariobench/guards-*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no guards corpora found: %v", err)
	}
	sort.Strings(paths)
	return paths
}

func loadGuardsCorpusAt(t *testing.T, path string) Corpus {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read guards corpus: %v", err)
	}
	var c Corpus
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("parse guards corpus: %v", err)
	}
	return c
}

// allowedDetectorVerbs are the invariant-enforcing command families a
// guards-corpus detector may name (docs/intent/invariants.md's own gate
// vocabulary: go test, task <name>, cue vet, bash scripts/*.sh). Anything
// else must at least resolve on PATH — never a command this repo cannot
// actually run.
var allowedDetectorVerbs = map[string]bool{"go": true, "task": true, "cue": true, "bash": true}

// runDetectorRe extracts a `go test <pkgDir> ... -run <Name>` detector's
// package directory and test name.
func parseGoTestDetector(cmd string) (pkgDir, testName string, ok bool) {
	fields := strings.Fields(cmd)
	if len(fields) < 2 || fields[0] != "go" || fields[1] != "test" {
		return "", "", false
	}
	for i := 2; i < len(fields); i++ {
		f := fields[i]
		if strings.HasPrefix(f, "./") {
			pkgDir = strings.TrimPrefix(f, "./")
		}
		if f == "-run" && i+1 < len(fields) {
			testName = strings.Trim(fields[i+1], `'"`)
		}
	}
	return pkgDir, testName, testName != ""
}

// testFuncExistsInTree reports whether some "func <name>(" declaration
// exists among the *_test.go files under root/pkgDir (root's whole tree,
// skipping skipTreeDir, when pkgDir is ""). Unlike findFuncInTree (which
// deliberately excludes _test.go — it locates PRODUCTION symbols for
// FalseDoneRate/ContractDrift), a guards-corpus detector names a Test*
// function, which lives only in a _test.go file.
func testFuncExistsInTree(root, pkgDir, name string) (bool, error) {
	var files []string
	if pkgDir != "" {
		dir := filepath.Join(root, filepath.FromSlash(pkgDir))
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false, err
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), "_test.go") {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	} else {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if path != root && skipTreeDir(info.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return false, err
		}
	}

	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		spans, err := publicloc.FuncSpans(src, path)
		if err != nil {
			continue
		}
		for _, sp := range spans {
			if sp.Name == name {
				return true, nil
			}
		}
	}
	return false, nil
}

// errGitHistoryUnavailable marks a lookup that cannot be answered because
// this copy of the tree has no usable git history (no git binary, not a
// git work tree, or the commit is not in a shallow or pruned clone). It is
// neither a pass nor a failure: the caller skips with the reason, since the
// detector's test could not be checked, not shown absent.
var errGitHistoryUnavailable = errors.New("git history unavailable")

// gitHistoryTimeout bounds each git call a guards lookup makes; every one
// is a local, read-only object read.
const gitHistoryTimeout = 30 * time.Second

// gitReadAt runs a read-only git command in root under gitHistoryTimeout.
func gitReadAt(root string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitHistoryTimeout)
	defer cancel()
	return exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...).Output()
}

// testFuncExistsAtCommit is testFuncExistsInTree for a package the tree
// has since retired: it reads pkgDir's *_test.go files at commit (the
// task's base_commit, where the scenario's detector actually runs) out of
// root's git history. A package git cannot produce at a commit that is
// present is an error, never a pass; a copy of the tree with no git
// history at all (or without that commit) returns errGitHistoryUnavailable
// so the caller can skip instead of guessing either way.
func testFuncExistsAtCommit(root, commit, pkgDir, name string) (bool, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return false, fmt.Errorf("%w: git is not on PATH", errGitHistoryUnavailable)
	}
	if _, err := gitReadAt(root, "cat-file", "-e", commit+"^{commit}"); err != nil {
		return false, fmt.Errorf("%w: commit %s is not readable under %s (not a git work tree, or a shallow or pruned clone)", errGitHistoryUnavailable, commit, root)
	}
	ls, err := gitReadAt(root, "ls-tree", "--name-only", commit+":"+pkgDir)
	if err != nil {
		return false, fmt.Errorf("git ls-tree %s:%s: %w", commit, pkgDir, err)
	}
	for _, file := range strings.Fields(string(ls)) {
		if !strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := gitReadAt(root, "show", commit+":"+pkgDir+"/"+file)
		if err != nil {
			return false, fmt.Errorf("git show %s:%s/%s: %w", commit, pkgDir, file, err)
		}
		spans, err := publicloc.FuncSpans(src, file)
		if err != nil {
			continue
		}
		for _, sp := range spans {
			if sp.Name == name {
				return true, nil
			}
		}
	}
	return false, nil
}

// TestGuardsCorpusDetectorsExist is SB-24's anchor: every task's detector
// command is runnable in this tree (a go/task/cue/bash verb, or something
// on PATH), every `go test -run TestX` detector names a TestX this tree
// actually declares, and every rule's `source` citation — "file#Lnn" or,
// for an ADR record, the heading-anchored "file#ADR-nnn" (item 3, wave N8,
// ResolveCitationLine in citation.go) — names a line that actually carries
// that rule's id.
func TestGuardsCorpusDetectorsExist(t *testing.T) {
	for _, path := range guardsCorpora(t) {
		t.Run(filepath.Base(path), func(t *testing.T) { checkGuardsCorpusDetectors(t, loadGuardsCorpusAt(t, path)) })
	}
}

func checkGuardsCorpusDetectors(t *testing.T, c Corpus) {
	t.Helper()
	// >= 6: guards-fathom.json and guards-adr.json carry 8 rules each;
	// guards-m1.json (RC-50) is a deliberately smaller, harder-to-derive
	// six — each rule placed far from its edit site by construction, which
	// costs more per task to source honestly than a wider, easier eight.
	if len(c.Tasks) < 6 {
		t.Fatalf("guards corpus has %d tasks, want >= 6", len(c.Tasks))
	}
	if c.Criterion.ID == "" {
		t.Fatalf("guards corpus has no criterion")
	}

	for _, task := range c.Tasks {
		task := task
		t.Run(task.ID, func(t *testing.T) {
			var unverified []string
			defer func() {
				if len(unverified) > 0 && !t.Failed() {
					t.Skipf("SKIPPED, not passed: %s", strings.Join(unverified, "; "))
				}
			}()
			if task.Rule == nil {
				t.Fatalf("task %s: no rule", task.ID)
			}
			if len(task.Oracle.Detectors) == 0 {
				t.Fatalf("task %s: no detectors", task.ID)
			}
			for _, d := range task.Oracle.Detectors {
				if d.ExpectExit != 0 {
					t.Errorf("task %s: detector %s expect_exit = %d, want 0", task.ID, d.Name, d.ExpectExit)
				}
				fields := strings.Fields(d.Command)
				if len(fields) == 0 {
					t.Fatalf("task %s: detector %s has an empty command", task.ID, d.Name)
				}
				verb := fields[0]
				if !allowedDetectorVerbs[verb] {
					if _, err := exec.LookPath(verb); err != nil {
						t.Errorf("task %s: detector %s's command %q starts with %q, which is none of go/task/cue/bash and is not on PATH", task.ID, d.Name, d.Command, verb)
					}
				}
				if pkgDir, testName, ok := parseGoTestDetector(d.Command); ok {
					if len(task.Subject.Setup) > 0 {
						// RC-50: a task whose subject.setup materializes its
						// own edit-site scaffold (a package this repo does
						// not actually commit, e.g. guards-m1's
						// fanout-determinism/store-search-accelerator) has
						// nothing for a static tree search to find — the
						// package exists only inside a prepared worktree.
						// TestGuardsM1ViolationProbesFireAndClear is the
						// real proof for these: it runs task.Subject.Setup
						// and then this exact detector command for real.
						continue
					}
					found, err := testFuncExistsInTree(moduleRoot, pkgDir, testName)
					where := fmt.Sprintf("%q", pkgDir)
					if errors.Is(err, fs.ErrNotExist) && pkgDir != "" && task.Subject.BaseCommit != "" {
						// A package retired from this tree after the task's
						// base_commit (guards-adr-028's internal/memory,
						// SR-13 / ADR-053) is still where the scenario runs:
						// the subject is checked out at base_commit, so the
						// detector's test must exist THERE. It is looked up
						// in git history, never assumed.
						where = fmt.Sprintf("%q at base_commit %s", pkgDir, task.Subject.BaseCommit)
						found, err = testFuncExistsAtCommit(moduleRoot, task.Subject.BaseCommit, pkgDir, testName)
					}
					if errors.Is(err, errGitHistoryUnavailable) {
						// Not a pass and not a failure: the test could not
						// be looked up. The rest of the task is still
						// checked; the subtest ends skipped with this.
						unverified = append(unverified, fmt.Sprintf("detector %s: %s in %s not checked: %v", d.Name, testName, where, err))
					} else if err != nil {
						t.Errorf("task %s: detector %s: searching for %s in %s: %v", task.ID, d.Name, testName, where, err)
					} else if !found {
						t.Errorf("task %s: detector %s names test %s, which does not exist under %s", task.ID, d.Name, testName, where)
					}
				}
			}

			// ResolveCitationLine (citation.go, item 3) accepts both the
			// original "path#Lnn" form and the heading-anchored
			// "path#ADR-nnn" form, resolving the latter to wherever the
			// "## ADR-nnn …" heading CURRENTLY sits — so a citation into
			// an ADR record survives an insertion above it, unlike a
			// hardcoded line number.
			path, line, err := ResolveCitationLine(moduleRoot, task.Rule.Source)
			if err != nil {
				t.Fatalf("task %s: rule source %q: %v", task.ID, task.Rule.Source, err)
			}
			data, err := os.ReadFile(moduleRoot + "/" + path)
			if err != nil {
				t.Fatalf("task %s: reading rule source %q: %v", task.ID, path, err)
			}
			lines := strings.Split(string(data), "\n")
			if line < 1 || line > len(lines) {
				t.Fatalf("task %s: rule source %q names line %d, file has %d lines", task.ID, task.Rule.Source, line, len(lines))
			}
			// The source line must carry the rule's id: an invariants table row
			// ("| I31 |") or, for a rule that lives in an ADR section, the
			// record's heading ("## ADR-006: …" — the id before "§").
			marker := fmt.Sprintf("| %s |", task.Rule.ID)
			head, _, _ := strings.Cut(task.Rule.ID, "§")
			if !strings.Contains(lines[line-1], marker) && !strings.Contains(lines[line-1], head) {
				// A rule quoted from inside an ADR section: the nearest
				// preceding "## " heading must be that record.
				ok := false
				for i := line - 1; i >= 0; i-- {
					if strings.HasPrefix(lines[i], "## ") {
						ok = strings.Contains(lines[i], head)
						break
					}
				}
				if !ok {
					t.Errorf("task %s: %s line %d is neither a %q row nor inside the %s section:\n%s", task.ID, path, line, marker, head, lines[line-1])
				}
			}
		})
	}
}

// TestTestFuncExistsAtCommitWithoutHistory pins the skip contract: a copy
// of the tree with no usable git history yields errGitHistoryUnavailable
// (the caller skips with the reason), never a pass and never a hard
// failure; and a commit git cannot resolve is that same sentinel, while a
// package missing at a commit that IS present stays a plain error.
func TestTestFuncExistsAtCommitWithoutHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH: nothing to construct a repository with")
	}
	// Under a git hook these name the repository being pushed; a scratch
	// repository must never inherit them (a commit here would land there).
	for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
		if old, had := os.LookupEnv(k); had {
			os.Unsetenv(k)
			t.Cleanup(func() { os.Setenv(k, old) })
		}
	}

	t.Run("not_a_git_work_tree", func(t *testing.T) {
		found, err := testFuncExistsAtCommit(t.TempDir(), "0123456789abcdef0123456789abcdef01234567", "internal/x", "TestX")
		if found || !errors.Is(err, errGitHistoryUnavailable) {
			t.Fatalf("got (%v, %v), want (false, errGitHistoryUnavailable)", found, err)
		}
	})

	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	if err := os.MkdirAll(filepath.Join(repo, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package pkg\n\nimport \"testing\"\n\nfunc TestPresent(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(repo, "pkg", "x_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "seed")
	head := run("rev-parse", "HEAD")

	t.Run("commit_missing_from_history", func(t *testing.T) {
		found, err := testFuncExistsAtCommit(repo, "0123456789abcdef0123456789abcdef01234567", "pkg", "TestPresent")
		if found || !errors.Is(err, errGitHistoryUnavailable) {
			t.Fatalf("got (%v, %v), want (false, errGitHistoryUnavailable)", found, err)
		}
	})
	t.Run("present_test_is_found", func(t *testing.T) {
		found, err := testFuncExistsAtCommit(repo, head, "pkg", "TestPresent")
		if err != nil || !found {
			t.Fatalf("got (%v, %v), want (true, nil)", found, err)
		}
	})
	t.Run("absent_test_is_not_found_and_not_a_skip", func(t *testing.T) {
		found, err := testFuncExistsAtCommit(repo, head, "pkg", "TestAbsent")
		if err != nil || found {
			t.Fatalf("got (%v, %v), want (false, nil)", found, err)
		}
	})
	t.Run("missing_package_is_a_plain_error", func(t *testing.T) {
		_, err := testFuncExistsAtCommit(repo, head, "nope", "TestPresent")
		if err == nil || errors.Is(err, errGitHistoryUnavailable) {
			t.Fatalf("err = %v, want a plain error naming the missing package, not the skip sentinel", err)
		}
	})
}
