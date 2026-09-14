package semantic

// layercoverage.go — O54 deliverable 2 (a second reading of pilot
// vs-pilot-3): the same execute-and-flag doctrine ADR-014 already applies
// elsewhere in this repo, applied to a compiled layer's OWN corpus
// completeness. The bug this answers: a PARTIAL compile
// (sections_compiled < sections_total, budget_stopped=true) served the
// same root-tier answer a FULL compile would, with nothing in the response
// saying the file's own governing guide was never compiled at all.
//
// Second review round, two more gaps in the first pass:
//
//   - NearestUncompiledGuide only ever named a NESTED guide — a file with
//     no nested CLAUDE.md above it (most files in most repos) got no note
//     at all even when the repo ROOT's own CLAUDE.md was the uncompiled
//     section. The root guide is now the fallback when no nested guide
//     governs the path.
//   - sections_total/sections_compiled came from the LATEST compile_runs
//     row, which goes stale the moment the corpus grows (a new CLAUDE.md
//     added after a full compile silently read back as "100% covered").
//     LiveLayerCoverage recomputes both from the CURRENT corpus (a cheap
//     split, no LLM) and reports CorpusGrew: sections the current corpus
//     has that NO compile run has ever even seen — distinct from a
//     section that was compiled once but whose content has since changed
//     (a stale digest: still counted as not-compiled, but not "growth").
import (
	"context"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// LayerCoverage summarizes how completely a layer's compile covers its
// corpus — the obligations MCP tool's `layer_coverage` field.
type LayerCoverage struct {
	SectionsTotal    int      `json:"sections_total"`
	SectionsCompiled int      `json:"sections_compiled"`
	BudgetStopped    bool     `json:"budget_stopped"`
	UncompiledScopes []string `json:"uncompiled_scopes,omitempty"`
	// CorpusGrew (O54, second review round) counts CURRENT-corpus sections
	// that NO compile run has ever seen at all — no existing claim of any
	// status for their subject. Populated only by LiveLayerCoverage
	// (ObligationsLayerCoverage's store-only numbers cannot distinguish
	// "never seen" from "budget-stopped before reaching it"). Zero
	// (omitted) when the corpus has not grown past what some run compiled.
	CorpusGrew int `json:"corpus_grew,omitempty"`
}

// ObligationsLayerCoverage reports layer's LATEST compile run completeness
// for serviceID, straight from the recorded compile_runs row — cheap (one
// store query), but SectionsTotal/SectionsCompiled go stale the moment the
// corpus changes since that run (LiveLayerCoverage is the corpus-aware
// sibling serving callers use). ok is false when the layer has never been
// compiled at all (no compile_runs row) — an honest absence, never a
// fabricated fully-covered zero value.
func ObligationsLayerCoverage(ctx context.Context, st store.Store, serviceID, layer string) (LayerCoverage, bool, error) {
	runs, err := st.CompileRuns(ctx, serviceID, layer)
	if err != nil {
		return LayerCoverage{}, false, err
	}
	if len(runs) == 0 {
		return LayerCoverage{}, false, nil
	}
	latest := runs[0] // CompileRuns is newest-first
	return LayerCoverage{
		SectionsTotal:    latest.SectionsTotal,
		SectionsCompiled: latest.SectionsCompiled,
		BudgetStopped:    latest.BudgetStopped,
		UncompiledScopes: latest.UncompiledScopes,
	}, true, nil
}

// LiveLayerCoverage is ObligationsLayerCoverage's live-corpus sibling: it
// recomputes SectionsTotal/SectionsCompiled/UncompiledScopes from the
// CURRENT corpus (a cheap split via SplitMarkdownSections, no LLM call)
// instead of trusting the latest compile_runs row's own counts, which go
// stale the moment the corpus grows past what that run counted. corpus is
// the layer's already-selected source files (semantic.SelectCorpus's
// output) — the caller (pkg/client) resolves it the same way a real
// compile does.
//
// A section counts as "compiled" only when an existing, non-superseded/
// non-refuted claim's subject matches it AND that claim's stored digest
// still matches the section's CURRENT content (allDigestsMatch — the same
// predicate Compile's own digest gate uses). CorpusGrew counts the subset
// of not-compiled sections whose subject has NO existing claim at all —
// never before seen by any run — distinct from a section that WAS
// compiled once but has since changed (a stale digest: not compiled, but
// not "growth" either).
//
// ok is false exactly like ObligationsLayerCoverage: no compile_runs row
// for this layer at all (never attempted) — corpus is never read in that
// case, so an empty/nil corpus argument is harmless.
func LiveLayerCoverage(ctx context.Context, st store.Store, serviceID string, layerDef LayerDef, corpus []core.SourceFile) (LayerCoverage, bool, error) {
	base, found, err := ObligationsLayerCoverage(ctx, st, serviceID, layerDef.Name)
	if err != nil || !found {
		return base, found, err
	}

	_, promptVersion, perr := promptForLayer(layerDef)
	if perr != nil {
		// Cannot resolve the layer's prompt identity (e.g. a project-local
		// pack whose prompt.md went missing) — this is a serve-time
		// enrichment, not a hard dependency; degrade to the last run's own
		// recorded numbers rather than failing obligations serving.
		return base, true, nil
	}

	claims, err := st.ClaimsForService(ctx, serviceID, layerDef.Name)
	if err != nil {
		return LayerCoverage{}, false, err
	}
	bySubject := map[string][]core.SemanticClaim{}
	for _, cl := range claims {
		if cl.Status == core.ClaimSuperseded || cl.Status == core.ClaimRefuted {
			continue
		}
		bySubject[cl.Subject] = append(bySubject[cl.Subject], cl)
	}

	docs := append([]core.SourceFile(nil), corpus...)
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })

	live := LayerCoverage{BudgetStopped: base.BudgetStopped}
	var uncompiled []orderedSection
	for _, doc := range docs {
		for _, sec := range SplitMarkdownSections(doc.Path, doc.Content) {
			live.SectionsTotal++
			prior, seen := bySubject[sec.Subject]
			switch {
			case seen && allDigestsMatch(prior, sec.Digest, promptVersion):
				live.SectionsCompiled++
			case !seen:
				live.CorpusGrew++
				uncompiled = append(uncompiled, orderedSection{doc: doc, sec: sec})
			default:
				uncompiled = append(uncompiled, orderedSection{doc: doc, sec: sec})
			}
		}
	}
	live.UncompiledScopes = uncompiledDocScopes(uncompiled)
	return live, true, nil
}

