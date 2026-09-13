package mcp

import "strconv"

// callerEntry is one find_callers row on the wire (SB-47, mirroring SB-1's
// referenceOccurrence): a combined "file:line name" location and a
// one-character confidence code — the same two facts a caller needs to
// decide the next move, at a fraction of symbolNodeDTO's bytes. Signature
// is never carried unless with_signatures: true asked for it explicitly.
type callerEntry struct {
	Loc       string `json:"loc"`
	C         string `json:"c"`
	Signature string `json:"sig,omitempty"`
}

// renderCallerEntry renders one find_callers caller for the wire: "file:line
// name" combined into a single location string, confidence collapsed to a
// single character (confidenceChar, shared with references' SB-1 render),
// and the full signature only when includeSignature is set. Unlike a
// references occurrence's free-text line, a file path and a symbol name are
// not safe to truncate without losing correctness, so there is no byte
// trimming loop here — the size guarantee comes from what this row leaves
// out (kind, tier, full-word confidence, service) rather than from cutting
// what it keeps.
func renderCallerEntry(n symbolNodeDTO, includeSignature bool) callerEntry {
	e := callerEntry{
		Loc: n.FilePath + ":" + strconv.Itoa(n.Line) + " " + n.Name,
		C:   confidenceChar(n.EdgeConfidence),
	}
	if includeSignature {
		e.Signature = n.Signature
	}
	return e
}

// callerEnvelopeFields decides which of service/tier/depth/edge_confidence
// are uniform across every rendered caller row and so belong on the
// envelope instead of the (deliberately minimal) per-row shape — the same
// SB-1 move references makes for occurrence_service/tier/depth. A
// dimension that is NOT uniform is simply not disclosed per row for
// find_callers' direct "callers" list (the "c" one-character code already
// carries confidence for every row regardless); by_depth and
// transitive_callers remain the source of full per-row detail when the
// walk goes beyond direct callers.
type callerEnvelopeFields struct {
	Service        string
	ServiceUniform bool
	Tier           string
	TierUniform    bool
	Depth          int
	DepthUniform   bool
	EdgeConfidence string
	ConfUniform    bool
}

func uniformCallerFields(nodes []symbolNodeDTO) callerEnvelopeFields {
	f := callerEnvelopeFields{ServiceUniform: true, TierUniform: true, DepthUniform: true, ConfUniform: true}
	for i, n := range nodes {
		if i == 0 {
			f.Service, f.Tier, f.Depth, f.EdgeConfidence = n.Service, n.Tier, n.Depth, n.EdgeConfidence
			continue
		}
		if n.Service != f.Service {
			f.ServiceUniform = false
		}
		if n.Tier != f.Tier {
			f.TierUniform = false
		}
		if n.Depth != f.Depth {
			f.DepthUniform = false
		}
		if n.EdgeConfidence != f.EdgeConfidence {
			f.ConfUniform = false
		}
	}
	if len(nodes) == 0 {
		f.ServiceUniform, f.TierUniform, f.DepthUniform, f.ConfUniform = false, false, false, false
	}
	return f
}
