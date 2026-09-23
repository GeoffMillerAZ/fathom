// manifest.cue — #TestManifest and #Suite: the CUE half ADR-0042
// specified in 2026-07 and nobody wrote.
//
// ADR-0042 says "governance is unification… a manifest below the floor
// fails `cue vet`", and the Go structs in ci-kit carry a comment saying
// "the CUE side owns vocabulary and floor enforcement, this side owns
// none of it". That file did not exist, so no manifest had ever failed
// `cue vet` and the enforcement half of the standard was a green gate
// that proved nothing (ADR-0070). This is that file.
//
// Every floor here is a UNIFICATION FAILURE, not a lint pass: a refused
// manifest fails `cue vet -c` naming the offending field, never a Go
// check running afterwards (ADR-0074).
package testkit

import "struct"

#SuiteClass: "unit" | "integration" | "e2e" | "conformance" | "bench" | "fixture-gate"

// #Tier has no "skipped" member, and that omission is the decision
// (ADR-0074 form 5). A quarantined suite RUNS and REPORTS and does not
// block; a test you have stopped running is deleted, not hidden behind a
// vocabulary word that makes the manifest look complete.
#Tier: "blocking" | "advisory" | "quarantined"

#Isolation: "gvisor" | "vcluster"

#Runtime: {
	// class names a runner class the executor understands. test-kit never
	// interprets it — see #Suite.inputs' comment for the same reasoning.
	class: string | *"trusted-spot"
	arch: [...("amd64" | "arm64")] & [_, ...] | *["amd64"]
	timeout:    string & =~"^[0-9]+(ms|s|m|h)$" | *"10m"
	isolation?: #Isolation
	// resources are scheduler concerns and never enter a skip key
	// (ADR-0042 §1). Declared here so a manifest is self-contained;
	// deliberately opaque so no keying code can be tempted to read them.
	resources?: {...}
}

// #Coverage takes a FLOOR. There is no `target` member: an aspirational
// number is not a floor, and a vocabulary that offers both invites every
// component to declare the one that cannot fail.
#Coverage: {
	min:            number & >=0 & <=100
	perPackage:     bool | *false
	perPackageMin?: number & >=0 & <=100
}

// #FlakePolicy — retry lineage is data, never hidden (ADR-0042 §2).
#FlakePolicy: {
	maxRetries:      int & >=0 | *0
	quarantineAfter: int & >=1 | *3
	// budget is a percentage string; the ceiling an org enforces over it
	// is the flake-budget-ceiling rule, not this field.
	budget: string & =~"^[0-9]+(\\.[0-9]+)?%$" | *"0.5%"
}

#Suite: {
	class:  #SuiteClass
	runner: string & !="" // a registered runner brick: go-test, cue-vet-fixtures, k6, …

	// inputs is the suite's TESTED BYTES, and it is deliberately an opaque
	// open struct.
	//
	// What an input closure is happens to be genuinely an executor's
	// concern — ci-kit unifies its own #NodeInputs over this field and
	// gets its keying back with no loss, while a repo adopting test-kit
	// alone writes whatever its runner needs and test-kit never inspects
	// it. That is the seam ADR-0091 draws, and it is why this kit is
	// standalone rather than a face on an engine.
	//
	// NON-EMPTY is the floor: ADR-0042's "tests are declared, never
	// discovered" is exactly the statement that a suite with no declared
	// inputs does not unify. A runner that globs the tree is a runner
	// whose skip key is a guess.
	inputs: {...}
	inputs: struct.MinFields(1)

	// needs names services and fixtures — postgres@16, localstack,
	// vcluster. Their presence is what routes a suite to a job executor
	// rather than a hermetic one; test-kit records, the executor decides.
	needs: [...string] | *[]

	runtime:   #Runtime
	coverage?: #Coverage
	// #FlakePolicy, NOT `#FlakePolicy | *{}`. The disjunction form looks
	// equivalent and is not: a bare `{}` default does not carry the
	// definition's own field defaults, so every suite that omitted `flake`
	// got an empty struct — maxRetries unset, budget the empty string —
	// and flake-budget-ceiling reported `flake budget "" is unparseable`
	// against a manifest nobody had done anything wrong in. Every field in
	// #FlakePolicy has a default, so the bare reference is already
	// concrete.
	flake: #FlakePolicy
	tier:  #Tier

	// conformance is reserved for PORT ADAPTERS (ADR-0042 §1), which is
	// ADR-0003's "every port has a shared conformance suite" requirement
	// finally getting a machine-checkable home. A conformance suite that
	// cannot say which port it conforms is not evidence of anything.
	if class == "conformance" {
		port: string & !=""
	}
}

#TestManifest: {
	component: string & =~"^[a-z0-9][a-z0-9-]{1,60}$"
	// Map-keyed by suite name, never a list — the same reason every policy
	// in this fleet is map-keyed: a list lets an override silently drop a
	// row by omission instead of having to say so.
	suites: [Name=string]: #Suite
	suites: struct.MinFields(1)
}
