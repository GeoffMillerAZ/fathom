package harness

// descriptors_data.go — the data-only descriptors (HP-20, ADR-029 §5):
// Cursor and GitHub Copilot document hook contracts, but no local install
// exists here and no adopter runs them (P20), so no codec is written. The
// descriptor carries the documented field maps as DATA — a codec is a
// fixture away when an adopter appears (HP-21, deferred) — and every hook
// vehicle stays absent: the generated table says "hooks: not wired",
// `init --harness=cursor|copilot` writes the stanza and skills and prints
// the MCP line, and the pull tier carries the same content.
//
// Contracts (verbatim notes gathered 2026-09-02):
//   Cursor   https://cursor.com/docs/hooks.md (no version stamp; example
//            cursor_version 1.7.2), /docs/reference/third-party-hooks.md,
//            /docs/mcp.md, /docs/skills.md, /docs/rules.md
//   Copilot  https://docs.github.com/en/copilot/reference/hooks-reference
//            (github/docs b87a8cf 2026-09-01), …/copilot-cli/customize-copilot/
//            {add-mcp-servers,add-custom-instructions,add-skills}

// cursorCommonIn is Cursor's common stdin (every hook): no session_id at
// the top level — conversation_id is the session identity, and the
// transcript is transcript_path.
var cursorCommonIn = map[string]string{
	"session_id":      "conversation_id",
	"transcript_path": "transcript_path",
	"cwd":             "cwd",
}

