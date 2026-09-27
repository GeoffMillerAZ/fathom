package certify

// git.go — reachUnchangedSince (CV-4, design doc §1.1, O114): a
// git-backed, injected check for whether a certificate's reach — its
// test file plus the files of the symbols it reaches — changed since the
// commit its newest passing bound run was attested at. Injected
// (GitRunner) so CertificateStateAt's own tests never shell to a real
// git binary — only reachUnchangedSince's own test
// (TestReachUnchangedSince_CommittedAndWorktreeEdits) needs a real
// repository.

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/gitro"
)

// GitRunner runs a git command with the given args inside dir and returns
// its combined output — the same injectable-exec shape
// internal/reviewbench.GitRunner and cmd/fathom/review.go's own gitRunner
// already use, so a caller can fake it in a test without a real git
// repository.
type GitRunner func(dir string, args ...string) ([]byte, error)

// ExecGit is the real GitRunner: os/exec against the system git binary.
// Its callers only read (rev-parse, show, ls-tree, a diff against the
// working tree), so it runs through gitro: a `git diff` would otherwise
// refresh and rewrite the repository's .git/index as a side effect — from a
// console page view, in a checkout an agent is committing in.
func ExecGit(dir string, args ...string) ([]byte, error) {
	return ExecGitContext(context.Background())(dir, args...)
}

// ExecGitContext is ExecGit under ctx: git is stopped when ctx ends, so a
// caller's deadline (the console's conserve report, RT-118) reaches every
// git its reads spawn.
func ExecGitContext(ctx context.Context) GitRunner {
	return func(dir string, args ...string) ([]byte, error) {
		out, err := gitro.CombinedOutput(ctx, dir, args...)
		if err != nil {
			return out, fmt.Errorf("git %s (in %s): %w\n%s", strings.Join(args, " "), dir, err, out)
		}
		return out, nil
	}
}

// reachUnchangedSince (CV-4, design doc §1.1) reports whether none of
// files changed between commit and the CURRENT WORKING TREE — committed
// history and uncommitted worktree edits alike, in one read: `git diff
// <commit> -- <files>` is a single-ref diff, which compares commit's tree
// against the working tree directly (the same one-sided git semantics
// design doc §0 uses for the base..head changed-file set), so a file
// edited but never committed shows up exactly like one committed after
// commit — no separate committed-vs-uncommitted union to get wrong.
//
// An empty files list is vacuously unchanged (nothing to check). A
// missing commit is refused rather than silently answered "unchanged":
// an empty commit reaching here would be a caller bug (CertificateStateAt
// only ever calls this with a bound run's own commit, which is never
// empty by construction — see its own doc comment), and a bug here must
// never masquerade as "nothing changed."
func reachUnchangedSince(run GitRunner, dir, commit string, files []string) (bool, error) {
	if commit == "" {
		return false, fmt.Errorf("reachUnchangedSince: empty commit")
	}
	if len(files) == 0 {
		return true, nil
	}
	args := append([]string{"diff", "--name-only", commit, "--"}, files...)
	out, err := run(dir, args...)
	if err != nil {
		return false, fmt.Errorf("reachUnchangedSince: git diff %s: %w", commit, err)
	}
	return len(bytes.TrimSpace(out)) == 0, nil
}

// ReachUnchangedSince exports reachUnchangedSince for a caller outside this
// package that needs the IDENTICAL O114 rule — "unchanged" defined exactly
// once, never a second implementation that could quietly drift from the
// certificate ladder's own. internal/conserve's reading set (design
// §3.4, ADR-044 amended 2026-09-25) is the first such caller: "attested"
// applies this same check to a reaching test's file plus the changed
// function's own file, without a declared target.
func ReachUnchangedSince(run GitRunner, dir, commit string, files []string) (bool, error) {
	return reachUnchangedSince(run, dir, commit, files)
}
