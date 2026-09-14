package semantic

// coverage.go — O54 deliverable 2 (a second reading of pilot vs-pilot-3):
// the same execute-and-flag doctrine ADR-014 already applies elsewhere in
// this repo, applied to a compiled layer's OWN corpus completeness. The
// bug this answers: a PARTIAL compile (sections_compiled < sections_total,
// budget_stopped=true) served the same root-tier answer a FULL compile
// would, with nothing in the response saying the file's own governing
// guide was never compiled at all.
import (
	"context"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// LayerCoverage summarizes how completely a layer's LATEST compile run
// covered its corpus — the obligations MCP tool's `layer_coverage` field.
type LayerCoverage struct {
	SectionsTotal    int      `json:"sections_total"`
	SectionsCompiled int      `json:"sections_compiled"`
	BudgetStopped    bool     `json:"budget_stopped"`
	UncompiledScopes []string `json:"uncompiled_scopes,omitempty"`
}

// ObligationsLayerCoverage reports layer's LATEST compile run completeness
// for serviceID. ok is false when the layer has never been compiled at all
// (no compile_runs row) — an honest absence, never a fabricated
// fully-covered zero value.
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

// NearestUncompiledGuide finds, among cov's uncompiled document scopes, the
// nearest NESTED instruction-file guide (scopeFloorDir != "") that governs
// file — the one whose absence from the compile a caller most needs to be
// told about, so a served answer never looks complete when the file's own
// governing guide simply never got compiled. ok=false when file is empty
// (a root-scope call has no path to be nearest to) or no uncompiled scope
// governs it; among several candidates the DEEPEST (nearest) ancestor
// directory wins, mirroring rankByGoverningScope's own tie-break.
func NearestUncompiledGuide(cov LayerCoverage, file string) (guide string, ok bool) {
	if file == "" {
		return "", false
	}
	fileDir := dirOf(file)
	bestDepth := -1
	for _, doc := range cov.UncompiledScopes {
		dir := scopeFloorDir(doc)
		if dir == "" {
			continue // root-scoped document, never "the nearest guide"
		}
		if dir != fileDir && !strings.HasPrefix(fileDir+"/", dir+"/") {
			continue // not an ancestor of (or equal to) the queried file's dir
		}
		if depth := dirDepth(dir); depth > bestDepth {
			bestDepth = depth
			guide = doc
		}
	}
	return guide, bestDepth >= 0
}
