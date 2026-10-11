package main

// edithook_pinstamp.go — PR-13 (docs/plans/served-not-kept.md Wave 3,
// ADR-067 decision 10): the pin check's HEAD stamp.
//
// The pin check (edithook_pin.go, PR-3) read the new commit's sha from git's
// `[branch sha]` summary line, and skipped every commit that printed none:
// 744 of 745 checks to 2026-10-10 were `no_summary` skips (`git commit -q`,
// a commit inside a compound command, `--amend` quietly, `-C <dir>`). On
// PreToolUse(Bash) the hook now stamps HEAD before a git commit statement,
// and the PostToolUse(Bash) check, finding no summary line, compares HEAD to
// the stamp: the new commits are HEAD's first-parent ancestors down to the
// stamp (an amend is HEAD alone, sharing the stamp's parent). HEAD unmoved
// is `no_commit` (the commit did not land, or `--porcelain` dry-ran), HEAD
// moved some other way is `head_moved`; nothing is checked for either.
//
// The stamp is a guard stamp (guardstamp.go, the pre/post pair mechanism):
// keyed by (session, a key no service can carry, the work-tree root), its
// one rule key the HEAD commit, single-use, and honored for headStampTTL. No
// new cache, table or job. A missing or stale stamp falls back to the
// summary line, as before. HEAD is read from the repository's files
// (gitro.Head): no git process on the pre path for a readable layout, and
// none at all for a command that names no git commit.

import (
	"context"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/harness"
)

// headStampService is the guard-stamp key's service slot for a HEAD stamp:
// a NUL byte no service name carries, so a HEAD stamp never collides with a
// guard's.
const headStampService = "\x00pin-head"

// headStampKeyPrefix marks the stamp's one rule key as a HEAD commit.
const headStampKeyPrefix = "head\x1f"

// headStampTTL bounds how long a HEAD stamp is honored: above the Bash
// tool's 10-minute ceiling, so the stamp outlives the command it precedes
// and no later one.
const headStampTTL = 15 * time.Minute

// pinWalkMax bounds the first-parent walk from HEAD back to the stamp: the
// commits one command makes, never a history.
const pinWalkMax = 8

// The skips a HEAD stamp names (beside the summary path's no_summary).
const (
	pinSkipNoCommit  pinSkip = "no_commit"  // HEAD did not move: the commit did not land, or a dry run
	pinSkipHeadMoved pinSkip = "head_moved" // HEAD moved, and not by commits on top of the stamp (a checkout, a reset)
)

// pinCommitRoot is the work-tree root the last `git commit` statement in
// hook's command ran in (cd, pushd and -C followed), found by stat; ok false
// when the command runs no git commit or the directory is in no work tree.
func pinCommitRoot(codec harness.Codec, hook harness.HookInput) (string, bool) {
	at, ok := lastGitCommitDir(hook.Command)
	if !ok {
		return "", false
	}
	start := hook.Cwd
	if start == "" {
		start = resolveProjDirFor(codec.ID(), "")
	}
	dir := start
	if at.known {
		dir = resolveBashDir(start, at.dir)
	}
	root := gitToplevelByStat(dir)
	return root, root != ""
}

// stampHeadBeforeBash is PR-13's pre half: on PreToolUse(Bash), for a
// command that runs a git commit, it stamps the work tree's HEAD for the
// session. A command that names no git commit returns at lastGitCommitDir's
// string gate; a session with no id, a pin check that is off, or a HEAD that
// cannot be read stamps nothing (the post half then falls back to the
// summary line).
func stampHeadBeforeBash(codec harness.Codec, hook harness.HookInput) {
	if hook.Tool != harness.ToolShell || hook.FilePath != "" || hook.SessionID == "" {
		return
	}
	root, ok := pinCommitRoot(codec, hook)
	if !ok || pinCheckMode(root) == config.PinCheckOff {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), pinStageBudget)
	defer cancel()
	head, err := gitro.Head(ctx, root)
	if err != nil || !gitro.IsObjectName(head) {
		return
	}
	writeHeadStamp(hook.SessionID, root, head, time.Now())
}

// writeHeadStamp stamps head for (sessionID, root) through the guard-stamp
// mechanism.
func writeHeadStamp(sessionID, root, head string, now time.Time) {
	writeGuardStamp(sessionID, headStampService, root, []string{headStampKeyPrefix + head}, false, now)
}

// takeHeadStamp returns the HEAD stamped for (sessionID, root) and deletes
// the stamp (single-use, fresh or not); ok false when there is none, it is
// older than headStampTTL, or it does not hold one commit.
func takeHeadStamp(sessionID, root string, now time.Time) (string, bool) {
	st, ok := readGuardStamp(sessionID, headStampService, root, now)
	if !ok {
		return "", false
	}
	deleteGuardStamp(sessionID, headStampService, root)
	at, err := time.Parse(time.RFC3339, st.WrittenAt)
	if err != nil || now.Sub(at) > headStampTTL || len(st.RuleKeys) != 1 {
		return "", false
	}
	head, found := strings.CutPrefix(st.RuleKeys[0], headStampKeyPrefix)
	if !found || !gitro.IsObjectName(head) {
		return "", false
	}
	return head, true
}

// commitsSinceStamp returns the commits made on top of stamp, oldest first:
// HEAD's first-parent ancestors down to (not including) stamp, at most
// pinWalkMax of them, or HEAD alone when it is an amend of stamp (the two
// share a first parent). no_commit when HEAD is stamp, head_moved when
// neither holds.
func commitsSinceStamp(ctx context.Context, root, stamp string) ([]string, error) {
	head, err := gitro.Head(ctx, root)
	if err != nil {
		return nil, err
	}
	if head == stamp {
		return nil, pinSkipNoCommit
	}
	batch, err := startCatFileBatch(ctx, root)
	if err != nil {
		return nil, err
	}
	defer batch.close()
	parentOf := func(sha string) (string, error) {
		c, err := batch.get(sha)
		if err != nil {
			return "", err
		}
		if !c.ok || c.typ != "commit" {
			return "", pinSkipNotFound
		}
		return firstParent(c.body), nil
	}
	var walked []string
	for c := head; c != "" && len(walked) < pinWalkMax; {
		walked = append(walked, c)
		p, err := parentOf(c)
		if err != nil {
			return nil, err
		}
		if p == stamp {
			for i, j := 0, len(walked)-1; i < j; i, j = i+1, j-1 {
				walked[i], walked[j] = walked[j], walked[i]
			}
			return walked, nil
		}
		if c == head {
			sp, err := parentOf(stamp)
			if err != nil {
				return nil, err
			}
			if p != "" && p == sp {
				return []string{head}, nil // an amend of the stamped commit
			}
		}
		c = p
	}
	return nil, pinSkipHeadMoved
}
