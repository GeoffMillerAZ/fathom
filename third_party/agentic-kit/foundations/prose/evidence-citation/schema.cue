// Package evidencecitation defines the CUE-first contract for evidence
// citation rendering and parsing. The Go implementation provides Cite
// (types.go) and Parser/Parse/Validate/Strip/Replace (parse.go); this
// schema captures the data shapes that flow through those APIs.
package evidencecitation

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #CitationRequest represents the set of evidence reference IDs to be
// rendered as a single citation string.
#CitationRequest: {
	// Refs is the list of evidence reference identifiers. May be empty
	// (produces an empty citation string).
	Refs: [...string]
}

// #CitationResult is the rendered output of a Cite call.
#CitationResult: {
	// Citation is the rendered citation string. Empty when no refs were
	// provided or all refs were empty/duplicate.
	Citation: string
}

// #TagWord is the grammar for citation tag words: an ASCII letter followed
// by ASCII letters, digits, '_' or '-'. Tag vocabulary (e.g. "span",
// "authority") is caller data — the kit defines only the shape.
#TagWord: string & =~"^[A-Za-z][A-Za-z0-9_-]*$"

// #ParserConfig captures the tag vocabulary a Parser accepts. Go mirror:
// the NewParser(tagNames ...string) constructor.
#ParserConfig: {
	// TagNames lists the accepted tag words. Empty means unrestricted:
	// any tag word is accepted and bare refs (Cite's output form) are
	// extracted too.
	TagNames: [...#TagWord]
}

// #Citation is one citation parsed out of text by Parser.Parse. Go mirror:
// Citation in parse.go.
#Citation: {
	// Tag is the tag word of a "tag:id" ref. Empty for bare refs.
	Tag: #TagWord | ""

	// ID is the reference identifier; never empty for a parsed citation.
	ID: _nonEmpty

	// Start/End are byte offsets into the parsed text addressing exactly
	// the ref (text[Start:End] == Raw).
	Start: int & >=0
	End:   int & >Start

	// Raw is the ref exactly as it appeared in the text.
	Raw: _nonEmpty
}
