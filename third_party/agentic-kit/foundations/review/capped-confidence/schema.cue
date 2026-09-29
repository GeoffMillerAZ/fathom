// Package cappedconfidence is the CUE contract for a ceiling table and the
// scores it produces. CUE owns the shape; Config, Ceiling and Score in
// types.go mirror these definitions and any drift is a bug. The classes are
// the caller's vocabulary — they exist only as rows here, never as Go
// constants — so re-tuning a ceiling is a reviewable diff in a data file.
// Run `cue vet ./...` and `cue vet ./testdata/` from this directory (or
// `task cue-vet` at the repo root) to validate the fixtures.
package cappedconfidence

// #Ceiling is one row: the maximum a score resting on this class of evidence
// may carry, and why.
#Ceiling: {
	// class is the evidence class the row governs.
	class: string & !=""

	// ceiling is the maximum on the table's scale; equal to the table's max
	// means "uncapped".
	ceiling: number & >=0

	// why must be re-derivable by a reader who was not in the room. The
	// rationale is the design.
	why: string & !=""
}

// #Table is the caller's whole table: the top of the scale and at least one
// row, every ceiling within the scale. Uniqueness of class across the list
// is Config.Validate's job — CUE cannot express it.
#Table: {
	// max is the top of the score scale: 100 for a percentage, 5 for a
	// five-point scale.
	max: number & >0

	// ceilings is one row per class the caller will ever score. A class not
	// here is unscored, never defaulted.
	ceilings: [#Ceiling & {ceiling: <=max}, ...#Ceiling & {ceiling: <=max}]
}

// #State is derived in Go from a #Score's fields, never stored.
#State: "supported" | "capped" | "unscored"

// #Score is what reaches a renderer or a store: the raw kept verbatim, the
// bound, and either the class it rests on or the reason nothing bounded it.
#Score: {
	// raw is what the scorer produced, verbatim.
	raw: number

	// ceiling is the bound the class permits; zero when unscored.
	ceiling: number & >=0

	// class is the evidence class the score rests on.
	class?: string & !=""

	// clamped records that raw exceeded ceiling; absent means false.
	clamped: *false | bool

	// why is the row's rationale, carried so the band can explain itself.
	why?: string & !=""

	// reason is why no ceiling could be established; present means unscored.
	reason?: string & !=""

	// A clamped score has a ceiling below its raw and a class to blame.
	if clamped {
		class:   string & !=""
		ceiling: <raw
	}
}
