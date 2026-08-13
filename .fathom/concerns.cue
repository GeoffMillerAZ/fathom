// .fathom/concerns.cue — the config custodian's contracts for this repo
// (project scope, GOVERNS — law 13). docs/design/config-custodian.md.
//
// Run `fathom config audit` for evidence. Fathom reports; you decide.
concerns: {
	deploy_safety: {
		want:  "no raw-cp deploy instruction anywhere in agent config — deploys go through `task deploy` (signed, daemon-verified)"
		why:   "O25: an unsigned binary copied over the running daemon took it down fleet-wide (OS_REASON_CODESIGNING)"
		check: {kind: "forbidden_pattern", pattern: "cp bin/fathom", where: "agent-config"}
	}
	gate_documented: {
		want:  "the deterministic gate is named in agent config and its target exists"
		check: {kind: "command_documented_and_exists", command: "task validate"}
	}
	router_thin: {
		want:  "the root CLAUDE.md stays a router (the 2026-08-13 compositional restructure holds)"
		check: {kind: "max_lines", file: "CLAUDE.md", lines: 120}
	}
	pointers_hold: {
		want:  "every relative markdown pointer in agent config resolves"
		check: {kind: "pointers_resolve"}
	}
}
