package worklines

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/gitro"
)

// fixtureNow is the injected clock every fixture commit is dated against
// (internal/AGENTS.md: a fixed fixture date is read with the same fixed now).
var fixtureNow = time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)

// fx is a throwaway repository whose commits carry fixed dates.
type fx struct {
	t    *testing.T
	dir  string
	when time.Time
}

func (f *fx) git(dir string, args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "-c", "init.defaultBranch=main"}, args...)...)
	cmd.Dir = dir
	stamp := f.when.Format(time.RFC3339)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		"GIT_AUTHOR_DATE="+stamp, "GIT_COMMITTER_DATE="+stamp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (f *fx) write(dir, rel, body string) {
	f.t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fx) commit(dir, msg string, files map[string]string) string {
	f.t.Helper()
	for rel, body := range files {
		f.write(dir, rel, body)
	}
	f.git(dir, "add", "-A")
	f.git(dir, "commit", "-q", "-m", msg)
	return f.git(dir, "rev-parse", "HEAD")
}

const planOpen = `# Plan

- [ ] Foo exists {id: P-1, symbol: Foo, package: pkg}
- [ ] Bar exists {id: P-2, symbol: Bar, package: pkg}
- [ ] A reading {id: P-3, oracle: lexical, path: docs/readings/r.md, pattern: "^ok"}
- [x] Done already {id: P-4, symbol: Baz, package: pkg}
`

const fooGo = "package pkg\n\nfunc Foo() int { return 1 }\n\nfunc Bar() int { return 2 }\n"

