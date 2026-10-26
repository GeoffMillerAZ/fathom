package governedprosekit

import (
	"context"
	"sort"
	"time"

	evidencespan "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-span"
)

func diffManifest(prev, cur ReadManifest) []string {
	var out []string
	pf, cf := map[string]string{}, map[string]string{}
	for _, f := range prev.Facts {
		pf[f.ID] = f.Digest
	}
	for _, f := range cur.Facts {
		cf[f.ID] = f.Digest
	}
	ids := map[string]bool{}
	for id := range pf {
		ids[id] = true
	}
	for id := range cf {
		ids[id] = true
	}
	for _, id := range sortedKeys(ids) {
		if pf[id] != cf[id] {
			out = append(out, "fact:"+id)
		}
	}
	pc, cc := map[string]string{}, map[string]string{}
	for _, c := range prev.Context {
		pc[c.ID] = c.ContentDigest
	}
	for _, c := range cur.Context {
		cc[c.ID] = c.ContentDigest
	}
	ctx := map[string]bool{}
	for id := range pc {
		ctx[id] = true
	}
	for id := range cc {
		ctx[id] = true
	}
	for _, id := range sortedKeys(ctx) {
		pv, pok := pc[id]
		cv, cok := cc[id]
		if pok != cok || pv != cv {
			out = append(out, "context:"+id)
		}
	}
	for _, c := range []struct{ name, a, b string }{
		{"goals", prev.GoalsDigest, cur.GoalsDigest},
		{"session", prev.SessionDigest, cur.SessionDigest},
		{"section", prev.SectionDigest, cur.SectionDigest},
		{"profile", prev.ProfileDigest, cur.ProfileDigest},
		{"config", prev.ConfigDigest, cur.ConfigDigest},
		{"template", prev.TemplateDigest, cur.TemplateDigest},
		{"instruction", prev.Instruction, cur.Instruction},
		{"text", prev.TextDigest, cur.TextDigest},
	} {
		if c.a != c.b {
			out = append(out, c.name)
		}
	}
	if prev.WholeInput != cur.WholeInput {
		out = append(out, "whole-input")
	}
	return out
}

// equivalentRevision reports whether two base facts differ only in source revision
// bookkeeping (source ref, record binding, locator and observation time).
func equivalentRevision(a, b Fact) bool {
	for _, f := range []*Fact{&a, &b} {
		f.Source, f.Span.RecordID, f.Span.RecordHash = SourceRef{}, "", ""
		f.Span.Locator, f.Span.ObservedAt = evidencespan.Locator{}, time.Time{}
	}
	da, _ := digestJSON(a)
	db, _ := digestJSON(b)
	return da == db
}

// PlanChange computes the deterministic impact floor from a verified parent
// version to next without calling the model or any source. See ImpactPlan for set
// semantics. Structural problems (cycles, missing nodes, omitted expected edges,
// unmatched rebind declarations) return ErrImpact/ErrSession rather than a plan.
func (e *DocumentEngine) PlanChange(prev DocumentVersion, next DocumentTarget, rebinds []SourceRebind) (ImpactPlan, error) {
	ix, err := e.index(next)
	if err != nil {
		return ImpactPlan{}, err
	}
	return e.plan(prev, ix, rebinds, nil)
}

