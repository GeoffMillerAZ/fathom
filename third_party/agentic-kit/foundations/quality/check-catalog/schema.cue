// Package checkcatalog is the CUE contract for a check catalog: the declared
// shape of every check a gate runs. CUE owns the shape; Row in types.go
// mirrors #Check and any drift between the two is a bug. What CUE cannot say
// — that ids are unique across the list, and that every row has an
// implementation — Go's Reconcile says. Run `cue vet ./...` and
// `cue vet ./testdata/` from this directory (or `task cue-vet` at the repo
// root) to validate the fixtures against these constraints.
package checkcatalog

// #Severity is how badly a finding matters, in a published four-tier
// parser-vendor taxonomy. fatal and major block a publish; data and
// improvement are reported and never gate — the line is drawn on the
// severity, once, never per check.
#Severity: "fatal" | "major" | "data" | "improvement"

// #Means is how a check reaches its verdict. A catalog publishes it so "how
// much of this gate is deterministic" is a number, not a claim.
#Means: "deterministic" | "model"

// #Check is one catalog row.
#Check: {
	// id is the stable identifier. Consumers narrow the pattern (a prefix
	// scheme, a fixed width); uniqueness across the list is Reconcile's job.
	id: string & !=""

	// name is the short human label.
	name: string & !=""

	// severity is how badly a finding from this check matters.
	severity: #Severity

	// means is how the check reaches its verdict.
	means: #Means

	// rejects is one sentence — what this check refuses — written for a
	// human deciding whether to care. It is the text a catalog prints.
	rejects: string & !=""

	// source names the evidence the threshold rests on, so a later reader can
	// re-check it instead of inheriting it. "judgement" is an honest value.
	source?: string & !=""

	// A fatal check must be deterministic: a model must not be what decides
	// an artifact is unreadable at all.
	if severity == "fatal" {
		means: "deterministic"
	}
}

// #Catalog is the whole declared list. CUE cannot express "every id appears
// once" over a list; Reconcile reports a duplicate as drift.
#Catalog: [...#Check]