// buildFixture lays out one repository with a line of every class:
//
//	old-landed  an ancestor of main                       → landed (ancestor)
//	picked      a commit cherry-picked onto main          → landed (patch-equivalent)
//	squashed    two commits squash-merged onto main       → landed (content-equal)
//	risky       one commit on no remote                   → at risk
//	stale       pushed, last commit 40 days before now    → idle
//	fresh       pushed, recent                            → active
//	wt-dirty    landed, checked out in a dirty worktree   → at risk (dirty)
//	wt-same     landed, its dirty edit reads the same on main → landed (worktree content-equal)
//	stash@{0}   an unlanded stash                         → at risk (stash)
func buildFixture(t *testing.T) (root, wtPath string) {
	t.Helper()
	f := &fx{t: t, dir: t.TempDir(), when: fixtureNow.Add(-30 * 24 * time.Hour)}
	root = filepath.Join(f.dir, "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	f.git(root, "init", "-q")
	f.commit(root, "base", map[string]string{"pkg/foo.go": fooGo, "README": "x\n"})
	f.git(root, "branch", "old-landed")

	// stale: an old line pushed long ago.
	f.when = fixtureNow.Add(-40 * 24 * time.Hour)
	f.git(root, "switch", "-q", "-c", "stale")
	f.commit(root, "stale note", map[string]string{"notes/stale.txt": "old\n"})
	f.git(root, "update-ref", "refs/remotes/origin/stale", "HEAD")
	f.git(root, "switch", "-q", "main")

	f.when = fixtureNow.Add(-10 * 24 * time.Hour)
	// picked: a commit that main later cherry-picks.
	f.git(root, "switch", "-q", "-c", "picked")
	pick := f.commit(root, "pickable", map[string]string{"notes/pick.txt": "pick\n"})
	f.git(root, "switch", "-q", "main")
	f.commit(root, "main moves", map[string]string{"README": "y\n"})
	f.git(root, "cherry-pick", pick)

	// squashed: two commits whose content main gains in one.
	f.git(root, "switch", "-q", "-c", "squashed")
	f.commit(root, "sq one", map[string]string{"notes/sq.txt": "one\n"})
	f.commit(root, "sq two", map[string]string{"notes/sq.txt": "one\ntwo\n", "notes/sq2.txt": "z\n"})
	f.git(root, "switch", "-q", "main")
	f.git(root, "merge", "-q", "--squash", "squashed")
	f.git(root, "commit", "-q", "-m", "squash")

	// risky: a commit on no remote.
	f.when = fixtureNow.Add(-2 * 24 * time.Hour)
	f.git(root, "switch", "-q", "-c", "risky")
	f.commit(root, "unpushed", map[string]string{"notes/risky.txt": "r\n"})
	f.git(root, "switch", "-q", "main")

	// fresh: pushed, recent.
	f.git(root, "switch", "-q", "-c", "fresh")
	f.commit(root, "fresh note", map[string]string{"notes/fresh.txt": "f\n"})
	f.git(root, "update-ref", "refs/remotes/origin/fresh", "HEAD")
	f.git(root, "switch", "-q", "main")

	f.git(root, "update-ref", "refs/remotes/origin/main", "main")

	// wt-dirty: a landed branch in a linked worktree with an uncommitted edit.
	f.git(root, "branch", "wt-dirty", "main")
	wtPath = filepath.Join(f.dir, "wt dirty")
	f.git(root, "worktree", "add", "-q", wtPath, "wt-dirty")
	f.write(wtPath, "notes/wip.txt", "uncommitted\n")

	// wt-same: a branch at the base commit in a worktree whose uncommitted
	// edit already reads the same on main.
	f.git(root, "branch", "wt-same", "old-landed")
	samePath := filepath.Join(f.dir, "wt-same")
	f.git(root, "worktree", "add", "-q", samePath, "wt-same")
	f.write(samePath, "README", "y\n")

	// stash@{0}: an unlanded stash on main.
	f.write(root, "README", "stashed edit\n")
	f.git(root, "stash", "push", "-q", "-m", "wip readme")
	return root, wtPath
}

func TestClassifyLinesLandedAtRiskIdle(t *testing.T) {
	root, wtPath := buildFixture(t)
	var calls [][]string
	run := func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return gitro.Output(ctx, dir, args...)
	}
	rep, err := ClassifyLines(context.Background(), Options{Root: root, Run: run, Now: func() time.Time { return fixtureNow }})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) == 0 {
		t.Fatal("ClassifyLines ran no git through the injected runner")
	}
	if rep.Integration != "main" || rep.IntegrationTip == "" {
		t.Fatalf("integration = %q at %q", rep.Integration, rep.IntegrationTip)
	}
	byName := map[string]Line{}
	for _, l := range rep.Lines {
		if _, dup := byName[l.Name]; dup {
			t.Fatalf("line %q reported twice", l.Name)
		}
		byName[l.Name] = l
	}
	want := map[string]struct {
		class  Class
		landed string
	}{
		"old-landed": {Landed, "ancestor"},
		"picked":     {Landed, "patch-equivalent"},
		"squashed":   {Landed, "content-equal"},
		"risky":      {AtRisk, ""},
		"stale":      {Idle, ""},
		"fresh":      {Active, ""},
		"wt-dirty":   {AtRisk, ""},
		"wt-same":    {Landed, "ancestor, worktree content-equal"},
		"stash@{0}":  {AtRisk, ""},
	}
	for name, w := range want {
		l, ok := byName[name]
		if !ok {
			t.Errorf("line %q missing; got %v", name, names(rep.Lines))
			continue
		}
		if l.Class != w.class {
			t.Errorf("%s: class %q, want %q (line %+v)", name, l.Class, w.class, l)
		}
		if l.Landed != w.landed {
			t.Errorf("%s: landed %q, want %q", name, l.Landed, w.landed)
		}
	}
	if len(rep.Lines) != len(want) {
		t.Errorf("lines = %v, want exactly %d (main is never a line)", names(rep.Lines), len(want))
	}

	// Age is carried on the line itself.
	if got := byName["stale"].AgeDays; got != 40 {
		t.Errorf("stale age = %d days, want 40", got)
	}
	if got := byName["fresh"].AgeDays; got != 2 {
		t.Errorf("fresh age = %d days, want 2", got)
	}
	if l := byName["risky"]; !hasPrefix(l.AtRisk, "1 commit on no remote") {
		t.Errorf("risky reasons = %v", l.AtRisk)
	}
	if l := byName["wt-dirty"]; l.Worktree != wtPath || !hasPrefix(l.AtRisk, "dirty worktree") || l.Landed != "" {
		t.Errorf("wt-dirty = %+v (a dirty worktree is never landed)", l)
	}
	if l := byName["stash@{0}"]; l.Kind != KindStash || !hasPrefix(l.AtRisk, "stash") {
		t.Errorf("stash = %+v", l)
	}

	// Each landed line names the command that would remove it; nothing else does.
	for _, l := range rep.Lines {
		if (l.Class == Landed) != (l.Removal != "") {
			t.Errorf("%s: class %s, removal %q", l.Name, l.Class, l.Removal)
		}
	}
	if got := byName["old-landed"].Removal; got != "git branch -d old-landed" {
		t.Errorf("old-landed removal = %q", got)
	}
	if got, wantRm := byName["wt-same"].Removal, "git worktree remove --force "+byName["wt-same"].Worktree+" && git branch -d wt-same"; got != wantRm {
		t.Errorf("wt-same removal = %q, want %q", got, wantRm)
	}
	if l := byName["wt-same"]; len(l.AtRisk) != 0 {
		t.Errorf("a worktree whose edits read the same on main is not at risk: %v", l.AtRisk)
	}
	if got := byName["picked"].Removal; got != "git branch -D picked" {
		t.Errorf("picked removal = %q (patch-equivalent needs -D)", got)
	}

	// Output order is deterministic.
	rep2, err := ClassifyLines(context.Background(), Options{Root: root, Run: gitro.Output, Now: func() time.Time { return fixtureNow }})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(names(rep.Lines)) != fmt.Sprint(names(rep2.Lines)) {
		t.Errorf("order differs between runs: %v vs %v", names(rep.Lines), names(rep2.Lines))
	}
	if rep.Counts[Landed] != 4 || rep.Counts[AtRisk] != 3 {
		t.Errorf("counts = %v", rep.Counts)
	}
}

