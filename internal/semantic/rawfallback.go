package semantic

// rawfallback.go — wave O1 (ADR-037): the product defect the benches found
// is that `obligations` serves NOTHING USEFUL when the compiled layer's
// coverage for the edited path's own scope is incomplete. vs-pilot-5's
// second reading: the obligations tool never served the predicted rule in
// 0 of 4 trials built to test it, because the compiled-rule layer hit
// budget_stopped before reaching the nested guide that mattered.
// guards-pilot-6's second reading: obligations never surfaced the
// task-specific governing rule on 6 of 8 tasks' wired trials — served only
// a fixed generic boilerplate subset of internal/CLAUDE.md, capped before
// the invariant appeared. Meanwhile grep finds the rule in the nearest
// CLAUDE.md in one call.
//
// RawFallbackRules is the fix: when a governing document's section was
// never turned into a compiled claim (no prior claim at its Subject, or a
// digest/prompt-version mismatch — the same freshness test LiveLayerCoverage
// already applies), the section is still a REAL, human-authored rule sitting
// on disk. Rank the layer's own UNCOMPILED sections by the exact same
// governing-scope proximity ranker rankByGoverningScope (VB-8) already
// applies to compiled claims, and serve the nearest ones — flagged
// Tier:"raw" and carrying a Why note — instead of silence or a root-tier
// rule that happens to have compiled.
//
// Deliberately small and deterministic: no LLM call on this path, a fixed
// RawFallbackReserve budget independent of the compiled MaxObligations cap
// (a raw row only ever ADDS to a compiled answer, never displaces one), and
// the same section-splitting/digest machinery LiveLayerCoverage already
// uses — no second corpus-scanning mechanism to keep in sync.
import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
)

