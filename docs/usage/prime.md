### `prime` — the Codex pull tier

```sh
fathom prime [--dir <project-dir>] [--url http://localhost:7575]
```

Prints the full session primer — exactly what the SessionStart hook composes
into `additionalContext` — as plain text, for a harness with no SessionStart
hook (Codex). Always exits 0 (daemon down degrades to an honest status
line). `--dir` defaults to `$CLAUDE_PROJECT_DIR`, else cwd.
