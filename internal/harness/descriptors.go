package harness

// descriptors.go — the registry at founding (vision 25 §2.3). Every field
// here is data an adopter can read with `fathom harness list --json`;
// every envelope field comes from the verified contract notes gathered
// 2026-09-02 from each harness's primary documentation, and a field the
// documentation does not show is not encoded. Claude Code is verified;
// Codex and Gemini CLI carry codecs (wave 2, HP-17/HP-19) and ship
// `unverified` until doctor hooks_firing sees a real session's beats;
// Cursor and Copilot are data rows (descriptors_data.go); tiers C/D are
// ladder rows (ladder.go), not descriptors (ADR-029 §5).

// registry is the fixed-order descriptor list. Order is priority: the
// verified primary first, then the tier-A descriptors with codecs, then
// the data rows, then the generic pull tier. Detect and doctor's auto
// resolution walk it in order.
var registry = []Descriptor{claudeCode, codex, geminiCLI, cursor, copilot, agents}

// Claude Code's stdin/stdout field maps, per the hooks reference of
// 2026-09-02 (§4 common stdin, §6 stdout, §7 per-event).
var (
	claudeCommonIn = map[string]string{
		"session_id":      "session_id",
		"cwd":             "cwd",
		"transcript_path": "transcript_path",
	}
	claudeToolIn = map[string]string{
		"session_id":      "session_id",
		"cwd":             "cwd",
		"transcript_path": "transcript_path",
		"tool_name":       "tool_name",
		"file_path":       "tool_input.file_path",
		"command":         "tool_input.command",
	}
	claudeContextOut = map[string]string{
		"context": "hookSpecificOutput.additionalContext",
		"event":   "hookSpecificOutput.hookEventName",
	}
)

