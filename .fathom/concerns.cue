// .fathom/concerns.cue — the config custodian's contracts for this repo
// (project scope, GOVERNS — law 13). docs/design/config-custodian.md.
//
// Run `fathom config audit` for evidence. Fathom reports; you decide.
//
// The shipped packs (CC-12, internal/custodian/packs) this repo holds itself
// to: the stanza in AGENTS.md (the one instruction file, ADR-059) within
// Codex's read budget, and the gate chain — docs → lefthook → Taskfile →
// scripts — unbroken.
packs: ["claude-code", "codex", "lefthook", "taskfile"]

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
		want:  "the root AGENTS.md stays a router (the 2026-08-13 compositional restructure holds)"
		check: {kind: "max_lines", file: "AGENTS.md", lines: 120}
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
	// The claude-code pack's stanza entry names CLAUDE.md, which this repo no
	// longer carries (ADR-059: AGENTS.md is the one instruction file, and
	// Claude Code reads it natively since 2.1.277). Law 13: the same id
	// replaces the pack entry, here pointed at the file that holds the stanza.
	claude_code_stanza_present: {
		want:  "AGENTS.md carries a well-formed fathom stanza region (the begin marker, then the end marker)"
		why:   "doctor stanza_present (DI-1): an AGENTS.md whose end marker a hand-edit deleted never tells a session to route through the index — and fleet reconcile refuses to repair a corrupted pair"
		check: {kind: "stanza_present", file: "AGENTS.md"}
	}
	// The claude-code/codex packs ship this same check; pointers_hold above
	// predates them and is what I34 cites, so the pack's copy is disabled
	// here (law 13: an entry disabled by id, named off in the audit).
	agent_config_pointers_resolve: {off: true}
	stanza_current: {
		want:  "every tool the fathom stanza in AGENTS.md names — backticked, or bare with an underscore (find_callers, pack_context) — is served by the running daemon's agent surface; a stanza pointing at a pruned or undeployed tool sends every session at a call that fails"
		why:   "CC-14: the operator contract's \"instructed means true\" applied to fathom's own instructions. Daemon down, or a 200 with no tools in it, is unable-to-check, never a violation. Bare single words (obligations, references, outline, ingest) are policed only when backticked — bare they are prose, and a false violation teaches people to ignore the custodian"
		check: {kind: "stanza_tools_served"}
	}
	wiring_current: {
		want:  "nothing fathom trusts changed behind its back — the hook entries, stanza regions, MCP server entries (argv and env NAMES) and fathom-owned skill files match the digests recorded in .fathom/owned.json"
		why:   "vision 26 §1.4: the year's worst agent incidents were failures of trust configuration, not detection. A drifted entry is reported by kind and path and never merged; an unreadable file is unable-to-check, never a violation (I38). Remedy: read the file; if the change is yours, fathom fleet reconcile --repo <root> --apply --rebaseline"
		check: {kind: "wiring_current"}
	}
	completion_cites: {
		want:  "a handoff thread note from this repo that claims shipped work cites the instrument that proves it — a commit sha (7–12 or 40 hex, letters and digits), a row id one of docs/plans/*.md carries, `fathom verify`, or a retro artifact under workspace/artifacts/retros"
		why:   "CC-15: the operator contract's completion rule made mechanical; an uncited claim is exactly the misdirection the next session inherits. UTF-8, SHA-256, deadbeef and a bare ticket number are prose, not citations. An undated note is skipped and named. Remedy: fathom thread close <id> --why <sha|row>"
		check: {kind: "handoff_cites_instruments", days: 7}
	}
}