// RawFallbackRules ranks corpus's sections that have NO fresh compiled
// claim on layerDef (no prior claim at the section's Subject at all, or a
// digest/prompt-version mismatch — allDigestsMatch, the identical freshness
// test LiveLayerCoverage uses) by proximity to file, via rankByGoverningScope
// — the same VB-8 ranker obligationsCore applies to compiled claims — and
// renders up to max of the nearest as raw Obligation rows: Tier "raw",
// Layer layerDef.Name, Src the section's own citation, Why the caller-
// supplied coverage-defect note. Deterministic: same claims/corpus/file,
// same rows, every call.
//
// Returns (nil, nil) when file is empty, max<=0, the corpus is empty, or no
// candidate section's implied scope (impliedGuideScope) reaches file at
// all — an honest "nothing to add here", never a fabricated row. Returns a
// non-nil error only when layerDef's own prompt identity cannot be
// resolved (promptForLayer) — a load-time defect, not a coverage gap; the
// caller (pkg/client.Client.RawFallbackObligations) treats this as a
// serve-time enrichment failure and degrades to no raw rows, exactly like
// every other optional enrichment in this package.
func RawFallbackRules(claims []core.SemanticClaim, layerDef LayerDef, corpus []core.SourceFile, file string, max int, why string) ([]Obligation, error) {
	if file == "" || max <= 0 || len(corpus) == 0 {
		return nil, nil
	}
	_, promptVersion, err := promptForLayer(layerDef)
	if err != nil {
		return nil, err
	}

	bySubject := map[string][]core.SemanticClaim{}
	for _, cl := range claims {
		if cl.Layer != layerDef.Name {
			continue
		}
		if cl.Status == core.ClaimSuperseded || cl.Status == core.ClaimRefuted {
			continue
		}
		bySubject[cl.Subject] = append(bySubject[cl.Subject], cl)
	}

	type candidate struct {
		sec Section
		gs  governingScope
	}
	var cands []candidate
	docs := append([]core.SourceFile(nil), corpus...)
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	for _, doc := range docs {
		for _, sec := range SplitMarkdownSections(doc.Path, doc.Content) {
			if prior, seen := bySubject[sec.Subject]; seen && allDigestsMatch(prior, sec.Digest, promptVersion) {
				continue // a fresh compiled claim already answers this exact section
			}
			scope := impliedGuideScope(sec.Path)
			// classifyGlob (rankByGoverningScope's own worker) documents
			// that it only classifies PROXIMITY for a glob already known to
			// match — it defensively falls back to root tier for a glob
			// shape it cannot place, rather than reporting no-match. The
			// real match gate is filter.Match, the exact one obligationsCore
			// (fileScopeMatch) already runs on a compiled claim's scope —
			// run it here too, or a nested guide that governs a WHOLLY
			// UNRELATED subtree would default straight to a false root-tier
			// "match".
			if !filter.New(scope).Match(file) {
				continue
			}
			gs := rankByGoverningScope(scope, file)
			if gs.tier < 0 {
				continue // defensive: rankByGoverningScope found no tier despite the match above
			}
			cands = append(cands, candidate{sec: sec, gs: gs})
		}
	}
	if len(cands) == 0 {
		return nil, nil
	}

	// Nearest tier first (VB-8's own proximity order: exact_file < package <
	// parent_dir < root, deeper ancestor wins a parent_dir tie); within a
	// tie, the same three-tier compile-order discipline compileorder.go
	// already established (a nested/root GUIDE's own section order is never
	// defeated by byte count; a root-OTHER document's smaller sections sort
	// first) so a truncated raw answer still reads like a deliberate pick,
	// not an arbitrary one.
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.gs.tier != b.gs.tier {
			return a.gs.tier < b.gs.tier
		}
		if a.gs.depth != b.gs.depth {
			return a.gs.depth > b.gs.depth
		}
		ta, tb := docCompileTier(a.sec.Path), docCompileTier(b.sec.Path)
		if ta != tb {
			return ta < tb
		}
		if ta == docTierRootOther {
			if la, lb := len(a.sec.Text), len(b.sec.Text); la != lb {
				return la < lb
			}
		}
		if a.sec.Path != b.sec.Path {
			return a.sec.Path < b.sec.Path
		}
		return a.sec.StartLine < b.sec.StartLine
	})
	if len(cands) > max {
		cands = cands[:max]
	}

	out := make([]Obligation, 0, len(cands))
	for _, c := range cands {
		out = append(out, Obligation{
			Rule:  rawRuleText(c.sec),
			Src:   fmt.Sprintf("%s#L%d-L%d", c.sec.Path, c.sec.StartLine, c.sec.EndLine),
			Tier:  "raw",
			Layer: layerDef.Name,
			Why:   why,
		})
	}
	return out, nil
}

// impliedGuideScope is a raw candidate section's scope for governing-scope
// ranking purposes, mirroring scopeFloorDir's own nested/root distinction
// (BP-32): a NESTED CLAUDE.md/AGENTS.md floors its section to its own
// directory subtree, exactly as the compiler floors a claim compiled from
// it; every other governing document in the layer's own corpus (the repo
// root's own CLAUDE.md/AGENTS.md, ADR.md, docs/plans/README.md, a
// project-local guard) reaches the whole repo, same as an unanchored
// compiled scope glob would.
func impliedGuideScope(docPath string) []string {
	if dir := scopeFloorDir(docPath); dir != "" {
		return []string{dir + "/**"}
	}
	return []string{"**"}
}

// rawRuleText renders a raw section's served "rule" text: the section's own
// ATX heading, stripped of its "#" markers, so a raw row reads like a named
// rule rather than a raw text dump. A heading-less section (the preamble, or
// a heading this parser could not read back out — SplitMarkdownSections
// itself never fails to find one when it split at a heading, so this is a
// defensive fallback, not an expected path) is named by its own document
// instead: the cited lines (served alongside via the same snippet mechanism
// every compiled rule's citation uses) carry the real content either way.
func rawRuleText(sec Section) string {
	first, _, _ := strings.Cut(sec.Text, "\n")
	first = strings.TrimSpace(first)
	if trimmed := strings.TrimSpace(strings.TrimLeft(first, "#")); trimmed != "" && trimmed != first {
		return trimmed
	}
	return path.Base(sec.Path) + " (untitled section)"
}
