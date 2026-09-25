package review

// Payload renders v as a plain map[string]any suitable for JSON encoding —
// shared by the review_diff MCP tool (internal/mcp) and the `fathom review`
// CLI mirror (cmd/fathom) so their JSON output never drifts into two
// slightly different shapes. findingsPersisted reports whether the C2-1
// findings bridge (AttachClaims + a store write) successfully persisted
// this run's claims; both callers pass this in since only they know
// whether the write happened.
func (v Verdict) Payload(findingsPersisted bool) map[string]any {
	dims := make([]any, len(v.Dimensions))
	for i, d := range v.Dimensions {
		dims[i] = d.payload()
	}
	return map[string]any{
		"service":            v.Service,
		"changed_files":      nonNilStrings(v.ChangedFiles),
		"dimensions":         dims,
		"summary":            v.Summary,
		"findings_persisted": findingsPersisted,
	}
}

func (d Dimension) payload() map[string]any {
	findings := make([]any, len(d.Findings))
	for i, f := range d.Findings {
		findings[i] = f.payload()
	}
	resp := map[string]any{
		"name":     d.Name,
		"checked":  d.Checked,
		"count":    d.Count,
		"findings": findings,
	}
	if !d.Checked {
		resp["reason"] = d.Reason
	}
	if len(d.Tests) > 0 {
		resp["tests"] = nonNilStrings(d.Tests)
	}
	if len(d.Files) > 0 {
		resp["files"] = nonNilStrings(d.Files)
	}
	if d.Name == dimCoverageName {
		items := make([]any, len(d.Coverage))
		for i, c := range d.Coverage {
			items[i] = c.payload()
		}
		resp["coverage"] = items
	}
	if d.Name == DimConservation {
		// CV-65/design §3.4: the conservation dimension carries the
		// reading set's own counts plus its capped item list on the wire
		// too, never only on the in-memory Dimension struct — the same
		// additive "residue" object fathom.conserve/v1 itself carries
		// (ConservationResidueSummary's own doc comment).
		resp["residue"] = d.Residue.payload()
	}
	return resp
}

func (r ConservationResidueSummary) payload() map[string]any {
	items := make([]any, len(r.Items))
	for i, it := range r.Items {
		items[i] = it.payload()
	}
	return map[string]any{
		"changed":    r.Changed,
		"attested":   r.Attested,
		"unattested": r.Unattested,
		"unreached":  r.Unreached,
		"items":      items,
		"truncated":  r.Truncated,
		"unanalyzed": nonNilStrings(r.Unanalyzed),
		"missing":    nonNilStrings(r.Missing),
	}
}

func (it ConservationResidueItem) payload() map[string]any {
	return map[string]any{
		"symbol":  it.Symbol,
		"file":    it.File,
		"line":    it.Line,
		"change":  it.Change,
		"class":   it.Class,
		"tests":   nonNilStrings(it.Tests),
		"callers": it.Callers,
	}
}

func (f Finding) payload() map[string]any {
	m := map[string]any{
		"kind":     f.Kind,
		"message":  f.Message,
		"severity": f.Severity,
	}
	if f.Symbol != "" {
		m["symbol"] = f.Symbol
	}
	if f.File != "" {
		m["file"] = f.File
	}
	if f.Line > 0 {
		m["line"] = f.Line
	}
	if f.Citation != "" {
		m["citation"] = f.Citation
	}
	if f.ClaimID != "" {
		m["claim_id"] = f.ClaimID
	}
	if f.Status != "" {
		m["status"] = f.Status
	}
	if f.Test != "" {
		m["test"] = f.Test
	}
	if f.Strength != "" {
		m["strength"] = f.Strength
	}
	if f.State != "" {
		m["state"] = f.State
	}
	return m
}

// nonNilStrings converts ss to []any, normalizing a nil slice to an empty
// (never-null) JSON array — matches internal/mcp's stringsToAny convention
// for every other tool's list fields.
func nonNilStrings(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
