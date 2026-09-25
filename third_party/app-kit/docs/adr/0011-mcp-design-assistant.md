# ADR-0011: Build the Assistant on the official MCP go-sdk, as one binary, with search-then-get tools

- **Status:** Accepted
- **Date:** 2026-06-10
- **Decision drivers:** the ambient-knowledge acceptance test (VISION.md §North-star #4);
  agents as first-class users (D15); multi-year API stability for a dependency every
  project will carry; token discipline — an assistant that floods context is worse
  than no assistant

## Context

The Assistant is pillar three of the vision: the kit's knowledge — components,
patterns, tokens, decisions — made queryable from inside any project an agent is
working on. It rides on the D10 data pipeline (one SQLite index: structural columns
+ FTS5 + sqlite-vec, see [0010-component-data-pipeline.md](0010-component-data-pipeline.md))
and it is the delivery mechanism for D15's promise that the kit's reasoning is
machine-readable everywhere.

The Go MCP ecosystem settled in late 2025. The facts, dated:

- **The official SDK won.** `github.com/modelcontextprotocol/go-sdk` (co-maintained
  with Google) shipped v1.0.0 on 2025-09-30 with an explicit guarantee — "going
  forward we won't make breaking API changes" — and sits at v1.6.1 as of 2026-05-22,
  with ~1,443 known importers
  (https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.0.0,
  https://github.com/modelcontextprotocol/go-sdk/releases).
- **It implements the full current spec.** v1.4.0 (2026-02-27) completed the
  2025-11-25 spec revision — typed tools with schemas generated from Go struct tags,
  resources, resource templates, prompts, sampling (including sampling-with-tools),
  elicitation, roots, completions — plus stdio, Streamable HTTP, SSE, in-memory, and
  command transports (https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp).
  v1.4.1 (2026-03-13) shipped DNS-rebinding and cross-origin hardening for the HTTP
  transport.
- **The flagship Go server migrated to it.** GitHub moved both local and remote
  GitHub MCP Server off mark3labs/mcp-go onto the official SDK, announced 2025-12-10,
  explicitly to stay aligned with the evolving spec
  (https://github.blog/changelog/2025-12-10-the-github-mcp-server-adds-support-for-tool-specific-configuration-and-more/).
- **mark3labs/mcp-go is alive but pre-1.0.** v0.54.1 (2026-05-25), 8.8k stars, no
  API stability promise (https://github.com/mark3labs/mcp-go).
- **Spec churn is scheduled.** The current revision is 2025-11-25; the next is a
  release candidate dated 2026-07-28, final about seven weeks from today
  (https://blog.modelcontextprotocol.io/posts/2026-07-28-release-candidate/).
- **The design-assistant tool shape has converged.** shadcn/ui ships exactly
  list/search/get — names from list and search, full source only from `get_item`
  (https://ui.shadcn.com/docs/mcp). Figma Dev Mode is structured-context-first
  (https://developers.figma.com/docs/figma-mcp-server/tools-and-prompts/). Storybook
  organizes by toolsets and returns IDs before content
  (https://storybook.js.org/docs/ai/mcp/overview). The TailwindPlus server we run
  today (observed firsthand) is the same names-then-fetch pattern parameterized by
  framework/tailwind_version/mode.
- **The anti-pattern is documented.** One production deployment burned 143k of a
  200k-token window on tool schemas alone — 72% of context gone before the first
  user query
  (https://agentmarketcap.ai/blog/2026/04/08/mcp-context-bloat-enterprise-scale-tool-definitions-agent-context-budget,
  2026-04-08). The fixes are small default pages with `has_more`, server-side
  filtering, and search-then-get shapes.
- **Host registration differs.** Correction, 2026-09-21: Claude Code reads project
  `.mcp.json`; Codex reads user or trusted-project `config.toml`. A project JSON
  file never proved Codex activation. See [agent setup](../design/agent-setup.md)
  for supported commands and separate host proof. Local adoption uses stdio.

## Decision

We build the Assistant — `appkit-mcp` — on the **official
`github.com/modelcontextprotocol/go-sdk`, pinned at ≥ v1.6.1**, and we do not use
mark3labs/mcp-go for any new work.

It is **one Go binary** at `cmd/appkit-mcp` in the kit monorepo (D13), versioned
with the kit. **stdio is the default transport** (`server.Run(ctx,
&mcp.StdioTransport{})`); a single `-http addr` flag serves the same `*mcp.Server`
through `mcp.NewStreamableHTTPHandler` for a long-lived shared instance. No SSE
transport, ever — it is already legacy.

The tool surface follows the **converged search-then-get shape**: list and search
tools return names plus one-line summaries, paginated (default 10–20) with
`has_more`; full source is returned only by exact name, one item per call. No tool
returns the source of multiple components at once. **Tools come first; resources
mirror them** — ADRs, DECISIONS.md, and pattern docs are also exposed as MCP
resources with stable URIs, but every capability is reachable via a tool because
today's clients use tools far more reliably than resources.

Distribution is a reviewed built binary or exact private module installation plus
host-specific registration and explicit kit root. Prefer absolute commands over
ambient PATH or compilation at startup. A user-scope owner installation may serve
multiple private projects; consumer intent and pins remain project-owned.
Server configuration is env vars and flags only. See [agent setup](../design/agent-setup.md).

Boundaries: this ADR decides the SDK, the binary shape, the transport policy, and
the tool-shape discipline. The full tool inventory, data backing, token budgets,
and migration from the existing tailwindplus server live in
[../design/mcp-design-assistant.md](../design/mcp-design-assistant.md). The index
the Assistant queries is decided in
[0010-component-data-pipeline.md](0010-component-data-pipeline.md). v1 is
deterministic: no sampling, no elicitation.

## Alternatives considered

- **mark3labs/mcp-go.** Attractive: more stars (8.8k), the longer track record, and
  until mid-2025 it was the pragmatic default with more examples in the wild. Lost
  because it is still v0.x with no compatibility promise (v0.54.1, 2026-05-25), and
  the ecosystem's flagship Go server already migrated off it — starting on it in
  mid-2026 means a probable forced migration later, which for a binary installed
  into every project is exactly the wrong place to absorb churn. The official SDK's
  real cost: its priorities are Google's and Anthropic's, not ours — we take spec-
  tracking minor versions on their schedule, and we budget an upgrade cycle after
  2026-07-28 whether or not the new revision gives us anything.
- **Per-project bespoke MCP servers.** Attractive: each project gets exactly the
  tools it needs, zero version coupling, no shared binary to keep compatible. Lost
  because it inverts the vision: the Assistant exists to make *one* body of
  knowledge ambient everywhere ("implement once, adopt everywhere", VISION.md §3).
  N servers means N drifting copies of the search logic and the corpus — the
  template-rot failure mode the kit was created to kill. The single-binary cost we
  accept: every project runs the same version surface, so a bad release is bad
  everywhere until the next `go install` (mitigated by versioned releases and the
  one-customer reality).
- **Resources-only design** (serve components and docs purely as MCP resources /
  resource templates, no tools). Attractive: resources are the spec's intended
  primitive for documents, URIs are stable and cacheable, and the design feels
  pure. Lost on observed client behavior: resources are less reliably used by
  today's clients than tools — agents call tools mid-task; resources often require
  explicit user attachment. A resources-only Assistant would fail the
  ambient-knowledge test in practice while passing it in theory. We keep resources
  as a secondary surface and accept the duplication cost of mirroring them with
  tools.
- **Folding the Assistant into the `appkit` CLI as a subcommand.** Attractive: one
  fewer binary. Lost narrowly: `.mcp.json` command lines stay cleanest when the
  binary's only job is MCP (no flag soup, no accidental CLI behavior changes
  breaking agent config), and a dedicated binary keeps the dependency graph of the
  CLI and the Assistant independent. Cheap to revisit; D13 keeps both in one module
  either way.

## Consequences

- Easier: typed tools via generic `AddTool[In,Out]` with schemas from struct tags —
  which pairs naturally with our CUE-first metadata (D7); dual transport is ~10
  lines; transport security patches (DNS rebinding, origin checks) arrive from
  upstream instead of being our problem.
- Easier: the ambient-knowledge acceptance test becomes mechanically checkable —
  connect the Assistant in a fresh repo, ask the four canonical questions, assert
  the answers cite real kit patterns.
- Harder: we are pinned to an SDK that tracks a moving spec. We treat go-sdk
  upgrades like compiler upgrades (deliberate, tested, one branch), and we budget
  exactly one bump cycle in Q3 2026 for the 2026-07-28 revision.
- Harder: search-then-get discipline must be enforced forever — every future tool
  proposal gets checked against the token budget table in the design doc before it
  lands.
- Risk: the 2026-07-28 revision reshapes experimental 2025-11-25 features (async
  tasks, icons). Tripwire: the RC final diverges from 2025-11-25 in anything we
  use → schedule the bump within a month of go-sdk support.
- Risk: `-http` mode has real attack surface. We keep the SDK's origin/DNS-
  rebinding protections on (never set `MCPGODEBUG` escape hatches), bind loopback
  only per D14, and keep the SDK current. Tripwire: any go-sdk security release →
  upgrade within a week.
- Risk: client support for resources improves to parity with tools, making our
  tools-first mirroring redundant weight. Tripwire: Claude Code reliably
  auto-attaches resources mid-task → slim the tool surface in favor of resources.

## Revisit when

2026-09-21 review: the protocol/SDK trigger fired. Exact SDK 1.8.0 passed the
candidate MCP suite and replaces 1.6.1 for bounded decoding/lifecycle hardening.
No new sampling, remote authentication or execution surface is admitted; see the
[setup assessment](../assessments/2026-09-21-mcp-adoption-setup.md) for evidence limits.

- The 2026-07-28 spec revision is final and go-sdk ships support (expected Q3
  2026) — mandatory upgrade-cycle review.
- The official go-sdk breaks its v1 compatibility guarantee or its maintenance
  visibly stalls (no release for two quarters).
- Clients (Claude Code first) start using MCP resources as reliably as tools —
  reconsider the tools-mirror-resources duplication.
- The Assistant needs to serve beyond loopback (shared remote instance) — the
  OAuth story (go-sdk v1.5.0+, parts still experimental as of 2026-06) gets its
  own ADR.
