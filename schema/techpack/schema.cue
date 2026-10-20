// Package techpack defines #Pack, the format of a tech pack: a directory of
// CUE data a technology's kit ships and fathom's own custodian packs already
// follow (docs/design/fathom-dir.md section 7.1, ADR-061, K10: one noun).
// The project's own .fathom/ is #Project, which is a pack without the header
// plus the config keys.
//
// A pack advises, only the repository governs: a kit pack may not carry
// layer_rules, capabilities, bindings, exclude_packs or orientation. #Pack is
// closed, so each of those is invalid by absence from the schema.
package techpack

import "github.com/geoffmilleraz/fathom/schema/guard"

// #Pack is a tech pack: a kit's fathom-pack/pack.cue or one of the
// custodian's built-in packs.
#Pack: {
	pack: #Header

	// guards are the existing #Guard shape, imported and not copied.
	guards?: [...guard.#Guard]
	generated?: [...#Generated]
	formats?: [...#Format]

	// concerns are the custodian's contracts about a technology's own
	// config files. They gate only when the repository names the pack in
	// concerns.cue's `packs:`.
	concerns?: [string]: #Concern

	// orientation is the project pack's alone (K9, section 7.6): a pack
	// carrying it is invalid.
	orientation?: _|_
}

// #Header is the `pack: {name, version, about}` block.
#Header: {
	name:    string & !=""
	version: int & >0
	about:   string & !=""
}

// #Generated declares an output produced from sources by a command that
// fathom renders and never runs.
#Generated: {
	output:     string & !=""
	source:     [string & !="", ...]
	regenerate: string & !=""
}

// #Format maps globs to a dialect id (ADR-048: a dialect fathom ships or the
// user trusts).
#Format: {
	dialect: string & !=""
	globs: [string & !="", ...]
}

// #Orientation is one authored anchor: a repository path or `path#Symbol`,
// plus why a reader starts there.
#Orientation: {
	anchor: string & !=""
	why:    string & !=""
}

#Concern: {
	want?:  string
	why?:   string
	off?:   bool
	check?: #Check
}

#Check: {
	kind:         "forbidden_pattern" | "required_pattern" | "command_documented_and_exists" | "commands_resolve" | "max_lines" | "max_bytes" | "pointers_resolve" | "stanza_present" | "stanza_tools_served" | "handoff_cites_instruments" | "adr_references_resolve" | "wiring_current" | "agents_md_reaches_claude"
	pattern?:      string
	where?:        "agent-config" | "lefthook" | "taskfile" | "ci"
	under?:        string
	command?:      string
	file?:         string
	lines?:        int & >0
	bytes?:        int & >0
	days?:         int & >0
	allow_import?: bool
	import_only?:  bool
}