// claudeCode reproduces `fathomHookSpecs` (cmd/fathom/init.go before the
// seam) event for event: same harness events, matchers, markers, verbs
// and timeouts, in the same order. cmd/fathom's TestHookSpecsForClaude
// pins the derived table to the pre-refactor literal.
var claudeCode = Descriptor{
	ID:      ClaudeCode,
	Display: "Claude Code",
	Label:   "claude",
	Tier:    "A",
	Detect:  []string{".claude/settings.json", "~/.claude.json"},
	// Claude Code runs a command hook marked "async": true in the
	// background (RT-169).
	AsyncHooks: true,
	Instructions: []InstructionFile{
		{Path: "AGENTS.md", Mode: "native", Note: "since Claude Code 2.1.277 (the agents-md plugin, agents-md@builtin through 2.1.284 and cc-plugin-agents-md@builtin since 2.1.285; default mode claude-md-or-agents-md): loaded where CLAUDE.md would load when no CLAUDE.md, .claude/CLAUDE.md or CLAUDE.local.md exists in the working directory or any parent; the setting is honored only in user and managed settings, and fathom never writes it"},
		{Path: "CLAUDE.md", Mode: "import", Note: "read natively, and its presence switches AGENTS.md off under the default mode, so fathom writes into the first present CLAUDE file one marked region holding @AGENTS.md; fathom never creates one"},
		{Path: ".claude/CLAUDE.md", Mode: "import", Note: "the import is @../AGENTS.md: imports resolve relative to the importing file"},
		{Path: "CLAUDE.local.md", Mode: "import", Note: "a CLAUDE file for the default-mode check like the other two"},
	},
	HookFiles: []HookFile{
		{Path: ".claude/settings.json", Scope: "project", Kind: "json-hook", Shape: "claude-settings"},
		{Path: ".claude/settings.local.json", Scope: "project-local", Kind: "json-hook", Shape: "claude-settings", Note: "read by doctor, never written by init"},
		{Path: "~/.claude/settings.json", Scope: "user", Kind: "json-hook", Shape: "claude-settings", Note: "read by init and doctor; a hook wired here serves every repo, so init never duplicates it at project scope"},
	},
	Hooks: []EventBinding{
		{
			Capability: CapPrime, Event: EventSessionStart, Harness: "SessionStart", Matcher: "startup|resume|clear|compact", Verb: "session-hook", Marker: "fathom session-hook", TimeoutSec: 10, Vehicle: VehicleHookInject,
			Input:  withKey(claudeCommonIn, "source", "source"),
			Output: claudeContextOut,
		},
		{
			Capability: CapDepositSessionEnd, Event: EventSessionEnd, Harness: "SessionEnd", Matcher: "", Verb: "session-end-hook", Marker: "fathom session-end-hook", TimeoutSec: 10, Vehicle: VehicleHookSilent,
			Input: withKey(claudeCommonIn, "reason", "reason"),
			Note:  "no decision control; JSON output is discarded; 1.5s shared budget",
		},
		{
			Capability: CapDepositPreCompact, Event: EventPreCompact, Harness: "PreCompact", Matcher: "manual|auto", Verb: "pre-compact-hook", Marker: "fathom pre-compact-hook", TimeoutSec: 10, Vehicle: VehicleHookSilent,
			Input: withKey(claudeCommonIn, "trigger", "trigger"),
		},
		{
			Capability: CapObligationsPostEdit, Event: EventPostTool, Harness: "PostToolUse", Matcher: "Edit|Write|MultiEdit", Verb: "edit-hook", Marker: "fathom edit-hook", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  claudeToolIn,
			Output: claudeContextOut,
		},
		{
			Capability: CapGuardPreEdit, Event: EventPreTool, Harness: "PreToolUse", Matcher: "Edit|Write|MultiEdit", Verb: "edit-hook --pre", Marker: "fathom edit-hook --pre", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  claudeToolIn,
			Output: claudeContextOut,
			Note:   "inject-only by decision (§5.3): permissionDecision is never emitted",
		},
		{
			Capability: CapGuardPreShell, Event: EventPreTool, Harness: "PreToolUse", Matcher: "Bash", Verb: "edit-hook --pre", Marker: "fathom edit-hook --pre", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  claudeToolIn,
			Output: claudeContextOut,
		},
		{
			Capability: CapDiscoveryObserve, Event: EventPreTool, Harness: "PreToolUse", Matcher: "Grep|Glob|Read", Verb: "edit-hook --pre", Marker: "fathom edit-hook --pre", TimeoutSec: 5, Vehicle: VehicleHookSilent,
			Input:       claudeToolIn,
			Note:        "counting only: zero daemon calls, zero git forks, zero stdout; registered async so the Read never waits on it (RT-131, RT-169)",
			ObserveOnly: true,
		},
		{
			Capability: CapStopVerify, Event: EventStop, Harness: "Stop", Matcher: "", Verb: "stop-hook", Marker: "fathom stop-hook", TimeoutSec: 10, Vehicle: VehicleHookInject,
			Input:  withKey(claudeCommonIn, "stop_hook_active", "stop_hook_active"),
			Output: withKey(withKey(claudeContextOut, "block", "decision"), "reason", "reason"),
			Note:   "advisory by default; {\"decision\":\"block\"} only behind --block",
		},
		// Vision 26 §4 (PC-23, PC-24): two additive rows AFTER the pre-seam
		// eight, so the first eight stay byte-identical to what every wired
		// repository carries (TestLookupClaudeCodeReproducesHookTable pins
		// the prefix; TestClaudeHookTableAdditions pins these).
		{
			Capability: CapStanzaLoaded, Event: EventInstructionsLoaded, Harness: "InstructionsLoaded", Matcher: "", Verb: "session-hook --event instructions-loaded", Marker: "fathom session-hook --event instructions-loaded", TimeoutSec: 5, Vehicle: VehicleHookSilent,
			Input: withKey(withKey(withKey(claudeCommonIn, "file_path", "file_path"), "memory_type", "memory_type"), "load_reason", "load_reason"),
			Note:  "observability only: exit code and JSON output are ignored; no matcher, so every load_reason (session_start|nested_traversal|path_glob_match|include|compact) is recorded",
		},
		{
			Capability: CapConfigAudit, Event: EventConfigChange, Harness: "ConfigChange", Matcher: "", Verb: "config audit --hook", Marker: "fathom config audit --hook", TimeoutSec: 10, Vehicle: VehicleHookSilent,
			Input: withKey(withKey(claudeCommonIn, "source", "source"), "file_path", "file_path"),
			Note:  "never blocks: a block surfaces no message anywhere in the harness; the beat is the record and the next session line carries the count",
		},
		// Wave 3 (HP-26..HP-31), appended after the vision 26 §4 pair so
		// every earlier row keeps its position. Of the six, ONLY the two
		// the reference documents as carrying additionalContext inject;
		// the other four are silent by the harness's contract, not by
		// fathom's choice, and their notes say so (rule 3: the wiring
		// never promises a channel the harness lacks).
		{
			Capability: CapSubagentPrime, Event: EventSubagentStart, Harness: "SubagentStart", Matcher: "", Verb: "session-hook --event subagent-start", Marker: "fathom session-hook --event subagent-start", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  withKey(withKey(claudeCommonIn, "agent_id", "agent_id"), "agent_type", "agent_type"),
			Output: claudeContextOut,
			Note:   "a subagent never sees SessionStart; the primer is capped at 600 B (subagentPrimerBudget) and is silent when this root is not an indexed service",
		},
		{
			Capability: CapToolFailureAdvice, Event: EventToolFailure, Harness: "PostToolUseFailure", Matcher: "mcp__fathom__.*|mcp__plugin_[^_]*_fathom__.*", Verb: "session-hook --event tool-failure", Marker: "fathom session-hook --event tool-failure", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  withKey(withKey(claudeToolIn, "error", "error"), "tool_use_id", "tool_use_id"),
			Output: claudeContextOut,
			Note:   "the matcher covers BOTH the plain server-scoped tool names and the plugin-scoped mcp__plugin_<plugin>_<server>__<tool> form; the verb is silent for any tool that is not fathom's, so a failure anyone else owns costs nothing",
		},
		{
			Capability: CapConfigAudit, Event: EventFileChanged, Harness: "FileChanged", Matcher: "CLAUDE.md|AGENTS.md|concerns.cue|config.toml", Verb: "config audit --hook --event file-changed", Marker: "fathom config audit --hook --event file-changed", TimeoutSec: 10, Vehicle: VehicleHookSilent,
			Input: withKey(claudeCommonIn, "file_path", "file_path"),
			Note:  "the same custodian audit and the same beat ConfigChange writes, on the other trigger: a config file changed ON DISK rather than through the harness. The matcher is LITERAL FILENAMES (the reference documents no glob here), so `.fathom/**` is spelled as the two files that actually carry contracts",
		},
		{
			Capability: CapCwdPrime, Event: EventCwdChanged, Harness: "CwdChanged", Matcher: "", Verb: "session-hook --event cwd-changed", Marker: "fathom session-hook --event cwd-changed", TimeoutSec: 5, Vehicle: VehicleHookSilent,
			Input: withKey(claudeCommonIn, "previous_cwd", "previous_cwd"),
			Note:  "no additionalContext on this event either; the identity line for the new root is a beat plus stdout",
		},
		{
			Capability: CapCwdPrime, Event: EventCwdChanged, Harness: "DirectoryAdded", Matcher: "", Verb: "session-hook --event cwd-changed", Marker: "fathom session-hook --event cwd-changed", TimeoutSec: 5, Vehicle: VehicleHookSilent,
			Input: withKey(withKey(claudeCommonIn, "directory_path", "directory_path"), "how_added", "how_added"),
			Note:  "the second harness event behind one canonical event: /add-dir and the SDK name the new root in directory_path",
		},
		{
			Capability: CapSetupDoctor, Event: EventSetup, Harness: "Setup", Matcher: "", Verb: "doctor --brief", Marker: "fathom doctor --brief", TimeoutSec: 15, Vehicle: VehicleHookSilent,
			Input: withKey(claudeCommonIn, "triggered_by", "triggered_by"),
			Note:  "FAIL rows only, at most 5 lines, exit 0 whatever doctor finds — a setup diagnosis that fails setup is worse than no diagnosis",
		},
		// O57 items 4/5, appended after wave 3 for the same reason that
		// group was appended after the vision 26 §4 pair: a repo wired
		// before this change gains exactly this one entry on its next
		// init/reconcile and loses nothing. DELIBERATE GOLDEN CHANGE
		// (2026-09-14): cmd/fathom/hookspecs_test.go and this file's own
		// TestLookupClaudeCodeReproducesHookTable/TestClaudeHookTableAdditions
		// twin were updated together.
		{
			Capability: CapPromptObligations, Event: EventUserPromptSubmit, Harness: "UserPromptSubmit", Matcher: "", Verb: "prompt-hook", Marker: "fathom prompt-hook", TimeoutSec: 10, Vehicle: VehicleHookInject,
			Input:  withKey(claudeCommonIn, "prompt", "prompt"),
			Output: claudeContextOut,
			Note:   "resolves the paths, packages and symbols the prompt itself names through the index and serves the union of rules governing them, capped — governance before the first edit, not only after it (edit-hook can only answer once a file has already been written to)",
		},
		{
			Capability: CapObligationsPostShell, Event: EventPostTool, Harness: "PostToolUse", Matcher: "Bash", Verb: "edit-hook", Marker: "fathom edit-hook", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  claudeToolIn,
			Output: claudeContextOut,
			Note:   "a Bash command that writes a file (heredoc/redirection, sed -i, tee, cp/mv into the tree, git apply) never matched Edit|Write|MultiEdit and so never got obligations at all; edit-hook parses tool_input.command for a write shape and serves the written path(s), or the cwd's nearest guide on doubt (O57 item 4/5b)",
		},
		// TR-2 (docs/plans/test-run-hook.md, ADR-056 decision 6), appended
		// after the O57 pair so nothing above moves. TR-V1 read Claude
		// Code 2.1.284: a failing test command arrives on PostToolUse
		// when piped (tool_response.stdout, no exit code) and on
		// PostToolUseFailure when not (error, led by "Exit code N"), so the
		// hook rides both. Neither marker is a substring of the other.
		{
			Capability: CapTestRunLine, Event: EventPostTool, Harness: "PostToolUse", Matcher: "Bash", Verb: "session-hook --event test-run", Marker: "fathom session-hook --event test-run", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  withKey(withKey(claudeToolIn, "tool_response.stdout", "tool_response.stdout"), "tool_response.stderr", "tool_response.stderr"),
			Output: claudeContextOut,
			Note:   "opt-in ([hooks] test_run, default off): acts only on a Bash command that runs a declared test command (Taskfile, Makefile, package.json, [tests] commands) — any other command exits before a daemon call; one line on a failure, nothing on a pass or an unknown result; never rewrites the output, blocks or runs a command",
		},
		{
			Capability: CapTestRunLine, Event: EventToolFailure, Harness: "PostToolUseFailure", Matcher: "Bash", Verb: "session-hook --event failed-test-run", Marker: "fathom session-hook --event failed-test-run", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  withKey(withKey(claudeToolIn, "error", "error"), "tool_use_id", "tool_use_id"),
			Output: claudeContextOut,
			Note:   "the same hook for an unpiped test command that exited non-zero: `error` carries the output after its leading \"Exit code N\" line, the only place the harness carries an exit code",
		},
	},
	// The full event surface of Claude Code 2.1.270, as the hooks
	// reference lists it (fetched 2026-09-12). Every row fathom does not
	// wire carries the reason it stays unwired — doctor's hooks_coverage
	// prints them, so an adopter reads one table instead of diffing two
	// documentations by hand. Advisory: an unwired event is a decision.
	Events: claudeEventCatalog,
	Tools: map[string]ToolClass{
		"Edit": ToolFileEdit, "Write": ToolFileEdit, "MultiEdit": ToolFileEdit,
		"Bash": ToolShell,
		"Read": ToolDiscoveryRead, "Grep": ToolDiscoveryGrep, "Glob": ToolDiscoveryGlob,
	},
	Vehicles: map[Capability]Vehicle{
		CapPrime:                VehicleHookInject,
		CapObligationsPostEdit:  VehicleHookInject,
		CapGuardPreEdit:         VehicleHookInject,
		CapGuardPreShell:        VehicleHookInject,
		CapDiscoveryObserve:     VehicleHookSilent,
		CapStopVerify:           VehicleHookInject,
		CapDepositSessionEnd:    VehicleHookSilent,
		CapDepositPreCompact:    VehicleHookSilent,
		CapMCPTools:             VehicleMCP,
		CapSkills:               VehiclePull,
		CapStanza:               VehicleStanza,
		CapVerifyGate:           VehicleGate,
		CapClaimGate:            VehicleHookInject,
		CapSessionLedger:        VehicleHookSilent,
		CapStanzaLoaded:         VehicleHookSilent,
		CapConfigAudit:          VehicleHookSilent,
		CapSubagentPrime:        VehicleHookInject,
		CapToolFailureAdvice:    VehicleHookInject,
		CapWorktreeAdvice:       VehicleAbsent,
		CapCwdPrime:             VehicleHookSilent,
		CapSetupDoctor:          VehicleHookSilent,
		CapPromptObligations:    VehicleHookInject,
		CapObligationsPostShell: VehicleHookInject,
		CapTestRunLine:          VehicleHookInject,
	},
	Notes: map[Capability]string{
		CapPrime:                "SessionStart, gated to startup/clear (resume and compact keep their context)",
		CapStopVerify:           "advisory; `--block` turns a proven-failing claim into hook-block",
		CapDepositSessionEnd:    "writes one thread item; injects nothing",
		CapDepositPreCompact:    "writes one thread pointer; injects nothing",
		CapSkills:               "`.claude/skills/<name>/SKILL.md` only: Claude Code does not discover `.agents/skills`",
		CapClaimGate:            "the PostToolUse edit hook verifies a depth-1 docs/plans/*.md the moment an edit adds a [x] and injects the refusal; silent on a clean claim (vision 26 §4)",
		CapSessionLedger:        "the Stop hook reads the Claude transcript; measurement only",
		CapStanzaLoaded:         "InstructionsLoaded writes one silent beat {file_path, memory_type, load_reason}; doctor stanza_loaded reads it. It fires for CLAUDE files and their imports, not for an AGENTS.md loaded natively through the agents-md setting, so a repo with no CLAUDE file has no harness-side evidence",
		CapConfigAudit:          "ConfigChange and FileChanged both run `fathom config audit --hook`: one beat, an attention line next session, never a block",
		CapSubagentPrime:        "SubagentStart injects a ≤600 B index-first primer (identity line + the opening moves), silent when the root is not indexed",
		CapToolFailureAdvice:    "PostToolUseFailure injects one remedy line after a FATHOM tool call fails (plain and plugin-scoped names); silent for anyone else's failure",
		CapWorktreeAdvice:       "not hooked: WorktreeCreate is a CREATION hook in Claude Code — a registered hook must create the worktree and print its path, and any other stdout is refused (O34); the worktree line rides on SubagentStart when the subagent's cwd is a worktree",
		CapCwdPrime:             "CwdChanged / DirectoryAdded record the new root's service identity; no additionalContext on either event, so the line is a beat",
		CapSetupDoctor:          "Setup runs `fathom doctor --brief` (FAIL rows only, ≤5 lines, exit 0); the event documents no additionalContext",
		CapPromptObligations:    "UserPromptSubmit resolves the paths/packages/symbols the prompt names through the index and injects the union of rules governing them, capped (O57 items 4/5)",
		CapObligationsPostShell: "PostToolUse on Bash: edit-hook parses tool_input.command for a write shape (redirection, sed -i, tee, cp/mv, git apply) and serves the written path(s), or the cwd's nearest guide on doubt",
		CapTestRunLine:          "opt-in ([hooks] test_run): PostToolUse and PostToolUseFailure on Bash; after a declared test command fails, one line naming the failing tests and the changed symbols they reach, the full output saved under workspace/tmp; silent on a pass",
	},
	FailMode:    "exit 2 blocks on blockable events; on exit 0 the JSON decides; a timed-out or crashed hook's output is discarded and the action proceeds (fail-open)",
	GatePosture: "advisory, fail-open (§5.3)",
	ScopeMerge:  "project, project-local and user settings all apply — a hook wired at user scope fires in every repo, so init skips it at project scope",
	Skills: SkillsShape{
		RepoDirs: []string{".claude/skills/{{name}}/SKILL.md"},
		UserDirs: []string{"~/.claude/skills/{{name}}/SKILL.md"},
		Variant:  "SKILL.md",
		Verified: true,
	},
	MCP: MCPTarget{
		ConfigFiles:     []string{"~/.claude.json", ".mcp.json"},
		Format:          "json",
		KeyPath:         []string{"mcpServers", "fathom"},
		HTTPKey:         "url",
		AddCommand:      "claude mcp add --transport http --scope user fathom {{url}}/rpc/agent",
		ToolNamePattern: "mcp__{{server}}__{{tool}}",
		Note:            "type:\"http\" beside url; doctor reads ~/.claude.json. Add \"alwaysLoad\": true beside them (BP-39, ADR-032 as amended 2026-09-13): with ENABLE_TOOL_SEARCH unset Claude Code defers EVERY MCP tool schema behind ToolSearch, and a headless (-p) session pays one ToolSearch turn per cell to load fathom's (O32: 11% of the arm's tokens); alwaysLoad exempts this server. Measured 2026-09-13: plain entry 1 ToolSearch turn, alwaysLoad 0.",
	},
	Env: EnvContract{
		ProjectDir:       "CLAUDE_PROJECT_DIR",
		SessionID:        "CLAUDE_SESSION_ID",
		SessionIDAliases: []string{"CLAUDE_CODE_SESSION_ID"},
		Present:          "CLAUDECODE",
	},
	Transcript:      "claude-jsonl",
	Codec:           "claude",
	CodecLanded:     "2026-09-02",
	VerifiedAgainst: "https://code.claude.com/docs/en/hooks (2026-09-12, 33 events, against Claude Code 2.1.270); https://code.claude.com/docs/en/memory (2026-09-29, AGENTS.md and skills discovery, against Claude Code 2.1.284); doctor hooks_firing on this machine",
}

