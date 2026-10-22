package mcp

import (
	"context"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func citationPath(ref string) string {
	if file, _, _, ok := parseCitation(ref); ok {
		return file
	}
	if file, line, ok := strings.Cut(ref, ":"); ok {
		if n, err := strconv.Atoi(line); err == nil && n > 0 {
			return file
		}
	}
	return ""
}

// Bind only locations whose coordinates come from the index. Spec search hits
// and commit-anchored receipt exports have their own current/commit identity.
func markReviewLocations(ctx context.Context, b QueryInterface, service string, v *review.Verdict, payload map[string]any, gap *impact.TestGapResult) {
	bind := func(file string, status **core.CitationStatus, m map[string]any) {
		bindCitation(ctx, b, service, file, func(s *core.CitationStatus) {
			*status = s
			if s != nil {
				m["citation_status"] = s
			} else {
				delete(m, "citation_status")
			}
		})
	}
	dims := payload["dimensions"].([]any)
	for i := range v.Dimensions {
		d := &v.Dimensions[i]
		wire := dims[i].(map[string]any)
		// Spec findings locate live source scans; integrity/conservation
		// findings locate current or commit-pinned evidence.
		indexed := d.Name == review.DimImpact || d.Name == review.DimObligations || d.Name == review.DimArch || d.Name == "coverage"
		for j := range d.Findings {
			if !indexed {
				continue
			}
			f := &d.Findings[j]
			file := citationPath(f.Citation)
			if file == "" && f.Line > 0 {
				file = f.File
			}
			if file != "" {
				bind(file, &f.CitationStatus, wire["findings"].([]any)[j].(map[string]any))
			}
		}
		for j := range d.Coverage {
			f := &d.Coverage[j]
			if f.Line > 0 {
				bind(f.File, &f.CitationStatus, wire["coverage"].([]any)[j].(map[string]any))
			}
		}
		if residue, ok := wire["residue"].(map[string]any); ok {
			for j := range d.Residue.Items {
				f := &d.Residue.Items[j]
				if f.Line > 0 {
					bind(f.File, &f.CitationStatus, residue["items"].([]any)[j].(map[string]any))
				}
			}
		}
	}
	wire := payload["test_impact"].(map[string]any)
	for i := range gap.AffectedTests {
		f := &gap.AffectedTests[i]
		if f.Line > 0 {
			bind(f.File, &f.CitationStatus, wire["affected_tests"].([]any)[i].(map[string]any))
		}
	}
	for i := range gap.Gap {
		f := &gap.Gap[i]
		if f.Line > 0 {
			bind(f.File, &f.CitationStatus, wire["test_gap"].([]any)[i].(map[string]any))
		}
	}
}
