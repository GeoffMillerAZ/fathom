package main

// edithook_dest.go — what every edit-hook answer records about itself.
//
// GL-24 (docs/plans/governance-learning.md, INC-27): 21,247 hook answers from
// 2026-10-03 to 2026-10-10 said nothing and carried no reason, so the 2.3%
// emission share could not be called a leak or a correct silence. Every
// answered beat with emitted=false now carries a reason from one closed
// vocabulary (the silenceReason type), the one the Bash-write path already
// used plus the reasons the other paths had left unnamed.
//
// XM-36 (docs/plans/exploration-maps.md Wave 2d): every file-target answer
// records where the write went (destinationClass) and the target path, the
// post-bash writes included, so XM-37's fold and XM-M8's reading count the
// clone and worktree traffic the hook answers silently today. Recording
// only: nothing here changes what the agent is told.
//
// XM-37 (the same wave): a `clone` or `worktree` target that is tracked in
// its own work tree is folded to its repository-relative key, and the
// indexed repository's rules for that key are served (foldCloneTarget). An
// untracked target there is not folded and stays a silent `foreign-target`.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/worktree"
)

// silenceReason is the closed vocabulary of an edit-hook answer that said
// nothing: silentAnswerMeta takes only this type, so every emitted=false
// answer names one of the constants below and a new reason is a new
// constant here, never a string at a call site.
type silenceReason string

const (
	silenceArmOff             silenceReason = "arm-off"              // the kill switch or an OFF measurement arm
	silenceCapabilityOff      silenceReason = "capability-off"       // `capabilities set obligations|guards off`
	silenceGitignored         silenceReason = "gitignored"           // git ignores the target (EW-55, GL-25)
	silenceForeignTarget      silenceReason = "foreign-target"       // the target sits under no candidate root of the project
	silenceDoubtAlreadyServed silenceReason = "doubt-already-served" // the unnamed-write line was already shown for this directory
	silenceSessionDeduped     silenceReason = "session-deduped"      // unchanged context already injected in this session
	silenceNoRules            silenceReason = "no-rules"             // resolved, and nothing governs it or cites a spec
	silenceNoGuard            silenceReason = "no-guard"             // --pre: no guard names the file or command
	silenceGuardSilent        silenceReason = "guard-silent"         // --pre: a guard matched and had nothing to say
	silenceUnresolved         silenceReason = "unresolved"           // --pre: the daemon could not resolve the guarded file's service
	silenceNoService          silenceReason = "no-service"           // no candidate service at all for the project directory
	silenceDaemonError        silenceReason = "daemon-error"         // every daemon call failed; an error beat names why
	silenceEncodeFailed       silenceReason = "encode-failed"        // the harness codec could not encode the answer
	silenceNoCandidate        silenceReason = "no-candidate"         // prompt-hook: the prompt names no path, package or symbol the index resolves (GL-26)
)

// silentAnswerMeta is the meta of an answered beat that said nothing: it
// sets emitted=false and the reason. meta may be nil. The prompt hook's
// silent answers use it too (GL-26, O822).
func silentAnswerMeta(meta map[string]string, reason silenceReason) map[string]string {
	if meta == nil {
		meta = map[string]string{}
	}
	meta["emitted"] = "false"
	meta["reason"] = string(reason)
	return meta
}

// Destination classes (XM-36): where a file-target answer's target sits.
const (
	destIndexed    = "indexed"    // under a candidate root of the project (the repository the hook serves)
	destClone      = "clone"      // under another clone of that repository
	destWorktree   = "worktree"   // under a git worktree of it
	destGitignored = "gitignored" // inside it, and ignored by git
	destScratch    = "scratch"    // a temp directory or a session scratchpad
	destOutside    = "outside"    // anywhere else
)

// withDestination stamps a file-target answer's meta with the target's
// destination class and its absolute path. An empty target leaves meta as it
// is: an answer that names no file has no destination.
func withDestination(meta map[string]string, projDir, target string) map[string]string {
	if target == "" {
		return meta
	}
	if meta == nil {
		meta = map[string]string{}
	}
	meta["dest_class"] = destinationClass(projDir, target)
	meta["dest_path"] = target
	return meta
}

// destinationClass classifies an absolute target path against the project's
// candidate roots (serviceCandidates: the project directory and its git
// toplevel). File reads and stats only, never a git process, except the one
// `git check-ignore` that confirms an in-process ignore verdict
// (gitIgnoredFile). In order:
//
//   - worktree: the nearest work tree holding the target is a linked worktree
//     (a `.git` file) whose common git directory is a candidate root's;
//   - clone: the nearest work tree is a separate checkout whose `origin`
//     names the same repository as a candidate root's `origin`, or the
//     candidate root itself;
//   - gitignored / indexed: the target sits under a candidate root and git
//     ignores it, or does not;
//   - scratch: the target sits under a temp directory or a scratchpad;
//   - outside: anything else.
func destinationClass(projDir, target string) string {
	if target == "" || !filepath.IsAbs(target) {
		return destOutside
	}
	roots := candidateRoots(projDir)
	if class, _, _ := cloneOrWorktreeOf(roots, target); class != "" {
		return class
	}
	for _, r := range roots {
		if _, ok := relativeToRoot(r, target); ok {
			if editTargetIgnored(projDir, target) {
				return destGitignored
			}
			return destIndexed
		}
	}
	if scratchPath(target) {
		return destScratch
	}
	return destOutside
}

