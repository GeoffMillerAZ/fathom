package governedprosekit

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// InvalidateSource returns an owned copy with the reverse dependency closure
// invalidated: base facts, derived facts, dependent prose and its approval. The
// previous artifact is never mutated. Publication remains blocked until a new
// CheckDraft succeeds with corrected evidence and newly reviewed wording.
func InvalidateSource(artifact SourcedArtifact, sourceID string) (SourcedArtifact, error) {
	if strings.TrimSpace(sourceID) == "" {
		return SourcedArtifact{}, fmt.Errorf("%w: source ID required", ErrEvidence)
	}
	out, err := cloneValue(artifact)
	if err != nil {
		return SourcedArtifact{}, err
	}
	affected := make(map[string]bool)
	for _, id := range out.InvalidFacts {
		affected[id] = true
	}
	found := false
	for _, fact := range out.Request.Facts {
		if fact.Source.ID == sourceID {
			affected[fact.Span.ID], found = true, true
		}
		if fact.Derivation != nil {
			for _, id := range fact.Derivation.Inputs {
				if affected[id] {
					affected[fact.Span.ID] = true
				}
			}
		}
	}
	if !found {
		return SourcedArtifact{}, fmt.Errorf("%w: source is not in artifact", ErrEvidence)
	}
	claims := make(map[string]bool)
	for _, claim := range out.Draft.Claims {
		for _, id := range append(append([]string(nil), claim.FactIDs...), claim.Contradictions...) {
			if affected[id] {
				claims[claim.ID] = true
			}
		}
	}
	units := make(map[string]bool)
	for _, id := range out.InvalidUnits {
		units[id] = true
	}
	for _, unit := range out.Draft.Units {
		for _, r := range unit.Claims {
			if claims[r.ClaimID] {
				units[unit.ID] = true
			}
		}
	}
	out.InvalidFacts, out.InvalidUnits = sortedKeys(affected), sortedKeys(units)
	out.ApprovalRef = ""
	out.Receipt.Integrity = "invalidated"
	out.Receipt.Support = nil
	return out, nil
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Export revalidates evidence and support at publication, checks the original
// artifact/config identity, then requires caller authorization for this audience.
// For public output, any restricted input taints the complete artifact: model IDs,
// classifications and omitted support edges cannot declassify context. All units
// are replaced and the sidecar omits their source/claim IDs and review metadata.
// Generate a fresh artifact from public-only facts to publish public sections.
// This returns bytes, never performs an external effect. To commit atomically with
// target state/version use a caller-owned transaction (e.g. kits/hitl). Concurrent
// source revocation after the final observation still requires destination policy.
func (p *SourcedPipeline) Export(ctx context.Context, artifact SourcedArtifact, audience string, authorize PublicationAuthorizer) (ProseExport, error) {
	if audience != "public" && audience != "restricted" {
		return ProseExport{}, fmt.Errorf("%w: invalid export audience", ErrArtifact)
	}
	if authorize == nil || len(artifact.InvalidFacts) != 0 || len(artifact.InvalidUnits) != 0 || artifact.Receipt.Integrity != "pass" {
		return ProseExport{}, fmt.Errorf("%w: authorization missing or artifact invalidated", ErrArtifact)
	}
	checked, err := p.CheckDraft(ctx, artifact.Request, artifact.Draft)
	if err != nil {
		return ProseExport{}, err
	}
	if checked.Receipt.ArtifactDigest != artifact.Receipt.ArtifactDigest || checked.Receipt.EvidenceDigest != artifact.Receipt.EvidenceDigest || checked.Receipt.ConfigDigest != artifact.Receipt.ConfigDigest {
		return ProseExport{}, fmt.Errorf("%w: artifact or policy changed since review", ErrArtifact)
	}
	// Authorization sees the freshly checked supplied draft, not unauthenticated
	// caller claims about which generator/prompt produced it. Historical generation
	// provenance can be retained separately, but is not an authority credential.
	if err := authorize(ctx, checked.Request.Principal, checked.Receipt, audience); err != nil {
		return ProseExport{}, fmt.Errorf("%w: publication authorization: %w", ErrArtifact, err)
	}
	facts, snapshots, err := p.resolveFacts(ctx, checked.Request)
	if err != nil {
		return ProseExport{}, err
	}
	privateContext := false
	for _, source := range snapshots {
		privateContext = privateContext || source.Access == "restricted"
	}
	claims := make(map[string]ProseClaim)
	for _, claim := range checked.Draft.Claims {
		claims[claim.ID] = claim
	}
	result := ProseExport{Audience: audience, ArtifactDigest: checked.Receipt.ArtifactDigest, EvidenceDigest: checked.Receipt.EvidenceDigest, SourceMap: []SourceMapEntry{}}
	var markdown strings.Builder
	sidecarBytes := 0
	for _, unit := range checked.Draft.Units {
		selected := make(map[string]bool)
		var unitClaims []ProseClaim
		for _, r := range unit.Claims {
			claim := claims[r.ClaimID]
			unitClaims = append(unitClaims, claim)
			for _, id := range append(append([]string(nil), claim.FactIDs...), claim.Contradictions...) {
				collectFactClosure(id, facts, selected)
			}
		}
		redacted := audience == "public" && privateContext
		var unitFacts []Fact
		for _, id := range sortedKeys(selected) {
			fact := facts[id]
			unitFacts = append(unitFacts, fact)
			if fact.Derivation == nil && snapshots[fact.Source].Access == "restricted" && audience == "public" {
				redacted = true
			}
		}
		if markdown.Len() != 0 {
			markdown.WriteString("\n\n")
		}
		if redacted {
			markdown.WriteString("[redacted]")
			result.RedactedUnits++
			if markdown.Len() > p.config.Limits.MaxBytes {
				return ProseExport{}, fmt.Errorf("%w: export byte limit", ErrArtifact)
			}
			continue
		}
		start := markdown.Len()
		escaped := escapeProseMarkdown(unit.Text)
		if len(escaped) > p.config.Limits.MaxBytes-markdown.Len()-sidecarBytes {
			return ProseExport{}, fmt.Errorf("%w: export byte limit", ErrArtifact)
		}
		markdown.WriteString(escaped)
		var ranges []ClaimRange
		for _, r := range unit.Claims {
			ranges = append(ranges, ClaimRange{ClaimID: r.ClaimID, Start: len(escapeProseMarkdown(unit.Text[:r.Start])), End: len(escapeProseMarkdown(unit.Text[:r.End]))})
		}
		entry := SourceMapEntry{UnitID: unit.ID, Start: start, End: markdown.Len(), Claims: unitClaims, Facts: unitFacts, Ranges: ranges}
		encoded, err := json.Marshal(entry)
		if err != nil || len(encoded) > p.config.Limits.MaxBytes-markdown.Len()-sidecarBytes {
			return ProseExport{}, fmt.Errorf("%w: sidecar byte limit", ErrArtifact)
		}
		sidecarBytes += len(encoded)
		result.SourceMap = append(result.SourceMap, entry)
	}
	result.Markdown = markdown.String()
	result.MarkdownDigest = ContentDigest(result.Markdown)
	result.SidecarDigest, err = exportDigest(result)
	if err != nil {
		return ProseExport{}, err
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > p.config.Limits.MaxBytes {
		return ProseExport{}, fmt.Errorf("%w: final export byte limit", ErrArtifact)
	}
	return result, ctx.Err()
}

func exportDigest(export ProseExport) (string, error) {
	export.Markdown, export.SidecarDigest = "", ""
	return digestJSON(export)
}

// VerifyProseExport checks transport integrity and exact rendered claim offsets.
// It does not authenticate a receipt or re-resolve source evidence; use Export for
// publication checks and bind expected digests through the destination's authority.
func VerifyProseExport(export ProseExport) error {
	digest, err := exportDigest(export)
	if err != nil || digest != export.SidecarDigest {
		return ErrArtifact
	}
	if export.Markdown == "" || ContentDigest(export.Markdown) != export.MarkdownDigest || !validDigest(export.ArtifactDigest) || !validDigest(export.EvidenceDigest) || (export.Audience != "public" && export.Audience != "restricted") || export.RedactedUnits < 0 {
		return ErrArtifact
	}
	last := 0
	seen := make(map[string]bool)
	for _, entry := range export.SourceMap {
		if entry.UnitID == "" || seen[entry.UnitID] || entry.Start < last || entry.End <= entry.Start || entry.End > len(export.Markdown) || len(entry.Claims) != len(entry.Ranges) {
			return ErrArtifact
		}
		seen[entry.UnitID], last = true, entry.End
		text := export.Markdown[entry.Start:entry.End]
		for i, r := range entry.Ranges {
			if r.ClaimID != entry.Claims[i].ID || r.Start < 0 || r.End <= r.Start || r.End > len(text) || text[r.Start:r.End] != escapeProseMarkdown(entry.Claims[i].Assertion) {
				return ErrArtifact
			}
		}
	}
	return nil
}

func collectFactClosure(id string, facts map[string]Fact, selected map[string]bool) {
	if selected[id] {
		return
	}
	selected[id] = true
	if f := facts[id]; f.Derivation != nil {
		for _, input := range f.Derivation.Inputs {
			collectFactClosure(input, facts, selected)
		}
	}
}

func escapeProseMarkdown(s string) string {
	// Render as text, never active HTML, images or model-authored links. Newlines
	// stay readable; escaping leading punctuation also prevents heading/list syntax.
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "#", "\\#", "!", "\\!", "|", "\\|", "-", "\\-", "+", "\\+", ".", "\\.")
	return r.Replace(s)
}
