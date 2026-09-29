// valid.cue — positive fixtures for the structuralrefusal schema;
// `cue vet ./testdata/` from the package directory checks them against
// ../schema.cue.
package structuralrefusal

// evidencedMeets: the affirmative claim, with what it rests on and the span
// a reader can check.
evidencedMeets: #Judgment & {
	subject: "five years operating a distributed system in production"
	verdict: "meets"
	evidence: [
		{kind: "record", id: "acme-r03", snapshot: "sha256:9f2c…"},
		{kind: "story", id: "on-call-rotation"},
	]
	citation: "Ran the on-call rotation for a 40-node cluster, 2019–2024."
}

// absence: "does-not-meet" needs no evidence — an absence has nothing to
// cite.
absence: #Judgment & {
	subject: "a professional certification in the field"
	verdict: "does-not-meet"
}

// refused: what an unevidenced "meets" becomes, with the audit trail.
refused: #Judgment & {
	subject: "led a team of ten"
	verdict: "unknown"
	refused: "refused \"meets\": no citable evidence (1 unresolvable reference(s) discarded)"
}

// unknownWithoutHistory: a judgment that was always unknown carries no
// refusal note.
unknownWithoutHistory: #Judgment & {
	subject: "familiarity with the regulator's reporting format"
	verdict: "unknown"
}

// rate: one of four could not be evidenced either way.
rate: #Rate & {
	unknown: 1
	total:   4
}
