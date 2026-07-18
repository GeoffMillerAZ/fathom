// invalid.cue — demonstrates values that VIOLATE the schema. This file is
// intentionally NOT included in the `cue vet` path (the valid.cue sibling is
// the one vetted in CI). It is kept here as a documentation artifact so
// reviewers understand what the schema rejects.
//
// Note: the rendering shapes (#CitationRequest / #CitationResult) are
// deliberately permissive — they accept any string (including empty) for
// Refs and Citation, since the enforcement logic lives in the Go Cite
// function rather than in the CUE types. For those, only type mismatches
// (e.g. integer in a string field) are violations.
//
// The parsing shapes added with the Parse direction DO carry structural
// constraints. Each of the following would fail `cue vet` (mirrored by Go
// rejection behavior: Parse can never produce these values):
//
//	// Tag must be a tag word (ASCII letter, then letters/digits/_/-) or "".
//	_badTag: #Citation & {Tag: "my tag", ID: "x", Start: 0, End: 1, Raw: "x"}
//
//	// ID and Raw must be non-empty.
//	_emptyID: #Citation & {Tag: "span", ID: "", Start: 0, End: 1, Raw: "x"}
//
//	// Offsets: Start >= 0 and End > Start (a citation is never empty).
//	_negStart: #Citation & {ID: "x", Start: -1, End: 1, Raw: "x"}
//	_endNotPastStart: #Citation & {ID: "x", Start: 5, End: 5, Raw: "x"}
//
//	// Parser vocabularies hold tag words only ("" is rejected here even
//	// though Go's NewParser silently ignores empty strings).
//	_badVocab: #ParserConfig & {TagNames: ["span", ""]}

package evidencecitation
