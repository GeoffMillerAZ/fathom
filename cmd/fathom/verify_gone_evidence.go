package main

// verify_gone_evidence.go — CV-M6 tooling fix (ADR-044 §3.1, CV-27/CV-28):
// cliEvidence's verify.GoneEvidence implementation, wiring `fathom verify
// --store` to actually prove (or honestly refuse to prove) a `gone:` row.
//
// Before this file, cliEvidence implemented Evidence/DetailedEvidence/
// FileEvidence/CertificationEvidence but not GoneEvidence — every `gone:`
// row read Unknown ("no gone evidence available on this evidence source")
// forever, independent of how true the claim was (internal/verify/
// verify.go's own type-assertion fallback). refactor_plan_test.go's
// fakeGoneEvidence comment named this "a later row." Discovered dogfooding
// CV-M6 (docs/plans/retire-unused-analyzers.md): a plan built entirely of
// `gone: import` rows against packages with zero importers could never be
// flipped to [x], since nothing in the CLI path could prove any of them.
//
// GoFiles/CorpusFiles/ModuleFile read the LIVE working tree (c.Sources —
// LocalConnector re-reads disk on every call, ADR-044 §0's Head is always
// the worktree), so they are correct regardless of when the store was last
// ingested. The one fact that genuinely goes stale between an ingest and a
// later edit is the call graph / typed-pass status TypedCallers and
// FailedTypedPackages read from the STORE — so IndexAtHead is the gate that
// protects those two, and (matching cmd/fathom/refactor_plan.go's own
// dirty-tree refusal for the same "today's count must mean today") it
// requires the WHOLE working tree to be clean and at the commit the store
// was ingested from: the ordinary, hermetic sequence this repo's own gate
// commands use is commit, then ingest, then verify — never a re-ingest per
// keystroke.
import (
	"fmt"
	"go/build"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// IndexAtHead implements verify.GoneEvidence: the store's ingested commit
// must equal the working tree's current HEAD, and the tree must carry no
// uncommitted state at all (staged, unstaged, or untracked) — the same
// "today's count must mean today" refusal cmd/fathom/refactor_plan.go
// already applies to `fathom refactor plan`'s own max: seeding. A repo
// with no git available, or a service never ingested with a recorded
// commit, is honestly not-at-HEAD, never a fabricated pass.
func (e *cliEvidence) IndexAtHead() (bool, string) {
	if e.root == "" {
		return false, "no repository root resolved for the gone: precondition check"
	}
	head, err := gitHEAD(e.root)
	if err != nil {
		return false, fmt.Sprintf("could not resolve git HEAD in %s: %v", e.root, err)
	}
	_, indexedCommit, found, err := e.c.ServiceMeta(e.ctx, e.service)
	if err != nil {
		return false, fmt.Sprintf("could not read ingest metadata for %q: %v", e.service, err)
	}
	if !found || indexedCommit == "" {
		return false, "no commit was recorded for this service's last ingest — re-ingest before evaluating gone: rows"
	}
	if indexedCommit != head {
		return false, fmt.Sprintf("the index was ingested at commit %s, HEAD is now %s — re-ingest", shortSHA(indexedCommit), shortSHA(head))
	}
	dirty, derr := gitPathDirty(e.root, ".")
	if derr != nil {
		return false, fmt.Sprintf("could not check working tree cleanliness in %s: %v", e.root, derr)
	}
	if dirty {
		return false, "the working tree has uncommitted changes since the index was ingested"
	}
	return true, ""
}

// FailedTypedPackages implements verify.GoneEvidence from the store's
// PERSISTED typed-edge status (DG-10, internal/store.TypedEdgeStatusRecord)
// — the CLI runs as a fresh process per invocation, so the in-memory,
// process-lifetime fusion.Pipeline.TypedEdgeStatus (what c.TypedEdgeStatus
// reports) never carries anything from the earlier `fathom ingest`
// invocation that actually ran the pass; the persisted row is what survives
// across processes (and daemon restarts, DG-10's own reason for existing).
// The persisted record carries one whole-service Reason, never a per-
// package list, so a real failure renders as a single named entry —
// honest, if less granular than the interface's own doc comment
// aspirationally describes; a clean pass (the overwhelmingly common case,
// including this repository's own "0 failed") returns nil, contributing
// nothing to gonePreconditionReasons.
func (e *cliEvidence) FailedTypedPackages() []string {
	rec, ok, err := e.c.Store().TypedEdgeStatus(e.ctx, e.service)
	if err != nil || !ok || !rec.Failed {
		return nil
	}
	reason := rec.Reason
	if reason == "" {
		reason = "typed pass reported failed with no reason recorded"
	}
	return []string{reason}
}

// GoFiles implements verify.GoneEvidence: every Go source file in the live
// corpus (c.Sources — always the current working tree), each marked
// Excluded when go/build's own MatchFile says the CURRENT build context
// (GOOS/GOARCH, no extra tags) would leave it out — the same evaluation
// `go build`/`go vet` apply, so a real build-tag-gated importer is still
// caught (never silently invisible) rather than skipped without a trace.
func (e *cliEvidence) GoFiles() []verify.GoFile {
	srcs, err := e.c.Sources(e.ctx, e.service)
	if err != nil {
		return nil
	}
	out := make([]verify.GoFile, 0, len(srcs))
	for _, f := range srcs {
		if !strings.HasSuffix(f.Path, ".go") {
			continue
		}
		excluded := ""
		if e.root != "" {
			dir := filepath.Dir(filepath.Join(e.root, filepath.FromSlash(f.Path)))
			base := filepath.Base(f.Path)
			if match, merr := build.Default.MatchFile(dir, base); merr == nil && !match {
				excluded = "build-tag-excluded (go/build context mismatch)"
			}
		}
		out = append(out, verify.GoFile{Path: f.Path, Content: f.Content, Excluded: excluded})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// ModuleFile implements verify.GoneEvidence: go.mod's live content, read
// through the same repo-root FileEvidence every lexical oracle uses.
func (e *cliEvidence) ModuleFile() ([]byte, bool) {
	if e.files == nil {
		return nil, false
	}
	b, err := e.files.ReadRepoFile("go.mod")
	if err != nil {
		return nil, false
	}
	return b, true
}

// CorpusFiles implements verify.GoneEvidence: every live corpus file
// (any kind, not just Go) whose path matches paths — an exact repo-relative
// file, or a "dir/" prefix matching every file under it (docs/plans/
// README.md's own `paths:` contract) — or every file when paths is empty.
func (e *cliEvidence) CorpusFiles(paths []string) []verify.CorpusFile {
	srcs, err := e.c.Sources(e.ctx, e.service)
	if err != nil {
		return nil
	}
	out := make([]verify.CorpusFile, 0, len(srcs))
	for _, f := range srcs {
		if !matchesGonePaths(f.Path, paths) {
			continue
		}
		out = append(out, verify.CorpusFile{Path: f.Path, Content: f.Content})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// matchesGonePaths reports whether p is named exactly by one of paths, or
// lies under one of its "dir/"-prefixed entries. An empty paths matches
// everything (the pattern oracle's own whole-corpus default).
func matchesGonePaths(p string, paths []string) bool {
	if len(paths) == 0 {
		return true
	}
	for _, want := range paths {
		if p == want {
			return true
		}
		if strings.HasSuffix(want, "/") && strings.HasPrefix(p, want) {
			return true
		}
	}
	return false
}

// TypedCallers implements verify.GoneEvidence: symbol's typed-tier
// referents (isResolvedKind — the same "typed, not heuristic" filter
// cmd/fathom/refactor_plan.go's own ratchet-seeding already applies),
// split into ordinary and Test* callers by file suffix. pkg, when set,
// disambiguates a bare name shared by more than one package the same way
// `package:` does for an ordinary symbol row (docs/plans/README.md):
// matches are filtered to that directory, and the row resolves only when
// exactly one distinct canonical symbol remains.
//
// Zero matches or an unresolved-ambiguous filter (more than one distinct
// canonical symbol survives the package: filter, or none does) NEVER reads
// as a proven-zero: a single unlocated referent is reported instead, so the
// row fails loud rather than silently passing a target it could not pin
// down — the same "never falsely proven" rule I44 states for the index
// itself, extended to this contract's own resolution step.
func (e *cliEvidence) TypedCallers(symbol, pkg string) (nonTest, test []verify.GoneReferent) {
	matches := e.FindSymbol(symbol)
	if pkg != "" {
		filtered := make([]core.SymbolNode, 0, len(matches))
		for _, n := range matches {
			if path.Dir(n.FilePath) == pkg {
				filtered = append(filtered, n)
			}
		}
		matches = filtered
	}
	keys := map[string]bool{}
	for _, n := range matches {
		keys[n.Key()] = true
	}
	if len(keys) != 1 {
		return []verify.GoneReferent{{File: symbol, Line: 0}}, nil
	}
	var key string
	for k := range keys {
		key = k
	}
	for _, ci := range callgraph.CallersDetail(e.graph, key) {
		if !isResolvedKind(ci.Kind) {
			continue
		}
		n, ok := e.byKey[ci.Key]
		if !ok {
			continue
		}
		ref := verify.GoneReferent{File: n.FilePath, Line: n.Line}
		if strings.HasSuffix(n.FilePath, "_test.go") {
			test = append(test, ref)
		} else {
			nonTest = append(nonTest, ref)
		}
	}
	sortReferents(nonTest)
	sortReferents(test)
	return nonTest, test
}

// sortReferents orders GoneReferent slices deterministically (file, then
// line) — every Evidence method in this package returns sorted results.
func sortReferents(refs []verify.GoneReferent) {
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].File != refs[j].File {
			return refs[i].File < refs[j].File
		}
		return refs[i].Line < refs[j].Line
	})
}
