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

// DriftedFindings computes remediation drift for finding-kind claims (FT-6,
// docs/vision/03-findings-and-experts.md §2): "has the SCOPED CODE — not the
// cited finding document — changed since compile?" This is a second,
// independent pass beside Sweep's citation check, over a different anchor
// (scope-fp links against code fingerprints, not cites links against doc
// content) — and, deliberately, a PURE READ: it never calls SetClaimStatus.
// code_drifted is a SERVING annotation, not a freshness verdict — scoped code
// changing can mean the finding was FIXED, not that the claim is wrong
// (docs/vision/03 §2 is explicit: never a status flip). Recomputing this
// live at serve time (ObligationsFor calls it directly) rather than
// persisting a drifted flag on the claim keeps both Sweep and ObligationsFor
// pure functions of current store state, with no third claim-status axis to
// keep consistent — the simplest design that satisfies the "never a status
// flip" constraint.
//
// Compile time persists each finding claim's scope-matched files' content
// fingerprints as ClaimLink{Rel: RelScopeFP} (verifyDraft, compile.go) — no
// disk read needed here, only the store's already-maintained fingerprint
// table (the same one the fusion pipeline's hash-gate keeps current on every
// ingest). A scope-fp path with no CURRENT fingerprint at all (re-ingest
// purged it, the file moved, or the service was never re-indexed after a
// corpus move) is treated as drifted: the scoped code's current state cannot
// be verified unchanged, so silence would be dishonest.
func DriftedFindings(ctx context.Context, st store.Store, serviceID string) (map[string]bool, error) {
	claims, err := st.ClaimsForService(ctx, serviceID, "")
	if err != nil {
		return nil, fmt.Errorf("semantic: drift load claims: %w", err)
	}
	var findingIDs []string
	for _, cl := range claims {
		if cl.Kind == KindFinding && cl.Status.Served() {
			findingIDs = append(findingIDs, cl.ClaimID)
		}
	}
	if len(findingIDs) == 0 {
		return nil, nil
	}
	sort.Strings(findingIDs) // deterministic iteration order below

	links, err := st.ClaimLinks(ctx, serviceID, findingIDs)
	if err != nil {
		return nil, fmt.Errorf("semantic: drift load links: %w", err)
	}
	scopeFPByClaim := map[string][]core.ClaimLink{}
	for _, l := range links {
		if l.Rel == core.RelScopeFP {
			scopeFPByClaim[l.ClaimID] = append(scopeFPByClaim[l.ClaimID], l)
		}
	}

	drifted := map[string]bool{}
	for _, id := range findingIDs {
		for _, l := range scopeFPByClaim[id] {
			cur, found, ferr := st.Fingerprint(ctx, serviceID, l.TargetRef)
			if ferr != nil {
				return nil, fmt.Errorf("semantic: drift fingerprint %s: %w", l.TargetRef, ferr)
			}
			if !found || cur != l.TargetHash {
				drifted[id] = true
				break
			}
		}
	}
	return drifted, nil
}
