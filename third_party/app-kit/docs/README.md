# app-kit docs — the map

This corpus is the kit's most durable asset. Per [../VISION.md](../VISION.md),
the unit of reuse is the *decision*: decisions first (ADRs), then contracts
(CUE schemas, Go interfaces), then implementations. Code without its decision
rots into cargo cult; these docs are where the decisions live.

## What lives where

| Path | What it is |
|---|---|
| [../VISION.md](../VISION.md) | The north star: three pillars, the stack, non-goals, four acceptance tests |
| [DECISIONS.md](DECISIONS.md) | The binding register, D1–D15, at-a-glance with teeth |
| [../ROADMAP.md](../ROADMAP.md) | Phased build-out; every phase advances at least one acceptance test |
| [adr/](adr/README.md) | Full reasoning per decision: context, alternatives, consequences, revisit triggers |
| [design/](design/) | Build plans: package names, type sketches, wire flows, file layouts |
| [../data/tailwindplus/](../data/tailwindplus/README.md) | Home of the licensed TailwindPlus dataset (input to D10) |

## Reading order, by audience

**Future-Geoff, returning cold.** Read [../VISION.md](../VISION.md) (why this
exists, what done looks like) → [DECISIONS.md](DECISIONS.md) (what is binding
right now) → [../ROADMAP.md](../ROADMAP.md) (what's built, what's next). That
restores the whole context in under an hour; drill into individual ADRs and
design docs only as the work demands.

**An AI agent mid-task.** Start at [DECISIONS.md](DECISIONS.md) — it is short,
binding, and indexed. Follow its link to the one relevant ADR for the *why*
and the revisit triggers, then to the relevant design doc for the *how*. Do
not read the corpus linearly. Note: the Assistant (`appkit-mcp`, ADR-0011)
serves ADRs and patterns as MCP resources, so in any project with the
committed `.mcp.json` you can query this knowledge instead of scrolling it —
that is the intended path (D15).

**Implementation work.** Start at [../ROADMAP.md](../ROADMAP.md) to find the
current phase and its exit criteria, then work from the design docs for that
phase. The governing ADRs are the tiebreaker when a design choice looks
questionable mid-build: if the ADR's reasoning no longer holds, supersede it
(see [adr/README.md](adr/README.md)) — don't silently drift.

## The design docs

**[design/platform-contract.md](design/platform-contract.md)** — the Loopback
Invariant (D5) made operational: the `http.Handler` tree every app exports,
the `127.0.0.1:0` listener, the per-launch bearer-token → session-cookie
bootstrap, the capability interface for native features (windows, tray,
dialogs, secrets, updates) with per-shell implementations and the explicit
degradation matrix, and shell lifecycle including the browser-tab tier's
server heartbeat. This is the contract that makes web, macOS, Windows, and
later mobile one binary.

**[design/ui-architecture-patterns.md](design/ui-architecture-patterns.md)** —
the operational companion to ADR-0003: the default kit shape (backend owns
all state, one long-lived SSE stream per tab, idempotent fat-morph snapshots,
short-POST commands, no optimistic UI), a scenario → pattern catalog for the
screen shapes we actually build, the escape-hatch ladder for heavy
interactivity (vendored JS islands bridged to signals — never a framework),
component sourcing rules, and the explicit never-do list.

**[design/component-pipeline.md](design/component-pipeline.md)** — the
build-time pipeline behind D10: the 336MB TailwindPlus JSON dump (657
components, `Section.Group.Category.Name` taxonomy) → CUE-validated typed
records → canonical zstd Parquet shards (each ≤ ~15MB, diffable, durable) →
one generated SQLite hybrid index (structural columns + contentless FTS5 +
sqlite-vec at 768 dims) with the embedding model digest pinned in the
artifact manifest. Rebuildable from source with one command.

**[design/mcp-design-assistant.md](design/mcp-design-assistant.md)** — the
Assistant: one Go binary on the official MCP go-sdk, stdio default with an
`-http` flag, read-mostly and deterministic in v1. Tool families: component
search/get/preview over the D10 index, pattern and design-token guidance, kit
adoption (audit_project/scaffold_app/upgrade_guide), and the ADR corpus served
as resources —
including in foreign projects, where the Assistant is the adoption wedge.

**[design/gallery-style-guide.md](design/gallery-style-guide.md)** — the
Gallery: dogfood #1 and the proof the platform contract holds. Information
architecture, the component registry with knob controls generated from typed
templ parameters, token visualization and theme/density switching, working
pattern demos, service exercisers, the TailwindPlus browser, and the
Playwright visual-regression baseline — with an honest one-developer scope
ladder from v1 outward.

**[design/throughline-rebuild.md](design/throughline-rebuild.md)** — the
proof of the vision (acceptance test #1): Throughline rebuilt on the kit with
feature parity and less app-specific code, shipping web + macOS + Windows
from one repo. Maps Throughline's existing features onto kit services and
patterns, identifies what the rebuild forces the kit to grow, and defines the
parity checklist that declares it done.

**[design/repo-layout.md](design/repo-layout.md)** — the target tree, the
build tooling, the CI outline, and the scaffold output: the concrete
expression of D13/D15, and the doc that resolves layout disputes — where a
package, app, or artifact lives is settled here, not re-argued per PR.

## Conventions

- **Names:** the Kit (`appkit` module), the Gallery, the Assistant
  (`appkit-mcp`), the Loopback Invariant (D5), the app manifest (`app.cue`).
  Use these exact names everywhere; cite decisions as D-numbers.
- **Facts are dated.** Every external claim (version, limitation, price)
  carries a version and a verification date, because it goes stale and
  future readers need to know when to re-verify.
- **Cross-reference by relative path**, exactly as in DECISIONS.md, so links
  survive the staging → repo move and agents can follow them mechanically.
- **Audience is Geoff + agents.** No enterprise fluff, no genericity for
  users we don't have. When human convenience and agent legibility conflict,
  agent legibility wins (D15).
