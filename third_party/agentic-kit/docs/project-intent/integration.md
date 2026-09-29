# Project Intent — agent integration

Always-loaded instructions are a router and a policy. The knowledge lives in
`docs/intent/`. The procedures live in the `intent-*` skill family, loaded on demand
by both Claude Code and Codex from one install. This file holds the canonical text of
every always-loaded stanza so no tool's file drifts from another's.

## 1. The global routing stanza (user level, every project)

Identical text in `~/.claude/CLAUDE.md` (Claude Code) and `~/.codex/AGENTS.md` (Codex).
Nothing project-specific. The only permitted difference is the last clause's
invocation syntax: `/intent-design` for Claude Code, `$intent-design` for Codex. Edit
here first, then propagate.

```markdown
## Project intent (all projects)

Some repos carry a project-intent system: a router at `docs/intent/README.md` routing
to vision, principles, invariants, domain truths, scope, future directions, and decision
records with revisit triggers. Before material product, design, domain, or architecture
work, read the router and load only what it routes you to; trivial changes load nothing.
Treat what it documents as binding design context: honor invariants and accepted
decisions, and state any conflict or fired `Revisit when` / `Promote when` condition
before designing around it. Build for now; foreclose nothing documented when keeping it
open is free, and never implement or pre-build a future. Update the records in the same
change when a material decision, scope change, or trigger firing occurs. If a repo has no
`docs/intent/`, do not invent intent; suggest bootstrapping only for substantial
long-lived work. Procedure: `/intent-design` before design work; the `intent-*` family
(init, decide, doctor, audit, prune, release, incident) for the rest.
```

## 2. The project stanzas (repo level)

Identical text in the repo's `CLAUDE.md` and `AGENTS.md`. Three short paragraphs:
routing (six lines), change records (four lines), autonomy summary (three lines). They
name real locations, because those vary.

```markdown
## Project intent

Durable intent lives at `docs/intent/README.md` (router). Vision: `VISION.md`.
Decisions: `docs/adr/` (general decision records, not only architecture; index at
`docs/adr/README.md`). Consult the router before material product, design, domain, or
architecture work and load only what it routes you to. Record decisions that pass its
threshold; state fired revisit triggers before designing around them.

Change records: conventional commit with a `Verified:` line, one commit per reviewable
unit, no WIP on main. A `CHANGELOG.md` line in the same change only for observable
behavior (compatibility surface: `docs/intent/scope.md`); `### Breaking` first; collapse
`[Unreleased]` to the delta, never the journey. Rules: `agentic-kit/docs/project-intent/changes.md`.

Autonomy (`docs/intent/autonomy.md`): commit and push to `<integration branch>` freely;
ask before merging to `main`, tagging, deleting, running the product against real user
state, or touching global agent config; never force-push `main`, skip a gate, or claim
done without `Verified:`.
```

### 2b. The nested stanza (a component with its own intent, monorepos only)

Identical text in `<component>/CLAUDE.md` and `<component>/AGENTS.md`. Their native
loading behavior differs; neither is a substitute for the root router explicitly
routing affected paths. A session started at root must read the relevant nested
instructions/router even if its host has not loaded them automatically.

```markdown
## Project intent (this component)

