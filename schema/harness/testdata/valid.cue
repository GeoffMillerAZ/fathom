// valid.cue — positive fixture for the harness descriptor schema.
// This file must pass `cue vet ./...` from schema/harness.
package harness

registry: #Registry & [
	{
		id:      "example-harness"
		display: "Example Harness"
		label:   "example"
		tier:    "A"
		detect: [".example/settings.json", "~/.example.json"]
		instruction_files: [
			{path: "AGENTS.md", mode: "native"},
		]
		hook_files: [
			{path: ".example/settings.json", scope: "project", kind: "json-hook", shape: "claude-settings"},
		]
		events: [
			{event: "SessionStart", wired: true},
			{event: "Notification", wired: false, reason: "nothing fathom knows that the session does not"},
		]
		hooks: [
			{
				capability:    "prime"
				event:         "session-start"
				harness_event: "SessionStart"
				matcher:       "startup|clear"
				verb:          "session-hook"
				marker:        "fathom session-hook"
				timeout_sec:   10
				vehicle:       "hook-inject"
				input: {session_id: "session_id", cwd: "cwd", source: "source"}
				output: {context: "hookSpecificOutput.additionalContext"}
			},
		]
		tools: {Edit: "file-edit", Bash: "shell"}
		vehicles: {
			"prime":                 "hook-inject"
			"obligations-post-edit": "mcp"
			"guard-pre-edit":        "absent"
			"guard-pre-shell":       "absent"
			"discovery-observe":     "absent"
			"stop-verify":           "absent"
			"deposit-session-end":   "pull"
			"deposit-pre-compact":   "absent"
			"mcp-tools":             "mcp"
			"skills":                "pull"
			"stanza":                "stanza"
			"verify-gate":           "gate"
			"claim-gate":            "absent"
			"session-ledger":        "absent"
			"stanza-loaded":         "absent"
			"config-audit":          "absent"
			"subagent-prime":        "hook-inject"
			"tool-failure-advice":   "absent"
			"worktree-advice":       "absent"
			"cwd-prime":             "absent"
			"setup-doctor":          "absent"
			"prompt-obligations":    "absent"
		}
		notes: {"claim-gate": "not built"}
		fail_mode:    "fail-open"
		gate_posture: "advisory, fail-open"
		skills: {repo_dirs: [".agents/skills/{{name}}/SKILL.md"], variant: "SKILL.md", verified: false}
		mcp: {add_command: "example mcp add fathom --url {{url}}/rpc/agent", format: "json"}
		env: {project_dir: "EXAMPLE_PROJECT_DIR"}
		codec:            "example"
		codec_landed:     "2026-09-02"
		verified_against: "unverified"
	},
]
