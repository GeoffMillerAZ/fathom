// Package proseascode defines the CUE-first contract for structured prose
// representation. Go types in types.go mirror these definitions; any drift is
// a bug.
package proseascode

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #ProseSegment is the atomic unit of structured prose. Each segment carries a
// stable ID, the rendered text, and zero or more evidence references.
#ProseSegment: {
	// ID uniquely identifies this segment within a ProseBlock.
	ID: _nonEmpty

	// Text is the rendered prose text for this segment.
	Text: _nonEmpty

	// EvidenceRefs holds evidence-span or citation IDs that support this
	// segment. May be empty for non-evidentiary segments.
	EvidenceRefs: [...string]
}

// #ProseBlock is an ordered collection of ProseSegments forming a coherent
// prose unit.
#ProseBlock: {
	// Segments must contain at least one ProseSegment.
	Segments: [#ProseSegment, ...#ProseSegment]
}