Intent for `<component>` lives at `<component>/docs/intent/README.md`. The repository
charter and shared invariants are at `docs/intent/README.md` (root): read applicable
root constraints and this router for affected paths, even from root cwd. Component
intent inherits autonomy and narrows applicable constraints; reopen shared decisions
at their owner rather than silently contradicting them.
```

Why the stanza appears in both files: Claude Code reads `CLAUDE.md` and not
`AGENTS.md`; Codex reads `AGENTS.md` and not `CLAUDE.md`. For a new repo prefer one
canonical `AGENTS.md` with `CLAUDE.md` containing `@AGENTS.md` plus Claude-only lines.
For an existing repo where both already carry different content, duplicate only these
paragraphs. `intent-doctor` check 15 proves they are identical.

## 3. The skill family

Eight small skills, one source, both tools. Hyphen-prefixed names (`intent-*`) keep
plain-file discovery independent of either host's plugin packaging. Each skill:
`name` and `description` frontmatter, short procedural body, rationale linked not
copied. Skills guide judgment; they do not enforce authorization or prove live wiring.

| Skill | When | Edits? |
|---|---|---|
| `intent-design` | before material design work; the default consult | no (reads, reports) |
| `intent-decide` | record, reject, defer, supersede a decision | records + index |
| `intent-init` | bootstrap `docs/intent/` in a new or existing repo; holds `templates/` | creates |
| `intent-doctor` | structural checks plus separately evidenced host wiring | never |
| `intent-audit` | truth: fired triggers, false assumptions, stale enforcement claims | proposes; edits per autonomy |
| `intent-prune` | shrink to budget; collapse `[Unreleased]`; compress closed incidents | deletes (git keeps) |
| `intent-release` | the release ritual; roll the block before the tag | changelog, tag with approval |
| `intent-incident` | record a failure and drive it to an invariant, test, or decision | `incidents.md` + outcome |

Invocation: `/intent-doctor` (Claude Code), `$intent-doctor` (Codex), or implicit when
the description matches. Description text is what loads at session start (eight short
descriptions); bodies load on use.

## 4. Distributing to a team (preferred) or a personal machine

Pin a reviewed snapshot for repeatable team/CI use. From an authorized adopting repo:

```sh
go run /path/to/agentic-kit/.agents/skills/intent-init/scripts/vendor_intent.go -source /path/to/agentic-kit -root .
go run .agentic-kit/upstream/.agents/skills/intent-init/scripts/vendor_intent.go -root . -check
```

The offline updater copies only the practice, eight skills/templates/scripts and
license, records a base commit and per-file hashes, and links project-local
`.agents/skills/intent-*` and `.claude/skills/intent-*` to that same snapshot. It refuses
local edits, unmanaged files, link collisions and source symlinks. It never fetches,
deletes, commits, modifies global configuration or overwrites a competing skill.
Updates are reviewed diffs, not automatic tracking of a mutable checkout. A working-
tree snapshot is labeled as such, not as a released version. See [monorepo.md](monorepo.md).

A local checksum proves consistency with the reviewed manifest, not publisher
authenticity. Review the source/update provenance before running its code. For a
personal checkout-following installation, the existing opt-in installer remains:

```sh
/path/to/agentic-kit/.agents/skills/install-intent.sh        # symlinks, idempotent
/path/to/agentic-kit/.agents/skills/install-intent.sh --uninstall
```

With authorization to change global configuration, it links `intent-*` into `~/.agents/skills/`
and `~/.claude/skills/` (Claude Code), then verifies both resolve. The kit checkout is
the single source; there are no copies to drift.

## 5. Claude Code

- Global: the stanza in §1 in `~/.claude/CLAUDE.md`.
- Skills: project `.claude/skills/intent-*` or personal installation. Only frontmatter
  descriptions load at session start; bodies on `/intent-<name>` or on description
  match; templates only when read.
- Project: the stanzas in §2 in `CLAUDE.md`. Never `@import` `docs/intent/`; imports
  load eagerly. Nested instructions load on demand when the host accesses that tree;
  still explicitly route affected paths for cross-component tasks.
- Path-scoped rules (`.claude/rules/*.md` with `paths:`) are optional and per-invariant
  only, as a pointer plus one invariant. Repos with an obligations engine (fathom
  `edit-hook`) already have that channel; do not add rules there.
- `--append-system-prompt-file`: stance only, two to four lines. Never the path or the
  procedure.

## 6. Codex

- Global: `~/.codex/AGENTS.md` (or `AGENTS.override.md`) with the stanza in §1.
  Concatenated with the project `AGENTS.md` chain every session under a 32 KiB default
  cap (`project_doc_max_bytes`); keep both small.
- Skills: `~/.agents/skills/intent-*` from the installer; also `.agents/skills/` inside
  a repo. `$intent-<name>` or implicit.
- Project: the stanzas in §2 in `AGENTS.md`. Nested `AGENTS.md` files concatenate root
  → cwd when the instruction chain is built. Do not assume reading a nested file from
  a root-started session reloads that chain. Root instructions must route the path.
- Do not use `model_instructions_file` (replaces the built-in prompt),
  `developer_instructions`, `~/.codex/prompts/` (legacy), or memories (recall, not
  policy) for any of this.

Host behavior references (consulted 2026-09-13): [Codex instructions](https://developers.openai.com/codex/guides/agents-md),
[Codex skills](https://developers.openai.com/codex/skills), [Claude Code memory](https://code.claude.com/docs/en/memory),
[Claude Code skills](https://code.claude.com/docs/en/skills). Documentation review is
not a live-host conformance result. Historical live receipt: Claude Code 2.1.245 /
Codex CLI 0.146.0, 2026-08-25. Re-probe after host upgrades; never just bump the receipt.

## 7. Any other `AGENTS.md`-aware agent

The project stanzas are enough. The router is self-sufficient: docs map, routing table,
evidence per surface, update rules, the trigger grep. The skills are a convenience, not
a dependency; an agent without a skill mechanism follows the router and the four docs
in `agentic-kit/docs/project-intent/`.

## 8. Fathom (optional, ecosystem-specific)

In a fathom-wired repo, invariants violated on specific paths can be compiled into an
obligations layer so they are injected on edit. `invariants.md` stays the readable
source; the layer cites it. `fathom verify` is the natural "Done needs" entry for plan
rows on the router's sensitive-surface table.
