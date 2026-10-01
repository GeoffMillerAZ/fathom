### `session-hook`

Emit Claude Code **SessionStart** hook JSON that primes the session to use fathom's index
before falling back to Grep/Read. It pings the shared serve-http daemon, maps the project
dir (`$CLAUDE_PROJECT_DIR`, else cwd) to a service, and prints a one-line `additionalContext`
— or an honest "daemon down / repo not indexed" signal. Always exits 0; never stalls startup.

```sh
fathom session-hook [--url http://localhost:7575] [--dir <project-dir>] [--event instructions-loaded]
```

| Flag | Default | Description |
|---|---|---|
| `--url` | `http://localhost:7575` | fathom `serve-http` base URL |
| `--dir` | `$CLAUDE_PROJECT_DIR`, else cwd | Project dir to resolve the service from |
| `--event` | off | Record a harness event instead of priming the session. `instructions-loaded` (the **InstructionsLoaded** hook, PC-23) writes one silent `instructions-loaded/loaded` beat `{file_path, memory_type, load_reason, session}` to `~/.fathom/hooks.jsonl` and prints nothing — the harness-side evidence `fathom doctor`'s `stanza_loaded` row reads. Never counts as a session start |

Wire it into `~/.claude/settings.json` (alongside any existing SessionStart hook):

```json
{
  "hooks": {
    "SessionStart": [
      { "matcher": "startup|resume|clear|compact",
        "hooks": [ { "type": "command", "command": "/abs/path/bin/fathom session-hook", "timeout": 10 } ] }
    ]
  }
}
```

The daemon URL can also be set with `FATHOM_HTTP`.

---
