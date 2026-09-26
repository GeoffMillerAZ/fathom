// A positive fixture: every check kind, loaded by TestValidFixtureLoads,
// and a pack list (CC-12).
packs: ["lefthook", "taskfile"]

concerns: {
	deploy_safety: {
		want:  "no raw-cp deploy instruction anywhere in agent config"
		why:   "O25: an unsigned binary took the daemon down fleet-wide"
		check: {kind: "forbidden_pattern", pattern: "cp .*libexec/fathom", where: "agent-config"}
	}
	gate_documented: {
		want:  "the validate command is documented and exists"
		check: {kind: "command_documented_and_exists", command: "task validate"}
	}
	router_thin: {
		want:  "the root CLAUDE.md stays a router"
		check: {kind: "max_lines", file: "CLAUDE.md", lines: 120}
	}
	pointers_hold: {
		want:  "every relative path in agent config resolves"
		check: {kind: "pointers_resolve"}
	}
	stanza_current: {
		want:  "every tool the fathom stanza names is served by the running daemon"
		check: {kind: "stanza_tools_served"}
	}
	completion_cites: {
		want:  "a handoff note that claims shipped work cites a commit, plan row, verify run, or retro"
		check: {kind: "handoff_cites_instruments", days: 7}
	}
	adr_citations_resolve: {
		want:  "every ADR-NNN cited in a design doc, an intent doc, the changelog or a plan resolves to a record that exists"
		why:   "INC-14: a design declared itself adopted under a record nobody had written"
		check: {kind: "adr_references_resolve"}
	}
	wiring_current: {
		want:  "hooks, stanza regions, MCP server entries and owned skills match the digests fathom recorded"
		check: {kind: "wiring_current"}
	}
	hook_runs_verify: {
		want:  "the pre-push hook runs fathom verify"
		why:   "I17: an unproven [x] row blocks a push only when the hook runs verify"
		check: {kind: "required_pattern", pattern: "fathom verify", where: "lefthook"}
	}
	claude_md_wired: {
		want:  "CLAUDE.md carries the fathom stanza"
		check: {kind: "stanza_present", file: "CLAUDE.md"}
	}
	agents_md_fits: {
		want:  "AGENTS.md fits in what Codex reads"
		check: {kind: "max_bytes", file: "AGENTS.md", bytes: 32768}
	}
	taskfile_chain: {
		want:  "every task and script the Taskfile calls exists"
		check: {kind: "commands_resolve", where: "taskfile"}
	}
}
