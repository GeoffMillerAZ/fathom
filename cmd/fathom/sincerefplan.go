package main

// sincerefplan.go — CV-14 (ADR-044 §1.4, I45): `fathom verify --since <ref>`
// git glue. Reads ONE prior version of a plan file — the version at an
// EXPLICIT, caller-named ref, unlike selfskip.go's implicit HEAD/HEAD~1
// choice — and hands it to internal/verify.AnnotateSelfDeclared as the
// comparison base. A single bounded `git show`, never a history walk.

import (
	"os/exec"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// refFileVersion returns rel's content at git ref inside repoDir, or nil
// when unavailable: ref does not resolve, the file did not exist at ref, or
// git itself is unavailable. Mirrors headFileVersion/headParentFileVersion
// (stophook.go, selfskip.go) exactly, parameterized by an arbitrary ref
// instead of HEAD/HEAD~1.
func refFileVersion(repoDir, ref, rel string) []byte {
	//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
	cmd := exec.Command("git", "-C", repoDir, "show", ref+":"+filepath.ToSlash(rel))
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return out
}

// sinceRefPlan returns the plan version AnnotateSelfDeclared compares the
// working tree's plan against for `fathom verify --since <ref>`,
// service-scoped so task IDs resolve the way ParseMarkdownPlan resolves
// them elsewhere.
//
// nil means the comparison is INAPPLICABLE: root is not a git work tree, or
// git is unavailable — AnnotateSelfDeclared treats nil exactly as
// AnnotateSelfSkip does (nothing flagged, no comparison attempted).
//
// Inside a real repo, a ref (or a plan path) that does not resolve there —
// the ref does not exist, or the plan file did not exist yet at that
// point — returns an EMPTY *verify.Plan{Service: service} rather than nil:
// within a repo, "the file wasn't there at ref" is itself informative,
// exactly as priorSelfSkipPlan already treats the analogous case — every
// row at head then reports every declaration it carries as self-declared,
// which is correct: none of them could have predated a base where the row
// did not exist.
func sinceRefPlan(root, rel, ref, service string) *verify.Plan {
	if gitToplevel(root) == "" {
		return nil // not a git repo (or git unavailable) — inapplicable
	}
	md := refFileVersion(root, ref, rel)
	p := verify.ParseMarkdownPlan(service, md)
	return &p
}