// codex is Codex CLI's descriptor with its codec (HP-17). Contract:
// hooks.md (learn.chatgpt.com/docs/hooks.md, fetched 2026-09-02, no
// version stamp on the page; local codex-cli 0.146.0). `.codex/hooks.json`
// is Claude-shaped — hooks.<Event>[{matcher, hooks:[{type,command,
// timeout}]}] — so the existing json-hook wiring editor addresses it
// unchanged; the project layer loads only when the project is trusted and
// a non-managed hook runs only after `/hooks` trust of its exact hash.
// Vehicles state what `init --harness=codex` writes TODAY; every row is
// unverified until doctor hooks_firing sees beats from a real session.
var codex = Descriptor{
	ID:      Codex,
	Display: "Codex CLI",
	Label:   "codex",
	Tier:    "A",
	Detect:  []string{".codex/config.toml", ".codex/hooks.json", "~/.codex/config.toml", "~/.codex/hooks.json"},
	Instructions: []InstructionFile{
		{Path: "AGENTS.md", Mode: "native", Note: "root-down merge of AGENTS.override.md / AGENTS.md per directory; 32 KiB default cap (project_doc_max_bytes)"},
	},
	HookFiles: []HookFile{
		{
			Path: "~/.codex/hooks.json", Scope: "user", Kind: "json-hook", Shape: "claude-settings", TimeoutUnit: "s",
			Trust: "trust the fathom entries in `/hooks` (Codex records trust against each hook's hash; a changed command re-prompts) and keep `[features] hooks = true` in config.toml",
			Note:  "THE hook file that works: hooks.<Event>[{matcher, hooks:[{type,command,timeout}]}], relocatable with CODEX_HOME. Read by doctor, NEVER written by init — user-global configuration is ask-first (docs/intent/autonomy.md) — so `init --harness=codex` prints the entries and the operator pastes them. Measured 2026-09-12 on codex-cli 0.153.1: SessionStart, Stop and SessionEnd entries here all fired; SessionEnd's timeout is clamped to 3s with a notice",
		},
		{
			Path: ".codex/hooks.json", Scope: "project", Shape: "claude-settings", TimeoutUnit: "s",
			Note: "DOCUMENTED BUT NOT LOADED by codex-cli 0.153.1 — measured 2026-09-12: `touch` hooks on SessionStart, Stop, SessionEnd and UserPromptSubmit placed here fired ZERO times in a headless `codex exec` run under a trusted parent project with --dangerously-bypass-hook-trust, while the identical entries under CODEX_HOME fired every time. No wiring Kind, so `init --harness=codex` does not write it: a file the harness ignores is worse than no file, because doctor would then read a wiring that cannot fire",
		},
	},
	Hooks: []EventBinding{
		{
			Capability: CapPrime, Event: EventSessionStart, Harness: "SessionStart", Matcher: "startup|resume|clear|compact", Verb: "session-hook --harness codex", Marker: "fathom session-hook", TimeoutSec: 10, Vehicle: VehicleHookInject,
			Input:  withKey(withKey(claudeCommonIn, "source", "source"), "model", "model"),
			Output: claudeContextOut,
			Note:   "plain stdout is also added as developer context; additionalContext capped at ~2,500 tokens (additionalContextLimit) — fathom cuts at 8,000 bytes with a named cut",
		},
		{
			Capability: CapDepositSessionEnd, Event: EventSessionEnd, Harness: "SessionEnd", Verb: "session-end-hook --harness codex", Marker: "fathom session-end-hook", TimeoutSec: 3, Vehicle: VehicleHookSilent,
			Input: withKey(claudeCommonIn, "reason", "reason"),
			Note:  "reason is always \"other\" (the verb deposits only on clear|logout|exit|prompt_input_exit, so nothing is deposited on Codex today); timeout max 3s; advisory, output never steers Codex",
		},
		{
			Capability: CapDepositPreCompact, Event: EventPreCompact, Harness: "PreCompact", Matcher: "manual|auto", Verb: "pre-compact-hook --harness codex", Marker: "fathom pre-compact-hook", TimeoutSec: 10, Vehicle: VehicleHookSilent,
			Input: withKey(claudeCommonIn, "trigger", "trigger"),
		},
		{
			Capability: CapObligationsPostEdit, Event: EventPostTool, Harness: "PostToolUse", Matcher: "apply_patch", Verb: "edit-hook --harness codex", Marker: "fathom edit-hook", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  withKey(claudeToolIn, "file_path", "tool_input.command"),
			Output: claudeContextOut,
			Note:   "apply_patch reports the patch in tool_input.command; the codec takes the first `*** Update/Add/Delete File:` header as the file (a multi-file patch is served for its first file only); the matcher may also spell Edit|Write",
		},
		{
			Capability: CapGuardPreEdit, Event: EventPreTool, Harness: "PreToolUse", Matcher: "apply_patch", Verb: "edit-hook --pre --harness codex", Marker: "fathom edit-hook --pre", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  withKey(claudeToolIn, "file_path", "tool_input.command"),
			Output: claudeContextOut,
			Note:   "inject-only (§5.3): permissionDecision is never emitted",
		},
		{
			Capability: CapGuardPreShell, Event: EventPreTool, Harness: "PreToolUse", Matcher: "Bash", Verb: "edit-hook --pre --harness codex", Marker: "fathom edit-hook --pre", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  claudeToolIn,
			Output: claudeContextOut,
			Note:   "shell and exec_command both report tool_name Bash",
		},
		{
			Capability: CapStopVerify, Event: EventStop, Harness: "Stop", Verb: "stop-hook --harness codex", Marker: "fathom stop-hook", TimeoutSec: 10, Vehicle: VehicleHookInject,
			Input:  withKey(claudeCommonIn, "stop_hook_active", "stop_hook_active"),
			Output: withKey(withKey(claudeContextOut, "block", "decision"), "reason", "reason"),
			Note:   "JSON required on exit 0 (plain text is invalid); {\"decision\":\"block\"} only behind --block",
		},
		// Wave 3: the ONE advisory event Codex shares with Claude Code.
		// The other five (PostToolUseFailure, FileChanged, WorktreeCreate,
		// CwdChanged, Setup) are absent from this harness, not declined —
		// codexAbsentEvents names each with what the adopter loses.
		{
			Capability: CapSubagentPrime, Event: EventSubagentStart, Harness: "SubagentStart", Matcher: "", Verb: "session-hook --event subagent-start --harness codex", Marker: "fathom session-hook --event subagent-start", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  withKey(withKey(claudeCommonIn, "agent_id", "agent_id"), "agent_type", "agent_type"),
			Output: claudeContextOut,
			Note:   "a Codex subagent never sees SessionStart either; the ≤600 B primer is well under the additionalContext cap, so no cut ever fires here",
		},
	},
	// Codex CLI 0.154's twelve events (0.153.1 locally). The eleven of
	// 2026-09-02 plus Interrupt.
	Events: codexEventCatalog,
	Tools: map[string]ToolClass{
		"apply_patch": ToolFileEdit, "Edit": ToolFileEdit, "Write": ToolFileEdit,
		"Bash": ToolShell,
	},
	Vehicles: map[Capability]Vehicle{
		CapPrime:                VehicleHookInject,
		CapObligationsPostEdit:  VehicleHookInject,
		CapGuardPreEdit:         VehicleHookInject,
		CapGuardPreShell:        VehicleHookInject,
		CapDiscoveryObserve:     VehicleAbsent,
		CapStopVerify:           VehicleHookInject,
		CapDepositSessionEnd:    VehicleHookSilent,
		CapDepositPreCompact:    VehicleHookSilent,
		CapMCPTools:             VehicleMCP,
		CapSkills:               VehiclePull,
		CapStanza:               VehicleStanza,
		CapVerifyGate:           VehicleGate,
		CapClaimGate:            VehicleAbsent,
		CapSessionLedger:        VehicleAbsent,
		CapStanzaLoaded:         VehicleAbsent,
		CapConfigAudit:          VehicleAbsent,
		CapSubagentPrime:        VehicleHookInject,
		CapToolFailureAdvice:    VehicleAbsent,
		CapWorktreeAdvice:       VehicleAbsent,
		CapCwdPrime:             VehicleAbsent,
		CapSetupDoctor:          VehicleAbsent,
		CapPromptObligations:    VehicleAbsent,
		CapObligationsPostShell: VehicleAbsent,
		CapTestRunLine:          VehicleAbsent,
	},
	Notes: map[Capability]string{
		CapPrime:                "SessionStart entry in CODEX_HOME/hooks.json, gated to startup/clear; runs only after `/hooks` trust. MEASURED firing 2026-09-12 — `fathom prime` stays the fallback for an untrusted or unwired install",
		CapObligationsPostEdit:  "PostToolUse on apply_patch (first file of the patch); MEASURED firing 2026-09-12 on a real patch. The `obligations` MCP tool is the fallback until the hook is trusted",
		CapGuardPreEdit:         "PreToolUse on apply_patch, inject-only (no permissionDecision); MEASURED firing 2026-09-12",
		CapGuardPreShell:        "PreToolUse on Bash (shell and exec_command), inject-only",
		CapDiscoveryObserve:     "Codex has no Read/Grep/Glob tools to observe; shell searches are unobserved",
		CapStopVerify:           "advisory; `--block` renders {\"decision\":\"block\"} — the pre-push verify gate is the backstop",
		CapDepositSessionEnd:    "SessionEnd reason is always \"other\", which the verb does not deposit on — `fathom thread note` before ending a session",
		CapDepositPreCompact:    "PreCompact entry writes one thread pointer; injects nothing",
		CapSkills:               "`.agents/skills/<name>/SKILL.md` (`fathom init --skills=agents`); `~/.codex/prompts` is the legacy twin. Discovery on this build is still unverified — the verifying session did not invoke a skill",
		CapClaimGate:            "the PostToolUse hook IS wired here, but it serves obligations only — the claim check rides Claude Code's edit payload and is not ported; the pre-push verify gate is the backstop (vision 26 §4)",
		CapSessionLedger:        "unmeasured by design (law 12): the session ledger and the ON/OFF arm take no Codex row; the rollout's token_count events are for cost only, never a savings claim; the reader, CodexSessionCost (PE-14, outcome economics), is a library no verb or hook calls yet",
		CapStanzaLoaded:         "no instructions-loaded event is documented; the stanza check reads the file, not the harness",
		CapConfigAudit:          "run `fathom config audit` by hand; no config-change event is documented",
		CapSubagentPrime:        "SubagentStart in CODEX_HOME/hooks.json injects the ≤600 B index-first primer; wired and documented, but no subagent ran in the verifying session — the one wave-3 row on this harness that is still unmeasured",
		CapToolFailureAdvice:    "Codex documents no post-failure event — a failed fathom tool call gets no remedy line; run `fathom doctor`",
		CapWorktreeAdvice:       "Codex documents no worktree event",
		CapCwdPrime:             "Codex documents no cwd-change or directory-added event",
		CapSetupDoctor:          "Codex documents no setup event — run `fathom doctor --brief` by hand after wiring",
		CapPromptObligations:    "Codex documents a UserPromptSubmit-shaped event but fathom has not ported prompt-hook to it yet — `fathom prime` covers identity/freshness, and `obligations` is a call away",
		CapObligationsPostShell: "not ported to Codex yet — PostToolUse on apply_patch answers only the patch's own file",
		CapTestRunLine:          "not ported to Codex yet — the test-run hook reads Claude Code's Bash envelope only",
	},
	FailMode:    "exit 2 + stderr blocks PreToolUse and UserPromptSubmit and continues Stop; other non-zero exits and timeouts are reported as hook failures and the action proceeds (fail-open); a non-managed hook runs only after /hooks trust of its exact hash; features.hooks gates all; SessionEnd's timeout is clamped to 3s with a notice (measured)",
	GatePosture: "advisory, fail-open (§5.3); the hooks run only after /hooks trust — the git-native verify gate is the only enforcement",
	ScopeMerge:  "MEASURED 2026-09-12 on codex-cli 0.153.1: only the CODEX_HOME layer loads. The documentation's project layer (<repo>/.codex/hooks.json) did not load even under directory trust, so a per-repo hook wiring is not achievable on this build — every fathom entry is user-global and therefore fires in EVERY repo, which is why the hook verbs resolve their repo from the envelope's cwd rather than assuming one",
	Skills: SkillsShape{
		RepoDirs: []string{".agents/skills/{{name}}/SKILL.md"},
		UserDirs: []string{"~/.agents/skills/{{name}}/SKILL.md", "~/.codex/prompts/{{name}}.md"},
		Variant:  "SKILL.md",
		Verified: false,
		Note:     "~/.codex/prompts/<name>.md is the deprecated custom-prompts twin (`--skills=codex`); .agents/skills is the documented path, unverified here",
	},
	MCP: MCPTarget{
		ConfigFiles:     []string{"~/.codex/config.toml", ".codex/config.toml"},
		Format:          "toml",
		KeyPath:         []string{"mcp_servers", "fathom"},
		HTTPKey:         "url",
		AddCommand:      "codex mcp add fathom --url {{url}}/rpc/agent",
		StdioCommand:    "codex mcp add fathom -- /abs/path/bin/fathom serve-mcp --store /abs/path/workspace/db/fleet.sqlite",
		ToolNamePattern: "mcp__{{server}}__{{tool}}",
		Note:            "`codex mcp add` writes ~/.codex/config.toml ([mcp_servers.fathom] url=); printed by init, never written. VERIFIED 2026-09-12: a headless `codex exec` session called the fathom MCP tool list_services over /rpc/agent and got all 20 services back, so streamable-HTTP negotiation is a measurement here, not a hope",
	},
	Env: EnvContract{
		Home: "CODEX_HOME",
		Note: "no project-dir or session-id variable is documented; hooks read cwd and session_id from stdin",
	},
	Codec:       "codex",
	CodecLanded: "2026-09-02",
	// VERIFIED (HP-34). A headless `codex exec --json` session on this
	// workstation ran fathom's own hook entries and left beats stamped
	// harness=codex: SessionStart (the session brief), PreToolUse and
	// PostToolUse on apply_patch (the guard and the obligations payload),
	// Stop and SessionEnd. The captured stdin of all five is replayed
	// through this codec by TestCodexHooksFixtureFromLiveSession
	// (testdata/codecs/codex/*.live.in.json), and it decoded every one
	// without a change to the codec — the 2026-09-02 documented contract
	// held against the running binary, including apply_patch reporting its
	// whole patch in tool_input.command.
	VerifiedAgainst: "codex-cli 0.153.1 (2026-09-12, headless exec): hooks fired from CODEX_HOME/hooks.json, five live stdin captures replay through CodexCodec, and the MCP surface answered list_services over /rpc/agent",
}

