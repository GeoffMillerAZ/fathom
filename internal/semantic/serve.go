package semantic

import (
	"context"
	"fmt"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// MaxObligations caps a served payload (ADR-016 §4.5: 3–6 rules, ≤150
// tokens; rules are word-budgeted at compile time so the cap is the whole
// serving budget).
const MaxObligations = 6

// Obligation is one served rule.
type Obligation struct {
	Rule string `json:"rule"`
	// Src is the primary citation, "path#Lstart-Lend" — always attached
	// (bounded blast radius: the agent can check the source in one Read).
	Src  string `json:"src"`
	Tier string `json:"tier"`
	// Kind labels a claim served from a non-obligation layer ("finding", …).
	// Omitted for the obligations layer (FT-2) — existing consumers
	// (cmd/fathom/edithook.go, the obligations MCP tool response) see the
	// exact same payload shape as before this field existed.
	Kind string `json:"kind,omitempty"`
	// Status is present only when not fresh ("stale" / "contradicted") —
	// drifted rules are served flagged, never silently dropped. A claim
	// never reaches this payload with status "refuted" or "superseded" at
	// all: ObligationsFor only walks claims where core.ClaimStatus.Served()
	// is true, and refuted/superseded are the two statuses Served()
	// excludes. This is the vocabulary the served/never-served line runs on
	// (P1-9, docs/plans/pulled-tools-excellence.md; the founding design at
	// docs/design/semantic-layers.md has the full claims-lifecycle
	// picture): "contradicted" is semantic.Sweep's own machine-detected,
	// anchor-dead judgment (a citation no longer resolves) — the strongest
	// drift signal short of a human verdict, and it stays SERVED, flagged
	// with this Status field, because the underlying rule may still be true
	// even though its evidence anchor broke; a recompile is what heals or
	// confirms it. "refuted" is a categorically different, deliberate
	// judgment — a human via `fathom claim refute`, or the obligations
	// tool's feedback action — and a refuted claim is NEVER served again,
	// so it can never appear here with any Status value. The two verdicts
	// share the append-only ClaimEvent audit mechanism (core.ClaimEvent.
	// Verdict) but must never share a verdict STRING; conflating them would
	// make the audit log claim a never-served verdict for a claim callers
	// can still see served right here.
	Status string `json:"status,omitempty"`
	// CodeDrifted flags a finding-kind claim whose SCOPED CODE (not the
	// cited finding document) has changed since compile — remediation drift
	// (docs/vision/03-findings-and-experts.md §2, FT-6). A serving
	// annotation, never a status flip: drifted code can mean the finding was
	// FIXED, not that the claim is wrong. Always absent for the obligations
	// layer — no scope-fp links are ever recorded for it.
	CodeDrifted bool `json:"code_drifted,omitempty"`
	// Layer names the manifest layer this rule was compiled from (e.g.
	// "obligations", "adr-rules", "spec-bindings", "findings") — T5-2/O14
	// (docs/dogfood-observations.md, docs/plans/trust-and-tokenomics.md):
	// once more than one layer blends into the same served set, an agent
	// (or a human debugging an eviction) needs to know WHICH layer each
	// surviving rule came from, not just its Kind (obligation/finding —
	// a different taxonomy: what KIND of claim, not which manifest layer
	// compiled it). Always populated — layer names are short, so this never
	// meaningfully grows the payload the way an unbounded field would.
	Layer string `json:"layer"`
}

// ObligationsPayload is the decision-moment payload. Zero rules means a
// clean file — serialize to nothing (zero-output-on-clean is the contract
// that makes every-edit injection affordable).
type Obligations struct {
	Service string       `json:"service"`
	File    string       `json:"file"`
	Rules   []Obligation `json:"rules,omitempty"`
	// Watermark is the corpus watermark the newest contributing claim was
	// compiled at — the honest "as of".
	Watermark string `json:"compiled_at_watermark,omitempty"`
	// Displaced is the count of served-ELIGIBLE claims (scope-matched,
	// Served() status) dropped by the MaxObligations cap this call — T5-2/O14:
	// a recompile that blends a new layer's exact-path claims can silently
	// evict a rule an agent relied on yesterday (the cap is a serving-time
	// slice of "matched", never a claim-lifecycle event — nothing about the
	// evicted claim itself changes). Zero (omitted) when nothing was cut.
	Displaced int `json:"displaced,omitempty"`
}

// ObligationsFor returns the claims governing file across EVERY compiled
// layer (FT-2: obligations plus, when compiled, findings and any future
// kind), ordered by scope specificity (narrowest glob first) then rule text,
// capped at max (MaxObligations when max<=0). Deterministic: same store, same
// answer.
func ObligationsFor(ctx context.Context, st store.Store, serviceID, file string, max int) (Obligations, error) {
	if max <= 0 || max > MaxObligations {
		max = MaxObligations
	}
	out := Obligations{Service: serviceID, File: file}

	claims, err := st.ClaimsForService(ctx, serviceID, "")
	if err != nil {
		return out, fmt.Errorf("semantic: obligations load: %w", err)
	}
	ids := make([]string, 0, len(claims))
	served := claims[:0]
	for _, cl := range claims {
		if cl.Status.Served() {
			served = append(served, cl)
			ids = append(ids, cl.ClaimID)
		}
	}
	if len(served) == 0 {
		return out, nil
	}
	links, err := st.ClaimLinks(ctx, serviceID, ids)
	if err != nil {
		return out, fmt.Errorf("semantic: obligations links: %w", err)
	}
	primaryCite := map[string]string{}
	for _, l := range links {
		if l.Rel == core.RelCites {
			if _, ok := primaryCite[l.ClaimID]; !ok {
				primaryCite[l.ClaimID] = l.TargetRef
			}
		}
	}

	type scored struct {
		cl          core.SemanticClaim
		specificity int
	}
	var matched []scored
	for _, cl := range served {
		best := -1
		for _, g := range cl.Scope {
			if filter.New([]string{g}).Match(file) {
				if len(g) > best {
					best = len(g)
				}
			}
		}
		if best >= 0 {
			matched = append(matched, scored{cl: cl, specificity: best})
		}
	}
	if len(matched) == 0 {
		return out, nil
	}
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].specificity != matched[j].specificity {
			return matched[i].specificity > matched[j].specificity
		}
		if matched[i].cl.Rule != matched[j].cl.Rule {
			return matched[i].cl.Rule < matched[j].cl.Rule
		}
		return matched[i].cl.ClaimID < matched[j].cl.ClaimID
	})
	if len(matched) > max {
		// T5-2/O14: record the cap's damage BEFORE slicing it away — this is
		// the only point that knows how many served-eligible claims existed
		// versus how many fit the budget.
		out.Displaced = len(matched) - max
		matched = matched[:max]
	}

	// Remediation-drift annotation (FT-6): computed once, only when a claim
	// from a drift-tracked layer (LC-2: serving.drift_tracked, not a
	// kind=="finding" Go hardcode; LC-3: resolved through the SAME embedded
	// ∪ project-local union a compile saw, via serviceRoot) actually reached
	// the served set — DriftedFindings itself short-circuits to a single
	// empty ClaimsForService read when the service has no drift-tracked
	// claims at all, but skipping the call entirely for pure-obligations
	// services avoids even that. layerIsDriftTracked recompiles the whole
	// pack union per call, so it is memoized per distinct layer name for
	// this ObligationsFor invocation (the same map pattern DriftedFindings
	// itself uses for fingerprints) — a served set of 6 same-layer rules
	// costs one union load, not six, on the edit-hook hot path.
	root := serviceRoot(ctx, st, serviceID)
	var drifted map[string]bool
	trackedByLayer := map[string]bool{}
	for _, m := range matched {
		tracked, ok := trackedByLayer[m.cl.Layer]
		if !ok {
			tracked = layerIsDriftTracked(root, m.cl.Layer)
			trackedByLayer[m.cl.Layer] = tracked
		}
		if tracked {
			drifted, err = DriftedFindings(ctx, st, serviceID)
			if err != nil {
				return out, fmt.Errorf("semantic: obligations drift: %w", err)
			}
			break
		}
	}

	for _, m := range matched {
		ob := Obligation{
			Rule:  m.cl.Rule,
			Src:   primaryCite[m.cl.ClaimID],
			Tier:  string(m.cl.Tier),
			Layer: m.cl.Layer,
		}
		if m.cl.Kind != KindObligation {
			ob.Kind = m.cl.Kind
		}
		if m.cl.Status != core.ClaimFresh {
			ob.Status = string(m.cl.Status)
		}
		if drifted[m.cl.ClaimID] {
			ob.CodeDrifted = true
		}
		out.Rules = append(out.Rules, ob)
		if m.cl.CompiledAtWatermark != "" && out.Watermark == "" {
			out.Watermark = m.cl.CompiledAtWatermark
		}
	}
	return out, nil
}
