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
	Instructions: []InstructionFile{
		{Path: "CLAUDE.md", Mode: "native"},
		{Path: "AGENTS.md", Mode: "mirror", Note: "the same stanza body, so every harness that reads AGENTS.md sees one contract"},
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
			Input: claudeToolIn,
			Note:  "counting only: zero daemon calls, zero stdout",
		},
		{
			Capability: CapStopVerify, Event: EventStop, Harness: "Stop", Matcher: "", Verb: "stop-hook", Marker: "fathom stop-hook", TimeoutSec: 10, Vehicle: VehicleHookInject,
			Input:  withKey(claudeCommonIn, "stop_hook_active", "stop_hook_active"),
			Output: withKey(withKey(claudeContextOut, "block", "decision"), "reason", "reason"),
			Note:   "advisory by default; {\"decision\":\"block\"} only behind --block",
		},
	},
	Tools: map[string]ToolClass{
		"Edit": ToolFileEdit, "Write": ToolFileEdit, "MultiEdit": ToolFileEdit,
		"Bash": ToolShell,
		"Read": ToolDiscoveryRead, "Grep": ToolDiscoveryGrep, "Glob": ToolDiscoveryGlob,
	},
	Vehicles: map[Capability]Vehicle{
		CapPrime:               VehicleHookInject,
		CapObligationsPostEdit: VehicleHookInject,
		CapGuardPreEdit:        VehicleHookInject,
		CapGuardPreShell:       VehicleHookInject,
		CapDiscoveryObserve:    VehicleHookSilent,
		CapStopVerify:          VehicleHookInject,
		CapDepositSessionEnd:   VehicleHookSilent,
		CapDepositPreCompact:   VehicleHookSilent,
		CapMCPTools:            VehicleMCP,
		CapSkills:              VehiclePull,
		CapStanza:              VehicleStanza,
		CapVerifyGate:          VehicleGate,
		CapClaimGate:           VehicleAbsent,
		CapSessionLedger:       VehicleHookSilent,
	},
	Notes: map[Capability]string{
		CapPrime:             "SessionStart, gated to startup/clear (resume and compact keep their context)",
		CapStopVerify:        "advisory; `--block` turns a proven-failing claim into hook-block",
		CapDepositSessionEnd: "writes one thread item; injects nothing",
		CapDepositPreCompact: "writes one thread pointer; injects nothing",
		CapSkills:            "`.claude/skills/<name>/SKILL.md` (+ `.agents/skills`)",
		CapClaimGate:         "not built (vision 26 §4)",
		CapSessionLedger:     "the Stop hook reads the Claude transcript; measurement only",
	},
	FailMode:    "exit 2 blocks on blockable events; on exit 0 the JSON decides; a timed-out or crashed hook's output is discarded and the action proceeds (fail-open)",
	GatePosture: "advisory, fail-open (§5.3)",
	ScopeMerge:  "project, project-local and user settings all apply — a hook wired at user scope fires in every repo, so init skips it at project scope",
	Skills: SkillsShape{
		RepoDirs: []string{".claude/skills/{{name}}/SKILL.md", ".agents/skills/{{name}}/SKILL.md"},
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
		Note:            "type:\"http\" beside url; doctor reads ~/.claude.json",
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
	VerifiedAgainst: "https://code.claude.com/docs/en/hooks (2026-09-02, text cites v2.1.251); local claude 2.1.258; doctor hooks_firing on this machine",
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
			Path: ".codex/hooks.json", Scope: "project", Kind: "json-hook", Shape: "claude-settings", TimeoutUnit: "s",
			Trust: "trust the fathom entries in `/hooks` (Codex records trust against each hook's hash; a changed command re-prompts) and keep `[features] hooks` enabled in config.toml",
			Note:  "hooks.<Event>[{matcher, hooks:[{type,command,timeout,statusMessage,additionalContextLimit}]}]; loads only when the project layer is trusted; written by `init --harness=codex` through the wiring engine (receipt, `fleet undo`)",
		},
		{Path: "~/.codex/hooks.json", Scope: "user", Shape: "claude-settings", Note: "inline [hooks] in config.toml is the TOML twin; one representation per layer; never written by init (user-global is ask-first)"},
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
	},
	Tools: map[string]ToolClass{
		"apply_patch": ToolFileEdit, "Edit": ToolFileEdit, "Write": ToolFileEdit,
		"Bash": ToolShell,
	},
	Vehicles: map[Capability]Vehicle{
		CapPrime:               VehicleHookInject,
		CapObligationsPostEdit: VehicleHookInject,
		CapGuardPreEdit:        VehicleHookInject,
		CapGuardPreShell:       VehicleHookInject,
		CapDiscoveryObserve:    VehicleAbsent,
		CapStopVerify:          VehicleHookInject,
		CapDepositSessionEnd:   VehicleHookSilent,
		CapDepositPreCompact:   VehicleHookSilent,
		CapMCPTools:            VehicleMCP,
		CapSkills:              VehiclePull,
		CapStanza:              VehicleStanza,
		CapVerifyGate:          VehicleGate,
		CapClaimGate:           VehicleAbsent,
		CapSessionLedger:       VehicleAbsent,
	},
	Notes: map[Capability]string{
		CapPrime:               "SessionStart entry in .codex/hooks.json, gated to startup/clear; runs only after `/hooks` trust — `fathom prime` is the fallback",
		CapObligationsPostEdit: "PostToolUse on apply_patch (first file of the patch); the `obligations` MCP tool is the fallback until the hook is trusted",
		CapGuardPreEdit:        "PreToolUse on apply_patch, inject-only (no permissionDecision)",
		CapGuardPreShell:       "PreToolUse on Bash (shell and exec_command), inject-only",
		CapDiscoveryObserve:    "Codex has no Read/Grep/Glob tools to observe; shell searches are unobserved",
		CapStopVerify:          "advisory; `--block` renders {\"decision\":\"block\"} — the pre-push verify gate is the backstop",
		CapDepositSessionEnd:   "SessionEnd reason is always \"other\", which the verb does not deposit on — `fathom thread note` before ending a session",
		CapDepositPreCompact:   "PreCompact entry writes one thread pointer; injects nothing",
		CapSkills:              "`.agents/skills/<name>/SKILL.md` (`fathom init --skills=agents`); `~/.codex/prompts` is the legacy twin",
		CapClaimGate:           "not built (vision 26 §4)",
		CapSessionLedger:       "unmeasured by design (law 12): the rollout transcript format is not stable and no instrument reads it",
	},
	FailMode:    "exit 2 + stderr blocks PreToolUse and UserPromptSubmit and continues Stop; other non-zero exits and timeouts are reported as hook failures and the action proceeds (fail-open); a non-managed hook runs only after /hooks trust of its exact hash; features.hooks gates all",
	GatePosture: "advisory, fail-open (§5.3); hooks written by init run only after /hooks trust — the git-native verify gate is the only enforcement",
	ScopeMerge:  "every matching hook from every layer runs (system, user, project, plugin, managed); a higher-precedence layer adds, never replaces; the project layer loads only when trusted",
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
		Note:            "`codex mcp add` writes ~/.codex/config.toml ([mcp_servers.fathom] url=); printed by init, never written; a project .codex/config.toml applies only to trusted projects; streamable-HTTP negotiation is unverified on this machine",
	},
	Env: EnvContract{
		Home: "CODEX_HOME",
		Note: "no project-dir or session-id variable is documented; hooks read cwd and session_id from stdin",
	},
	Codec:           "codex",
	CodecLanded:     "2026-09-02",
	VerifiedAgainst: "unverified",
}

