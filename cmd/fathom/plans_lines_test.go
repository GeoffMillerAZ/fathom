package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/worklines"
)

// linesGit runs git for the fixture, with fixed identity and no hooks.
func linesGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "-c", "init.defaultBranch=main"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func linesWrite(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const linesPlan = "# P\n\n- [ ] Widget renders {id: L-1, symbol: Widget, package: w}\n- [ ] Gadget renders {id: L-2, symbol: Gadget, package: w}\n"

// linesReadSubcommands is every git subcommand a read-only lines report may
// run. Anything else is a write the guard refuses.
var linesReadSubcommands = map[string]bool{
	"rev-parse": true, "for-each-ref": true, "ls-tree": true, "show": true, "log": true,
	"cherry": true, "merge-base": true, "diff": true, "rev-list": true, "status": true, "cat-file": true,
}

// readOnlyGuard wraps gitro.Output and fails (and records) any git
// subcommand that is not a read.
func readOnlyGuard(violations *[]string, mu *sync.Mutex) worklines.Runner {
	return func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		sub, rest := "", []string(nil)
		for i := 0; i < len(args); i++ {
			a := args[i]
			if a == "-c" || a == "-C" {
				i++
				continue
			}
			if strings.HasPrefix(a, "-") {
				continue
			}
			sub, rest = a, args[i+1:]
			break
		}
		ok := linesReadSubcommands[sub] || (sub == "worktree" && len(rest) > 0 && rest[0] == "list")
		if !ok {
			mu.Lock()
			*violations = append(*violations, strings.Join(args, " "))
			mu.Unlock()
			return nil, fmt.Errorf("refused git write: %v", args)
		}
		return gitro.Output(ctx, dir, args...)
	}
}

// repoState is everything a git write could change: refs (stash included),
// every reflog, every index (bytes and mtime), HEADs and the worktree list.
func repoState(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString(linesGit(t, root, "for-each-ref", "--format=%(refname) %(objectname)"))
	b.WriteString("\n--worktrees\n")
	b.WriteString(linesGit(t, root, "worktree", "list", "--porcelain"))
	gitDir := filepath.Join(root, ".git")
	var files []string
	_ = filepath.WalkDir(gitDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(gitDir, p)
		if strings.HasPrefix(rel, "logs"+string(filepath.Separator)) || filepath.Base(rel) == "index" || filepath.Base(rel) == "HEAD" || strings.HasPrefix(rel, "refs"+string(filepath.Separator)) || rel == "packed-refs" {
			files = append(files, rel)
		}
		return nil
	})
	sort.Strings(files)
	for _, rel := range files {
		p := filepath.Join(gitDir, rel)
		raw, _ := os.ReadFile(p)
		st, _ := os.Stat(p)
		fmt.Fprintf(&b, "\n--%s %d %s\n%x", rel, len(raw), st.ModTime().Format(time.RFC3339Nano), raw)
	}
	return b.String()
}

