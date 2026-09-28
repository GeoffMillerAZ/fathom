package fusion

// violationtrend.go — CX-20 (docs/plans/change-and-compliance.md; design
// docs/design/change-and-compliance.md §5 step 2 "layer violations over
// time", §6 "the radar's violation counts are an ingest-time fact"): at the
// ingest's final commit, a repository with an archconformance rule pack
// (.fathom/rules/archconformance.cue) gets one reading appended to its
// arch_violation_trend — the violation count per deny rule over the same
// normalized edge set the arch_conformance tool checks
// (archconformance.EdgesFromNodes), stamped with the ingest's commit. The
// store records each rule's change since the previous reading, so the
// commits that added violations are named from stored rows.
//
// Best-effort like the rest of recordIngestMeta: a missing, relative or
// malformed rule pack, or a failed read, records nothing rather than
// aborting the ingest — and nothing is never zero: a repository with no
// rules has no readings at all, never a reading of 0.

import (
	"context"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/archconformance"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/projectrules"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// recordViolationTrend appends one reading when svc's corpus carries a rule
// pack with at least one rule. An ingest that changed no file at the same
// commit as the newest reading, under the same deny rules, records nothing
// (the counts cannot have moved).
func (p *Pipeline) recordViolationTrend(ctx context.Context, svc core.ServiceRef, commit string, changed int) {
	root := absCorpusPath(svc.Path)
	if root == "" || !filepath.IsAbs(root) {
		return
	}
	pack, err := projectrules.Load(root)
	if err != nil || !pack.Found || len(pack.Rules) == 0 {
		return
	}
	var opts []archconformance.Option
	if pack.DenyByDefault {
		opts = append(opts, archconformance.DenyByDefault())
	}
	if changed == 0 && commit != "" {
		if prev, err := p.st.ArchViolationTrend(ctx, svc.ID); err == nil && sameReadingKey(prev, commit, archconformance.RuleCounts(nil, pack.Rules, opts...)) {
			return
		}
	}
	obs, err := p.st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		return
	}
	nodes := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	rc := archconformance.RuleCounts(archconformance.EdgesFromNodes(nodes), pack.Rules, opts...)
	counts := make([]store.ArchRuleCount, 0, len(rc))
	for _, c := range rc {
		counts = append(counts, store.ArchRuleCount{Rule: c.Rule, Count: c.Count})
	}
	_, _ = p.st.AppendArchViolations(ctx, svc.ID, commit, time.Now().UnixNano(), counts)
}

// sameReadingKey reports whether the newest reading in prev was taken at
// commit over exactly the rules in empty (RuleCounts over no edges: every
// rule key, zero counts).
func sameReadingKey(prev []store.ArchViolationReading, commit string, empty []archconformance.RuleCount) bool {
	if len(prev) == 0 {
		return false
	}
	last := prev[len(prev)-1].Seq
	var latest []store.ArchViolationReading
	for _, r := range prev {
		if r.Seq == last {
			latest = append(latest, r)
		}
	}
	if len(latest) != len(empty) {
		return false
	}
	for i, r := range latest {
		if r.Commit != commit || r.Rule != empty[i].Rule {
			return false
		}
	}
	return true
}
