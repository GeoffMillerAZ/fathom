package mcp

// selfskip.go — O56 defect 2, server-side half: verify_plan is usually
// reached through the CLI (`fathom verify`), which does its own local
// git-aware self-skip overlay (cmd/fathom/selfskip.go) after the call
// returns, since the daemon does not have the CALLER's working tree. But an
// agent can also call verify_plan directly over MCP — and this daemon
// typically DOES have git access to the SERVICE's own corpus root on the
// same filesystem it indexed (exactly what currentGitHead above already
// assumes for staleness_report). When the caller supplies plan_path (the
// plan file's repo-relative path), this lets verify_plan run the SAME
// bounded, git-log/blame-free comparison the CLI runs, so a direct MCP call
// catches a self-skip too — not only a call routed through `fathom verify`.
// Best effort throughout: any git failure degrades to "no prior"
// (AnnotateSelfSkip treats nil as inapplicable — the skip stands), never a
// false pass and never a false self-skip.

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// gitShowFile returns rel's content at ref within the git repo at repoPath,
// or nil when unavailable (not a repo, ref/path absent, git missing).
func gitShowFile(ctx context.Context, repoPath, ref, rel string) []byte {
	if repoPath == "" {
		return nil
	}
	out, err := exec.CommandContext(ctx, "git", "-C", repoPath, "show", ref+":"+filepath.ToSlash(rel)).Output()
	if err != nil {
		return nil
	}
	return out
}

// gitIsRepo reports whether repoPath is inside a git work tree.
func gitIsRepo(ctx context.Context, repoPath string) bool {
	if repoPath == "" {
		return false
	}
	return exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "--git-dir").Run() == nil
}

// selfSkipPriorPlan mirrors cmd/fathom's priorSelfSkipPlan exactly — the
// same two-mode, O(1), git-log/blame-free choice of comparison target: HEAD
// when workingTreeMD (the plan_markdown argument, which IS the caller's
// working tree when the caller is the CLI or an editor-integrated agent)
// differs from HEAD's own content, else HEAD's first parent. nil
// (inapplicable) when repoPath is not a real git work tree.
func selfSkipPriorPlan(ctx context.Context, repoPath, rel, service string, workingTreeMD []byte) *verify.Plan {
	if !gitIsRepo(ctx, repoPath) {
		return nil
	}
	headMD := gitShowFile(ctx, repoPath, "HEAD", rel)
	if headMD != nil && !bytes.Equal(headMD, workingTreeMD) {
		p := verify.ParseMarkdownPlan(service, headMD)
		return &p
	}
	p := verify.ParseMarkdownPlan(service, gitShowFile(ctx, repoPath, "HEAD~1", rel))
	return &p
}