// geminiCLI is Gemini CLI's descriptor with its codec (HP-19). Contract:
// geminicli.com/docs/hooks (reference.md e09410b 2026-04-10, index.md
// a4318f2 2026-04-13; release v0.58.0 2026-09-01; a local install exists
// on this workstation). Hooks live under the `hooks` key of settings.json
// — project `.gemini/settings.json` is a documented location, so init
// writes there through the json-hook editor with timeouts in ms. Output
// carries a flat decision plus hookSpecificOutput.additionalContext; a
// parse failure means Allow (failMode). fathom never emits deny.
// geminiWave3Note is the honest row for every wave-3 capability on a
// harness whose event surface wave 3 did not catalogue: absent HERE is
// "fathom has not verified an equivalent event", never "the harness has
// none" — a claim nobody checked may not be printed as a fact.
const geminiWave3Note = "not catalogued: wave 3 catalogued Claude Code and Codex event surfaces only — `absent` here means unverified, not known-missing"

var geminiCLI = Descriptor{
	ID:      GeminiCLI,
	Display: "Gemini CLI",
	Label:   "gemini",
	Tier:    "A",
	Detect:  []string{".gemini/settings.json", "~/.gemini/settings.json"},
	Instructions: []InstructionFile{
		{Path: "GEMINI.md", Mode: "native", Note: "global ~/.gemini/GEMINI.md, then workspace and parent directories, then just-in-time per accessed directory; all concatenated"},
		{
			Path: "AGENTS.md", Mode: "settings-key",
			Note:    "read only when settings.json context.fileName lists it — the key REPLACES the default list, so it names GEMINI.md too; `init --harness=gemini-cli` writes it through the wiring engine (ADR-059)",
			Setting: &InstructionSetting{File: ".gemini/settings.json", Key: []string{"context", "fileName"}, Value: []string{"AGENTS.md", "GEMINI.md"}},
		},
	},
	HookFiles: []HookFile{
		{
			Path: ".gemini/settings.json", Scope: "project", Kind: "json-hook", Shape: "gemini-settings", TimeoutUnit: "ms",
			Trust: "accept the fathom entries when Gemini CLI warns about new project hooks (`/hooks panel`; project hooks are fingerprinted and any command change re-prompts)",
			Note:  "hooks.<Event>[{matcher, sequential, hooks:[{name,type:\"command\",command,timeout(ms),description}]}] under the settings file; written by `init --harness=gemini-cli` through the wiring engine (receipt, `fleet undo`) beside any mcpServers/context keys already there",
		},
		{Path: "~/.gemini/settings.json", Scope: "user", Shape: "gemini-settings", Note: "never written by init (user-global is ask-first)"},
	},
	Hooks: []EventBinding{
		{
			Capability: CapPrime, Event: EventSessionStart, Harness: "SessionStart", Matcher: "", Verb: "session-hook --harness gemini-cli", Marker: "fathom session-hook", TimeoutSec: 10, Vehicle: VehicleHookInject,
			Input:  withKey(withKey(claudeCommonIn, "source", "source"), "timestamp", "timestamp"),
			Output: claudeContextOut,
			Note:   "lifecycle matchers are exact strings, not regexes, so the entry matches every source and the verb's own startup/clear gate decides; source is startup|resume|clear",
		},
		{
			Capability: CapDepositSessionEnd, Event: EventSessionEnd, Harness: "SessionEnd", Verb: "session-end-hook --harness gemini-cli", Marker: "fathom session-end-hook", TimeoutSec: 10, Vehicle: VehicleHookSilent,
			Input: withKey(claudeCommonIn, "reason", "reason"),
			Note:  "reason is exit|clear|logout|prompt_input_exit|other; not awaited (best effort)",
		},
		{
			Capability: CapDepositPreCompact, Event: EventPreCompact, Harness: "PreCompress", Matcher: "", Verb: "pre-compact-hook --harness gemini-cli", Marker: "fathom pre-compact-hook", TimeoutSec: 10, Vehicle: VehicleHookSilent,
			Input: withKey(claudeCommonIn, "trigger", "trigger"),
			Note:  "fired asynchronously; advisory; exact-string matcher, so the entry matches auto and manual alike",
		},
		{
			Capability: CapObligationsPostEdit, Event: EventPostTool, Harness: "AfterTool", Matcher: "write_file|replace", Verb: "edit-hook --harness gemini-cli", Marker: "fathom edit-hook", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  withKey(claudeToolIn, "file_path", "tool_input.file_path"),
			Output: claudeContextOut,
			Note:   "the per-tool shape of tool_input is UNSHOWN in the hooks docs; file_path is the built-in write_file/replace argument name; additionalContext is appended to the tool result",
		},
		{
			Capability: CapGuardPreEdit, Event: EventPreTool, Harness: "BeforeTool", Matcher: "write_file|replace", Verb: "edit-hook --pre --harness gemini-cli", Marker: "fathom edit-hook --pre", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  withKey(claudeToolIn, "file_path", "tool_input.file_path"),
			Output: map[string]string{"context": "systemMessage"},
			Note:   "BeforeTool documents hookSpecificOutput.tool_input (rewrite) and decision deny; fathom emits neither (§5.3) — additionalContext on BeforeTool is UNSHOWN, so the guard text goes out as systemMessage (the user's terminal, not the model)",
		},
		{
			Capability: CapStopVerify, Event: EventStop, Harness: "AfterAgent", Verb: "stop-hook --harness gemini-cli", Marker: "fathom stop-hook", TimeoutSec: 10, Vehicle: VehicleHookInject,
			Input:  withKey(claudeCommonIn, "stop_hook_active", "stop_hook_active"),
			Output: map[string]string{"context": "systemMessage", "reason": "systemMessage"},
			Note:   "decision deny would force a retry with reason as the new prompt; fathom never emits deny, so the verdict (even behind --block) is a systemMessage — advisory by construction",
		},
	},
	Tools: map[string]ToolClass{
		"write_file": ToolFileEdit, "replace": ToolFileEdit,
	},
	Vehicles: map[Capability]Vehicle{
		CapPrime:                VehicleHookInject,
		CapObligationsPostEdit:  VehicleHookInject,
		CapGuardPreEdit:         VehicleHookInject,
		CapGuardPreShell:        VehicleAbsent,
		CapDiscoveryObserve:     VehicleAbsent,
		CapStopVerify:           VehicleHookInject,
		CapDepositSessionEnd:    VehicleHookSilent,
		CapDepositPreCompact:    VehicleHookSilent,
		CapMCPTools:             VehicleMCP,
		CapSkills:               VehiclePull,
		CapStanza:               VehicleStanza,
		CapVerifyGate:           VehicleGate,
		CapClaimGate:            VehicleAbsent,
		CapSessionLedger:        VehicleAbsent,
		CapStanzaLoaded:         VehicleAbsent,
		CapConfigAudit:          VehicleAbsent,
		CapSubagentPrime:        VehicleAbsent,
		CapToolFailureAdvice:    VehicleAbsent,
		CapWorktreeAdvice:       VehicleAbsent,
		CapCwdPrime:             VehicleAbsent,
		CapSetupDoctor:          VehicleAbsent,
		CapPromptObligations:    VehicleAbsent,
		CapObligationsPostShell: VehicleAbsent,
		CapTestRunLine:          VehicleAbsent,
	},
	Notes: map[Capability]string{
		CapSubagentPrime:        geminiWave3Note,
		CapToolFailureAdvice:    geminiWave3Note,
		CapWorktreeAdvice:       geminiWave3Note,
		CapCwdPrime:             geminiWave3Note,
		CapSetupDoctor:          geminiWave3Note,
		CapPromptObligations:    "not ported to Gemini CLI yet — `fathom prime` covers identity/freshness, and `obligations` is a call away",
		CapObligationsPostShell: "not ported to Gemini CLI yet",
		CapTestRunLine:          "not ported to Gemini CLI yet",
		CapPrime:                "SessionStart entry in .gemini/settings.json (all sources; the verb gates to startup/clear); runs once accepted in `/hooks` — `fathom prime` is the fallback",
		CapObligationsPostEdit:  "AfterTool on write_file|replace via additionalContext; the `obligations` MCP tool is the fallback until the hook is accepted",
		CapGuardPreEdit:         "BeforeTool on write_file|replace as systemMessage (terminal, not model context) — additionalContext on BeforeTool is UNSHOWN",
		CapGuardPreShell:        "the shell tool's hook name is UNSHOWN in the verified notes; not encoded",
		CapDiscoveryObserve:     "read/search tool names are UNSHOWN in the verified notes; not encoded",
		CapStopVerify:           "AfterAgent as systemMessage; never deny — the pre-push verify gate is the backstop",
		CapDepositSessionEnd:    "SessionEnd entry writes one thread item; injects nothing",
		CapDepositPreCompact:    "PreCompress entry writes one thread pointer; injects nothing",
		CapSkills:               "`.agents/skills/<name>/SKILL.md` (`fathom init --skills=agents`); the alias takes precedence over .gemini/skills",
		CapStanza:               "GEMINI.md natively; AGENTS.md only via settings context.fileName",
		CapClaimGate:            "no AfterTool hook is wired by this build; the pre-push verify gate is the backstop (vision 26 §4)",
		CapSessionLedger:        "unmeasured by design (law 12): no instrument reads a Gemini transcript",
		CapStanzaLoaded:         "no instructions-loaded event is documented; the stanza check reads the file, not the harness",
		CapConfigAudit:          "run `fathom config audit` by hand; no config-change event is documented",
	},
	FailMode:    "fail-open: any exit other than 0 or 2 is a warning and the action proceeds; any non-JSON byte on stdout is a parse failure that means Allow with the whole output shown as systemMessage; exit 2 blocks with stderr as the reason; SessionEnd and PreCompress are not awaited",
	GatePosture: "advisory, fail-open (§5.3); parse failure means allow; hooks written by init run once accepted in /hooks — the git-native verify gate is the only enforcement",
	ScopeMerge:  "project settings over user over system over extensions (documented as precedence; whether a lower layer's same-event hooks still run is UNSHOWN)",
	Skills: SkillsShape{
		RepoDirs: []string{".agents/skills/{{name}}/SKILL.md", ".gemini/skills/{{name}}/SKILL.md"},
		UserDirs: []string{"~/.agents/skills/{{name}}/SKILL.md", "~/.gemini/skills/{{name}}/SKILL.md"},
		Variant:  "SKILL.md",
		Verified: false,
		Note:     ".claude/skills is not read",
	},
	MCP: MCPTarget{
		ConfigFiles:     []string{".gemini/settings.json", "~/.gemini/settings.json"},
		Format:          "json",
		KeyPath:         []string{"mcpServers", "fathom"},
		HTTPKey:         "httpUrl",
		AddCommand:      "add to .gemini/settings.json (or ~/.gemini/settings.json): {\"mcpServers\":{\"fathom\":{\"httpUrl\":\"{{url}}/rpc/agent\"}}}",
		ToolNamePattern: "mcp_{{server}}_{{tool}}",
		Note:            "httpUrl selects streamable HTTP; url means SSE; printed by init, never written",
	},
	Env: EnvContract{
		ProjectDir: "GEMINI_PROJECT_DIR",
		SessionID:  "GEMINI_SESSION_ID",
		Note:       "given to hook processes (CLAUDE_PROJECT_DIR is an alias); whether the agent's shell sees them is UNSHOWN",
	},
	Codec:           "gemini",
	CodecLanded:     "2026-09-02",
	VerifiedAgainst: "unverified",
}

