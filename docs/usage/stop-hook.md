### `stop-hook`

Claude Code **Stop** hook (ADR-018 §4). Diffs which `docs/plans/*.md` tasks were newly
claimed `[x]` **during this session** (working tree vs. the file's state at git HEAD) and
re-verifies just those against the index. Silent (exit 0) on `stop_hook_active`, no git,
no plans, or any operational failure. If any newly-claimed task fails verification, it
emits advisory `additionalContext` by default, or `{"decision":"block"}` with `--block`.

```sh
fathom stop-hook [--url http://localhost:7575] [--block]
```

| Flag | Default | Description |
|---|---|---|
| `--url` | `defaultFathomURL()` | fathom `serve-http` base URL |
| `--block` | `false` | Block session end on a disproven claim instead of just surfacing it |

Every plan it verifies leaves a `~/.fathom/verify-ledger.jsonl` row with
`origin: stop` (PC-22); `fathom verify` rows carry `origin: cli`, and the
edit hook's claim-moment rows carry `origin: claim-moment` — the PostToolUse
`edit-hook` runs the same verify for one plan the moment an edit adds a
`[x]` to a depth-1 `docs/plans/*.md` and injects the refusal (silent on a
clean claim or any operational failure).
