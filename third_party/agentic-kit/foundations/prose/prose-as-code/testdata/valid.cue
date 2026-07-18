// valid.cue — concrete values that satisfy the #ProseSegment and #ProseBlock
// constraints defined in schema.cue. Running `cue vet ./...` from the package
// root validates these against the schema.
package proseascode

import "github.com/geoffmilleraz/agentic-kit/foundations/prose/prose-as-code:proseascode"

// A well-formed segment with evidence references.
_seg1: proseascode.#ProseSegment & {
	ID:           "seg-001"
	Text:         "The payments service processes 10 000 requests per second."
	EvidenceRefs: ["span-perf-001", "span-perf-002"]
}

// A non-evidentiary segment (heading/transition) with no evidence refs.
_seg2: proseascode.#ProseSegment & {
	ID:           "seg-002"
	Text:         "Introduction"
	EvidenceRefs: []
}

// A minimal block with a single segment.
_block1: proseascode.#ProseBlock & {
	Segments: [{
		ID:           "seg-a"
		Text:         "The repository was last updated in Q3."
		EvidenceRefs: ["span-meta-001"]
	}]
}

// A block with multiple segments.
_block2: proseascode.#ProseBlock & {
	Segments: [
		{
			ID:           "seg-b1"
			Text:         "Ownership is tracked via CODEOWNERS."
			EvidenceRefs: ["span-owner-001"]
		},
		{
			ID:           "seg-b2"
			Text:         "The primary language is Go."
			EvidenceRefs: ["span-lang-001"]
		},
	]
}