// agents is the generic pull tier (tier C/D harnesses: AGENTS.md + MCP +
// skills, no hooks). It is never auto-detected and never demoted; its
// capability table is the honest floor every other descriptor degrades to.
var agents = Descriptor{
	ID:      Agents,
	Display: "AGENTS.md harness (generic pull tier)",
	Label:   "agents",
	Tier:    "D",
	Detect:  []string{},
	Instructions: []InstructionFile{
		{Path: "AGENTS.md", Mode: "native", Note: "native or opt-in on 13 of 15 surveyed harnesses"},
	},
	Vehicles: pullTierVehicles(),
	Notes: map[Capability]string{
		CapPrime:               "run `fathom prime` at session start",
		CapObligationsPostEdit: "call the `obligations` MCP tool before editing",
		CapGuardPreEdit:        "no injection here",
		CapGuardPreShell:       "no injection here",
		CapDiscoveryObserve:    "no injection here",
		CapStopVerify:          "the pre-push verify gate is the backstop",
		CapDepositSessionEnd:   "`fathom thread note` before ending a session",
		CapDepositPreCompact:   "no injection here",
		CapSkills:              "`.agents/skills/<name>/SKILL.md` (`fathom init --skills=agents`)",
		CapClaimGate:           "no hooks; the pre-push verify gate is the backstop (vision 26 §4)",
		CapSessionLedger:       "unmeasured by design (law 12)",
		CapStanzaLoaded:        "no hooks; the stanza check reads the file, not the harness",
		CapConfigAudit:         "run `fathom config audit` by hand",
	},
	FailMode:    "no hook mechanism: nothing to fail",
	GatePosture: "no hooks — the git-native verify gate is the only enforcement",
	Skills: SkillsShape{
		RepoDirs: []string{".agents/skills/{{name}}/SKILL.md"},
		Variant:  "SKILL.md",
		Verified: false,
	},
	MCP: MCPTarget{
		AddCommand: "register {{url}}/rpc/agent as a streamable-HTTP MCP server named fathom with your harness's own `mcp add` (or its MCP config file)",
	},
	VerifiedAgainst: "unverified",
}

