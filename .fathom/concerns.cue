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
	adr_citations_resolve: {
		want:  "every ADR-NNN cited in a design doc, an intent doc, the changelog or a plan resolves to a record that exists"
		why:   "INC-14: a design declared itself adopted under ADR-026 while ADR.md had no such section, and three intent docs cited the phantom — a panel caught it, nothing mechanical would have"
		check: {kind: "adr_references_resolve"}
	}
	pointers_hold: {
		want:  "every relative markdown pointer in agent config resolves"
		check: {kind: "pointers_resolve"}
	}
	stanza_current: {
		want:  "every tool the fathom stanza in CLAUDE.md/AGENTS.md names — backticked, or bare with an underscore (find_callers, pack_context) — is served by the running daemon's agent surface; a stanza pointing at a pruned or undeployed tool sends every session at a call that fails"
		why:   "CC-14: the operator contract's \"instructed means true\" applied to fathom's own instructions. Daemon down, or a 200 with no tools in it, is unable-to-check, never a violation. Bare single words (obligations, references, outline, ingest) are policed only when backticked — bare they are prose, and a false violation teaches people to ignore the custodian"
		check: {kind: "stanza_tools_served"}
	}
	completion_cites: {
		want:  "a handoff thread note from this repo that claims shipped work cites the instrument that proves it — a commit sha (7–12 or 40 hex, letters and digits), a row id one of docs/plans/*.md carries, `fathom verify`, or a retro artifact under workspace/artifacts/retros"
		why:   "CC-15: the operator contract's completion rule made mechanical; an uncited claim is exactly the misdirection the next session inherits. UTF-8, SHA-256, deadbeef and a bare ticket number are prose, not citations. An undated note is skipped and named. Remedy: fathom thread close <id> --why <sha|row>"
		check: {kind: "handoff_cites_instruments", days: 7}
	}
}
