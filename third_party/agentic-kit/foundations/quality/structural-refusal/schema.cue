// Package structuralrefusal is the CUE contract for an evidenced judgment.
// CUE owns the shape; Judgment, Evidence and Rate in types.go mirror these
// definitions and any drift is a bug. The structural rule lives here too: an
// affirmative verdict carries evidence, so a store vetted against this
// schema refuses an unevidenced "meets" at load, and Go's Normalize catches
// what was never vetted. Run `cue vet ./...` and `cue vet ./testdata/` from
// this directory (or `task cue-vet` at the repo root) to validate the
// fixtures.
package structuralrefusal

// #Verdict is the closed vocabulary, strongest claim first. Only "meets"
// carries a burden of proof.
#Verdict: "meets" | "does-not-meet" | "unknown"

// #Evidence is one citation. Consumers narrow kind to their own closed
// vocabulary of resolvable things; an id whose kind nothing can resolve is
// a string, not a citation.
#Evidence: {
	// kind names what id resolves against.
	kind: string & !=""

	// id names the one thing that substantiates the verdict.
	id: string & !=""

	// snapshot pins which version of the source the id resolves in, so a
	// stale citation can be told from a wrong one.
	snapshot?: string & !=""
}

// #Judgment is one subject, judged.
#Judgment: {
	// subject is what was judged, in the asker's own words.
	subject: string & !=""

	// verdict is the judgment after the rule below has had its say.
	verdict: #Verdict

	// evidence is what the verdict rests on.
	evidence?: [...#Evidence]

	// citation is the verbatim span a reader can check the claim against.
	citation?: string

	// refused records why a claimed verdict was refused. Only an "unknown"
	// can carry one: a verdict that stands has nothing to explain.
	refused?: string & !=""

	// The rule, in the shape: an affirmative verdict carries evidence. An
	// absent list is incomplete rather than wrong, so this refuses at export
	// or decode (`cue vet -c`), where every value must be concrete.
	if verdict == "meets" {
		evidence: [_, ...]
	}

	// A standing verdict has nothing to explain: refused may only be present
	// on an "unknown" (any value conflicts with "" below).
	if verdict != "unknown" {
		refused?: ""
	}
}

// #Rate is the headline number: unknown judgments over all of them.
#Rate: {
	// unknown counts the judgments whose verdict is "unknown".
	unknown: int & >=0

	// total counts every judgment; never below unknown.
	total: int & >=unknown
}
