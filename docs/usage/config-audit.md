### `config audit` — the config custodian (vision 22, "instructed means true")

```sh
fathom config audit [--repo .] [--json] [--format text|json|sarif] [--hook [--harness claude-code]]
fathom config audit [--repo .] --code-facts [--json] [--strict]
```

`--hook` is the harness **ConfigChange** hook (PC-24): it reads the event
envelope from stdin, audits the changed repo, writes one
`custodian/config-change` beat `{source, file_path, result, violated}` and
exits `0` always — never a block (a block on that event surfaces no message
anywhere in the harness); the next session's attention line carries the
violated count.

Audits registered concerns (`~/.fathom/concerns.cue` user defaults;
`.fathom/concerns.cue` repo scope, governs) against the repo: per-concern
`holds` / `violated (file:line evidence)` / `unable-to-check (reason)` /
`off (named)`. Exit `1` iff violated; zero concerns exits `0` with the
absence named. Check kinds: `forbidden_pattern`,
`command_documented_and_exists`, `max_lines`, `pointers_resolve`, plus
(CC-14/CC-15, **experimental**) `stanza_tools_served` — every tool the
fathom stanza names must be served by the daemon's `/rpc/agent`
`tools/list` (the one kind that asks the daemon; daemon down → `unable to
check` naming the URL, never violated) — and
`handoff_cites_instruments` (`days: N`) — a handoff thread note claiming
shipped work must cite a commit sha, an active plan row id, `fathom
verify`, or a `workspace/artifacts/retros/` path — and
`adr_references_resolve` — every path a decision record cites must exist,
scoped to this repo's own id shape so a sibling repo's records are named
rather than reported missing. CC-12 adds `required_pattern`,
`stanza_present`, `max_bytes` and `commands_resolve` (the gate chain: every
`task` target, local Taskfile include and repo script a corpus invokes
exists), and `where:` names a corpus — `agent-config` (default),
`lefthook`, `taskfile`, `ci` (CC-11). `packs: [...]` in a `concerns.cue`
enables the shipped `claude-code`, `codex`, `lefthook` and `taskfile` packs;
a same-id entry overrides one, `off: true` disables it. Fathom reports and
never edits your prose — the session applies fixes. Full model:
`docs/design/config-custodian.md`.

`agents_md_reaches_claude` (ADR-059) holds when Claude Code will load AGENTS.md: no
root CLAUDE file (2.1.277 and later read AGENTS.md natively), or one that imports
`@AGENTS.md`. With `import_only: true` (CS-5, ADR-059 amendment 2026-10-02) it admits only
a plain `CLAUDE.md` whose whole content is exactly `@AGENTS.md`, beside an existing
AGENTS.md, and refuses `.claude/CLAUDE.md` and `CLAUDE.local.md`; once a root CLAUDE.md
exists, every nested AGENTS.md needs its own one-line CLAUDE.md beside it, or Claude Code
stops loading that guide. `import_only: true` with `allow_import: false` is refused at load
as a contradiction.

`--code-facts` (VB-28, *experimental*, **off by default**): audit
code-fact-shaped sentences across every auto-loaded file — CLAUDE.md,
AGENTS.md, `.cursor/rules/*`, `.windsurf/rules/*`, the Cline memory bank
(`memory-bank/*.md`), and Claude Code's per-project auto-memory
(`~/.claude/projects/<encoded-root>/memory/*.md`) — classifying each as
`anchored-fresh`, `anchored-drifted` (with a receipt naming what the tree
says now), `unanchorable-code-claim`, or `preference`. Advisory: exits `0`
regardless of findings unless `--strict` is also given, in which case any
`anchored-drifted` sentence exits `1`. A file this cannot read is named on
stderr and the run reports itself as a lower bound, never a silent clean
bill. Stays off by default until precision holds ≥ 0.8 on a real labelled
set (VB-29, `docs/vision/22-config-custodian.md` §3c).
