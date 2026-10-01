### `edit-hook`

Claude Code **PostToolUse** hook (`Edit`/`Write`/`MultiEdit`). Reads the hook JSON from
stdin, resolves the edited file, calls the `obligations` MCP tool for it, and injects
governing rules (≤150 tokens, ≤6 rules, each with a `src:` citation, evidence tier, and a
`code drifted since compile` flag when applicable) plus budget-permitting spec-reference
lines. Silent on a clean file and silent on any error (daemon down, file not indexed,
layer never compiled) — it never blocks a write and always exits 0.

```sh
fathom edit-hook [--url http://localhost:7575] [--pre]
```

| Flag | Default | Description |
|---|---|---|
| `--url` | `http://localhost:7575` | fathom `serve-http` base URL |
| `--pre` | off | Run as the **PreToolUse guard hook** (`docs/design/guarded-edits.md`, `.fathom/guard.cue` + `~/.fathom/guard.cue`) instead of the PostToolUse obligations hook |

Wire it under `hooks.PostToolUse` with matcher `Edit|Write|MultiEdit` (see
[adoption/claude-code.md](../adoption/claude-code.md) §7.2 for the full JSON snippet). The
`obligations` layer must be compiled for the service at least once
(`fathom compile --service <id> --dry-run=false`) before this hook has anything to inject.

**Enforcement (ADR-038).** A `.fathom/guard.cue` guard or `layer_rules` entry marked
`enforce: true` DENIES a violating PreToolUse `Edit`/`Write`/`MultiEdit` or `Bash` call
outright — the tool call never runs, and Claude Code/Codex show a one-line reason (the rule,
its citation, the remedy) in place of it — instead of only advising after the fact. Loading an
`enforce: true` rule with no deterministic check behind it (a note-only guard, or a layer rule
of kind `no_cycles`/`cross_service_direction`) is refused, naming it. `FATHOM_ENFORCE=off`, or
`[guards] enforce = false` in `.fathom/config.toml` / `~/.fathom/config.toml`, turns every
enforced rule in this repo back to advisory.

**The one-key verdict (SR-48, SR-48w, F-31).** When the same served rule tops a file's
obligations two resolved turns running, the next turn asks for a verdict and names the rule's
claim id. The user answers with their next prompt: exactly `c` (corroborate) or `r` (refute),
optionally followed by a colon and a note (`r: does not apply to generated code`). The `prompt-hook` records
only that exact shape, and only while the session has an ask pending (4 h); a word that starts
with c or r, a key followed by prose with no colon (`r u sure?`), or any other prose, is an
ordinary prompt. The following edit-hook turn writes the answer
through the same core as `fathom claim corroborate|refute` into the store the daemon records
(`--url`), attributed to the session: the claim event, the rule-outcome row, and for a refute
the status flip and the closed validity window. It says so in one line, once; if the store
cannot be written it names the `fathom claim` command that records it by hand.

The anchored-memory block (AM-6) is gone: SR-13 (surface reckoning F-45, ADR-053)
removed anchored memory — 0 memories in any repository.

---
