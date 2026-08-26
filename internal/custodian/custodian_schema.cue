// #Concerns — the schema for ~/.fathom/concerns.cue (user scope) and
// <repo>/.fathom/concerns.cue (project scope, governing — law 13).
//
// docs/design/config-custodian.md §1. The Go loader (load.go) carries the
// load-bearing refusals — optional fields, unknown kinds, kind-specific
// requirements, non-compiling patterns — with better messages; this
// definition is the belt to those braces.

#Concerns: {
	concerns?: [string]: #Concern
}

#Concern: {
	// want states the falsifiable want in a sentence. Required unless the
	// entry is only `off: true` (disabling an inherited concern).
	want?: string
	// why records the failure this concern prevents (an observation ID
	// earns its place here).
	why?: string
	// off disables an inherited concern; the audit still names it.
	off?: bool
	check?: #Check
}

#Check: {
	kind:     "forbidden_pattern" | "command_documented_and_exists" | "max_lines" | "pointers_resolve" | "stanza_tools_served" | "handoff_cites_instruments"
	pattern?: string
	where?:   "agent-config"
	command?: string
	file?:    string
	lines?:   int & >0
	// days is the handoff_cites_instruments window (default 7); the Go
	// loader refuses it on any other kind.
	days?: int & >0
}
