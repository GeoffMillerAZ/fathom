// valid.cue — concrete values that satisfy the #CitationRequest and
// #CitationResult constraints defined in schema.cue. Running `cue vet ./...`
// from the package root validates these against the schema.
package evidencecitation

import "github.com/geoffmilleraz/agentic-kit/foundations/prose/evidence-citation:evidencecitation"

// An empty citation request produces an empty result.
_emptyRequest: evidencecitation.#CitationRequest & {
	Refs: []
}

// A request with two evidence refs.
_twoRefRequest: evidencecitation.#CitationRequest & {
	Refs: ["span-perf-001", "span-owner-002"]
}

// A rendered citation result.
_result1: evidencecitation.#CitationResult & {
	Citation: "[span-owner-002; span-perf-001]"
}

// An empty citation result (no refs provided).
_emptyResult: evidencecitation.#CitationResult & {
	Citation: ""
}

// A throughline-style parser vocabulary (tag names are caller data).
_chatParser: evidencecitation.#ParserConfig & {
	TagNames: ["span", "authority"]
}

// An unrestricted parser: accepts any tag word and bare refs, which is
// what recovering Cite output requires.
_anyParser: evidencecitation.#ParserConfig & {
	TagNames: []
}

// A tagged citation as parsed from "Fact one [span:a] …" (offsets are
// byte offsets of the ref within the parsed text).
_taggedCitation: evidencecitation.#Citation & {
	Tag:   "span"
	ID:    "a"
	Start: 10
	End:   16
	Raw:   "span:a"
}

// A bare citation as parsed from Cite output "[span-a; span-b]".
_bareCitation: evidencecitation.#Citation & {
	Tag:   ""
	ID:    "span-b"
	Start: 9
	End:   15
	Raw:   "span-b"
}

// A unicode ID: offsets count bytes, not runes.
_unicodeCitation: evidencecitation.#Citation & {
	Tag:   "span"
	ID:    "héllo-✓"
	Start: 1
	End:   16
	Raw:   "span:héllo-✓"
}
