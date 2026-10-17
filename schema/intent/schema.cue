// Package intent declares the parsed form of the Project Intent records
// fathom reads (ADR-074 decision 5). Markdown stays the authoring surface:
// a record is written in docs/intent/, internal/intent parses it, and this
// schema is the declared contract the parser's output is tested against
// and the gate vets every record with. Nobody hand-authors intent CUE.
//
// internal/intent/intent_schema.cue is a byte-identical copy embedded in
// the binary (`fathom intent audit` vets with it, U16); this file is the
// authoritative source, and TestIntentSchemaMatchesSource holds the copy
// to it.
package intent

// #Future is one future direction as internal/intent.ParseFutures reads it
// from a "## F<n>. <title>" record (docs/intent/futures/*.md, or a
// futures.md that is not split). The key is docs/intent/futures.md's.
#Future: {
	// id is the record id: "F" and a number.
	id: =~"^F[0-9]+$"

	// title is the heading text after the id.
	title: string & !=""

	// confidence (required): likely (a commitment or sponsor statement
	// exists), plausible (fits the vision, no commitment) or speculative
	// (must not influence design; listed only to stop re-proposal).
	confidence: "likely" | "plausible" | "speculative"

	// horizon (required): next, later or someday.
	horizon: "next" | "later" | "someday"

	// value (optional) is "Value: <level>, <why>".
	value?: #Value

	// basis (optional) is "Basis: <kind>[, <cite>]": how the rating was
	// reached.
	basis?: #Basis

	// conditions are the record's labelled conditions (Promote when, Drop
	// if, ...) in document order, annotation blocks removed.
	conditions?: [...#Condition]
}

// #Value is a future's value rating: a level and why it is that level.
#Value: {
	level: "low" | "medium" | "high" | "very-high"
	why:   string & !=""
}

// #Basis is how a rating was reached. A measured basis cites what it
// measured; argued and guess may cite, and need not.
#Basis: {
	kind:  "measured" | "argued" | "guess"
	cite?: string
	if kind == "measured" {
		cite: #Citation
	}
}

// #Citation is what a measured basis rests on: a repository path (a token
// with a slash or a file extension), an instrument (a backticked command
// such as `fathom instrument --adoption`, or a reading id such as K1 or
// IV-M1) or a commit (7 to 40 hex digits).
#Citation: string & =~#"([A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+)|([A-Za-z0-9_-]\.[a-z]{1,5}\b)|(`[^`]+`)|(\b[A-Z]+-M[0-9]+\b)|(\bK[0-9]+\b)|(\b[0-9a-f]{7,40}\b)"#

// #Condition is one labelled condition. The labels are the practice's
// expiry labels (internal/intent conditionLabels).
#Condition: {
	label: "Done when" | "Promote when" | "Drop if" | "Revisit when" | "If false"
	text:  string
}