// geminiCLI is Gemini CLI's descriptor with its codec (HP-19). Contract:
// geminicli.com/docs/hooks (reference.md e09410b 2026-04-10, index.md
// a4318f2 2026-04-13; release v0.58.0 2026-09-01; a local install exists
// on this workstation). Hooks live under the `hooks` key of settings.json
// — project `.gemini/settings.json` is a documented location, so init
// writes there through the json-hook editor with timeouts in ms. Output
// carries a flat decision plus hookSpecificOutput.additionalContext; a
// parse failure means Allow (failMode). fathom never emits deny.
var geminiCLI = Descriptor{
	ID:      GeminiCLI,
	Display: "Gemini CLI",
	Label:   "gemini",
	Tier:    "A",
	Detect:  []string{".gemini/settings.json", "~/.gemini/settings.json"},
	Instructions: []InstructionFile{
		{Path: "GEMINI.md", Mode: "native", Note: "global ~/.gemini/GEMINI.md, then workspace and parent directories, then just-in-time per accessed directory; all concatenated"},
		{Path: "AGENTS.md", Mode: "settings-key", Note: "read only when settings.json context.fileName lists it — the key REPLACES the default list, so include GEMINI.md explicitly"},
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
		CapPrime:               VehicleHookInject,
		CapObligationsPostEdit: VehicleHookInject,
		CapGuardPreEdit:        VehicleHookInject,
		CapGuardPreShell:       VehicleAbsent,
		CapDiscoveryObserve:    VehicleAbsent,
		CapStopVerify:          VehicleHookInject,
		CapDepositSessionEnd:   VehicleHookSilent,
		CapDepositPreCompact:   VehicleHookSilent,
		CapMCPTools:            VehicleMCP,
		CapSkills:              VehiclePull,
		CapStanza:              VehicleStanza,
		CapVerifyGate:          VehicleGate,
		CapClaimGate:           VehicleAbsent,
		CapSessionLedger:       VehicleAbsent,
	},
	Notes: map[Capability]string{
		CapPrime:               "SessionStart entry in .gemini/settings.json (all sources; the verb gates to startup/clear); runs once accepted in `/hooks` — `fathom prime` is the fallback",
		CapObligationsPostEdit: "AfterTool on write_file|replace via additionalContext; the `obligations` MCP tool is the fallback until the hook is accepted",
		CapGuardPreEdit:        "BeforeTool on write_file|replace as systemMessage (terminal, not model context) — additionalContext on BeforeTool is UNSHOWN",
		CapGuardPreShell:       "the shell tool's hook name is UNSHOWN in the verified notes; not encoded",
		CapDiscoveryObserve:    "read/search tool names are UNSHOWN in the verified notes; not encoded",
		CapStopVerify:          "AfterAgent as systemMessage; never deny — the pre-push verify gate is the backstop",
		CapDepositSessionEnd:   "SessionEnd entry writes one thread item; injects nothing",
		CapDepositPreCompact:   "PreCompress entry writes one thread pointer; injects nothing",
		CapSkills:              "`.agents/skills/<name>/SKILL.md` (`fathom init --skills=agents`); the alias takes precedence over .gemini/skills",
		CapStanza:              "GEMINI.md natively; AGENTS.md only via settings context.fileName",
		CapClaimGate:           "not built (vision 26 §4)",
		CapSessionLedger:       "unmeasured by design (law 12): no instrument reads a Gemini transcript",
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
		CapClaimGate:           "not built (vision 26 §4)",
		CapSessionLedger:       "unmeasured by design (law 12)",
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