// cursor is Cursor's descriptor as data. Its native shape is FLAT
// (event → [{command, matcher, timeout, failClosed}]) and its outputs are
// snake_case (`additional_context`, `permission`), so the json-hook editor
// does not address `.cursor/hooks.json`: Kind is "" and init writes no
// hook file for it.
var cursor = Descriptor{
	ID:      "cursor",
	Display: "Cursor",
	Label:   "cursor",
	Tier:    "A",
	Detect:  []string{".cursor/hooks.json", ".cursor/rules", ".cursor/mcp.json", "~/.cursor/hooks.json"},
	Instructions: []InstructionFile{
		{Path: "AGENTS.md", Mode: "native", Note: "project root and any subdirectory; nested files combine, more specific instructions take precedence; no settings key"},
		{Path: ".cursor/rules/*.mdc", Mode: "settings-key", Note: "Cursor's own rules (frontmatter description/globs/alwaysApply); a plain .md there is silently ignored — fathom writes AGENTS.md instead"},
	},
	HookFiles: []HookFile{
		{Path: ".cursor/hooks.json", Scope: "project", Shape: "cursor-hooks", Note: "{version:1, hooks:{<event>:[{command, type, timeout(s), matcher, loop_limit, failClosed}]}}; FLAT entries, camelCase events; runs from the project root; trusted workspace required — not addressed by the json-hook editor, not written by init"},
		{Path: "~/.cursor/hooks.json", Scope: "user", Shape: "cursor-hooks", Note: "runs from ~/.cursor/; Enterprise → Team → Project → User priority, all matching hooks run"},
		{Path: ".claude/settings.json", Scope: "project", Shape: "claude-settings", Note: "read only when 'Include third-party Plugins, Skills, and other configs' is enabled: PreToolUse→preToolUse, PostToolUse→postToolUse, Stop→stop, SessionStart→sessionStart, SessionEnd→sessionEnd, PreCompact→preCompact; Bash→Shell, Edit→Write; Glob unsupported"},
	},
	Hooks: []EventBinding{
		{
			Capability: CapPrime, Event: EventSessionStart, Harness: "sessionStart", Verb: "session-hook --harness cursor", Marker: "fathom session-hook", TimeoutSec: 10, Vehicle: VehicleHookInject,
			Input:  map[string]string{"session_id": "session_id", "transcript_path": "transcript_path"},
			Output: map[string]string{"context": "additional_context"},
			Note:   "fire-and-forget; also returns env for later hooks; does not fire in cloud agents",
		},
		{
			Capability: CapDepositSessionEnd, Event: EventSessionEnd, Harness: "sessionEnd", Verb: "session-end-hook --harness cursor", Marker: "fathom session-end-hook", TimeoutSec: 10, Vehicle: VehicleHookSilent,
			Input: cursorCommonIn,
			Note:  "no reason field is documented",
		},
		{
			Capability: CapDepositPreCompact, Event: EventPreCompact, Harness: "preCompact", Verb: "pre-compact-hook --harness cursor", Marker: "fathom pre-compact-hook", TimeoutSec: 10, Vehicle: VehicleHookSilent,
			Input: cursorCommonIn,
			Note:  "observational; no trigger field is documented",
		},
		{
			Capability: CapObligationsPostEdit, Event: EventPostTool, Harness: "afterFileEdit", Verb: "edit-hook --harness cursor", Marker: "fathom edit-hook", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  withKey(cursorCommonIn, "file_path", "file_path"),
			Output: map[string]string{"context": "additional_context"},
			Note:   "afterFileEdit carries file_path + edits[]; additional_context is documented on postToolUse (matcher Write), whether afterFileEdit honors it is UNSHOWN",
		},
		{
			Capability: CapGuardPreEdit, Event: EventPreTool, Harness: "preToolUse", Matcher: "Write", Verb: "edit-hook --pre --harness cursor", Marker: "fathom edit-hook --pre", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input: withKey(withKey(cursorCommonIn, "tool_name", "tool_name"), "file_path", "tool_input.file_path"),
			Note:  "native preToolUse output is flat (permission, user_message, agent_message, updated_input) with NO context injection — a fathom guard here would be block-only, which §5.3 refuses; declared absent",
		},
		{
			Capability: CapGuardPreShell, Event: EventPreTool, Harness: "beforeShellExecution", Verb: "edit-hook --pre --harness cursor", Marker: "fathom edit-hook --pre", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input: withKey(cursorCommonIn, "command", "command"),
			Note:  "output is permission allow|deny|ask + user_message/agent_message; no context injection — declared absent (§5.3)",
		},
		{
			Capability: CapStopVerify, Event: EventStop, Harness: "stop", Verb: "stop-hook --harness cursor", Marker: "fathom stop-hook", TimeoutSec: 10, Vehicle: VehicleHookInject,
			Input:  withKey(withKey(cursorCommonIn, "status", "status"), "loop_count", "loop_count"),
			Output: map[string]string{"reason": "followup_message"},
			Note:   "followup_message auto-continues the agent (loop_limit 5); no advisory context output is documented",
		},
	},
	Tools: map[string]ToolClass{
		"Write": ToolFileEdit, "Shell": ToolShell, "Read": ToolDiscoveryRead, "Grep": ToolDiscoveryGrep,
	},
	Vehicles: pullTierVehicles(),
	Notes: map[Capability]string{
		CapPrime:               "run `fathom prime` at session start; sessionStart's additional_context is documented but no codec drives it (hooks: not wired)",
		CapObligationsPostEdit: "call the `obligations` MCP tool before editing; no afterFileEdit/postToolUse hook is wired by this build",
		CapGuardPreEdit:        "preToolUse has no context channel (permission only) — absent by §5.3, not merely unwired",
		CapGuardPreShell:       "beforeShellExecution has no context channel — absent by §5.3",
		CapDiscoveryObserve:    "beforeReadFile is a permission hook, not an observer; not encoded",
		CapStopVerify:          "the pre-push verify gate is the backstop; stop's followup_message would auto-continue, which fathom never asks for",
		CapDepositSessionEnd:   "`fathom thread note` before ending a session",
		CapDepositPreCompact:   "documented preCompact contract, not wired",
		CapSkills:              "`.agents/skills/<name>/SKILL.md` (`fathom init --skills=agents`); .cursor/skills, .claude/skills and .codex/skills are also read",
		CapStanza:              "AGENTS.md natively (root + nested)",
		CapClaimGate:           "not built (vision 26 §4)",
		CapSessionLedger:       "unmeasured by design (law 12): no instrument reads a Cursor transcript",
	},
	FailMode:    "fail-open by default: crash, timeout or invalid JSON allows the action through; failClosed: true per hook blocks instead; exit 2 = permission deny; other exit codes = hook failed, action proceeds",
	GatePosture: "fail-open unless failClosed; hooks not wired by this build (data row, no codec) — the git-native verify gate is the only enforcement",
	ScopeMerge:  "Enterprise → Team → Project → User; all matching hooks from every source run and higher-priority sources win when responses conflict",
	Skills: SkillsShape{
		RepoDirs: []string{".agents/skills/{{name}}/SKILL.md", ".cursor/skills/{{name}}/SKILL.md", ".claude/skills/{{name}}/SKILL.md"},
		UserDirs: []string{"~/.agents/skills/{{name}}/SKILL.md", "~/.cursor/skills/{{name}}/SKILL.md", "~/.claude/skills/{{name}}/SKILL.md"},
		Variant:  "SKILL.md",
		Verified: false,
		Note:     "nested .cursor/skills or .agents/skills anywhere in the repo are auto-scoped to that subtree",
	},
	MCP: MCPTarget{
		ConfigFiles:     []string{".cursor/mcp.json", "~/.cursor/mcp.json"},
		Format:          "json",
		KeyPath:         []string{"mcpServers", "fathom"},
		HTTPKey:         "url",
		AddCommand:      "add to .cursor/mcp.json (or ~/.cursor/mcp.json): {\"mcpServers\":{\"fathom\":{\"url\":\"{{url}}/rpc/agent\"}}}",
		ToolNamePattern: "MCP:{{tool}}",
		Note:            "no type key — transport is inferred from url; ${env:NAME}/${workspaceFolder} interpolation; printed by init, never written",
	},
	Env: EnvContract{
		ProjectDir: "CURSOR_PROJECT_DIR",
		Note:       "CLAUDE_PROJECT_DIR is an alias; CURSOR_TRANSCRIPT_PATH, CURSOR_VERSION also exported to hooks; no session-id variable is documented",
	},
	VerifiedAgainst: "unverified",
}

