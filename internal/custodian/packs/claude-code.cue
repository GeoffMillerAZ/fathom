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
	about:   "Claude Code's instruction surface: Claude Code reaches AGENTS.md (natively, or through a root CLAUDE file's @AGENTS.md import), and every command and pointer the agent config cites still exists"
}

concerns: {
	// The id is kept from the CLAUDE.md-stanza check it replaces (ADR-059),
	// so a repo's same-id override keeps working (law 13).
	claude_code_stanza_present: {
		want:  "Claude Code reaches AGENTS.md: no root CLAUDE file (CLAUDE.md, .claude/CLAUDE.md, CLAUDE.local.md), or the first present one imports @AGENTS.md"
		why:   "ADR-059: under Claude Code's default agents-md mode (2.1.277 and later) a root CLAUDE file switches AGENTS.md off, so the fathom stanza there never tells a session to route through the index unless that file imports it"
		check: {kind: "agents_md_reaches_claude"}
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
