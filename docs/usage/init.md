### `init`

One-command, idempotent, additive-only repo wiring (§5.5 of
[coding-agent-excellence.md](../design/coding-agent-excellence.md)): writes
`.fathomignore` if absent, ingests the repo via the daemon and confirms it indexed,
wires the SessionStart/PostToolUse/Stop hooks into `.claude/settings.json`, stamps the
fathom operating-contract stanza into `AGENTS.md` (ADR-059: the one instruction file, read
natively by Claude Code 2.1.277+ and Codex; a root `CLAUDE.md`, `.claude/CLAUDE.md` or
`CLAUDE.local.md` that already exists gets one marked region holding `@AGENTS.md`, and
init never creates one), and write-if-absent scaffolds `~/.fathom/config.toml`.

```sh
fathom init [--path <dir>] [--service <id>] [--url http://localhost:7575] [--bin <path>] \
  [--harness claude-code|codex|gemini-cli|cursor|copilot|agents|none] \
  [--skills[=claude|codex|agents|all|codex-prompts]] [--seed-user-ignore] [--no-scip] [--json]
```

| Flag | Default | Description |
|---|---|---|
| `--path` | cwd | Repo root to wire |
| `--service` | resolved from `--path` | Service ID — the same identity the hooks use |
| `--url` | `http://localhost:7575` | fathom `serve-http` base URL |
| `--bin` | `os.Executable()`, symlink-resolved | fathom binary path baked into the hook commands |
| `--harness` | `claude-code` | Which registry descriptor's vehicles to write (ADR-029); `none` = stanza + `.fathom/` only. Prints the descriptor's own `mcp add` line, never runs it |
| `--skills` | omit = skip | Install the ten shipped agent skills into `.claude/skills/`, the one real copy (spec-only frontmatter, held by test). Bare `--skills` (= `all`), `codex` and `agents` also make `.agents/skills` (the Agent Skills standard path) a relative link to it, through the wiring engine (receipted; an earlier real copy of fathom's own files is replaced, a foreign entry blocks); `claude` writes `.claude/skills/` only; `codex-prompts` writes the deprecated user-global `~/.codex/prompts/` twin — the only user-global write, explicit only |
| `--seed-user-ignore` | off | Seed `~/.fathom/ignore` with machine-junk defaults; prints exactly what it writes |
| `--no-scip` | off | Skip the SCIP step below: by default init runs `scip-typescript` / `scip-python` when already installed for a repo of that language |
| `--harness` | `claude-code` | Which registry descriptor's vehicles to write (ADR-029): `claude-code` (hooks into `.claude/settings.json`), `codex` (hooks into the repo's `.codex/hooks.json`, run after `/hooks` trust), `gemini-cli` (hooks into the repo's `.gemini/settings.json`, ms timeouts, and its `context.fileName` set to `["AGENTS.md", "GEMINI.md"]` so Gemini CLI reads the stanza — no `GEMINI.md` stanza), `cursor` / `copilot` (data rows: stanza + skills, no hook file), `agents` (AGENTS.md + MCP), `none` (stanza + `.fathom/` only). Every harness gets the stanza in `AGENTS.md`. Non-Claude descriptors are *unverified on this machine*; every one prints its own `mcp add` line and never writes a user-global file; a descriptor 60 days unverified after its codec landed is refused |
| `--json` | off | Emit the run as JSON (below) instead of the narration |

**SCIP typed edges (SR-65, F-74).** After the wiring, once the ingest confirmed the
service, init runs `scip-typescript` (a repo with `tsconfig.json` or `package.json` at its
root; `--infer-tsconfig` when there is no `tsconfig.json`) and `scip-python` (a repo with
`pyproject.toml`, `setup.py`, `setup.cfg`, `requirements.txt` or `Pipfile`) **only when that
toolchain is already on `PATH`** — init never installs anything; a producer that is not
installed is named once ("not installed by init") and skipped, and a language the repo is
not says nothing. Each runs in the repo root under a 10-minute deadline and writes
`.fathom/scip/<name>.scip` (a directory that ignores itself). Init then calls the daemon's
`ingest` tool once more with `indexers: [...]` naming the producers that left an index;
the daemon imports each after the syntax pass, holding the ingest slot and declining an
index over its admission budget, and init prints `ok <indexer>: N typed edge(s) (D direct
/ U unresolved) via <producer>` — or `warning: <indexer> index not imported: <reason>`, or
the O115 count of definitions that could not be mapped. A producer that fails or an
import that is declined is a printed warning, never a non-zero exit. `--no-scip` skips the
step (a producer type-checks the whole repository and can take minutes).

**`--json` envelope.** `{service, root, exit_code, writes: [{path, action:
created|updated|linked|ok|shim, detail}], blocked: [{path, action: "blocked",
detail}], receipt: {id, time, files} | null, manual_steps: [{text,
command?}], narration: [...], stderr: [...]}` — every write init narrates,
the receipt the wiring engine appended during the run (`fathom fleet undo
<id>` reverses it), and every `MANUAL:` step as data with its runnable line
in `command` where there is one. Exit code is init's own.

Exit `0` — fully wired and the daemon confirms the service is indexed; exit `2` — wiring
is complete but the daemon is unreachable or the service still shows zero symbols
(operational, mirrors `verify`'s fail-open doctrine). Prints the steps it deliberately
does not do for you: interactive MCP registration and the lefthook pre-push snippet. See
[docs/adoption/claude-code.md](../adoption/claude-code.md) for the full walkthrough.

**Law 15 (SH-7/SH-8).** The AGENTS.md stanza region, a CLAUDE file's import region and `.claude/settings.json`
hooks are written only through `internal/wiring`: one plan → apply → one receipt per repo
(`.fathom/receipts.jsonl`, gitignored; `fathom fleet undo` reverses it). A tracked region
(`<!-- fathom:begin v3 sha256:… -->`) that a human edited is **blocked** (exit 1, file
byte-identical, the other files still applied, one line per file distinguishing
"drifted (hand-edited) region" from "could not be read/parsed"); a v1 region
(`<!-- fathom:begin -->`, no digest) is adopted and upgraded to v3 on the first write —
not drift. `init` continues its other steps around a blocked file and lists the blocked
files at the end. A `CLAUDE.md` region that still holds the stanza (the two-copy layout
ADR-059 retired) is rewritten in place to the `@AGENTS.md` import, with the stanza written
into `AGENTS.md` in the same receipt (`docs/upgrades/0.9.0-agents-md-one-instruction-file.md`).

**Stanza line budget.** When `.fathom/concerns.cue`'s `max_lines` concern governs a stanza
file (`AGENTS.md` since ADR-059), `init`/`reconcile --apply` trim the stanza body —
dropping the opening-moves worked example first, then the continuity paragraph — to the
smallest cut that fits the budget; a trim prints the note naming which tier it dropped.

---
