package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/iacfacts"
)

// HazardDimension projects current catalog findings, not the future hazard engine.
// Checked means every relevant input was checked against its bound catalog.
// AsOf is the caller's revision verbatim, never a clock or a resolved timestamp.
type HazardDimension struct {
	Checked  bool                  `json:"checked"`
	Reason   string                `json:"reason"`
	AsOf     string                `json:"as_of"`
	Findings []iacfacts.IaCFinding `json:"findings"`
}

type iacReviewCoverage struct {
	relevant bool
	reasons  map[string]bool
}

func (c *iacReviewCoverage) unknown(reason string) {
	if c == nil {
		return
	}
	if c.reasons == nil {
		c.reasons = map[string]bool{}
	}
	c.reasons[reason] = true
}

func (c *iacReviewCoverage) reason() string {
	reasons := make([]string, 0, len(c.reasons))
	for r := range c.reasons {
		reasons = append(reasons, r)
	}
	sort.Strings(reasons)
	return strings.Join(reasons, "; ")
}

// reviewIaCDimension retains the compatibility section independently of coverage:
// missing catalogs and dated reviews still omit that section, but never the dimension.
func reviewIaCDimension(ctx context.Context, backend QueryInterface, service string, changed []string, base, asOf string) (HazardDimension, map[string]any) {
	d := HazardDimension{AsOf: asOf, Findings: []iacfacts.IaCFinding{}}
	if asOf != "" {
		d.Reason = "dated review: current source and catalogs cannot establish IaC coverage at as_of"
		return d, nil
	}
	coverage := &iacReviewCoverage{}
	legacy := reviewIaCFindings(ctx, backend, service, changed, coverage)
	cfn := reviewCFNFindings(ctx, backend, service, changed, base, coverage)
	if cfn != nil {
		if legacy == nil {
			legacy = map[string]any{}
		}
		legacy["cloudformation"] = cfn
	}
	for _, section := range []map[string]any{legacy, cfn} {
		if fs, ok := section["findings"].([]iacfacts.IaCFinding); ok {
			d.Findings = append(d.Findings, fs...)
		}
	}
	sort.SliceStable(d.Findings, func(i, j int) bool {
		a, b := d.Findings[i], d.Findings[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Block != b.Block {
			return a.Block < b.Block
		}
		return a.Path < b.Path
	})
	if ctx.Err() != nil {
		coverage.unknown("IaC review cancelled or deadline exceeded")
	}
	if !coverage.relevant {
		coverage.unknown("no relevant Terraform or CloudFormation change")
	}
	d.Reason = coverage.reason()
	d.Checked = d.Reason == ""
	return d, legacy
}

func (d HazardDimension) payload() map[string]any {
	return map[string]any{"name": "iac", "checked": d.Checked, "reason": d.Reason, "as_of": d.AsOf, "count": len(d.Findings), "findings": d.Findings}
}

func terraformReviewFile(file string) bool {
	file = filepath.ToSlash(file)
	return strings.HasSuffix(file, ".tf") || strings.HasSuffix(file, ".tofu") || strings.HasSuffix(file, ".tf.json") || strings.HasSuffix(file, ".tofu.json")
}

// iacReviewSummary keeps catalog findings separate from core engine findings,
// whose receipt and persistence semantics are unchanged.
func iacReviewSummary(service string, changed, checked, coreActive, iacFindings int, unchecked []string) string {
	summary := fmt.Sprintf("service %s: %d changed file(s); %d/9 dimensions checked; %d core active finding(s); %d IaC finding(s)", service, changed, checked, coreActive, iacFindings)
	if len(unchecked) > 0 {
		summary += "; unchecked: " + strings.Join(unchecked, ", ")
	}
	return summary
}
