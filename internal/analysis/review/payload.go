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
	return resp
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
