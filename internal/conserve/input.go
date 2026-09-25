package conserve

// input.go — CV-18's Input type, Declarations (I45's "was this excused
// before the change started, or only after"), and NewInput, the one place
// in this package that touches git. Every Plane.Check gets its facts
// through Input alone — BaseFile/HeadFile/ChangedFiles/Store — so a plane
// itself never shells out and never re-derives the change range on its
// own (ADR-044 §0: "Costs scale with the change. Base facts come from the
// changed files only.").

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// HeadWorktree is the Head value NewInput always uses: the actual working
// tree, uncommitted edits included — never a re-ingest, never a re-read of
// HEAD's committed blob. A future `--head <ref>` variant would set Head to
// a resolved sha instead; nothing in this package assumes Head is either
// shape beyond the literal string "worktree" meaning exactly this.
const HeadWorktree = "worktree"

// Input carries everything a Plane.Check needs, resolved ONCE per Run so
// every plane sees the identical facts (ADR-044 §0's "base"/"head", the
// same changed-file set `fathom review --since` already computes).
type Input struct {
	Service string
	Root    string
	// Base is the resolved base commit sha (never a symbolic ref — NewInput
	// resolves it once via `git rev-parse`, so every plane compares against
	// the identical commit even if a branch tip moves mid-run).
	Base string
	// Head is "worktree" (NewInput's only shape today) or a resolved sha —
	// never a symbolic ref, for the same reason Base never is.
	Head string
	// ChangedFiles is `git diff --name-only <Base>` plus untracked files
	// (NewInput), or whatever a caller with its own resolution supplies.
	ChangedFiles []string
	// BaseFile reads path's content AS OF Base (git show <Base>:<path>,
	// for NewInput's callers). ok is false — never an error — when path
	// simply did not exist at Base; that is the ordinary case for most of
	// a diff's files, not a failure.
	BaseFile func(path string) (content []byte, ok bool, err error)
	// HeadFile reads path's content AS OF Head — the worktree file, for
	// NewInput's callers. Same ok/err contract as BaseFile.
	HeadFile func(path string) (content []byte, ok bool, err error)
	// Store is the live index, for a plane that needs it (certificates,
	// governance). nil is valid and means exactly that: a plane needing it
	// reports "no store" in its Missing and goes Unproven (I44) — Input
	// itself never refuses to construct over a nil Store.
	Store store.Store
	// Declarations answers, for an inventory/capability key, whether a
	// plan declared losing or changing it BEFORE the range started
	// ("prior", excuses a finding), only inside the range ("self", excuses
	// nothing — I45), or not at all ("none").
	Declarations Declarations
	// Retired is the base+head union of every retirement CLAIM a set of
	// plans makes (ADR-044 §3.3, resurrection honesty) — a `retires:` key
	// naming the function, or a `gone: import <path>` row naming its
	// package or an import it lived behind. Deliberately NOT
	// Declarations: I45's prior/self split excuses a loss only when the
	// declaration predates the range, but resurrection.go uses Retired
	// for the opposite purpose — telling an ordinary, undeclared rename
	// (nothing to contradict, informational) from a body that reappears
	// AFTER the range explicitly claimed it gone (the declaration is
	// false, a loss) — so a retirement declared inside the very same
	// range that deletes the code still counts as a claim here, per
	// Retired's own doc comment.
	Retired Retirement

	// IndexAtHead/IndexAtHeadReason/FailedTypedPackages are the reading
	// set's own preconditions (design §3.4, I44's "index not at HEAD,
	// typed packages failed") — pre-resolved ONCE per NewInput call
	// exactly like every other Input fact above, so Residue itself stays
	// a pure function of Input and never shells out on its own (this
	// file's own doc comment: "the one place in this package that
	// touches git"). IndexAtHeadReason is always "" when IndexAtHead is
	// true. A caller building Input directly (most tests) sets these
	// itself; NewInput computes them via git plus Store.GetServiceMeta/
	// TypedEdgeStatus, mirroring verify.GoneEvidence's IndexAtHead/
	// FailedTypedPackages methods (cmd/fathom/verify_gone_evidence.go) —
	// the same rule, re-derived here rather than depending on cmd/fathom.
	IndexAtHead         bool
	IndexAtHeadReason   string
	FailedTypedPackages []string

	// ReachUnchangedSince reports whether none of files changed between
	// commit and HEAD (certify.ReachUnchangedSince, O114's rule, applied
	// here without a declared target — design §3.4's "attested" check).
	// nil means no repository is available to ask; Residue then treats
	// every candidate as not attested through this path rather than
	// guessing "unchanged". NewInput sets this to a real git-backed
	// closure (certify.ExecGit against Root); a caller building Input
	// directly (most tests) supplies its own, deterministic fake.
	ReachUnchangedSince func(commit string, files []string) (bool, error)
}

