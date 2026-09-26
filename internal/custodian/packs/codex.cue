// The codex pack (CC-12, vision 22 §7): what a Codex repo's agent config
// must keep true. Enable it in .fathom/concerns.cue with
// `packs: ["codex"]`; disable one entry with `<id>: {off: true}` or
// replace it wholesale by writing the same id (law 13) — a repo that
// raised project_doc_max_bytes restates codex_agents_md_fits with its
// own budget.
//
// The agent_config_* entries are shared with the claude-code pack,
// identically: enabling both evaluates each once.
pack: {
	name:    "codex"
	version: 1
	about:   "Codex's instruction surface: AGENTS.md carries the fathom stanza and fits in what Codex reads, and every command and pointer the agent config cites still exists"
}

concerns: {
	codex_stanza_present: {
		want:  "AGENTS.md carries a well-formed fathom stanza region (the begin marker, then the end marker)"
		why:   "doctor stanza_present (DI-1): an AGENTS.md that was never wired, or whose end marker a hand-edit deleted, never tells a Codex session to route through the index — and fleet reconcile refuses to repair a corrupted pair"
		check: {kind: "stanza_present", file: "AGENTS.md"}
	}
	codex_agents_md_fits: {
		want:  "AGENTS.md fits in the 32 KiB Codex reads by default (project_doc_max_bytes)"
		why:   "Codex stops reading project docs at project_doc_max_bytes (32 KiB unless configured) without telling the session: every instruction past the cut is simply absent"
		check: {kind: "max_bytes", file: "AGENTS.md", bytes: 32768}
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
