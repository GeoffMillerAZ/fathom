// valid.cue — positive fixture for the intent schema.
// This file must pass `cue vet ./...` from schema/intent.
package intent

// Every field a future carries.
full: #Future & {
	id:         "F86"
	title:      "Component corpora: per-component audit, plans, guard and review in a monorepo"
	confidence: "plausible"
	horizon:    "later"
	value: {
		level: "high"
		why:   "a monorepo's components get the audit a single-scope repository gets"
	}
	basis: {
		kind: "measured"
		cite: "docs/assessments/2026-10-17-machine-situation.md"
	}
	conditions: [
		{label: "Promote when", text: "the owner asks fathom to audit or plan a monorepo component"},
		{label: "Drop if", text: "platform-engineering consolidates to one intent scope"},
	]
}

// The two required fields alone.
minimal: #Future & {
	id:         "F2"
	title:      "Public release"
	confidence: "speculative"
	horizon:    "someday"
}

// A measured basis may cite an instrument or a commit; argued needs none.
instrument: #Basis & {kind: "measured", cite: "`fathom instrument --adoption`"}
reading:    #Basis & {kind: "measured", cite: "IV-M1"}
commit:     #Basis & {kind: "measured", cite: "444640c"}
argued:     #Basis & {kind: "argued"}