// copilotCommonIn is Copilot CLI's common stdin in its PascalCase
// ("Claude-format") spelling, which is the one whose payload matches the
// interchange envelope (snake_case fields, Claude tool names).
var copilotCommonIn = map[string]string{
	"session_id":      "session_id",
	"cwd":             "cwd",
	"transcript_path": "transcript_path",
}

// copilot is GitHub Copilot's descriptor as data (CLI and cloud agent;
// VS Code's Copilot Chat differs in tool names and ignores matchers). Its
// native entries are FLAT ({type, bash|powershell|command|exec, timeoutSec})
// under `.github/hooks/*.json`, so the json-hook editor does not address
// them; the CLI also reads `.claude/settings.json`, which is why a Claude
// Code wiring reaches Copilot without a codec at all.
var copilot = Descriptor{
	ID:      "copilot",
	Display: "GitHub Copilot",
	Label:   "copilot",
	Tier:    "A",
	Detect:  []string{".github/copilot-instructions.md", ".github/hooks", ".github/copilot/settings.json", "~/.copilot/mcp-config.json"},
	Instructions: []InstructionFile{
		{Path: "AGENTS.md", Mode: "native", Note: "discovered in the standard locations (repo root, cwd, intermediate dirs) with no settings key; CLAUDE.md and GEMINI.md are read the same way; VS Code needs chat.useAgentsMdFile"},
		{Path: ".github/copilot-instructions.md", Mode: "settings-key", Note: "repository-wide instructions, always on; @relative imports work — fathom writes AGENTS.md instead"},
	},
	HookFiles: []HookFile{
		{Path: ".github/hooks/*.json", Scope: "project", Shape: "copilot-hooks", Note: "{version:1, hooks:{<event>:[{type:\"command\", bash|powershell|command|exec, args, cwd, env, timeoutSec}]}}; FLAT entries; camelCase events natively, PascalCase accepted with snake_case payloads; the cloud agent reads only this file — not addressed by the json-hook editor, not written by init"},
		{Path: ".claude/settings.json", Scope: "project", Shape: "claude-settings", Note: "read by the CLI (with .claude/settings.local.json) and by VS Code; a Claude Code wiring already reaches Copilot here"},
		{Path: "~/.copilot/hooks/*.json", Scope: "user", Shape: "copilot-hooks", Note: "$COPILOT_HOME/hooks/ when set; policy .d files cannot be disabled"},
	},
	Hooks: []EventBinding{
		{
			Capability: CapPrime, Event: EventSessionStart, Harness: "SessionStart", Verb: "session-hook --harness copilot", Marker: "fathom session-hook", TimeoutSec: 10, Vehicle: VehicleHookInject,
			Input:  withKey(withKey(copilotCommonIn, "source", "source"), "timestamp", "timestamp"),
			Output: map[string]string{"context": "additionalContext"},
			Note:   "source is startup|resume|new; output is a FLAT additionalContext (the full sessionStart output shape is UNSHOWN beyond the field name)",
		},
		{
			Capability: CapDepositSessionEnd, Event: EventSessionEnd, Harness: "SessionEnd", Verb: "session-end-hook --harness copilot", Marker: "fathom session-end-hook", TimeoutSec: 10, Vehicle: VehicleHookSilent,
			Input: withKey(copilotCommonIn, "reason", "reason"),
			Note:  "reason is complete|error|abort|timeout|user_exit — none of the reasons the verb deposits on",
		},
		{
			Capability: CapDepositPreCompact, Event: EventPreCompact, Harness: "PreCompact", Matcher: "manual|auto", Verb: "pre-compact-hook --harness copilot", Marker: "fathom pre-compact-hook", TimeoutSec: 10, Vehicle: VehicleHookSilent,
			Input: withKey(copilotCommonIn, "trigger", "trigger"),
			Note:  "notification only; the cloud agent fires it with trigger auto only",
		},
		{
			Capability: CapObligationsPostEdit, Event: EventPostTool, Harness: "PostToolUse", Matcher: "Edit|Write", Verb: "edit-hook --harness copilot", Marker: "fathom edit-hook", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input:  withKey(copilotCommonIn, "tool_name", "tool_name"),
			Output: map[string]string{"context": "additionalContext"},
			Note:   "additionalContext is appended to textResultForLlm, capped at 10 KB; tool_input's per-tool shape (edit/create arguments) is UNSHOWN, so file_path is not mapped",
		},
		{
			Capability: CapGuardPreEdit, Event: EventPreTool, Harness: "PreToolUse", Matcher: "Edit|Write", Verb: "edit-hook --pre --harness copilot", Marker: "fathom edit-hook --pre", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input: withKey(copilotCommonIn, "tool_name", "tool_name"),
			Note:  "the CLI's preToolUse output is permissionDecision/permissionDecisionReason/modifiedArgs — no context field is documented for the CLI (VS Code documents hookSpecificOutput.additionalContext); preToolUse command hooks are FAIL-CLOSED on crash or non-zero exit, so an unwired guard here would deny edits on any fathom failure — absent by construction",
		},
		{
			Capability: CapGuardPreShell, Event: EventPreTool, Harness: "PreToolUse", Matcher: "Bash", Verb: "edit-hook --pre --harness copilot", Marker: "fathom edit-hook --pre", TimeoutSec: 5, Vehicle: VehicleHookInject,
			Input: withKey(copilotCommonIn, "tool_name", "tool_name"),
			Note:  "same fail-closed preToolUse path — absent by construction",
		},
		{
			Capability: CapStopVerify, Event: EventStop, Harness: "Stop", Verb: "stop-hook --harness copilot", Marker: "fathom stop-hook", TimeoutSec: 10, Vehicle: VehicleHookInject,
			Input:  withKey(copilotCommonIn, "stop_hook_active", "stop_hook_active"),
			Output: map[string]string{"block": "decision", "reason": "reason"},
			Note:   "decision block forces another turn (runaway guard: 8 consecutive blocks); no advisory context output is documented on agentStop",
		},
	},
	Tools: map[string]ToolClass{
		"Edit": ToolFileEdit, "Write": ToolFileEdit, "Bash": ToolShell, "Read": ToolDiscoveryRead, "Grep": ToolDiscoveryGrep, "Glob": ToolDiscoveryGlob,
		"edit": ToolFileEdit, "create": ToolFileEdit, "bash": ToolShell, "view": ToolDiscoveryRead, "grep": ToolDiscoveryGrep, "glob": ToolDiscoveryGlob,
	},
	Vehicles: pullTierVehicles(),
	Notes: map[Capability]string{
		CapPrime:               "run `fathom prime` at session start; sessionStart's additionalContext is documented but no codec drives it (hooks: not wired)",
		CapObligationsPostEdit: "call the `obligations` MCP tool before editing; no postToolUse hook is wired by this build",
		CapGuardPreEdit:        "preToolUse is fail-closed on hook error (a crashed guard would deny the edit) — absent by construction, never emulated",
		CapGuardPreShell:       "same fail-closed preToolUse path — absent by construction",
		CapDiscoveryObserve:    "view/grep/glob are hookable but no observer is wired",
		CapStopVerify:          "the pre-push verify gate is the backstop",
		CapDepositSessionEnd:   "`fathom thread note` before ending a session",
		CapDepositPreCompact:   "documented preCompact contract, not wired",
		CapSkills:              "`.agents/skills/<name>/SKILL.md` (`fathom init --skills=agents`); .github/skills and .claude/skills are also read",
		CapStanza:              "AGENTS.md natively (also CLAUDE.md, GEMINI.md); VS Code needs chat.useAgentsMdFile",
		CapClaimGate:           "not built (vision 26 §4)",
		CapSessionLedger:       "unmeasured by design (law 12): no instrument reads a Copilot transcript",
	},
	FailMode:    "fail-open for every event except preToolUse command hooks, which are fail-closed on exit 2, crash or non-zero exit (timeouts are always fail-open); exit 2 is a warning elsewhere; 10 MiB output cap; disableAllHooks skips a file",
	GatePosture: "fail-open except preToolUse (fail-closed on hook error); hooks not wired by this build (data row, no codec) — the git-native verify gate is the only enforcement",
	ScopeMerge:  "policy → user → project → plugins, all combined; workspace hooks take precedence over user hooks in VS Code",
	Skills: SkillsShape{
		RepoDirs: []string{".github/skills/{{name}}/SKILL.md", ".agents/skills/{{name}}/SKILL.md", ".claude/skills/{{name}}/SKILL.md"},
		UserDirs: []string{"~/.copilot/skills/{{name}}/SKILL.md", "~/.agents/skills/{{name}}/SKILL.md"},
		Variant:  "SKILL.md",
		Verified: false,
		Note:     "first found wins; ~/.claude/skills is not a listed CLI path",
	},
	MCP: MCPTarget{
		ConfigFiles:     []string{"~/.copilot/mcp-config.json", ".mcp.json", ".github/mcp.json"},
		Format:          "json",
		KeyPath:         []string{"mcpServers", "fathom"},
		HTTPKey:         "url",
		AddCommand:      "copilot mcp add --transport http fathom {{url}}/rpc/agent",
		ToolNamePattern: "{{server}}-{{tool}}",
		Note:            "type:\"http\" beside url; .vscode/mcp.json is NOT read (servers key); project files need folder trust; printed by init, never written",
	},
	Env: EnvContract{
		Home: "COPILOT_HOME",
		Note: "no project-dir or session-id variable is documented; hooks read cwd and session_id from stdin",
	},
	VerifiedAgainst: "unverified",
}

// DataOnlyDescriptors returns, in registry order, every descriptor that
// documents a hook contract as data but carries no codec — the rows the
// generated table labels "hooks: not wired". Today: cursor and copilot.
// The generic pull tier (no hooks at all) is not a data row; it is the
// floor every descriptor degrades to.
func DataOnlyDescriptors() []Descriptor {
	var out []Descriptor
	for _, d := range registry {
		if d.Codec == "" && d.HasHooks() {
			out = append(out, d)
		}
	}
	return out
}