// NearestUncompiledGuide finds, among cov's uncompiled document scopes, the
// nearest instruction-file guide that governs file — the one whose absence
// from the compile a caller most needs to be told about, so a served
// answer never looks complete when the file's own governing guide simply
// never got compiled (never seen at all, per LiveLayerCoverage's
// CorpusGrew, counts exactly the same as a stale one here — both simply
// appear in UncompiledScopes). ok=false when file is empty (a root-scope
// call has no path to be nearest to) or no uncompiled scope governs it.
//
// Two passes, nearest-scope-first (mirroring rankByGoverningScope's own
// tiers): a NESTED guide (scopeFloorDir != "") that is an ancestor of (or
// equal to) file's directory wins outright — among several, the DEEPEST
// wins. Only when NO nested guide governs the path at all does the repo
// ROOT's own CLAUDE.md/AGENTS.md (instructionFileBases, never any other
// root-scoped document — ADR.md is never "the nearest guide" for
// anything) serve as the fallback: it governs every file by construction,
// so its own incompleteness is still worth a note even when no nearer
// guide exists to blame instead.
func NearestUncompiledGuide(cov LayerCoverage, file string) (guide string, ok bool) {
	if file == "" {
		return "", false
	}
	fileDir := dirOf(file)
	bestDepth := -1
	rootGuide := ""
	for _, doc := range cov.UncompiledScopes {
		dir := scopeFloorDir(doc)
		if dir == "" {
			if instructionFileBases[doc] {
				rootGuide = doc // the repo root's own guide — fallback only
			}
			continue // never any other root-scoped document (e.g. ADR.md)
		}
		if dir != fileDir && !strings.HasPrefix(fileDir+"/", dir+"/") {
			continue // not an ancestor of (or equal to) the queried file's dir
		}
		if depth := dirDepth(dir); depth > bestDepth {
			bestDepth = depth
			guide = doc
		}
	}
	if bestDepth >= 0 {
		return guide, true
	}
	if rootGuide != "" {
		return rootGuide, true
	}
	return "", false
}