func (e *DocumentEngine) plan(prev DocumentVersion, ix *docIndex, rebinds []SourceRebind, extra map[string]string) (ImpactPlan, error) {
	if err := VerifyDocument(prev); err != nil {
		return ImpactPlan{}, err
	}
	prevNodes := map[string]DocNode{}
	for _, n := range prev.Nodes {
		prevNodes[n.ID] = n
	}
	prevFacts := map[string]Fact{}
	for _, f := range prev.Target.Input.Facts {
		prevFacts[f.Span.ID] = f
	}
	declared := map[string]SourceRebind{}
	for _, r := range rebinds {
		if _, dup := declared[r.FactID]; dup {
			return ImpactPlan{}, impactErr("duplicate rebind for fact %q", r.FactID)
		}
		declared[r.FactID] = r
	}
	class, used := map[string]string{}, map[string]bool{}
	for id, nf := range ix.facts {
		pf, ok := prevFacts[id]
		if !ok {
			class[id] = "added"
			continue
		}
		nd, _ := digestJSON(nf)
		pd, _ := digestJSON(pf)
		switch {
		case nd == pd:
		case pf.Derivation == nil && nf.Derivation == nil && pf.Source != nf.Source && pf.Source.ID == nf.Source.ID && pf.Span.Value == nf.Span.Value && equivalentRevision(pf, nf):
			r, ok := declared[id]
			used[id] = ok
			if ok && r.From == pf.Source && r.To == nf.Source && !blank(r.Equivalence) && !blank(r.Actor) {
				class[id] = "rebind"
			} else {
				class[id] = "hold"
			}
		default:
			class[id] = "changed"
		}
	}
	for id := range declared {
		if !used[id] {
			return ImpactPlan{}, impactErr("rebind for %q matches no equivalent source revision", id)
		}
	}
	regen, hold, rebind := map[string][]string{}, map[string][]string{}, map[string][]string{}
	content := map[string]string{}
	for _, n := range prev.Nodes {
		content[n.ID] = n.ContentDigest
	}
	for _, id := range ix.leaves {
		if why, ok := extra[id]; ok {
			regen[id] = append(regen[id], why)
		}
		pn, existed := prevNodes[id]
		if !existed || pn.Kind != ix.nodes[id].Kind || pn.GenManifest == nil {
			regen[id] = append(regen[id], "added")
			continue
		}
		cur, err := e.genManifest(ix, id, content)
		if err != nil {
			return ImpactPlan{}, err
		}
		reasons := diffManifest(*pn.GenManifest, cur)
		if len(reasons) == 0 {
			continue
		}
		onlyRebind, anyHold := true, false
		for _, r := range reasons {
			if len(r) > 5 && r[:5] == "fact:" && class[r[5:]] == "rebind" {
				continue
			}
			onlyRebind = false
			if len(r) > 5 && r[:5] == "fact:" && class[r[5:]] == "hold" {
				anyHold = true
				reasons = append(reasons, "source-revision-without-equivalence:"+r[5:])
			}
		}
		switch {
		case anyHold:
			hold[id] = reasons
		case onlyRebind:
			rebind[id] = reasons
		default:
			regen[id] = append(regen[id], reasons...)
		}
	}
	for changed := true; changed; {
		changed = false
		for _, id := range ix.leaves {
			for _, d := range ix.deps[id] {
				if _, h := hold[d]; h {
					if _, own := hold[id]; !own {
						hold[id], changed = []string{"depends-held:" + d}, true
					}
				}
			}
		}
	}
	for id := range hold {
		delete(regen, id)
		delete(rebind, id)
	}
	for changed := true; changed; {
		changed = false
		for _, id := range ix.leaves {
			if _, h := hold[id]; h {
				continue
			}
			for _, d := range ix.deps[id] {
				if _, r := regen[d]; r {
					if !contains(regen[id], "depends:"+d) {
						regen[id], changed = append(regen[id], "depends:"+d), true
					}
				}
			}
		}
	}
	for id := range regen {
		delete(rebind, id)
	}
	plan := ImpactPlan{Regenerate: []NodeImpact{}, Rerender: []NodeImpact{}, Rebind: []NodeImpact{}, ReviewAgain: []NodeImpact{}, Reuse: []string{}, Hold: []NodeImpact{}, Removed: []string{}}
	review := map[string][]string{}
	for _, id := range ix.leaves {
		_, r1 := regen[id]
		_, r2 := hold[id]
		_, r3 := rebind[id]
		if r1 || r2 || r3 {
			continue
		}
		pn := prevNodes[id]
		rv, err := e.reviewManifest(ix, id, ContentDigest(pn.Text))
		if err != nil {
			return ImpactPlan{}, err
		}
		if pn.ReviewMan == nil {
			review[id] = []string{"no-review-manifest"}
		} else if rs := diffManifest(*pn.ReviewMan, rv); len(rs) > 0 {
			review[id] = rs
		}
	}
	hasRegen := func(id string) bool {
		var any func(string) bool
		any = func(x string) bool {
			if _, ok := regen[x]; ok {
				return true
			}
			for _, k := range ix.kids[x] {
				if any(k) {
					return true
				}
			}
			return false
		}
		return any(id)
	}
	for _, id := range ix.order {
		switch {
		case isLeaf(ix.nodes[id].Kind):
			if r, ok := regen[id]; ok {
				plan.Regenerate = append(plan.Regenerate, NodeImpact{id, dedupInOrder(r)})
			} else if r, ok := hold[id]; ok {
				plan.Hold = append(plan.Hold, NodeImpact{id, dedupInOrder(r)})
			} else if r, ok := rebind[id]; ok {
				plan.Rebind = append(plan.Rebind, NodeImpact{id, r})
			} else if r, ok := review[id]; ok {
				plan.ReviewAgain = append(plan.ReviewAgain, NodeImpact{id, r})
			} else {
				plan.Reuse = append(plan.Reuse, id)
			}
		default:
			pn, existed := prevNodes[id]
			var reasons []string
			if !existed {
				reasons = append(reasons, "added")
			} else if !sameChildren(pn.ID, prev, ix) {
				reasons = append(reasons, "structure")
			}
			if hasRegen(id) {
				reasons = append(reasons, "child-regenerated")
			}
			if len(reasons) > 0 {
				plan.Rerender = append(plan.Rerender, NodeImpact{id, reasons})
			} else {
				plan.Reuse = append(plan.Reuse, id)
			}
		}
	}
	for _, n := range prev.Nodes {
		if _, ok := ix.nodes[n.ID]; !ok {
			plan.Removed = append(plan.Removed, n.ID)
		}
	}
	sort.Strings(plan.Removed)
	return plan, nil
}

