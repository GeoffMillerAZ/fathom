# ADR index

The reasoning layer behind [../DECISIONS.md](../DECISIONS.md). The register
gives you the binding one-liner; the ADR gives you the context, the honest
alternatives, the accepted costs, and the triggers that would reopen the
question. This index is the canonical list (per ADR-0001).

## Index

| # | Title | Status | Decision (one line, matches DECISIONS.md) |
|---|---|---|---|
| [0001](0001-record-architecture-decisions.md) | Record architecture decisions | Accepted | Decisions are recorded as ADRs |
| [0002](0002-go-and-cue-only-no-node.md) | Go and CUE are the only languages; no Node, ever | Superseded in part by [0017](0017-multi-target-layered-kit.md) (§3 first clause) | Go + CUE are the only languages; no npm toolchain ever |
| [0003](0003-hypermedia-first-datastar.md) | Build all UI hypermedia-first on Datastar | Accepted | Hypermedia-first UI on Datastar v1 |
| [0004](0004-templ-templates.md) | templ is the template layer | Accepted | templ for templates, compile-time checked |
| [0005](0005-loopback-invariant.md) | Serve every app from a real loopback HTTP listener; shells are chrome | Accepted | The Loopback Invariant: every app is an HTTP server; shells are chrome |
| [0006](0006-wails-v3-desktop-shell.md) | Adopt Wails v3 (pinned alpha) as the desktop shell | Accepted | Wails v3 (pinned alpha) is the desktop shell; fallbacks documented |
| [0007](0007-cue-single-source-of-truth.md) | Author every schema once, in CUE | Accepted | CUE is the single source of truth for schemas, manifests, tokens |
| [0008](0008-embedded-nats-spine.md) | Embed NATS JetStream in-process as the event/state spine | Accepted | Embedded NATS JetStream is the event/state spine |
| [0009](0009-sqlite-default-store.md) | SQLite via ncruces/go-sqlite3 is the default durable store | Accepted | SQLite (pure-Go driver) is the default durable store |
| [0010](0010-component-data-pipeline.md) | Compile the TailwindPlus corpus into Parquet shards and one SQLite hybrid index | Accepted | TailwindPlus corpus: JSON → CUE → Parquet → SQLite index + embeddings |
| [0011](0011-mcp-design-assistant.md) | Build the Assistant on the official MCP go-sdk, as one binary, with search-then-get tools | Accepted | The Assistant: MCP server on the official go-sdk |
| [0012](0012-gallery-living-style-guide.md) | Build the Gallery — a kit app that IS the living style guide | Accepted | The Gallery: living style guide as a kit app |
| [0013](0013-monorepo-versioning.md) | One repo, one Go module, tagged semver — propagation is `go get -u` | Accepted | Monorepo, single Go module, semver propagation |
| [0014](0014-security-baseline.md) | Set the security baseline: token-bootstrapped loopback, eyes-open CSP, keychain secrets, signed updates | Accepted | Security baseline: loopback token, CSP, signed updates |
| [0015](0015-ai-first-surface.md) | Treat agents as first-class users of the kit | Accepted | AI-first surface: the kit is built for developer+agent pairs |
| [0016](0016-client-view-router.md) | Ship a vanilla client view-router for static-client surfaces | Superseded by [0017](0017-multi-target-layered-kit.md) | Client view-router: zero-dep History-API router for static SPAs; not for hypermedia apps |
| [0017](0017-multi-target-layered-kit.md) | The kit is multi-target and layered — it guides, it does not dictate | Accepted | Multi-target layered kit: take any layer and stop; the promotion bar scales with the layer |
| [0018](0018-terminal-target-cli-kit.md) | The terminal is a kit target — ship cli-kit on cobra + charm v2 | Accepted | cli-kit: the terminal target — L0 tokens to lipgloss/fang, one command/output/config/tui discipline |

## The founding corpus

ADRs 0002–0015 were authored 2026-06-10 as the founding corpus, Status
Accepted, during the vision phase — before implementation. They encode the
best decisions available from research and prior-project experience, not
from running kit code. Implementation may surface evidence that supersedes
some of them; **that is expected and healthy**. The failure mode to avoid is
not "an ADR turned out wrong" — it is drifting away from one silently. When
reality disagrees with an ADR, write the superseding ADR.

## Process

### When to write an ADR — the ADR-0001 test

Write an ADR when the decision passes any leg of the test in ADR-0001:

1. it **constrains downstream apps** (every kit app inherits it), or
2. it **picks a dependency the kit will live with** (versions, escape
   hatches, fallback plans), or
3. it **resolves a tension between the kit's principles** (e.g. bleeding-edge
   bets vs. multi-decade survival).

If none of the three apply, it is not an ADR — put it in a design doc, the
ROADMAP, or a code comment. Every substantial design conversation ends by
either writing an ADR or consciously deciding it isn't architecturally
significant.

### How to supersede

ADRs are immutable once Accepted; corrections happen by superseding, not
editing, so the decision trail stays honest.

1. Write a new ADR at the next free number (currently 0018) using the
   template. Its Context section states what changed in the world or what
   the old ADR got wrong; its Decision states the replacement in full.
2. Edit **only the status line** of the superseded ADR to
   `Superseded by ADR-NNNN`. The body stays untouched.

   **One exception, added 2026-08-07 with ADR-0017:** when a supersession
   retires *one clause* of an otherwise-standing ADR, strike that clause in
   place (`~~…~~`) and name the successor beside it. The alternative — a bare
   status line and an unmarked body — leaves a reader who arrives via an old
   citation reading a retired rule as current, which is exactly the silent
   drift this process exists to prevent. Nothing is deleted: the struck
   sentence stays visible, so the trail is still honest. See
   [0002](0002-go-and-cue-only-no-node.md) §3 for the shape.
3. Update this index (both rows) and the corresponding D-row in
   [../DECISIONS.md](../DECISIONS.md) to point at the new ADR.
4. If kit code changes as a result, the release's upgrade notes cite the new
   ADR (D13), and the Assistant's `audit_project`/`upgrade_guide` tools carry
   the change to downstream apps.

Partial reversals are still supersessions: a new ADR that narrows or amends
an old one supersedes it and restates the surviving parts, so no reader ever
has to merge two documents in their head.

### Format

Follow [template.md](template.md) exactly: Status / Date / Decision drivers,
then Context, Decision, Alternatives considered, Consequences, Revisit when.
Alternatives sections must be honest — including the real costs of the
winner. External facts carry versions and dates. Each accepted risk names
its revisit trigger. ADRs are written to be read by future-Geoff and by
agents mid-task (the Assistant serves them as MCP resources, D11/D15) — no
institutional shorthand, no context assumed beyond VISION.md and this index.