// TestClassifyLinesClassesOnlyLandedAtRiskIdleActive pins SV-M1's CUT
// (ADR-060 amendment 2026-10-01, O293): the claims-rows and touches-rows
// classes are gone, a line that flips an open plan row or whose diff names an
// open row's symbol is classed by its remaining facts, and no plan file is
// read.
func TestClassifyLinesClassesOnlyLandedAtRiskIdleActive(t *testing.T) {
	if got, want := fmt.Sprint(Classes), fmt.Sprint([]Class{"landed", "at-risk", "idle", "active"}); got != want {
		t.Fatalf("Classes = %s, want %s", got, want)
	}
	f := &fx{t: t, dir: t.TempDir(), when: fixtureNow.Add(-40 * 24 * time.Hour)}
	root := filepath.Join(f.dir, "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	f.git(root, "init", "-q")
	f.commit(root, "base", map[string]string{"docs/plans/p.md": planOpen, "docs/plans/README.md": "- [ ] not a plan {id: R-1}\n", "pkg/foo.go": fooGo})

	// names-idle: pushed 40 days ago; its diff names Bar (open row P-2) under pkg/.
	f.git(root, "switch", "-q", "-c", "names-idle")
	f.commit(root, "bar body", map[string]string{"pkg/foo.go": strings.Replace(fooGo, "return 2", "return 3", 1)})
	f.git(root, "update-ref", "refs/remotes/origin/names-idle", "HEAD")
	f.git(root, "switch", "-q", "main")

	// flips-active: pushed, recent; flips open row P-1 and names Foo.
	f.when = fixtureNow.Add(-2 * 24 * time.Hour)
	f.git(root, "switch", "-q", "-c", "flips-active")
	f.commit(root, "claim P-1", map[string]string{
		"docs/plans/p.md": strings.Replace(planOpen, "- [ ] Foo exists", "- [x] Foo exists", 1),
		"pkg/foo.go":      fooGo + "\n// Foo is proven.\n",
	})
	f.git(root, "update-ref", "refs/remotes/origin/flips-active", "HEAD")
	f.git(root, "switch", "-q", "main")

	// flips-risky: on no remote; flips open row P-2 and changes P-3's oracle path.
	f.git(root, "switch", "-q", "-c", "flips-risky")
	f.commit(root, "claim P-2", map[string]string{
		"docs/plans/p.md":    strings.Replace(planOpen, "- [ ] Bar exists", "- [x] Bar exists", 1),
		"docs/readings/r.md": "ok\n",
	})
	f.git(root, "switch", "-q", "main")
	f.git(root, "update-ref", "refs/remotes/origin/main", "main")

	var calls [][]string
	run := func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return gitro.Output(ctx, dir, args...)
	}
	rep, err := ClassifyLines(context.Background(), Options{Root: root, Run: run, Now: func() time.Time { return fixtureNow }})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Class{"names-idle": Idle, "flips-active": Active, "flips-risky": AtRisk}
	byName := map[string]Line{}
	for _, l := range rep.Lines {
		byName[l.Name] = l
	}
	for name, class := range want {
		l, ok := byName[name]
		if !ok {
			t.Errorf("line %q missing; got %v", name, names(rep.Lines))
			continue
		}
		if l.Class != class {
			t.Errorf("%s: class %q, want %q (line %+v)", name, l.Class, class, l)
		}
	}
	if len(rep.Lines) != len(want) {
		t.Errorf("lines = %v, want exactly %d", names(rep.Lines), len(want))
	}
	if l := byName["flips-risky"]; !hasPrefix(l.AtRisk, "1 commit on no remote") {
		t.Errorf("flips-risky reasons = %v", l.AtRisk)
	}
	known := map[Class]bool{}
	for _, c := range Classes {
		known[c] = true
	}
	for c := range rep.Counts {
		if !known[c] {
			t.Errorf("counts carry class %q, not one of %v", c, Classes)
		}
	}

	// Nothing needs the plan files any more, so none is listed or read.
	for _, args := range calls {
		for _, a := range args {
			if strings.Contains(a, "docs/plans") {
				t.Errorf("ClassifyLines read the plans: git %s", strings.Join(args, " "))
				break
			}
		}
	}
}

