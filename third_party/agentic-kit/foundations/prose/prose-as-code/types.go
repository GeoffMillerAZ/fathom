// Package proseascode provides structured prose representation types.
// Prose is treated as structured data rather than opaque strings, enabling
// diff-based comparison, targeted regeneration, and evidence traceability.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema.
//
// Concurrency: values are immutable and safe for concurrent use.
package proseascode

import "fmt"

// ProseSegment is the atomic unit of structured prose. Each segment carries a
// stable ID, the rendered text, and zero or more evidence references that
// support the claim expressed in the text.
//
// CUE mirror: #ProseSegment in schema.cue.
type ProseSegment struct {
	// ID uniquely identifies this segment within a ProseBlock. Must be non-empty.
	ID string

	// Text is the rendered prose text for this segment. Must be non-empty.
	Text string

	// EvidenceRefs holds the identifiers of evidence spans or citations that
	// support this segment's claim. May be empty when the segment is
	// non-evidentiary (e.g. headings, transitions).
	EvidenceRefs []string
}

// Validate returns an error if any field is invalid.
// Rules:
//   - ID and Text must be non-empty.
//   - EvidenceRefs elements must all be non-empty when present.
func (s ProseSegment) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("proseascode: ProseSegment.ID must not be empty")
	}
	if s.Text == "" {
		return fmt.Errorf("proseascode: ProseSegment.Text must not be empty")
	}
	for i, ref := range s.EvidenceRefs {
		if ref == "" {
			return fmt.Errorf("proseascode: ProseSegment.EvidenceRefs[%d] must not be empty", i)
		}
	}
	return nil
}

// ProseBlock is an ordered collection of ProseSegments that together form a
// coherent prose unit (a paragraph, section, or document body).
//
// CUE mirror: #ProseBlock in schema.cue.
type ProseBlock struct {
	// Segments is the ordered list of segments making up this block.
	// Must contain at least one segment.
	Segments []ProseSegment
}

// Validate returns an error if the block or any of its segments is invalid.
// Rules:
//   - Segments must not be empty.
//   - Each segment must pass its own Validate.
//   - Segment IDs must be unique within the block.
func (b ProseBlock) Validate() error {
	if len(b.Segments) == 0 {
		return fmt.Errorf("proseascode: ProseBlock.Segments must not be empty")
	}
	seen := make(map[string]struct{}, len(b.Segments))
	for i, seg := range b.Segments {
		if err := seg.Validate(); err != nil {
			return fmt.Errorf("proseascode: ProseBlock.Segments[%d]: %w", i, err)
		}
		if _, dup := seen[seg.ID]; dup {
			return fmt.Errorf("proseascode: ProseBlock.Segments[%d]: duplicate ID %q", i, seg.ID)
		}
		seen[seg.ID] = struct{}{}
	}
	return nil
}
