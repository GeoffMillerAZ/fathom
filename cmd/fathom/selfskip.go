package main

// selfskip.go — O56 defect 2 (git glue): fathom verify may not let an agent
// grade its own skip_test escape. The pure comparison lives in
// internal/verify.AnnotateSelfSkip/OverlaySelfSkip; this file supplies the
// ONE prior plan version it compares the working tree's plan against,
// chosen deterministically without git log -L or git blame:
//
//   - the plan file differs from HEAD (an uncommitted edit) → compare
//     against HEAD: catches a skip added in the working tree right before
//     committing (O56's first worker, who added `skip_test: true` to its
//     own row "to make the automated verifier pass cleanly").
//   - the plan file matches HEAD (nothing uncommitted — the common case at
//     push time, when the working tree already equals HEAD) → compare HEAD
//     against its own first parent: catches a skip added in the SAME
//     commit that flipped the row's box to [x] (O56's second worker, who
//     "checked a box straight past a fail").
//
// A skip present at whichever of those two the row compares against
// "predates the claim" and stands, full stop — this is a bounded, O(1) git
// read either way, never a history walk. Outside a git repo entirely (no
// .git, git unavailable), the comparison is inapplicable and nothing is
// flagged — the SAME fail-open discipline verify uses for any other
// operational limit.

import (
	"bytes"
	"os/exec"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// headParentFileVersion returns rel's content at git HEAD's first parent
// (HEAD~1), or nil when unavailable (HEAD has no parent — the root commit —
// or the path did not exist there).
func headParentFileVersion(repoDir, rel string) []byte {
	//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
	cmd := exec.Command("git", "-C", repoDir, "show", "HEAD~1:"+filepath.ToSlash(rel))
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return out
}

// priorSelfSkipPlan returns the ONE prior plan version AnnotateSelfSkip
// compares the working tree's plan against, service-scoped so task IDs
// resolve the same way ParseMarkdownPlan resolves them elsewhere. nil means
// the check is inapplicable (root is not a git work tree) — AnnotateSelfSkip
// treats that as "stands," never as "added." Inside a real repo, an empty
// prior Plan (the ref lacked the file, or lacked a parent commit) is
// returned as &verify.Plan{} rather than nil: within a repo, "the file
// wasn't there yet" IS informative — it means a currently-claimed skipped
// row's skip was, in fact, just added.
func priorSelfSkipPlan(root, rel, service string, workingTreeMD []byte) *verify.Plan {
	return priorSelfSkipPlanWith(root, rel, service, workingTreeMD, verify.ParseMarkdownPlan)
}

// priorSelfSkipPlanWith is priorSelfSkipPlan reading the prior version with
// parse — a foreign spec file's own dialect (ADR-049), so its task ids
// match the working tree's.
func priorSelfSkipPlanWith(root, rel, service string, workingTreeMD []byte, parse func(string, []byte) verify.Plan) *verify.Plan {
	if gitToplevel(root) == "" {
		return nil // not a git repo (or git unavailable) — inapplicable
	}
	// A plan moved in the working tree or in HEAD's own commit is read at
	// the path it had (renamedFrom), so the move flags nothing by itself.
	headRel, headMD := rel, headFileVersion(root, rel)
	if headMD == nil {
		if old := renamedFrom(root, "HEAD", "", rel); old != "" {
			headRel, headMD = old, headFileVersion(root, old)
		}
	}
	if headMD != nil && !bytes.Equal(headMD, workingTreeMD) {
		p := parse(service, headMD)
		return &p
	}
	prev := headParentFileVersion(root, headRel)
	if prev == nil {
		if old := renamedFrom(root, "HEAD~1", "HEAD", headRel); old != "" {
			prev = refFileVersion(root, "HEAD~1", old)
		}
	}
	p := parse(service, prev)
	return &p
}
