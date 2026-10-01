### `instrument`

Channel-health report over the Claude Code hooks beat log: injection coverage
(seen/answered), a per-day rollup, and liveness alarms (e.g. a session that fired
`session-hook` but never saw an `edit-hook` within a week). Advisory only — exit 0
always, even on a missing or empty log; a broken instrument must never gate anything.

```sh
fathom instrument [--log <path>] [--window 168h]
fathom instrument --adoption [--service <id>] [--call-log <path>] [--store <path>] [--root <dir>] [--url U] [--json]
```

| Flag | Default | Description |
|---|---|---|
| `--log` | `~/.fathom/hooks.jsonl` | Hooks beat log (JSONL) |
| `--window` | `168h` (7 days) | Coverage window, measured back from now |
| `--adoption` | off | The flywheel gauge: adoption report from the MCP call log — read-error classification, a surface-scoped zero-call list (tools unused on the agent surface vs not exposed there) |
| `--service` | all | (`--adoption`) scope to one service's call-log rows; unattributed rows are excluded and counted |
| `--call-log` | `[daemon].call_log`, else `<repo>/workspace/logs/mcp-calls.jsonl` (see §CLI subcommands) | (`--adoption`) MCP tool-call log |
| `--store` | — | (`--adoption`) build the registered-tool list the way `serve-http` does |
| `--root` / `--json` | `.` / off | (`--adoption`) project root for `.fathom/observations.md`; JSON output |

---
