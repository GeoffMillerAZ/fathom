// Package harness is the CUE schema for fathom's harness descriptor
// registry — the JSON `fathom harness list --json` prints and
// internal/harness.ExportJSON renders (docs/vision/25-harness-port.md §2,
// ADR-029).
//
// Go literals AUTHOR the descriptors (internal/harness/descriptors.go);
// CUE VALIDATES the exported data — never an authoring surface. The shape
// is kept ready to move to agentic-kit when a second consumer deletes its
// own copy onto it (FA-22, D8).
package harness

// #Capability is what fathom can deliver to a session, independent of any
// harness. Every descriptor must declare a vehicle for every one of them.
#Capability: "prime" | "obligations-post-edit" | "guard-pre-edit" | "guard-pre-shell" |
	"discovery-observe" | "stop-verify" | "deposit-session-end" | "deposit-pre-compact" |
	"mcp-tools" | "skills" | "stanza" | "verify-gate" | "claim-gate" | "session-ledger" |
	"stanza-loaded" | "config-audit"

// #Vehicle is HOW a capability reaches a session on one harness.
#Vehicle: "hook-inject" | "hook-block" | "hook-silent" | "pull" | "stanza" | "mcp" | "gate" | "absent"

// #Event is fathom's canonical hook event.
#Event: "session-start" | "pre-tool" | "post-tool" | "stop" | "session-end" | "pre-compact" |
	"instructions-loaded" | "config-change"

// #ToolClass normalizes a harness's tool names.
#ToolClass: "file-edit" | "shell" | "discovery-read" | "discovery-grep" | "discovery-glob" | "other"

// #Tier is the degradation ladder rung: A hooks + inject, B block-only,
// C in-process JS only, D none.
#Tier: "A" | "B" | "C" | "D"

#InstructionFile: {
	path: string
	mode: "native" | "mirror" | "settings-key"
	note?: string
}

#HookFile: {
	path:  string
	scope: "project" | "project-local" | "user"
	kind?: string
	shape: string
	timeout_unit?: "s" | "ms"
	trust?: string
	note?: string
}

#EventBinding: {
	capability:    #Capability
	event:         #Event
	harness_event: string
	matcher?:      string
	verb:          string
	marker:        string
	timeout_sec:   int & >0
	vehicle:       #Vehicle
	input?: {[string]: string}
	output?: {[string]: string}
	note?: string
}

#SkillsShape: {
	repo_dirs?: [...string]
	user_dirs?: [...string]
	variant:  string
	verified: bool
	note?:    string
}

#MCPTarget: {
	config_files?: [...string]
	format?: "json" | "toml"
	key_path?: [...string]
	http_key?:          string
	add_command:        string
	stdio_command?:     string
	tool_name_pattern?: string
	note?:              string
}

#EnvContract: {
	project_dir?: string
	session_id?:  string
	session_id_aliases?: [...string]
	present?: string
	home?:    string
	note?:    string
}

// #Descriptor is one harness as data.
#Descriptor: {
	id:      =~"^[a-z][a-z0-9-]*$"
	display: string
	label:   =~"^[a-z]+$"
	tier:    #Tier
	detect: [...string]
	instruction_files: [#InstructionFile, ...#InstructionFile]
	hook_files?: [...#HookFile]
	hooks?: [...#EventBinding]
	tools?: {[string]: #ToolClass}
	// Every capability, always: an absent vehicle is a declared "absent",
	// never a missing key.
	vehicles: {
		"prime":                 #Vehicle
		"obligations-post-edit": #Vehicle
		"guard-pre-edit":        #Vehicle
		"guard-pre-shell":       #Vehicle
		"discovery-observe":     #Vehicle
		"stop-verify":           #Vehicle
		"deposit-session-end":   #Vehicle
		"deposit-pre-compact":   #Vehicle
		"mcp-tools":             #Vehicle
		"skills":                #Vehicle
		"stanza":                #Vehicle
		"verify-gate":           #Vehicle
		"claim-gate":            #Vehicle
		"session-ledger":        #Vehicle
		"stanza-loaded":         #Vehicle
		"config-audit":          #Vehicle
	}
	notes?: {[#Capability]: string}
	fail_mode:    string
	gate_posture: string
	scope_merge?: string
	skills:       #SkillsShape
	mcp:          #MCPTarget
	env:          #EnvContract
	transcript?:  string
	codec?:       string
	codec_landed?: =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"
	// The literal "unverified", or the doc URL + date + version a real
	// session verified the descriptor against.
	verified_against: string
}

// #Registry is the exported list, in registry order.
#Registry: [#Descriptor, ...#Descriptor]