// pullTierVehicles is what fathom delivers with no hooks wired: the pull
// tier plus the git-native gates. Each descriptor gets its own copy.
func pullTierVehicles() map[Capability]Vehicle {
	return map[Capability]Vehicle{
		CapPrime:               VehiclePull,
		CapObligationsPostEdit: VehicleMCP,
		CapGuardPreEdit:        VehicleAbsent,
		CapGuardPreShell:       VehicleAbsent,
		CapDiscoveryObserve:    VehicleAbsent,
		CapStopVerify:          VehicleAbsent,
		CapDepositSessionEnd:   VehiclePull,
		CapDepositPreCompact:   VehicleAbsent,
		CapMCPTools:            VehicleMCP,
		CapSkills:              VehiclePull,
		CapStanza:              VehicleStanza,
		CapVerifyGate:          VehicleGate,
		CapClaimGate:           VehicleAbsent,
		CapSessionLedger:       VehicleAbsent,
		CapStanzaLoaded:        VehicleAbsent,
		CapConfigAudit:         VehicleAbsent,
		// Wave 3: all five ride hooks, and the pull tier has none.
		CapSubagentPrime:        VehicleAbsent,
		CapToolFailureAdvice:    VehicleAbsent,
		CapWorktreeAdvice:       VehicleAbsent,
		CapCwdPrime:             VehicleAbsent,
		CapSetupDoctor:          VehicleAbsent,
		CapPromptObligations:    VehicleAbsent,
		CapObligationsPostShell: VehicleAbsent,
		CapTestRunLine:          VehicleAbsent,
	}
}

// withKey returns a copy of m with one more (or replaced) entry, so the
// shared field-map literals above are never mutated.
func withKey(m map[string]string, k, v string) map[string]string {
	out := make(map[string]string, len(m)+1)
	for kk, vv := range m {
		out[kk] = vv
	}
	out[k] = v
	return out
}