func TestCmdPlansLinesReportsAndNeverActs(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	linesGit(t, root, "init", "-q")
	linesWrite(t, root, "docs/plans/p.md", linesPlan)
	linesWrite(t, root, "w/w.go", "package w\n\nfunc Widget() {}\n\nfunc Gadget() {}\n")
	linesGit(t, root, "add", "-A")
	linesGit(t, root, "commit", "-q", "-m", "base")
	linesGit(t, root, "branch", "done")

	linesGit(t, root, "switch", "-q", "-c", "claims")
	linesWrite(t, root, "docs/plans/p.md", strings.Replace(linesPlan, "- [ ] Widget", "- [x] Widget", 1))
	linesGit(t, root, "commit", "-q", "-am", "claim L-1")
	linesGit(t, root, "update-ref", "refs/remotes/origin/claims", "HEAD")
	linesGit(t, root, "switch", "-q", "main")

	linesGit(t, root, "switch", "-q", "-c", "gadget")
	linesWrite(t, root, "w/w.go", "package w\n\nfunc Widget() {}\n\nfunc Gadget() { _ = 1 }\n")
	linesGit(t, root, "commit", "-q", "-am", "gadget body")
	linesGit(t, root, "switch", "-q", "main")

	linesGit(t, root, "switch", "-q", "-c", "risky")
	linesWrite(t, root, "notes.txt", "r\n")
	linesGit(t, root, "add", "-A")
	linesGit(t, root, "commit", "-q", "-m", "unpushed")
	linesGit(t, root, "switch", "-q", "main")
	linesWrite(t, root, "README", "main moves\n")
	linesGit(t, root, "add", "-A")
	linesGit(t, root, "commit", "-q", "-m", "main moves")
	linesGit(t, root, "update-ref", "refs/remotes/origin/main", "main")

	wt := filepath.Join(base, "wt")
	linesGit(t, root, "worktree", "add", "-q", "-b", "wtb", wt, "main")
	linesWrite(t, wt, "README", "uncommitted\n")
	linesWrite(t, root, "README", "stashed\n")
	linesGit(t, root, "stash", "push", "-q", "-m", "wip")
	// Stat-dirty files: the shape in which a plain `git status` or `git diff`
	// would refresh and write an index back.
	future := time.Now().Add(90 * time.Second)
	for _, p := range []string{filepath.Join(root, "w/w.go"), filepath.Join(wt, "w/w.go")} {
		if err := os.Chtimes(p, future, future); err != nil {
			t.Fatal(err)
		}
	}

	var violations []string
	var mu sync.Mutex
	prev := linesGitRunner
	linesGitRunner = readOnlyGuard(&violations, &mu)
	t.Cleanup(func() { linesGitRunner = prev })

	before := repoState(t, root)
	var out, errb bytes.Buffer
	if code := cmdPlans([]string{"lines", "--root", root}, &out, &errb); code != 0 {
		t.Fatalf("plans lines exit %d: %s", code, errb.String())
	}
	var jout, jerr bytes.Buffer
	if code := cmdPlans([]string{"lines", "--root", root, "--json"}, &jout, &jerr); code != 0 {
		t.Fatalf("plans lines --json exit %d: %s", code, jerr.String())
	}
	after := repoState(t, root)

	if len(violations) > 0 {
		t.Fatalf("plans lines ran git writes: %v", violations)
	}
	if before != after {
		t.Fatalf("plans lines changed the repository (refs, reflogs, an index or the worktree list):\nbefore:\n%s\nafter:\n%s", before, after)
	}

	text := out.String()
	for _, want := range []string{
		"landed", "done", "would remove: git branch -d done",
		"claims-rows", "p.md#L-1",
		"touches-rows", "p.md#L-2 (lexical)",
		"at-risk", "risky", "1 commit on no remote",
		"wtb", "dirty worktree",
		"stash stash@{0}", "stash",
		"fathom ran no git write",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text output lacks %q:\n%s", want, text)
		}
	}
	// Only landed lines carry a removal command.
	for _, ln := range strings.Split(text, "\n") {
		if strings.Contains(ln, "would remove") && !strings.HasPrefix(ln, "landed") {
			t.Errorf("a removal command on a line that is not landed: %q", ln)
		}
	}

	var rep worklines.Report
	if err := json.Unmarshal(jout.Bytes(), &rep); err != nil {
		t.Fatalf("--json: %v\n%s", err, jout.String())
	}
	got := map[string]worklines.Line{}
	for _, l := range rep.Lines {
		got[l.Name] = l
	}
	if l := got["done"]; l.Class != worklines.Landed || l.Removal != "git branch -d done" {
		t.Errorf("done = %+v", l)
	}
	if l := got["claims"]; l.Class != worklines.ClaimsRows || strings.Join(l.Claims, ",") != "p.md#L-1" {
		t.Errorf("claims = %+v", l)
	}
	if l := got["gadget"]; l.Class != worklines.TouchesRows || strings.Join(l.Touches, ",") != "p.md#L-2" || l.Tier != "lexical" {
		t.Errorf("gadget = %+v", l)
	}
	if l := got["wtb"]; l.Class != worklines.AtRisk || l.Worktree == "" || l.Removal != "" {
		t.Errorf("wtb = %+v", l)
	}
	if l := got["stash@{0}"]; l.Kind != worklines.KindStash || l.Class != worklines.AtRisk {
		t.Errorf("stash = %+v", l)
	}
	if rep.Integration != "main" || rep.OpenRows != 2 {
		t.Errorf("report header = %+v", rep)
	}
}
