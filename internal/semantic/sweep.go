package semantic

import (
	"context"
	"fmt"
	"sort"
	"time"

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
//
// P1-9 vocabulary (docs/plans/pulled-tools-excellence.md; see also
// internal/semantic/serve.go and docs/design/semantic-layers.md): "contradicted"
// and "refuted" name two DIFFERENT things and must never share a verdict
// string. Contradicted is this function's own machine-detected, anchor-dead
// state — a claim whose citation no longer resolves; it is served WITH the
// status label (Served() includes ClaimContradicted) so a caller sees the
// flagged answer, never silence, pending a recompile that either heals it or
// confirms it. Refuted (core.ClaimRefuted) is a deliberate human — or
// human-equivalent — judgment via `fathom claim refute` / the obligations
// tool's feedback action (D2-1): a terminal verdict that is NEVER served
// again. Sweep only ever produces the former.
//
// D2-2 (docs/plans/expert-depth.md): a transition INTO contradicted — an
// anchor-dead claim, the strongest drift signal short of a human refutation —
// also appends a ClaimEvent{Verdict:"contradicted", Author:"sweep"} beside
// the status flip, so the automated judgment leaves the same append-only
// audit trail a human `fathom claim refute` / the obligations tool's feedback
// action (D2-1) would leave for its own "refuted" verdict — same mechanism,
// distinct vocabulary, because they are distinct judgments. Only fired on the
// transition (next != cl.Status), not on every sweep of an already-
// contradicted claim — durability needs one event per verdict, not unbounded
// re-affirmation noise. now is the injected clock (repo convention, mirrors
// Compiler.Now); nil uses time.Now.
func Sweep(ctx context.Context, st store.Store, serviceID string, fetch func(path string) ([]byte, bool), now func() time.Time) (SweepReport, error) {
	clock := time.Now
	if now != nil {
		clock = now
	}
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
		deadCite := "" // the citation that killed the anchor, for the refute-event note
		for _, cite := range citesByClaim[cl.ClaimID] {
			path, start, end, ok := core.ParseCiteRef(cite.TargetRef)
			if !ok {
				next = core.ClaimContradicted
				deadCite = cite.TargetRef
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
				deadCite = cite.TargetRef
				break
			}
			hash, ok := core.HashLines(data, start, end)
			if !ok {
				next = core.ClaimContradicted
				deadCite = cite.TargetRef
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
			if next == core.ClaimContradicted {
				note := "citation anchor no longer resolves"
				if deadCite != "" {
					note += ": " + deadCite
				}
				ev := core.ClaimEvent{
					ClaimID: cl.ClaimID,
					Verdict: "contradicted",
					Author:  "sweep",
					Note:    note,
					At:      clock(),
				}
				if err := st.AppendClaimEvent(ctx, ev); err != nil {
					return rep, fmt.Errorf("semantic: sweep claim event %s: %w", cl.ClaimID, err)
				}
			}
		}
	}
	return rep, nil
}

// DriftedFindings computes remediation drift for drift-tracked-layer claims
// (FT-6, docs/vision/03-findings-and-experts.md §2): "has the SCOPED CODE —
// not the cited finding document — changed since compile?" This is a second,
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
// Compile time persists each drift-tracked claim's scope-matched files'
// content fingerprints as ClaimLink{Rel: RelScopeFP} (verifyDraft,
// compile.go) — no disk read needed here, only the store's already-
// maintained fingerprint table (the same one the fusion pipeline's hash-gate
// keeps current on every ingest). A scope-fp path with no CURRENT
// fingerprint at all (re-ingest purged it, the file moved, or the service
// was never re-indexed after a corpus move) is treated as drifted: the
// scoped code's current state cannot be verified unchanged, so silence would
// be dishonest.
//
// LC-2 (docs/plans/layer-catalog.md): which claims are in scope is now the
// manifest's serving.drift_tracked field, not a kind=="finding" Go hardcode
// — resolved per distinct claim.Layer via the union loader (LoadLayerIn,
// LC-3: embedded packs ∪ the service's own project-local .fathom/layers/, so
// a project-installed drift-tracked layer works exactly like an embedded
// one), memoized so an N-claim service costs at most one manifest lookup per
// distinct layer, not per claim. A claim whose layer no longer resolves
// (e.g. a project-local pack later uninstalled) is treated as not
// drift-tracked rather than erroring this read-hot serving path.
//
// serviceRoot resolves a service's on-disk root the same way SweepClaims and
// pkg/client.CompileLayer do (service_meta.Path) — the ONE place a
// service's project directory is knowable from just (ctx, st, serviceID).
// Every serve-time drift check funnels through this so LoadLayerIn always
// sees the union its compile-time counterpart saw.
func serviceRoot(ctx context.Context, st store.Store, serviceID string) string {
	if meta, found, err := st.GetServiceMeta(ctx, serviceID); err == nil && found {
		return meta.Path
	}
	return ""
}

// layerIsDriftTracked reports whether the named layer's
// serving.drift_tracked is true, consulting the union of embedded packs and
// root's project-local .fathom/layers/ (LC-2/LC-3). Both DriftedFindings'
// per-claim filter and ObligationsFor's serve.go short-circuit share this
// resolution. An unresolvable layer name (e.g. claims left behind by a
// project-local pack later uninstalled) is treated as not drift-tracked
// rather than erroring this serve-time hot path — DriftedFindings' own doc
// comment explains the same leniency for the per-claim case.
func layerIsDriftTracked(root, name string) bool {
	def, err := LoadLayerIn(root, name)
	return err == nil && def.Serving.DriftTracked
}

func DriftedFindings(ctx context.Context, st store.Store, serviceID string) (map[string]bool, error) {
	claims, err := st.ClaimsForService(ctx, serviceID, "")
	if err != nil {
		return nil, fmt.Errorf("semantic: drift load claims: %w", err)
	}
	root := serviceRoot(ctx, st, serviceID)
	driftTrackedLayer := map[string]bool{}
	var findingIDs []string
	for _, cl := range claims {
		if !cl.Status.Served() {
			continue
		}
		tracked, ok := driftTrackedLayer[cl.Layer]
		if !ok {
			tracked = layerIsDriftTracked(root, cl.Layer)
			driftTrackedLayer[cl.Layer] = tracked
		}
		if tracked {
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

	// Per-call fingerprint cache: this runs on EVERY ObligationsFor serve
	// (the edit-hook hot path), and a broad-but-legal scope (e.g. **/*.go)
	// can anchor hundreds of files shared across N claims — each distinct
	// file is queried at most once per call, regardless of how many claims
	// link it. A map, not memoization across calls: DriftedFindings stays a
	// pure read of current store state.
	type fpState struct {
		hash  string
		found bool
	}
	cache := map[string]fpState{}

	drifted := map[string]bool{}
	for _, id := range findingIDs {
		// A claim with ZERO scope-fp links falls through this loop as
		// never-drifted. That is safe by the FT-6 compile invariant: the
		// compiler writes ≥1 scope-fp link for every finding-kind claim —
		// scopes matching no indexed file are rejected at compile time, and
		// a fingerprint read failure aborts the whole compile loudly
		// (errDriftAnchor, compile.go) instead of silently omitting links.
		// Zero links therefore only occurs for claims that never went
		// through this compiler (e.g. hand-seeded test data), never as a
		// degraded state of a real compile.
		for _, l := range scopeFPByClaim[id] {
			cur, ok := cache[l.TargetRef]
			if !ok {
				hash, found, ferr := st.Fingerprint(ctx, serviceID, l.TargetRef)
				if ferr != nil {
					return nil, fmt.Errorf("semantic: drift fingerprint %s: %w", l.TargetRef, ferr)
				}
				cur = fpState{hash: hash, found: found}
				cache[l.TargetRef] = cur
			}
			if !cur.found || cur.hash != l.TargetHash {
				drifted[id] = true
				break
			}
		}
	}
	return drifted, nil
}
