// #Concerns — the schema for ~/.fathom/concerns.cue (user scope) and
// <repo>/.fathom/concerns.cue (project scope, governing — law 13).
//
// docs/design/config-custodian.md §1. The Go loader (load.go) carries the
// load-bearing refusals — optional fields, unknown kinds, kind-specific
// requirements, non-compiling patterns — with better messages; this
// definition is the belt to those braces.

#Concerns: {
	// packs enables shipped concern packs (CC-12, internal/custodian/packs);
	// the union of both scopes' lists is enabled.
	packs?: [...#PackName]
	concerns?: [string]: #Concern
}

#PackName: "claude-code" | "codex" | "lefthook" | "taskfile"

// #Pack is a shipped pack file (internal/custodian/packs/<name>.cue).
#Pack: {
	pack: {
		name:    #PackName
		version: int & >0
		about:   string
	}
	concerns: [string]: #Concern
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
	kind:     "forbidden_pattern" | "required_pattern" | "command_documented_and_exists" | "commands_resolve" | "max_lines" | "max_bytes" | "pointers_resolve" | "stanza_present" | "stanza_tools_served" | "handoff_cites_instruments" | "adr_references_resolve" | "wiring_current"
	pattern?: string
	// where names a corpus in the registry (corpus.go, CC-11); the Go
	// loader also refuses a corpus the kind cannot read.
	where?:   "agent-config" | "lefthook" | "taskfile" | "ci"
	// under holds a required_pattern over a YAML corpus to the lines under
	// one top-level key (a lefthook hook); the Go loader refuses it
	// elsewhere.
	under?:   string
	command?: string
	file?:    string
	lines?:   int & >0
	bytes?:   int & >0
	// days is the handoff_cites_instruments window (default 7); the Go
	// loader refuses it on any other kind.
	days?: int & >0
}
