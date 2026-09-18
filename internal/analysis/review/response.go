package review

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// ReviewSchemaVersion is ReviewResponse's wire schema version (vision 30
// §3.1, ADR-040 decision 6: "All five are versioned with an explicit
// schema_version; a consumer that sees a version it does not know refuses").
const ReviewSchemaVersion = 1

// ProducerInfo names the binary that produced a ReviewResponse.
type ProducerInfo struct {
	Name    string
	Version string
	Build   string
}

// ReviewInputs are the exact input fingerprints a ReviewResponse's verdict
// depends on — the review-side counterpart to reviewkey.Key's components
// (internal/analysis/reviewkey), computed independently by the caller (FL-16
// is scoped to the response ENVELOPE, not the review-key token itself).
type ReviewInputs struct {
	ManifestDigest    string
	SourceWatermark   string
	IndexWatermark    string
	RulesDigest       string
	PlansDigest       string
	ExtractorVersions map[string]string
}

// Coverage is ReviewResponse's top-level honesty summary: Complete iff
// every dimension ran (Checked) and the run was neither truncated, stale,
// nor cancelled — never inferred from an empty Findings list. Unknown names
// every reason it is not: one entry per unchecked dimension ("<dim>:
// <reason>"), plus "truncated"/"stale_index"/"cancelled" when set.
type Coverage struct {
	Complete bool
	Unknown  []string
}

// ReviewResponse wraps a Verdict (unchanged — see engine.go) in the P0
// response envelope (vision 30 §3.1, ADR-040 decision 6, docs/plans/
// findings-loop.md row FL-16): producer/input identity, mode, and the
// honesty fields (Coverage, Truncated, StaleIndex, Cancelled) that must
// never read as clean by omission. `fathom review --json` emits this;
// review_diff (internal/mcp) keeps emitting Verdict.Payload's existing map
// shape, additively gaining only "schema_version".
type ReviewResponse struct {
	SchemaVersion int
	Producer      ProducerInfo
	Inputs        ReviewInputs
	// Mode is "report" or "persist" — the run's DECLARED mode, independent
	// of whether persistence actually happened (see FindingsPersisted).
	Mode string

	Service      string
	ChangedFiles []string
	Dimensions   []Dimension
	Summary      string

	// FindingsPersisted mirrors Verdict.Payload's own field: whether the
	// C2-1 findings bridge actually wrote claims this run (false in report
	// mode, or when persist was refused — e.g. a stale index).
	FindingsPersisted bool

	Coverage Coverage
	// Truncated, StaleIndex, Cancelled are explicit — never inferred from
	// an empty Findings list (vision 30 §3.1).
	Truncated  bool
	StaleIndex bool
	Cancelled  bool

	// UnanchoredPaths lists every ChangedFiles entry the index knows no
	// symbol for (e.g. "docs/x.md", ".github/workflows/y.yml") — accepted
	// as a valid process subject, never given an invented symbol anchor.
	UnanchoredPaths []string
}

// BuildResponse composes a ReviewResponse from an already-run Verdict plus
// the envelope fields the caller resolved. Pure — does no I/O.
func BuildResponse(v Verdict, mode string, producer ProducerInfo, inputs ReviewInputs, findingsPersisted bool, coverage Coverage, truncated, staleIndex, cancelled bool, unanchoredPaths []string) ReviewResponse {
	return ReviewResponse{
		SchemaVersion:     ReviewSchemaVersion,
		Producer:          producer,
		Inputs:            inputs,
		Mode:              mode,
		Service:           v.Service,
		ChangedFiles:      v.ChangedFiles,
		Dimensions:        v.Dimensions,
		Summary:           v.Summary,
		FindingsPersisted: findingsPersisted,
		Coverage:          coverage,
		Truncated:         truncated,
		StaleIndex:        staleIndex,
		Cancelled:         cancelled,
		UnanchoredPaths:   unanchoredPaths,
	}
}

// BuildCoverage derives Coverage from v plus the run-level honesty flags:
// Complete iff every dimension is Checked and none of truncated/staleIndex/
// cancelled is set. Unknown always names WHY, one reason per gap.
func BuildCoverage(v Verdict, truncated, staleIndex, cancelled bool) Coverage {
	var unknown []string
	for _, d := range v.Dimensions {
		if !d.Checked {
			unknown = append(unknown, d.Name+": "+d.Reason)
		}
	}
	if truncated {
		unknown = append(unknown, "truncated")
	}
	if staleIndex {
		unknown = append(unknown, "stale_index")
	}
	if cancelled {
		unknown = append(unknown, "cancelled")
	}
	sort.Strings(unknown)
	return Coverage{Complete: len(unknown) == 0, Unknown: unknown}
}

// UnanchoredPaths returns the sorted subset of changedFiles for which nodes
// carries no symbol at all (FL-16's "process subject" handling): a changed
// path that is not a source file the index knows (a doc, a workflow file,
// …) is accepted, listed here, and never given an invented symbol anchor.
func UnanchoredPaths(changedFiles []string, nodes []core.SymbolNode) []string {
	known := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		known[n.FilePath] = true
	}
	var out []string
	for _, f := range changedFiles {
		if !known[f] {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// Payload renders r as a plain map[string]any suitable for JSON encoding —
// the same "map, not struct tags" convention Verdict.Payload uses, so
// Dimension/Finding's existing shape (nested findings, omit-when-zero
// fields) is reused byte-for-byte rather than re-derived from json struct
// tags.
func (r ReviewResponse) Payload() map[string]any {
	dims := make([]any, len(r.Dimensions))
	for i, d := range r.Dimensions {
		dims[i] = d.payload()
	}
	return map[string]any{
		"schema_version": r.SchemaVersion,
		"producer": map[string]any{
			"name":    r.Producer.Name,
			"version": r.Producer.Version,
			"build":   r.Producer.Build,
		},
		"inputs": map[string]any{
			"manifest_digest":    r.Inputs.ManifestDigest,
			"source_watermark":   r.Inputs.SourceWatermark,
			"index_watermark":    r.Inputs.IndexWatermark,
			"rules_digest":       r.Inputs.RulesDigest,
			"plans_digest":       r.Inputs.PlansDigest,
			"extractor_versions": extractorVersionsPayload(r.Inputs.ExtractorVersions),
		},
		"mode":               r.Mode,
		"service":            r.Service,
		"changed_files":      nonNilStrings(r.ChangedFiles),
		"dimensions":         dims,
		"summary":            r.Summary,
		"findings_persisted": r.FindingsPersisted,
		"coverage": map[string]any{
			"complete": r.Coverage.Complete,
			"unknown":  nonNilStrings(r.Coverage.Unknown),
		},
		"truncated":        r.Truncated,
		"stale_index":      r.StaleIndex,
		"cancelled":        r.Cancelled,
		"unanchored_paths": nonNilStrings(r.UnanchoredPaths),
	}
}

func extractorVersionsPayload(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
