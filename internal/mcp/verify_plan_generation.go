package mcp

import (
	"context"
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/sdd"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// generationRefsBackend is deliberately optional. An older store or a
// backend without this read cannot satisfy an explicit generation request.
type generationRefsBackend interface {
	SourceRefProjection(context.Context, string) (store.SourceRefProjection, bool, error)
}

func buildGenerationSpecMatrix(ctx context.Context, backend QueryInterface, service string, nodes []core.SymbolNode, specs []string) (sdd.Matrix, store.SourceRefProjection, error) {
	reader, ok := backend.(generationRefsBackend)
	if !ok {
		return sdd.Matrix{}, store.SourceRefProjection{}, fmt.Errorf("sealed ingest-generation reference facts unavailable")
	}
	p, found, err := reader.SourceRefProjection(ctx, service)
	if err != nil {
		return sdd.Matrix{}, store.SourceRefProjection{}, err
	}
	if !found || p.ServiceID != service || p.Scope != store.SourceRefScopeIngestGeneration || p.CurrentWorktreeFreshness != "unknown" || !store.ValidSourceRefProjection(p) {
		return sdd.Matrix{}, store.SourceRefProjection{}, fmt.Errorf("sealed ingest-generation reference facts unavailable or invalid")
	}
	return buildProjectionSpecMatrix(p, nodes, specs), p, nil
}

func buildProjectionSpecMatrix(p store.SourceRefProjection, nodes []core.SymbolNode, specs []string) sdd.Matrix {
	refs := make(map[string][]string)
	for _, n := range nodes {
		if tokens := sdd.ExtractRefs(n.Signature + " " + n.Name); len(tokens) > 0 {
			refs[n.Name] = tokens
		}
	}
	for _, row := range p.Rows {
		if row.Excluded == "" && len(row.Refs) > 0 {
			refs[row.Path] = append(refs[row.Path], row.Refs...)
		}
	}
	return sdd.BuildMatrix(specs, refs)
}

// Generation facts have unknown current-worktree freshness. Preserve failures
// from other criteria, but never pass or fail a row solely on these refs.
func markGenerationSpecUnknown(rep *verify.Report) {
	rep.Summary = verify.Summary{Total: len(rep.Tasks)}
	for i := range rep.Tasks {
		t := &rep.Tasks[i]
		hasSpec, otherFail := false, false
		for j := range t.Criteria {
			c := &t.Criteria[j]
			if c.Name == "spec_covered" {
				hasSpec = true
				c.Status = verify.Unknown
				c.Detail = "ingest-generation refs; current worktree freshness unknown"
			} else if c.Status == verify.Fail {
				otherFail = true
			}
		}
		if hasSpec && !otherFail {
			t.Status = verify.Unknown
			t.Label = "spec-generation-only"
		} else if hasSpec && t.Label == "spec-uncovered" {
			t.Label = "other-criterion-failed"
		}
		switch t.Status {
		case verify.Pass:
			rep.Summary.Pass++
		case verify.Fail:
			rep.Summary.Fail++
		default:
			rep.Summary.Unknown++
		}
	}
}

// Excluded bytes were not examined at ingest. A present binding in an
// examined row is usable after full window identity; absence remains Unknown.
func markCurrentExcludedUnknown(rep *verify.Report, matrix sdd.Matrix, p store.SourceRefProjection) {
	excluded := false
	for _, row := range p.Rows {
		if row.Excluded != "" {
			excluded = true
			break
		}
	}
	if !excluded {
		return
	}
	rep.Summary = verify.Summary{Total: len(rep.Tasks)}
	for i := range rep.Tasks {
		t := &rep.Tasks[i]
		otherFail := false
		for _, c := range t.Criteria {
			if c.Name != "spec_covered" && c.Status == verify.Fail {
				otherFail = true
			}
		}
		for j := range t.Criteria {
			c := &t.Criteria[j]
			if c.Name != "spec_covered" || c.Status != verify.Fail {
				continue
			}
			c.Status = verify.Unknown
			c.Detail = "excluded source bytes were not examined at ingest; absence is unknown"
			if !otherFail {
				t.Status = verify.Unknown
				t.Label = "spec-excluded-unknown"
			}
		}
		switch t.Status {
		case verify.Pass:
			rep.Summary.Pass++
		case verify.Fail:
			rep.Summary.Fail++
		default:
			rep.Summary.Unknown++
		}
	}
	_ = matrix
}

func generationProjectionReport(p store.SourceRefProjection, matrix sdd.Matrix, specs []string) map[string]any {
	excluded := make([]map[string]string, 0)
	examined := 0
	bindings := make(map[string][]string)
	for _, spec := range specs {
		bindings[spec] = matrix.Coverage(spec)
		if bindings[spec] == nil {
			bindings[spec] = []string{}
		}
	}
	for _, row := range p.Rows {
		if row.Excluded != "" {
			excluded = append(excluded, map[string]string{"path": row.Path, "reason": row.Excluded})
		} else {
			examined++
		}
	}
	return map[string]any{
		"watermark": p.Watermark, "root": p.Root, "policy": p.Policy,
		"extractor_version": p.ExtractorVersion, "membership_hash": p.MembershipHash,
		"produced_at": p.ProducedAt, "complete": p.Complete,
		"examined": examined, "total": len(p.Rows), "excluded": excluded,
		"bindings": bindings,
	}
}
