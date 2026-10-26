package governedprosekit

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

// maxDocumentSegments bounds the private document response per node.
const maxDocumentSegments = 128

// documentSegment is one ordered text segment in the private document response.
// All five fields are required on the wire and arrays may not be null. Go, never
// the model, owns IDs, byte offsets, status, actor, hashes and review.
type documentSegment struct {
	Kind           string   `json:"kind"` // assertion or label
	Text           string   `json:"text"`
	FactIDs        []string `json:"fact_ids"`
	Contradictions []string `json:"contradictions"`
	Reason         string   `json:"reason"`
}

// documentResponse is the closed top-level private response shape.
type documentResponse struct {
	Segments []documentSegment `json:"segments"`
}

// documentResponseSchema is the provider-facing mirror of documentResponse.
// decodeDocumentResponse remains the authoritative runtime gate.
var documentResponseSchema = []byte(`{"type":"object","additionalProperties":false,"required":["segments"],"properties":{"segments":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["kind","text","fact_ids","contradictions","reason"],"properties":{"kind":{"type":"string","enum":["assertion","label"]},"text":{"type":"string"},"fact_ids":{"type":"array","items":{"type":"string"}},"contradictions":{"type":"array","items":{"type":"string"}},"reason":{"type":"string"}}}}}}`)

func segErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrArtifact, fmt.Sprintf(format, args...))
}

// decodeDocumentResponse strictly decodes the private response for host node
// nodeID of nodeKind and derives the ProseDraft in Go: one unit with the host node
// ID, claim IDs "<node>#c<ordinal>", assertions joined by one ASCII space, and
// half-open UTF-8 byte ranges computed from the exact joined bytes. Text is never
// trimmed or rewritten: leading/trailing whitespace is refused. A label is allowed
// only as the single segment of a heading and becomes the independently reviewed
// NonFactualReason path. No model field can set an ID, offset, status or actor.
// MaxBytes bounds complete wire and derived artifacts, including JSON overhead,
// as in ProseLimits; it is not a promise that that many prose text bytes fit.
func decodeDocumentResponse(raw string, limits ProseLimits, nodeID, nodeKind string) (ProseDraft, error) {
	if len(raw) == 0 || len(raw) > limits.MaxBytes || !utf8.ValidString(raw) {
		return ProseDraft{}, segErr("response empty, oversized or invalid UTF-8")
	}
	var resp documentResponse
	if err := jsonwire.DecodeStrict([]byte(raw), &resp); err != nil {
		return ProseDraft{}, segErr("response shape: %v", err)
	}
	n := len(resp.Segments)
	if n == 0 || n > maxDocumentSegments || n > limits.MaxClaims {
		return ProseDraft{}, segErr("segment count %d outside 1..%d", n, maxDocumentSegments)
	}
	for i, s := range resp.Segments {
		if strings.TrimSpace(s.Text) == "" || strings.TrimSpace(s.Text) != s.Text {
			return ProseDraft{}, segErr("segment %d text blank or has leading/trailing whitespace", i)
		}
		switch s.Kind {
		case "assertion":
			if len(s.FactIDs) == 0 || s.Reason != "" {
				return ProseDraft{}, segErr("assertion %d needs facts and an empty reason", i)
			}
			for _, id := range s.FactIDs {
				if strings.TrimSpace(id) == "" {
					return ProseDraft{}, segErr("assertion %d has a blank fact ID", i)
				}
			}
		case "label":
			if n != 1 || nodeKind != KindHeading || len(s.FactIDs) != 0 || len(s.Contradictions) != 0 || strings.TrimSpace(s.Reason) == "" {
				return ProseDraft{}, segErr("a label must be the single reasoned, fact-free segment of a heading")
			}
		default:
			return ProseDraft{}, segErr("segment %d has unknown kind", i)
		}
	}
	if resp.Segments[0].Kind == "label" {
		s := resp.Segments[0]
		return ProseDraft{Claims: []ProseClaim{}, Units: []ProseUnit{{ID: nodeID, Text: s.Text, Claims: []ClaimRange{}, NonFactualReason: s.Reason}}}, nil
	}
	var text strings.Builder
	draft := ProseDraft{Claims: make([]ProseClaim, 0, n)}
	unit := ProseUnit{ID: nodeID, Claims: make([]ClaimRange, 0, n)}
	for i, s := range resp.Segments {
		if i > 0 {
			text.WriteByte(' ')
		}
		start := text.Len()
		if start+len(s.Text) > limits.MaxBytes {
			return ProseDraft{}, segErr("assembled text exceeds the byte limit")
		}
		text.WriteString(s.Text)
		id := fmt.Sprintf("%s#c%d", nodeID, i+1)
		draft.Claims = append(draft.Claims, ProseClaim{ID: id, Assertion: s.Text, FactIDs: append([]string{}, s.FactIDs...), Contradictions: append([]string{}, s.Contradictions...)})
		unit.Claims = append(unit.Claims, ClaimRange{ClaimID: id, Start: start, End: start + len(s.Text)})
	}
	unit.Text = text.String()
	draft.Units = []ProseUnit{unit}
	return draft, nil
}
