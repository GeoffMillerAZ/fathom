package agenthooks

import (
	"list"
	"strings"
)

// #Policy is the token-economics policy agent-hooks reads from
// ~/.config/agent-hooks/policy.json (or $AGENT_HOOKS_POLICY). The binary
// mirrors every rule here with stdlib validation (policy.go validate);
// CUE remains the authoring and interchange contract, not a runtime
// dependency. Rules a regex or cross-field check enforces only at runtime are
// noted inline.
#Policy: {
	version: 1
	// Cheapest first. Order is meaning: class ceilings and escalation
	// markers compare tier positions.
	tiers: [...#Tier] & list.MinItems(1) & list.MaxItems(8)
	classes: #Classes
	spawn:   #Spawn
	advice?: [...#Advice] & list.MaxItems(32)
	ledger:      #Ledger
	main_thread: #MainThread
}

#Ident: string & =~"^[a-z][a-z0-9_-]{0,63}$"
#Text:  string & strings.MaxRunes(1024)

// A model value: an alias ("haiku"), a family word that matches full ids
// containing it as a hyphen token (Claude only), or an exact id. Runtime also
// rejects a value listed in two tiers of the same host.
#Model: string & =~"^\\S{1,128}$" & !~"^(?i)inherit$"

#Tier: {
	name:    #Ident
	use_for: #Text & !=""
	claude: [...#Model] & list.MaxItems(32)
	codex: [...#Model] & list.MaxItems(32)
	effort?: #Text
	// Lets a brief escalate past its class ceiling to this tier.
	marker?: #Text
}

#Classes: {
	// RE2 regular expression over the brief; a match makes a critic.
	critic_pattern: #Text
	worker_marker:  #Text
	critic_marker:  #Text
	// Runtime checks that each max_tier names a tier.
	worker: max_tier: #Ident
	critic: max_tier: #Ident
}

#Spawn: {
	require_model: bool
	require_effort?: [...("claude" | "codex")] & list.UniqueItems()
	fork: "allow" | "marker" | "deny"
	// Runtime requires a non-blank marker when fork is "marker".
	fork_marker: #Text
	// Consecutive denies in one session before the next spawn is let through
	// with a note, so a session that cannot comply is never stuck.
	deny_loop_limit: int & >=1 & <=20
}

#Advice: {
	id:      #Ident
	when:    #Text & !=""
	unless?: #Text & !=""
	classes?: [...("worker" | "critic")] & list.UniqueItems()
	message: #Text & !=""
}

#Ledger: {
	enabled:            bool
	max_bytes:          int & >=1048576 & <=1073741824
	keep_rotated:       int & >=0 & <=20
	record_tool_output: bool
}

// Containment of large main-thread Bash output (Claude Code only). 0
// disables it; runtime also requires head_chars + tail_chars to be positive
// and below bash_max_chars.
#MainThread: {
	bash_max_chars: 0 | (int & >=2000 & <=150000)
	head_chars:     int & >=0
	tail_chars:     int & >=0
}
