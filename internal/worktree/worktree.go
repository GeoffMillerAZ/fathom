// Package worktree detects linked git worktrees so an ingest never re-roots
// an existing service at one of its own worktrees (PC-33,
// docs/plans/proof-credibility.md; invariant I30; observation O22).
//
// O22: a worktree's post-commit hook called the daemon's `ingest` tool with
// its own directory as `path`; the daemon accepted it, re-rooted the fleet's
// "fathom" service at .claude/worktrees/…, answered {ok:true} four times,
// and every "refreshed" verify afterwards read a tree that was not main —
// honestly fresh about the wrong tree. Two signals name a linked worktree,
// cheapest first: the Claude Code agent-worktree marker in the path
// ("/.claude/worktrees/"), and git's own answer (`rev-parse
// --git-common-dir` differs from `--git-dir`). Both callers of the check —
// the daemon's ingest handler (internal/mcp) and the CLI's store-direct
// ingest (pkg/client) — share CheckReroot, so the refusal is one rule with
// one wording.
package worktree

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/gitro"
)

// Marker is the path segment Claude Code's agent worktrees live under. Any
// path containing it is a worktree regardless of what git says (a re-root
// there is refused even when git is unavailable or the marker directory is
// not yet a checkout).
const Marker = "/.claude/worktrees/"

// RerootError is the typed refusal: re-rooting Service from Recorded to
// Requested was declined because Requested is a linked worktree of the
// repository Recorded is (Reason says which signal fired). The message
// carries both paths so the caller — a hook, an agent, a person — sees
// exactly what was left alone and where the truth still lives.
type RerootError struct {
	Service   string
	Recorded  string
	Requested string
	MainRoot  string
	Reason    string
}

func (e *RerootError) Error() string {
	return fmt.Sprintf("worktree re-root refused: service %q stays rooted at %s; %s is a git worktree (%s) — ingest from the main checkout, or register the worktree as its own service",
		e.Service, e.Recorded, e.Requested, e.Reason)
}

// CheckReroot decides whether service, currently recorded at recorded, may
// be re-rooted at requested. It returns nil for a new service (recorded ==
// ""), for the same path, for a requested path that is not a linked
// worktree, and for a linked worktree of some OTHER repository; it returns
// a *RerootError when requested lies under Marker, or when requested is a
// linked worktree sharing its git common dir with recorded. Moving a
// service from a worktree BACK to its main checkout is allowed — that is the
// O22 fix itself, not the O22 failure.
func CheckReroot(service, recorded, requested string) error {
	if recorded == "" || requested == "" {
		return nil
	}
	rec, req := canonical(recorded), canonical(requested)
	if rec == req {
		return nil
	}
	if strings.Contains(filepath.ToSlash(req)+"/", Marker) {
		main := req[:strings.Index(filepath.ToSlash(req), Marker)]
		return &RerootError{Service: service, Recorded: rec, Requested: req, MainRoot: main, Reason: "under " + strings.Trim(Marker, "/")}
	}
	main, linked := Linked(req)
	if !linked {
		return nil
	}
	reqCommon, ok1 := CommonDir(req)
	recCommon, ok2 := CommonDir(rec)
	if ok1 && ok2 && reqCommon == recCommon {
		return &RerootError{Service: service, Recorded: rec, Requested: req, MainRoot: main, Reason: "shares its git common dir " + reqCommon + " with the recorded root"}
	}
	return nil
}

// Linked reports whether dir lies inside a linked git worktree (`git
// worktree add`) rather than a repository's main working tree, and returns
// the main working tree's root when it does: the prefix before Marker when
// the path carries it, else the parent of the git common dir when git's
// `--git-dir` and `--git-common-dir` disagree. A directory that is not a
// git checkout, or one git cannot answer for, is reported as not linked.
func Linked(dir string) (mainRoot string, ok bool) {
	slash := filepath.ToSlash(dir)
	if i := strings.Index(slash+"/", Marker); i >= 0 {
		return filepath.FromSlash(slash[:i]), true
	}
	//fathom:tenancy-allow exec-noctx git worktree probe used by CLI verbs and hooks; its exported API takes no context (a context-taking variant is follow-up work)
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--git-dir", "--git-common-dir").Output()
	if err != nil {
		return "", false
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		return "", false
	}
	gitDir, common := absFrom(dir, strings.TrimSpace(lines[0])), absFrom(dir, strings.TrimSpace(lines[1]))
	if gitDir == common {
		return "", false
	}
	return filepath.Dir(common), true
}

// CommonDir returns the canonical absolute git common dir for dir (the
// shared .git of a main checkout and all its linked worktrees), or ok=false
// when dir is not inside a git repository git can answer for.
func CommonDir(dir string) (string, bool) {
	//fathom:tenancy-allow exec-noctx git worktree probe used by CLI verbs and hooks; its exported API takes no context (a context-taking variant is follow-up work)
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return "", false
	}
	common := strings.TrimSpace(string(out))
	if common == "" {
		return "", false
	}
	return absFrom(dir, common), true
}

// RepositoryIdentity names the repository dir is a checkout of: its root
// commit or commits (`rev-list --max-parents=0 HEAD`), which every clone and
// worktree of one repository shares and no other repository does — a remote
// URL differs from clone to clone and the common dir differs between
// clones, so neither names a repository. ok is false when dir is not a git
// checkout git can answer for, or HEAD has no commit. Read-only (gitro).
func RepositoryIdentity(ctx context.Context, dir string) (id string, ok bool) {
	out, err := gitro.Output(ctx, dir, "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return "", false
	}
	roots := strings.Fields(string(out))
	if len(roots) == 0 {
		return "", false
	}
	sort.Strings(roots)
	return strings.Join(roots, ","), true
}

// absFrom resolves p — as git printed it, relative to dir when not absolute
// — to a canonical absolute path.
func absFrom(dir, p string) string {
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	return canonical(p)
}

// canonical resolves symlinks (macOS's /var → /private/var) and cleans p;
// a path that does not exist is only cleaned.
func canonical(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}
