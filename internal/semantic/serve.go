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
	// Status is present only when not fresh ("stale" / "contradicted") —
	// drifted rules are served flagged, never silently dropped.
	Status string `json:"status,omitempty"`
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
}

// ObligationsFor returns the obligations governing file, ordered by scope
// specificity (narrowest glob first) then rule text, capped at max
// (MaxObligations when max<=0). Deterministic: same store, same answer.
func ObligationsFor(ctx context.Context, st store.Store, serviceID, file string, max int) (Obligations, error) {
	if max <= 0 || max > MaxObligations {
		max = MaxObligations
	}
	out := Obligations{Service: serviceID, File: file}

	claims, err := st.ClaimsForService(ctx, serviceID, LayerObligations)
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
		matched = matched[:max]
	}
	for _, m := range matched {
		ob := Obligation{
			Rule: m.cl.Rule,
			Src:  primaryCite[m.cl.ClaimID],
			Tier: string(m.cl.Tier),
		}
		if m.cl.Status != core.ClaimFresh {
			ob.Status = string(m.cl.Status)
		}
		out.Rules = append(out.Rules, ob)
		if m.cl.CompiledAtWatermark != "" && out.Watermark == "" {
			out.Watermark = m.cl.CompiledAtWatermark
		}
	}
	return out, nil
}
