// The claude-code pack (CC-12, vision 22 §7): what a Claude Code repo's
// agent config must keep true. Enable it in .fathom/concerns.cue with
// `packs: ["claude-code"]`; disable one entry with `<id>: {off: true}` or
// replace it wholesale by writing the same id (law 13).
//
// The agent_config_* entries are shared with the codex pack, identically:
// enabling both evaluates each once.
pack: {
	name:    "claude-code"
	version: 1
	about:   "Claude Code's instruction surface: CLAUDE.md carries the fathom stanza, and every command and pointer the agent config cites still exists"
}

concerns: {
	claude_code_stanza_present: {
		want:  "CLAUDE.md carries a well-formed fathom stanza region (the begin marker, then the end marker)"
		why:   "doctor stanza_present (DI-1): a CLAUDE.md that was never wired, or whose end marker a hand-edit deleted, never tells a Claude Code session to route through the index — and fleet reconcile refuses to repair a corrupted pair"
		check: {kind: "stanza_present", file: "CLAUDE.md"}
	}
	agent_config_commands_resolve: {
		want:  "every `task X` target and every script the agent config invokes in code exists"
		why:   "O25: a documented command outlived the thing it named, and every session obeyed it — the docs → Taskfile link of the gate chain"
		check: {kind: "commands_resolve", where: "agent-config"}
	}
	agent_config_pointers_resolve: {
		want:  "every relative pointer in the agent config resolves"
		why:   "vision 22 §1: a dead pointer is confident misdirection — the agent believes the file exists and plans around it"
		check: {kind: "pointers_resolve", where: "agent-config"}
	}
}
