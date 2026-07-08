package semantic

import (
	"context"
	"fmt"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// SweepReport summarises one citation sweep.
type SweepReport struct {
	ClaimsChecked int
	Fresh         int
	Stale         int
	Contradicted  int
	Flipped       int // status changes written this sweep
}

// Sweep is the token-free post-ingest freshness pass (ADR-016 §4.3): for
// every served claim of the service, re-hash its cited line ranges against
// the current corpus and flip status on drift. fetch returns a corpus file's
// current content by repo-relative path (ok=false when the file is gone).
//
// Status semantics: all citations hash-match → fresh (a stale claim heals if
// the doc is restored); any mismatch → stale; any citation that no longer
// resolves (file gone, range out of bounds) → contradicted (dominant).
// Superseded/refuted claims are never touched — those are terminal verdicts,
// not freshness states.
func Sweep(ctx context.Context, st store.Store, serviceID string, fetch func(path string) ([]byte, bool)) (SweepReport, error) {
	var rep SweepReport
	claims, err := st.ClaimsForService(ctx, serviceID, "")
	if err != nil {
		return rep, fmt.Errorf("semantic: sweep load claims: %w", err)
	}
	served := claims[:0]
	ids := make([]string, 0, len(claims))
	for _, cl := range claims {
		if cl.Status.Served() {
			served = append(served, cl)
			ids = append(ids, cl.ClaimID)
		}
	}
	if len(served) == 0 {
		return rep, nil
	}
	links, err := st.ClaimLinks(ctx, serviceID, ids)
	if err != nil {
		return rep, fmt.Errorf("semantic: sweep load links: %w", err)
	}
	citesByClaim := map[string][]core.ClaimLink{}
	for _, l := range links {
		if l.Rel == core.RelCites {
			citesByClaim[l.ClaimID] = append(citesByClaim[l.ClaimID], l)
		}
	}

	// Deterministic order: by claim ID.
	sort.Slice(served, func(i, j int) bool { return served[i].ClaimID < served[j].ClaimID })

	content := map[string][]byte{} // per-file cache
	missing := map[string]bool{}
	for _, cl := range served {
		rep.ClaimsChecked++
		next := core.ClaimFresh
		for _, cite := range citesByClaim[cl.ClaimID] {
			path, start, end, ok := core.ParseCiteRef(cite.TargetRef)
			if !ok {
				next = core.ClaimContradicted
				break
			}
			data, have := content[path]
			if !have && !missing[path] {
				if d, found := fetch(path); found {
					content[path] = d
					data = d
				} else {
					missing[path] = true
				}
			}
			if missing[path] {
				next = core.ClaimContradicted
				break
			}
			hash, ok := core.HashLines(data, start, end)
			if !ok {
				next = core.ClaimContradicted
				break
			}
			if hash != cite.TargetHash {
				next = core.ClaimStale // keep scanning: a later broken cite still dominates
			}
		}
		switch next {
		case core.ClaimFresh:
			rep.Fresh++
		case core.ClaimStale:
			rep.Stale++
		case core.ClaimContradicted:
			rep.Contradicted++
		}
		if next != cl.Status {
			if err := st.SetClaimStatus(ctx, serviceID, cl.ClaimID, next); err != nil {
				return rep, fmt.Errorf("semantic: sweep flip %s: %w", cl.ClaimID, err)
			}
			rep.Flipped++
		}
	}
	return rep, nil
}