func names(ls []Line) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.Name
	}
	return out
}

func hasPrefix(ss []string, p string) bool {
	for _, s := range ss {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// A landed branch checked out in the main worktree cannot be removed with
// `git worktree remove`: the command switches to the integration branch
// first. A detached main worktree is never removed either.
func TestClassifyLinesRemovalInMainWorktree(t *testing.T) {
	f := &fx{t: t, dir: t.TempDir(), when: fixtureNow.Add(-3 * 24 * time.Hour)}
	root := filepath.Join(f.dir, "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	f.git(root, "init", "-q")
	f.commit(root, "base", map[string]string{"README": "x\n"})
	f.git(root, "switch", "-q", "-c", "feat")
	rep, err := ClassifyLines(context.Background(), Options{Root: root, Run: gitro.Output, Now: func() time.Time { return fixtureNow }})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Lines) != 1 || rep.Lines[0].Name != "feat" {
		t.Fatalf("lines = %+v", rep.Lines)
	}
	if got := rep.Lines[0].Removal; got != "git switch main && git branch -d feat" {
		t.Errorf("removal = %q", got)
	}

	f.git(root, "switch", "-q", "--detach", "main")
	rep, err = ClassifyLines(context.Background(), Options{Root: root, Run: gitro.Output, Now: func() time.Time { return fixtureNow }})
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range rep.Lines {
		if l.Kind == KindWorktree && strings.Contains(l.Removal, "worktree remove") {
			t.Errorf("the main worktree named for removal: %+v", l)
		}
	}
}