// Declarations is the base/head split of every retires:/surface_change:
// pattern a set of plan files declared — built once per Run (NewInput, or
// NewDeclarations for a caller with its own plan versions) and consulted
// by any plane that needs to know whether a loss was excused ahead of
// time. Patterns match a key exactly, or as a "<prefix>*" wildcard prefix
// (ADR-044 §2.2).
type Declarations struct {
	base []string
	head []string
}

// Declared reports whether key was declared retired/changed at Base
// ("prior"), only somewhere inside base..head ("self" — I45: excuses
// nothing), or not at all ("none"). Pure; matches DeclaredPrior/
// DeclaredSelf/DeclaredNone.
func (d Declarations) Declared(key string) string {
	if declPatternsMatch(d.base, key) {
		return DeclaredPrior
	}
	if declPatternsMatch(d.head, key) {
		return DeclaredSelf
	}
	return DeclaredNone
}

// declPatternsMatch reports whether key matches any of patterns: an exact
// match, or a "<prefix>*" wildcard whose prefix key starts with.
func declPatternsMatch(patterns []string, key string) bool {
	for _, p := range patterns {
		if rest, ok := strings.CutSuffix(p, "*"); ok {
			if strings.HasPrefix(key, rest) {
				return true
			}
			continue
		}
		if p == key {
			return true
		}
	}
	return false
}

// NewDeclarations builds a Declarations from already-parsed plan versions —
// every retires:/surface_change: key from basePlans is a "prior" pattern,
// every one from headPlans (whether or not it also appeared at base — a
// key present at both is still "prior", the stronger of the two) is a
// "self" pattern. A caller that already has both plan versions in hand
// (e.g. a test, or a command that read them for another reason) builds a
// Declarations this way without any git I/O; NewInput uses it as its own
// last step after reading each plan path at both points.
func NewDeclarations(basePlans, headPlans []verify.Plan) Declarations {
	return Declarations{
		base: declPatternsFromPlans(basePlans),
		head: declPatternsFromPlans(headPlans),
	}
}

