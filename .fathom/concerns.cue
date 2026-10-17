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
	// K8 (owner, 2026-10-09): the nested guides are pointer routers too, each
	// budgeted just above its size when it became one, so a new rule is paid
	// for by a pointer rather than a restatement (O664).
	docs_guide_thin: {
		want:  "docs/AGENTS.md stays pointers plus the plan-row traps; the plan grammar lives once, in docs/plans/README.md"
		check: {kind: "max_lines", file: "docs/AGENTS.md", lines: 40}
	}
	internal_guide_thin: {
		want:  "internal/AGENTS.md stays the engine rules and the adding-things recipes, pointing at the registry and docs rather than restating them"
		check: {kind: "max_lines", file: "internal/AGENTS.md", lines: 60}
	}
	cmd_fathom_guide_thin: {
		want:  "cmd/fathom/AGENTS.md stays the observatory's app-kit doctrine"
		check: {kind: "max_lines", file: "cmd/fathom/AGENTS.md", lines: 55}
	}
	schema_guide_thin: {
		want:  "schema/AGENTS.md stays the CUE schema conventions"
		check: {kind: "max_lines", file: "schema/AGENTS.md", lines: 15}
	}
	intent_router_thin: {
		want:  "docs/intent/README.md stays a router within the project-intent kit's 80-line router budget"
		check: {kind: "max_lines", file: "docs/intent/README.md", lines: 80}
	}
	// The registry owns the count (internal/mcp/surface.go, docs/mcp-tools.md,
	// TestMcpToolsDocMatchesRegistry); a number typed into an instruction
	// file is the copy that drifts.
	tool_count_from_registry: {
		want:  "no agent-config file hand-types an MCP tool count: the registry and docs/mcp-tools.md own it"
		why:   "O663: internal/AGENTS.md and docs/AGENTS.md both said \"25 tools\" while the default agent surface served eight; nothing compared the prose to the registry"
		check: {kind: "forbidden_pattern", pattern: "[0-9]+ (MCP )?tools\\b", where: "agent-config"}
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
	// ADR-059: AGENTS.md is this repo's one instruction file. The claude-code
	// pack's claude_code_stanza_present (agents_md_reaches_claude) holds when
	// Claude Code reaches it; this repo holds itself to the stricter form: no
	// CLAUDE file anywhere in the agent-config corpus, not even an import
	// (test fixtures under testdata/ are input, not instruction files).
	one_instruction_file: {
		want:  "AGENTS.md is this repo's only instruction file: no CLAUDE.md, .claude/CLAUDE.md, CLAUDE.local.md or nested CLAUDE.md, so Claude Code (2.1.277 and later, default mode) loads AGENTS.md natively and Codex reads the same nested guides"
		why:   "ADR-059 decision 7 (the owner's \"do not maintain CLAUDE.md\"): under Claude Code's default agents-md mode any root CLAUDE file switches AGENTS.md off, `/init` still writes one, and the FileChanged config-audit hook names it the moment it lands"
		check: {kind: "agents_md_reaches_claude", allow_import: false}
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

// K8: every intent living doc, family router and member within the
// project-intent kit's ~150-line budget (a router lists, its members under
// docs/intent/<doc>/ hold the records; `fathom intent audit` U15 holds the
// listing to the members). A new member gets its line here.
_intentDocs: {
	intent_principles_thin:            "docs/intent/principles.md"
	intent_invariants_thin:            "docs/intent/invariants.md"
	intent_questions_thin:             "docs/intent/questions.md"
	intent_autonomy_thin:              "docs/intent/autonomy.md"
	intent_incidents_thin:             "docs/intent/incidents.md"
	intent_domain_router_thin:         "docs/intent/domain.md"
	intent_domain_truths_thin:         "docs/intent/domain/truths.md"
	intent_domain_platform_facts_thin: "docs/intent/domain/platform-facts.md"
	intent_domain_assumptions_thin:    "docs/intent/domain/assumptions.md"
	intent_scope_router_thin:          "docs/intent/scope.md"
	intent_scope_goals_1_thin:         "docs/intent/scope/goals-1.md"
	intent_scope_goals_2_thin:         "docs/intent/scope/goals-2.md"
	intent_scope_goals_3_thin:         "docs/intent/scope/goals-3.md"
	intent_scope_boundaries_thin:      "docs/intent/scope/boundaries.md"
	intent_scope_non_goals_thin:       "docs/intent/scope/non-goals.md"
	intent_futures_router_thin:        "docs/intent/futures.md"
	intent_futures_f01_f10_thin:       "docs/intent/futures/f01-f10.md"
	intent_futures_f11_f22_thin:       "docs/intent/futures/f11-f22.md"
	intent_futures_f23_f32_thin:       "docs/intent/futures/f23-f32.md"
	intent_futures_f33_f44_thin:       "docs/intent/futures/f33-f44.md"
	intent_futures_f45_f53_thin:       "docs/intent/futures/f45-f53.md"
	intent_futures_f54_f61_thin:       "docs/intent/futures/f54-f61.md"
	intent_futures_f62_f67_thin:       "docs/intent/futures/f62-f67.md"
}

concerns: {
	for id, doc in _intentDocs {
		(id): {
			want:  "\(doc) stays within the project-intent kit's ~150-line living-doc budget"
			why:   "O665: futures.md reached 873 lines, scope.md 411 and domain.md 221 with nothing measuring them; a living doc past its budget is a roadmap in disguise"
			check: {kind: "max_lines", file: doc, lines: 150}
		}
	}
}
