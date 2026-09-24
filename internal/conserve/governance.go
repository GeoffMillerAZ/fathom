package conserve

// governance.go — the "governance" plane (ADR-044 §2.5): a compiled claim
// (obligations, adr-rules, spec-bindings, findings) whose scope glob(s)
// matched at least one file in the corpus at Base and match none at Head
// is orphaned — the governing rule outlived the code its own scope named.
//
// A DELIBERATE, disclosed exception to "base facts come from the changed
// files only" (vision 34 §3): a claim's scope can be orphaned by files
// nowhere near this particular diff (a directory-wide glob emptied one
// file at a time over several earlier, unrelated changes), so this plane
// reads the FULL file PATH list at Base and Head via `git ls-tree`/`git
// ls-files` — never file CONTENT, and never a re-ingest (design doc §8's
// refused "full base re-ingest" is about re-extracting SYMBOLS from every
// base file; listing paths costs nothing like that).
//
// Registers itself from init() (conserve.go's own doc comment).

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/filter"
)

func init() {
	Register(governancePlane{})
}

type governancePlane struct{}

func (governancePlane) Name() string { return "governance" }

// governanceLayers is the set of compiled-claim layers this plane
// considers (design doc §2.5's own list): obligations (internal/semantic.
// LayerObligations), adr-rules, spec-bindings, findings (internal/
// semantic.LayerFindings) — named as plain strings rather than importing
// internal/semantic, which conserve otherwise has no reason to depend on
// (internal/certify.resolveRule already uses the same literal-string
// convention for "obligations").
var governanceLayers = map[string]bool{
	"obligations":   true,
	"adr-rules":     true,
	"spec-bindings": true,
	"findings":      true,
}

// Check delegates to GovernanceOrphans — the plane wrapper is a one-line
// shim so the exported analysis function (CV-23's own contract symbol)
// can be called and tested directly, mirroring ConsumerLosses/surfacePlane.
func (governancePlane) Check(ctx context.Context, in Input) ([]Finding, []string) {
	return GovernanceOrphans(ctx, in)
}

// GovernanceOrphans lists every file path in the corpus at Base (`git
// ls-tree -r`) and at Head (`git ls-files` for the worktree —
// Head==HeadWorktree, NewInput's only shape today; `git ls-tree -r
// <head>` for a future resolved-sha Head), then, for every
// governance-layer claim with a non-empty Scope, tests whether any of
// its glob(s) (internal/filter's gitignore-style matching, the SAME
// package `.fathom/config.toml` surface patterns use) match at least one
// Base path and none of the Head paths (CV-23, design doc §2.5). Unproven
// (I44) when in.Store, in.Root, or in.Base is unset, or a git listing
// itself fails — never silently "no orphans found" from a listing that
// didn't actually run.
func GovernanceOrphans(ctx context.Context, in Input) ([]Finding, []string) {
	if in.Store == nil {
		return nil, []string{"no store"}
	}
	if in.Root == "" {
		return nil, []string{"no corpus root"}
	}
	if in.Base == "" {
		return nil, []string{"no base ref"}
	}

	baseFiles, err := gitLsTree(ctx, in.Root, in.Base)
	if err != nil {
		return nil, []string{fmt.Sprintf("git ls-tree %s: %v", in.Base, err)}
	}
	headFiles, err := headFileList(ctx, in.Root, in.Head)
	if err != nil {
		return nil, []string{fmt.Sprintf("listing head files: %v", err)}
	}

	claims, err := in.Store.ClaimsForService(ctx, in.Service, "")
	if err != nil {
		return nil, []string{fmt.Sprintf("claims for service: %v", err)}
	}

	var findings []Finding
	seen := map[string]bool{}
	for _, c := range claims {
		if !governanceLayers[c.Layer] || len(c.Scope) == 0 {
			continue
		}
		id := c.Layer + "/" + c.ClaimID
		if seen[id] {
			continue
		}
		seen[id] = true

		m := filter.New(c.Scope)
		if !anyMatch(m, baseFiles) {
			continue // the claim's scope never matched anything at base either — not this plane's concern
		}
		if anyMatch(m, headFiles) {
			continue // still governs at least one file at head — not orphaned
		}

		key := c.Layer + ":" + c.Subject
		findings = append(findings, Finding{
			Plane: "governance", Kind: "orphaned", Key: key,
			Detail:   fmt.Sprintf("%s (%s) scope %v matched files at base, none at head", c.Subject, c.Layer, c.Scope),
			Declared: in.Declarations.Declared(key),
			Remedy:   fmt.Sprintf("retire or re-scope the %s rule for %s", c.Layer, c.Subject),
		})
	}

	sort.Slice(findings, func(i, j int) bool { return findings[i].Key < findings[j].Key })
	return findings, nil
}

func anyMatch(m *filter.Matcher, files []string) bool {
	for _, f := range files {
		if m.Match(f) {
			return true
		}
	}
	return false
}

// gitLsTree lists every file path in root's git tree at ref via `git
// ls-tree -r --name-only <ref>` — paths only, never content.
func gitLsTree(ctx context.Context, root, ref string) ([]string, error) {
	return gitOutputLines(ctx, root, "ls-tree", "-r", "--name-only", ref)
}

// headFileList lists every file path actually present at head: the
// worktree (HeadWorktree, the only shape NewInput produces today) or
// `git ls-tree -r` against a resolved sha (a future `--head <ref>`
// caller, per input.go's own doc comment on HeadWorktree).
//
// For the worktree case, plain `git ls-files` is NOT enough on its own:
// it lists the INDEX, so a file removed from disk but not yet `git
// rm`/`git add`ed still shows up — exactly the uncommitted-deletion case
// HeadWorktree's own doc comment ("uncommitted edits included") must
// see as gone. tracked-minus-deleted (`git ls-files --deleted`) plus
// untracked (`git ls-files --others --exclude-standard`) reconstructs
// the real file set on disk, the same union NewInput's own
// changedFilesSinceBase (input.go) builds for its own worktree read.
func headFileList(ctx context.Context, root, head string) ([]string, error) {
	if head != "" && head != HeadWorktree {
		return gitLsTree(ctx, root, head)
	}
	tracked, err := gitOutputLines(ctx, root, "ls-files")
	if err != nil {
		return nil, err
	}
	deleted, err := gitOutputLines(ctx, root, "ls-files", "--deleted")
	if err != nil {
		return nil, err
	}
	untracked, err := gitOutputLines(ctx, root, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	deletedSet := toSet(deleted)
	out := make([]string, 0, len(tracked)+len(untracked))
	for _, f := range tracked {
		if !deletedSet[f] {
			out = append(out, f)
		}
	}
	return append(out, untracked...), nil
}

func gitOutputLines(ctx context.Context, root string, args ...string) ([]string, error) {
	full := append([]string{"-C", root}, args...)
	out, err := exec.CommandContext(ctx, "git", full...).Output()
	if err != nil {
		return nil, err
	}
	return splitNonEmptyLines(string(out)), nil
}

func splitNonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if l := strings.TrimSpace(line); l != "" {
			out = append(out, l)
		}
	}
	return out
}