func dedupInOrder(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// sameChildren compares the parent's ordered child IDs and kinds with the target.
func sameChildren(id string, prev DocumentVersion, ix *docIndex) bool {
	_, kids := nodeOrderIndex(prev)
	old := kids[id]
	now := ix.kids[id]
	if len(old) != len(now) {
		return false
	}
	for i := range old {
		if old[i].ID != now[i] || old[i].Kind != ix.nodes[now[i]].Kind {
			return false
		}
	}
	return true
}

// Rebuild plans and executes a change from prev to next. The deterministic
// Regenerate floor can only be widened: a non-nil opts.Requested that omits a floor
// node is refused, and SemanticWiden nodes are added (with their dependents). Held
// plans (e.g. a source revision without declared equivalence) refuse before any
// model call. Rebound nodes keep exact words and are re-resolved and re-reviewed.
func (e *DocumentEngine) Rebuild(ctx context.Context, prev DocumentVersion, next DocumentTarget, rebinds []SourceRebind, opts RebuildOptions) (RebuildResult, error) {
	ix, err := e.index(next)
	if err != nil {
		return RebuildResult{}, err
	}
	floor, err := e.plan(prev, ix, rebinds, nil)
	if err != nil {
		return RebuildResult{}, err
	}
	if len(floor.Hold) > 0 {
		return RebuildResult{}, impactErr("%d node(s) held, first %q: %v", len(floor.Hold), floor.Hold[0].NodeID, floor.Hold[0].Reasons)
	}
	if opts.Requested != nil {
		for _, n := range floor.Regenerate {
			if !contains(opts.Requested, n.NodeID) {
				return RebuildResult{}, impactErr("requested closure omits floor node %q", n.NodeID)
			}
		}
	}
	extra := map[string]string{}
	for _, id := range opts.Requested {
		extra[id] = "requested"
	}
	for _, id := range opts.SemanticWiden {
		extra[id] = "semantic-widen"
	}
	for id := range extra {
		if n, ok := ix.nodes[id]; !ok || !isLeaf(n.Kind) {
			return RebuildResult{}, impactErr("widened node %q is not a generated leaf", id)
		}
	}
	plan, err := e.plan(prev, ix, rebinds, extra)
	if err != nil {
		return RebuildResult{}, err
	}
	regen := map[string]bool{}
	for _, n := range plan.Regenerate {
		regen[n.NodeID] = true
	}
	v, calls, err := e.build(ctx, ix, &prev, plan, regen)
	if err != nil {
		return RebuildResult{}, err
	}
	return RebuildResult{Version: v, Plan: plan, Delta: documentDelta(prev, v, len(regen), calls)}, nil
}

func documentDelta(prev, next DocumentVersion, regenerated, calls int) DocumentDelta {
	d := DocumentDelta{WordsChanged: []string{}, ProvenanceChanged: []string{}, Added: []string{}, Removed: []string{}, RegeneratedLeaves: regenerated, ModelCalls: calls}
	old := map[string]DocNode{}
	for _, n := range prev.Nodes {
		old[n.ID] = n
	}
	seen := map[string]bool{}
	for _, n := range next.Nodes {
		if !isLeaf(n.Kind) {
			continue
		}
		d.LeafTotal++
		d.BytesTotal += len(n.Text)
		seen[n.ID] = true
		p, ok := old[n.ID]
		switch {
		case !ok:
			d.Added = append(d.Added, n.ID)
			d.BytesChanged += len(n.Text)
		case p.Text != n.Text:
			d.WordsChanged = append(d.WordsChanged, n.ID)
			d.BytesChanged += len(n.Text)
		case p.Version != n.Version:
			d.ProvenanceChanged = append(d.ProvenanceChanged, n.ID)
		}
	}
	for _, n := range prev.Nodes {
		if isLeaf(n.Kind) && !seen[n.ID] {
			d.Removed = append(d.Removed, n.ID)
		}
	}
	return d
}
