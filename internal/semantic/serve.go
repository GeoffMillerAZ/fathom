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
	// Module names the bound platform module this rule was federated in
	// from (docs/design/platform-modules.md §3.3) — omitted for every
	// repo-local rule (the pre-phase-O default; existing consumers see
	// exactly the same payload shape they always have). Set by
	// internal/modserve.Bind, never by ObligationsFor itself, which knows
	// nothing about modules or bindings.
	Module string `json:"module,omitempty"`
}

// ModuleNote is an explicit, never-silent note attached to an Obligations
// payload about a bound platform module — most commonly "this repo binds a
// module the daemon does not have mounted" (docs/design/platform-modules.md
// §3.3's doctrine: "Unmounted-but-bound module = an explicit note in the
// payload, never silence"). Populated by internal/modserve.Bind.
type ModuleNote struct {
	Module string `json:"module"`
	Note   string `json:"note"`
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
	//
	// O14 split (docs/design/platform-modules.md §3.3 deliverable 2): once
	// module rules can blend into the same served set, Displaced is the
	// TOTAL (DisplacedRepo + DisplacedModule) — a compatible superset of
	// its pre-phase-O meaning (when no module ever contributes,
	// DisplacedModule is always 0 and Displaced == DisplacedRepo exactly as
	// before this field existed). Existing consumers reading only
	// "displaced" keep seeing a meaningful number; a phase-O-aware consumer
	// reads the two named fields for the split.
	Displaced int `json:"displaced,omitempty"`
	// DisplacedRepo is the count ObligationsFor itself cut from this
	// service's own claims (identical to the pre-phase-O Displaced value).
	// Always 0 (omitted) before internal/modserve.Bind runs.
	DisplacedRepo int `json:"displaced_repo,omitempty"`
	// DisplacedModule is the count of bound-module rules that were
	// eligible (scope- and tier-matched under the binding) but did not fit
	// the shared budget — set only by internal/modserve.Bind, never by
	// ObligationsFor itself.
	DisplacedModule int `json:"displaced_module,omitempty"`
	// ModuleNotes carries explicit, never-silent notes about this service's
	// bound platform modules (e.g. bound but not mounted on this daemon) —
	// set only by internal/modserve.Bind.
	ModuleNotes []ModuleNote `json:"module_notes,omitempty"`
}

// ObligationsFor returns the claims governing file across EVERY compiled
// layer (FT-2: obligations plus, when compiled, findings and any future
// kind), capped at max (MaxObligations when max<=0). Deterministic: same
// store, same answer.
//
// Three things decide what the agent actually sees, in this order
// (ADR-032 §4, the fixes the stopped bench run found — D1, D2, D5):
//
//  1. SCOPE (BP-19, D2). A claims-role layer's rule is re-scoped at serve
//     time from the {package, symbol} contract its cited plan row names
//     (ContractScope), and a row that names no contract binds to nothing
//     and is not served. Prose-matched scopes are what let six plan rows
//     about C++, CUE and tree-sitter bind to one Go file (O23). The
//     compile-time, LLM-proposed scope survives only as the fallback for a
//     claims layer over a NON-plan corpus.
//
//  2. BUDGET (BP-18, D1). Governs-role rules (obligations, adr-rules — the
//     binding rules a human wrote) fill the cap FIRST; claims-role rules
//     (spec-bindings, findings) take only the slots left. Displaced still
//     counts both roles, so an eviction is never silent.
//
//  3. ORDER (BP-17, D5). What survives is ordered fresh before non-fresh,
//     then governs before claims, then by scope specificity (narrowest
//     glob first), then rule text, then claim id. Both T02 payloads in the
//     stopped run led with stale rules; the agent reads top-down, so the
//     freshest binding rule goes first.
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

	anchorText := map[string]string{}
	for _, l := range links {
		if l.Rel == core.RelCites && l.AnchorText != "" {
			if _, ok := anchorText[l.ClaimID]; !ok {
				anchorText[l.ClaimID] = l.AnchorText
			}
		}
	}

	// serviceRoot is the ONE place a service's project directory is knowable
	// from (ctx, st, serviceID); both LayerRole (BP-18) and the citation
	// re-read behind ContractScope (BP-19) resolve through it, so serving
	// sees the same embedded ∪ project-local union a compile saw.
	root := serviceRoot(ctx, st, serviceID)
	roleByLayer := map[string]string{}
	layerRole := func(layer string) string {
		r, ok := roleByLayer[layer]
		if !ok {
			r = LayerRole(root, layer)
			roleByLayer[layer] = r
		}
		return r
	}
	citedText := citedTextResolver(root, anchorText)
	lookup := symbolFileLookup(ctx, st, serviceID)

	type scored struct {
		cl          core.SemanticClaim
		specificity int
		claims      bool // RoleClaims: fills the budget only after governs
	}
	var matched []scored
	for _, cl := range served {
		claims := layerRole(cl.Layer) == RoleClaims
		scope := cl.Scope
		if claims {
			// BP-19/D2: the plan row's own contract, not its prose. ok=false
			// means the citation is not a plan row — a findings-style corpus
			// — and the compile-time scope stands.
			if cs, ok := ContractScope(citedText(primaryCite[cl.ClaimID], cl.ClaimID), lookup); ok {
				scope = cs
			}
		}
		best := -1
		for _, g := range scope {
			if filter.New([]string{g}).Match(file) {
				if len(g) > best {
					best = len(g)
				}
			}
		}
		if best >= 0 {
			matched = append(matched, scored{cl: cl, specificity: best, claims: claims})
		}
	}
	if len(matched) == 0 {
		return out, nil
	}
	less := func(a, b scored) bool {
		if af, bf := a.cl.Status == core.ClaimFresh, b.cl.Status == core.ClaimFresh; af != bf {
			return af // BP-17/D5: fresh before stale or contradicted
		}
		if a.claims != b.claims {
			return !a.claims // BP-18/D1: governs before claims
		}
		if a.specificity != b.specificity {
			return a.specificity > b.specificity
		}
		if a.cl.Rule != b.cl.Rule {
			return a.cl.Rule < b.cl.Rule
		}
		return a.cl.ClaimID < b.cl.ClaimID
	}
	sort.Slice(matched, func(i, j int) bool { return less(matched[i], matched[j]) })
	if len(matched) > max {
		// T5-2/O14: record the cap's damage BEFORE slicing it away — this is
		// the only point that knows how many served-eligible claims existed
		// versus how many fit the budget. BP-18: the cut is role-partitioned,
		// so a claims rule can never evict a governs rule, but Displaced
		// still counts BOTH — the eviction stays one honest integer.
		out.Displaced = len(matched) - max
		kept := make([]scored, 0, max)
		for _, m := range matched {
			if !m.claims {
				kept = append(kept, m)
			}
		}
		if len(kept) > max {
			kept = kept[:max]
		}
		for _, m := range matched {
			if len(kept) >= max {
				break
			}
			if m.claims {
				kept = append(kept, m)
			}
		}
		// Re-order the survivors: the role partition above is a BUDGET
		// decision, not a presentation one, and it can leave a stale governs
		// rule ahead of a fresh claims rule. The served payload is ordered
		// by less() — fresh first (BP-17) — whatever the budget did.
		sort.Slice(kept, func(i, j int) bool { return less(kept[i], kept[j]) })
		matched = kept
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
