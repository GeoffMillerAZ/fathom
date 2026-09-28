// Package trace defines the CUE schema for a repository-authored standard
// mapping file (U8, docs/design/change-and-compliance.md §3, plan CX-15):
// `.fathom/trace/<standard>.cue` names a standard's clauses and which of
// the repository's own requirements satisfy each — by requirement id
// (a plan row id, a foreign spec task id, or "<row>/accept") or by a
// regular-expression pattern over those ids.
//
// Fathom ships NO standard's clause text or clause list (ADR-011, CX-X3
// refused): every clause id and title below is the repository's own
// configuration. The traceability matrix (internal/review.BuildTraceMatrix)
// is computed without a mapping; the mapping only groups the matrix's
// requirements under the clauses an operator names, for
// `fathom trace export --standard <id>`.
package trace

// #StandardMapping is the top-level shape of one mapping file.
#StandardMapping: {
	// standard is the mapping's own identifier — the value
	// `fathom trace export --standard` selects. Lower-case slug.
	standard: =~"^[a-z0-9][a-z0-9._-]*$"

	// title is an optional human label for the standard, authored by the
	// repository (never shipped by fathom).
	title?: string

	// clauses lists the clauses this repository maps. At least one.
	clauses: [#Clause, ...#Clause]
}

// #Clause is one clause the repository maps to its requirements. A clause
// with neither requirements nor patterns is legal: it is reported in the
// pack as mapped to nothing (an honest gap, never a silent omission).
#Clause: {
	// id is the clause identifier as the repository writes it.
	id: string & !=""

	// title is an optional short label the repository authors.
	title?: string

	// requirements names satisfying requirement ids exactly.
	requirements?: [...string & !=""]

	// patterns are RE2 regular expressions matched against requirement ids.
	patterns?: [...string & !=""]
}
