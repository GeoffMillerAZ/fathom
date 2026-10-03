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
	// ADR-059 (amended 2026-10-02): AGENTS.md carries every instruction. The
	// claude-code pack's claude_code_stanza_present (agents_md_reaches_claude)
	// holds when Claude Code reaches it; this repo holds itself to the stricter
	// import_only form: a CLAUDE.md may exist only as the one line `@AGENTS.md`
	// beside an AGENTS.md (the root one, which cloud sessions are documented to
	// load, and one beside each nested guide, which a root CLAUDE.md would
	// otherwise strand), never .claude/CLAUDE.md or CLAUDE.local.md (test
	// fixtures under testdata/ are input, not instruction files).
	one_instruction_file: {
		want:  "AGENTS.md carries every instruction: each CLAUDE.md is exactly `@AGENTS.md` beside an AGENTS.md, every nested AGENTS.md has one once the root CLAUDE.md exists, and no .claude/CLAUDE.md or CLAUDE.local.md exists, so Claude Code (any version, local or cloud) and Codex read the same files"
		why:   "ADR-059 decision 7 as amended 2026-10-02 (the owner-architect cloud-session decisions C1-C4): the owner's \"do not maintain CLAUDE.md\" still holds — a CLAUDE.md here carries nothing but the import, `/init` still writes a prose one, and the FileChanged config-audit hook names it the moment it lands"
		check: {kind: "agents_md_reaches_claude", import_only: true}
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