func declPatternsFromPlans(plans []verify.Plan) []string {
	seen := map[string]bool{}
	for _, p := range plans {
		for _, t := range p.Tasks {
			for _, k := range t.Retires {
				seen[k] = true
			}
			for _, k := range t.SurfaceChange {
				seen[k] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Retirement (ADR-044 §3.3, resurrection honesty) is the base+head UNION
// of every retirement claim a set of plans makes: a `retires:` key
// (matched, like Declarations, as an exact string or a "<prefix>*"
// wildcard) or a `gone: import <path>` row's target. Built once per Run
// by NewInput (or NewRetirement, for a caller/test with its own plan
// versions already in hand) and consulted ONLY by resurrection.go's
// Resurrected — see Input.Retired's own doc comment for why this is a
// distinct type from Declarations rather than a second use of it.
type Retirement struct {
	keys    []string // retires: patterns, base+head combined
	imports []string // gone: import <path> targets, base+head combined
}

// ClaimsKey reports whether a retires: pattern (base or head, either
// counts here) matches key — the exact/"<prefix>*" rule
// declPatternsMatch already implements for Declarations.
func (r Retirement) ClaimsKey(key string) bool {
	return declPatternsMatch(r.keys, key)
}

// ClaimsImport reports whether some `gone: import <path>` row's target
// (base or head) is path itself, or ends in a "/"+path segment equal to
// path — the second form matches "an import it lived behind" a package
// whose OWN directory-derived key (api:<dir>.Name) uses just the
// directory, not a real module-qualified import path: a `gone: import
// example.com/x/legacy` row and a package literally named/dirred "legacy"
// are the same claim by the ordinary Go convention that an import path's
// last segment names its package.
func (r Retirement) ClaimsImport(path string) bool {
	for _, imp := range r.imports {
		if imp == path || strings.HasSuffix(imp, "/"+path) {
			return true
		}
	}
	return false
}

// NewRetirement builds a Retirement from already-parsed plan versions —
// NewDeclarations' own precedent, for a caller (a test, or one that read
// both plan versions for another reason) that wants Retired without any
// git I/O. Every retires: key and gone: import target from EITHER
// basePlans or headPlans counts, unlike NewDeclarations' base/head split:
// see Input.Retired's doc comment for why.
func NewRetirement(basePlans, headPlans []verify.Plan) Retirement {
	var keys, imports []string
	for _, plans := range [][]verify.Plan{basePlans, headPlans} {
		for _, p := range plans {
			for _, t := range p.Tasks {
				keys = append(keys, t.Retires...)
				if imp, ok := goneImportTarget(t.Gone); ok {
					imports = append(imports, imp)
				}
			}
		}
	}
	sort.Strings(keys)
	sort.Strings(imports)
	return Retirement{keys: keys, imports: imports}
}

// goneImportTarget extracts <path> from a `gone: import <path>` row's raw
// Task.Gone text — the same "import " prefix internal/verify.
// parseGoneForm's own "import" case cuts, re-derived here rather than
// exported from internal/verify since this package needs only this one
// form, never module/pattern/callers.
func goneImportTarget(raw string) (string, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(raw), "import ")
	if !ok {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// NewInput resolves baseRef against root's git history and fills every
// Input field from it: Base (the resolved sha), Head ("worktree"),
// ChangedFiles (`git diff --name-only <base>` plus untracked files — the
// same set `fathom review --since` computes, ADR-044 §0), BaseFile/
// HeadFile (git show / worktree reads), and Declarations (built by
// reading each of planPaths — repo-relative — at base and at the
// worktree). st may be nil; it is carried straight onto Input.Store.
//
// Returns an error only for an operational failure resolving baseRef or
// listing changed files (a bad ref, root not a git repo, git
// unavailable) — never for an individual plan path being absent at base
// or at head, which Declarations treats as "no declarations from that
// file at that point," not a fatal condition.
func NewInput(ctx context.Context, root, baseRef string, st store.Store, service string, planPaths []string) (Input, error) {
	base, err := resolveRef(ctx, root, baseRef)
	if err != nil {
		return Input{}, fmt.Errorf("resolving base ref %q in %s: %w", baseRef, root, err)
	}
	changed, err := changedFilesSinceBase(ctx, root, base)
	if err != nil {
		return Input{}, fmt.Errorf("listing changed files since %s: %w", base, err)
	}

	var basePlans, headPlans []verify.Plan
	for _, raw := range planPaths {
		rel := planPathRelativeTo(root, raw)
		if b, ok, _ := gitShowFile(root, base, rel); ok {
			basePlans = append(basePlans, verify.ParseMarkdownPlan(service, b))
		}
		if b, ok, _ := worktreeFile(root, rel); ok {
			headPlans = append(headPlans, verify.ParseMarkdownPlan(service, b))
		}
	}

	indexAtHead, indexAtHeadReason := resolveIndexAtHead(ctx, root, st, service)

	return Input{
		Service:             service,
		Root:                root,
		Base:                base,
		Head:                HeadWorktree,
		ChangedFiles:        changed,
		BaseFile:            func(path string) ([]byte, bool, error) { return gitShowFile(root, base, path) },
		HeadFile:            func(path string) ([]byte, bool, error) { return worktreeFile(root, path) },
		Store:               st,
		Declarations:        NewDeclarations(basePlans, headPlans),
		Retired:             NewRetirement(basePlans, headPlans),
		IndexAtHead:         indexAtHead,
		IndexAtHeadReason:   indexAtHeadReason,
		FailedTypedPackages: resolveFailedTypedPackages(ctx, st, service),
		ReachUnchangedSince: func(commit string, files []string) (bool, error) {
			return certify.ReachUnchangedSince(certify.ExecGit, root, commit, files)
		},
	}, nil
}

// resolveIndexAtHead reports whether store's last-ingested commit for
// service equals git HEAD in root, with a clean working tree — the reading
// set's own precondition (design §3.4, I44's "index not at HEAD"),
// mirroring verify.GoneEvidence's IndexAtHead (cmd/fathom/
// verify_gone_evidence.go) so the SAME rule governs both `gone:` rows and
// the reading set rather than two implementations that could drift.
// atHead=false always carries a named reason, never silently empty.
func resolveIndexAtHead(ctx context.Context, root string, st store.Store, service string) (atHead bool, reason string) {
	if root == "" {
		return false, "no repository root resolved for the reading set's index-freshness check"
	}
	head, err := resolveRef(ctx, root, "HEAD")
	if err != nil {
		return false, fmt.Sprintf("could not resolve git HEAD in %s: %v", root, err)
	}
	if st == nil {
		return false, "no store"
	}
	meta, found, err := st.GetServiceMeta(ctx, service)
	if err != nil {
		return false, fmt.Sprintf("could not read ingest metadata for %q: %v", service, err)
	}
	if !found || meta.IndexedCommit == "" {
		return false, "no commit was recorded for this service's last ingest — re-ingest before evaluating the reading set"
	}
	if meta.IndexedCommit != head {
		return false, fmt.Sprintf("the index was ingested at commit %s, HEAD is now %s — re-ingest", meta.IndexedCommit, head)
	}
	dirty, derr := gitDirty(ctx, root)
	if derr != nil {
		return false, fmt.Sprintf("could not check working tree cleanliness in %s: %v", root, derr)
	}
	if dirty {
		return false, "the working tree has uncommitted changes since the index was ingested"
	}
	return true, ""
}

// resolveFailedTypedPackages reads the store's PERSISTED typed-edge status
// for service (DG-10, store.TypedEdgeStatusRecord) — mirroring
// verify.GoneEvidence's FailedTypedPackages exactly (see that method's own
// doc comment for why the persisted row, not an in-process cache, is what
// survives across the CLI's fresh-process-per-invocation lifetime). nil
// (never an error) when st is nil, unrecorded, or clean.
func resolveFailedTypedPackages(ctx context.Context, st store.Store, service string) []string {
	if st == nil {
		return nil
	}
	rec, ok, err := st.TypedEdgeStatus(ctx, service)
	if err != nil || !ok || !rec.Failed {
		return nil
	}
	reason := rec.Reason
	if reason == "" {
		reason = "typed pass reported failed with no reason recorded"
	}
	return []string{reason}
}

// gitDirty reports whether root's whole working tree carries any
// uncommitted state (staged, unstaged, or untracked) — mirrors
// cmd/fathom/plans_archive.go's gitPathDirty, re-derived here (this
// package cannot import cmd/fathom) with a context for cancellation.
func gitDirty(ctx context.Context, root string) (bool, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

// planPathRelativeTo makes p safe for gitShowFile/worktreeFile, both of
// which require a repo-relative path (NewInput's own doc comment) — an
// absolute p is silently mishandled by each in a different way: git's own
// `rev:path` syntax refuses a leading "/", so gitShowFile's `git show
// <ref>:<p>` always reports the path absent, and worktreeFile's
// filepath.Join(root, p) does not special-case an already-absolute p, so
// it doubles the path (filepath.Join("/a/b", "/a/b/c") == "/a/b/a/b/c")
// into one that never exists. Both failures come back ok=false, err=nil —
// indistinguishable from "this plan genuinely doesn't exist here" — so a
// caller whose plan discovery globs under an absolute root (every
// verifyRepoRoot()-based caller: cmd/fathom's discoverPlanPaths, the sole
// path lefthook.yml's real pre-push invocation ever takes with no --plan)
// silently loses every retires:/surface_change: declaration at both base
// and head (O117, .fathom/observations.md). An absolute p already inside
// root is rewritten relative to it; a p that is already relative, or
// absolute but outside root, is returned unchanged so gitShowFile/
// worktreeFile report it plainly missing rather than silently resolving a
// different path.
func planPathRelativeTo(root, p string) string {
	if !filepath.IsAbs(p) {
		return p
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return p
	}
	return filepath.ToSlash(rel)
}

// resolveRef resolves ref to a commit sha via `git rev-parse`.
func resolveRef(ctx context.Context, root, ref string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", ref).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// changedFilesSinceBase returns the sorted, deduplicated union of `git
// diff --name-only <base>` (every tracked change against base, staged or
// not) and `git ls-files --others --exclude-standard` (untracked files,
// gitignore respected) — a brand-new file never `git add`ed shows up in
// neither the index nor a tracked diff, so it needs the second call
// (ADR-044 §0).
func changedFilesSinceBase(ctx context.Context, root, base string) ([]string, error) {
	diffOut, err := exec.CommandContext(ctx, "git", "-C", root, "diff", "--name-only", base).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only %s: %w", base, err)
	}
	untrackedOut, err := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files --others --exclude-standard: %w", err)
	}
	set := map[string]bool{}
	for _, line := range strings.Split(string(diffOut), "\n") {
		if l := strings.TrimSpace(line); l != "" {
			set[l] = true
		}
	}
	for _, line := range strings.Split(string(untrackedOut), "\n") {
		if l := strings.TrimSpace(line); l != "" {
			set[l] = true
		}
	}
	out := make([]string, 0, len(set))
	for f := range set {
		out = append(out, f)
	}
	sort.Strings(out)
	return out, nil
}

// gitShowFile reads path's content at ref inside root via `git show
// <ref>:<path>`. ok is false — never an error — when git ran but the path
// simply did not exist at ref (the ordinary shape for a new file); err is
// non-nil only when git itself could not be run at all (not installed, not
// on PATH).
func gitShowFile(root, ref, path string) ([]byte, bool, error) {
	cmd := exec.Command("git", "-C", root, "show", ref+":"+filepath.ToSlash(path))
	out, err := cmd.Output()
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return nil, false, err
		}
		return nil, false, nil
	}
	return out, true, nil
}

// worktreeFile reads path's content from the working tree under root. ok
// is false — never an error — when the file simply does not exist there.
// Refuses (a real error) a path that would resolve outside root.
func worktreeFile(root, path string) ([]byte, bool, error) {
	r := filepath.Clean(root)
	full := filepath.Clean(filepath.Join(r, filepath.FromSlash(path)))
	if full != r && !strings.HasPrefix(full, r+string(filepath.Separator)) {
		return nil, false, fmt.Errorf("path %q escapes root %q", path, root)
	}
	b, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return b, true, nil
}
