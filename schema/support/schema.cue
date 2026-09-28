// Package support is the CUE schema for fathom's support matrix
// (docs/design/support-matrix.md): the target file
// docs/plans/support.targets.json, the non-language support readings under
// docs/bench/readings/support/<kind>/<id>.json, the readings ledger rows,
// and the `fathom support --json` output (fathom.support/v1).
//
// JSON files and Go registries AUTHOR the data; CUE VALIDATES its shape —
// never an authoring or execution surface (ADR-018). Languages and tool
// formats keep their own target list (docs/plans/language-breadth.queue.json,
// LB-F22) and readings (docs/bench/readings/lang/, LB-F16); this schema
// covers the matrix they project into.
package support

// #Kind is the closed list of supported-thing kinds.
#Kind: "language" | "tool" | "client" | "spec" | "rules" | "coverage" | "contract" | "ecosystem"

// #ID is an entry's registry id, the join key with (kind).
#ID: =~"^[a-z0-9][a-z0-9.+-]*$"

// #Level is a support level (ADR-048 §1): L1–L4 for languages, L1–L3 for
// tool formats; other kinds declare a capability as a boolean target.
#Level: "L1" | "L2" | "L3" | "L4"

// #Grade is what a reading computed for a met level or capability.
#Grade: "excellent" | "good"

// #PlanRow names the row that owns the entry: `<ID>` in the owning plan, or
// `<plan>.md#<ID>` across plans (plan grammar v2's needs: convention).
#PlanRow: =~"^([a-z0-9-]+\\.md#)?[A-Z][A-Z0-9]*(-[A-Za-z0-9]+)+$"

#Date: =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"

// #Target is one row of docs/plans/support.targets.json: what is declared,
// never what is achieved. `target` maps a capability to the level it must
// reach or to true (the capability must be proven at all).
#Target: {
	kind: #Kind & !="language" & !="tool" // those live in the language queue
	id:   #ID
	name: string & !=""
	target: {[string]: #Level | true}
	plan_row: #PlanRow
	wave:     int & >=1
	notes?:   string
}

#Targets: [...#Target]

// #Reading is a computed support reading for a non-language kind. A reading
// carries metrics and the capabilities they met; nobody types a grade in —
// the harness that writes the reading computes `met` from `metrics`.
#Reading: {
	kind:         #Kind & !="language" & !="tool"
	id:           #ID
	reading_date: #Date
	commit:       =~"^[0-9a-f]{7,40}$"
	instrument:   string & !="" // the verb or test that produced it
	metrics: {[string]: number}
	met: {[string]: #Grade}
}

// #LedgerRow is one line of docs/bench/readings/support-ledger.jsonl.
#LedgerRow: {
	kind:         #Kind
	id:           #ID
	capability:   string & !=""
	level_met?:   #Level
	grade?:       #Grade
	reading_date: #Date
	commit:       =~"^[0-9a-f]{7,40}$"
}

// #State is the closed computed-state vocabulary (design §3). There is no
// state a person sets: every one is derived from registry, targets,
// readings and plan verdicts.
#State: "proven-excellent" | "proven-good" | "below-target" | "tolerated" |
	"declared" | "regressed" | "refused" | "deferred" | "n/a"

// #Cell is one (entry, capability) of the matrix.
#Cell: {
	capability: string & !=""
	state:      #State
	target?:    #Level | true
	level_met?: #Level
	// Evidence: a proven, below-target or regressed cell always names its
	// reading; a tolerated, declared or n/a cell never claims a level.
	reading?:  string
	plan_row?: #PlanRow
	verdict?:  "proven" | "claimed-unproven" | "open" | "dispositioned"
	if state == "proven-excellent" || state == "proven-good" || state == "below-target" || state == "regressed" {
		reading: string & !=""
	}
	if state == "tolerated" || state == "declared" || state == "n/a" {
		level_met?: _|_
	}
}

// #Entry is one row of the matrix.
#Entry: {
	kind:       #Kind
	id:         #ID
	name:       string & !=""
	registered: bool
	wave?:      int & >=1
	engine?:    string
	cells: [...#Cell]
}

// #Matrix is the `fathom support --json` document.
#Matrix: {
	schema:       "fathom.support/v1"
	generated_at: string
	commit:       string
	entries: [...#Entry]
}