// candidateRoots is serviceCandidates' roots for projDir, in order.
func candidateRoots(projDir string) []string {
	var roots []string
	for _, c := range serviceCandidates(projDir) {
		roots = append(roots, c.root)
	}
	return roots
}

// cloneOrWorktreeOf names the work tree holding target when it is a linked
// worktree of a candidate root (destWorktree) or a separate clone of one
// (destClone), and the candidate root it belongs to; class "" otherwise.
func cloneOrWorktreeOf(roots []string, target string) (class, wt, root string) {
	return worktree.RelatedTarget(roots, target)
}

// The fold markers (XM-37) a clone or worktree target's answered beat
// records under "fold".
const (
	foldTracked   = "tracked"   // tracked in its own work tree: folded to its repository-relative key (fold_key)
	foldUntracked = "untracked" // not tracked there: not folded, and the answer stays a silent foreign-target
)

// foldedRulesNote is the payload's fold marker: the rules shown are the
// indexed repository's for the written path's key, not rules read from the
// clone or worktree itself (which may sit at another commit).
const foldedRulesNote = "the indexed repository's rules for this path"

// cloneFold is foldCloneTarget's answer.
type cloneFold struct {
	class   string // destClone or destWorktree; "" when the target is neither
	tracked bool   // the key is tracked in the target's own work tree
	key     string // the repository-relative key (slash form), when tracked
	path    string // the indexed candidate root joined with key, when tracked
}

// foldCloneTarget is XM-37's fold (XM-M1's repository-relative fold,
// generalized to `git worktree`): when the absolute target sits in a clone
// or a linked worktree of a candidate root of projDir (cloneOrWorktreeOf)
// and its path under that work tree is tracked there (one read-only
// `git ls-files`, trackedInWorkTree), the target folds to that key under the
// indexed root, and the indexed repository's rules for the key are what the
// hook serves. A clone at another commit is served the indexed repository's
// rules for the same key: the rules govern the path in the repository the
// index holds. An untracked path is never folded: a file the clone does not
// track (new there, or present only in the indexed checkout) is never
// answered with rules read for a different file of the same name (O821).
func foldCloneTarget(projDir, target string) cloneFold {
	if target == "" || !filepath.IsAbs(target) {
		return cloneFold{}
	}
	class, wt, root := cloneOrWorktreeOf(candidateRoots(projDir), target)
	if class == "" {
		return cloneFold{}
	}
	f := cloneFold{class: class}
	localCtx, done := testRunContext()
	defer done()
	ctx, cancel := context.WithTimeout(localCtx, time.Second)
	defer cancel()
	match, ok := worktree.Resolve(ctx, target, []worktree.IndexedRoot{{Service: "project", Root: root}})
	if !ok || match.Class == "" {
		return f
	}

	rel, err := filepath.Rel(wt, filepath.Clean(target))
	if err != nil || rel == "." || escapesRoot(rel) {
		return f
	}
	key := filepath.ToSlash(rel)
	if !trackedInWorkTree(ctx, wt, key) {
		return f
	}
	f.tracked, f.key, f.path = true, key, filepath.Join(root, rel)
	return f
}

// trackedInWorkTree reports whether key (slash form, relative to the work
// tree root wt) is in wt's index: `git --literal-pathspecs ls-files -z --
// key`, a fixed argument vector run through gitro (no index refresh, no
// optional lock, no fetch, no prompt) under a one-second deadline. Exact
// match only: a directory key lists the files under it, never itself. Any
// failure reads as untracked (no fold).
func trackedInWorkTree(parent context.Context, wt, key string) bool {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	out, err := gitro.Output(ctx, wt, "--literal-pathspecs", "ls-files", "-z", "--", key)
	return err == nil && string(out) == key+"\x00"
}

// withFold records a clone or worktree target's fold on an answered beat's
// meta: fold=tracked with the key it folded to (fold_key), or
// fold=untracked. A target in neither class leaves meta as it is.
func withFold(meta map[string]string, f cloneFold) map[string]string {
	if f.class == "" {
		return meta
	}
	if meta == nil {
		meta = map[string]string{}
	}
	if f.tracked {
		meta["fold"], meta["fold_key"] = foldTracked, f.key
	} else {
		meta["fold"] = foldUntracked
	}
	return meta
}

// foldLabel is the payload header's fold marker for a folded target, ""
// for any other.
func foldLabel(f cloneFold) string {
	if !f.tracked {
		return ""
	}
	return " (" + f.class + "; " + foldedRulesNote + ")"
}

// scratchPath reports whether p sits under the OS temp directory, /tmp, or a
// directory whose name says it is scratch (tmp, temp, scratch, scratchpad).
func scratchPath(p string) bool {
	rp := resolveSymlinks(p)
	for _, base := range []string{os.TempDir(), "/tmp", "/private/tmp", "/var/folders"} {
		if base == "" {
			continue
		}
		for _, b := range []string{base, resolveSymlinks(base)} {
			if _, ok := relativeToRoot(b, rp); ok {
				return true
			}
		}
	}
	for _, seg := range strings.Split(filepath.ToSlash(filepath.Dir(rp)), "/") {
		switch strings.ToLower(seg) {
		case "tmp", "temp", "scratch", "scratchpad":
			return true
		}
	}
	return false
}
