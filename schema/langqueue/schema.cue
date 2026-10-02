// Package langqueue defines the CUE schema for the language/tool queue,
// docs/plans/language-breadth.queue.json (docs/design/language-breadth.md
// §15, ADR-048, plan row LB-F108). Per ADR-018 the JSON authors the data and
// CUE validates it; internal/langbench/queue.go (ParseQueue) stays the
// loader, and #QueueEntry mirrors its QueueEntry field for field. The Go side
// (internal/langbench/queueschema_test.go) asserts the commented negatives in
// testdata/invalid.cue and vets the committed queue concretely.
package langqueue

// #Tier is the excellence-program tier (design §18): T1 languages, T2 build,
// package and CI, T3 IaC, T4 policy as code, T5 proxies/gateways/mesh, T6
// fathom's own formats. Not the oracle tiers A-D of a reading.
#Tier: "T1" | "T2" | "T3" | "T4" | "T5" | "T6"

#QueueEntry: {
	id:   =~"^[a-z0-9-]+$"
	name: string & !=""
	kind: "language" | "tooling"
	// L4 is not-applicable for tool formats (design §1.1).
	target_level:   "L1" | "L2" | "L3" | "L4"
	grammar_source: string
	licence:        string
	oracle: [...string]
	calibration_subjects: [...string]
	depends_on: [...string]
	wave:         int & >=1
	engine:       "tags" | "doc" | "native" | "tags+doc"
	plan_row:     "LB-Q-\(id)"
	worker_model: "sonnet" | "haiku"
	port_of?:     string
	notes?:       string
	retargeted?:  string
	// tier is optional: an entry without it is outside the excellence set.
	tier?: #Tier

	if kind == "tooling" {
		target_level: "L1" | "L2" | "L3"
	}
}

#Queue: [...#QueueEntry]
