package certify

// sweep.go — SweepCertifications (TC-8, design doc §3's "Sweep: on every
// ingest, like every other claim"): re-derives every EXISTING
// certifications-layer claim's current status/state from the live index,
// without re-scanning declarations from scratch (CompileCertifications'
// job). A test symbol no longer in the index -> contradicted; a symbol
// whose Key is unchanged but whose FILE moved -> stale (design doc §3's
// "the test… moved"); otherwise the claim is fully re-derived via the
// SAME evaluateCertificate path CompileCertifications uses, so "a newly
// ingested run updates state" falls out of that shared logic rather than
// a second, divergent one.
//
// A target that no longer resolves (its invariant row/incident
// record/spec/rule/port vanished) also contradicts the claim (design doc
// §3: "contradicted when the test is gone OR the target does not
// resolve").
import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// SweepCertifications re-evaluates every certifications-layer claim for
// service against the CURRENT index and writes back whatever changed
// (status flip, or a fully re-derived state/tier). A claim with no Meta
// block (written before this field existed, or by a foreign producer) is
// left untouched — nothing to re-derive it from.
func SweepCertifications(ctx context.Context, st store.Store, service string) (Report, error) {
	var report Report

	claims, err := st.ClaimsForService(ctx, service, Layer)
	if err != nil {
		return report, fmt.Errorf("SweepCertifications: ClaimsForService: %w", err)
	}
	if len(claims) == 0 {
		return report, nil
	}

	nodes, err := serviceSymbols(ctx, st, service)
	if err != nil {
		return report, fmt.Errorf("SweepCertifications: serviceSymbols: %w", err)
	}
	edges, err := st.TypedEdges(ctx, service)
	if err != nil {
		return report, fmt.Errorf("SweepCertifications: TypedEdges: %w", err)
	}
	byKey := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byKey[n.Key()] = n
	}
	g := callgraph.Build(nodes)
	callgraph.AddTypedEdges(g, edges)

	targetCache := make(map[string]Target)
	var updated []core.SemanticClaim
	for _, c := range claims {
		if len(c.Meta) == 0 {
			continue
		}
		var m meta
		if err := json.Unmarshal(c.Meta, &m); err != nil {
			report.Skipped = append(report.Skipped, fmt.Sprintf("%s: unreadable meta: %v", c.ClaimID, err))
			continue
		}
		report.Declared++

		targetKey := m.TargetKind + ":" + m.TargetID
		target, ok := targetCache[targetKey]
		if !ok {
			target, err = ResolveTarget(ctx, st, service, m.TargetKind, m.TargetID)
			if err != nil {
				return report, fmt.Errorf("SweepCertifications: ResolveTarget(%s, %s): %w", m.TargetKind, m.TargetID, err)
			}
			targetCache[targetKey] = target
		}

		testNode, testExists := byKey[m.TestSymbolKey]
		switch {
		case !testExists:
			c.Status = core.ClaimContradicted
			updated = append(updated, c)
		case !target.Resolved:
			c.Status = core.ClaimContradicted
			updated = append(updated, c)
		case m.TestFile != "" && testNode.FilePath != m.TestFile:
			c.Status = core.ClaimStale
			updated = append(updated, c)
		default:
			e := declaredEntry{
				TestSymbolKey: m.TestSymbolKey, Kind: m.TargetKind, TargetID: m.TargetID,
				Strength: m.Strength, Suite: m.Suite, Class: m.Class,
			}
			fresh, ferr := evaluateCertificate(ctx, st, service, c.CompiledAtWatermark, c.CompiledAt, e, target, g, byKey)
			if ferr != nil {
				return report, ferr
			}
			if fresh == nil {
				c.Status = core.ClaimContradicted
				updated = append(updated, c)
				continue
			}
			updated = append(updated, *fresh)
		}
	}

	if len(updated) > 0 {
		if err := st.PutClaims(ctx, updated, nil); err != nil {
			return report, fmt.Errorf("SweepCertifications: PutClaims: %w", err)
		}
	}
	report.ClaimsWritten = len(updated)
	return report, nil
}
