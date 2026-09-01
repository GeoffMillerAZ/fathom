# app-kit — Vision

**One person. Many apps. Zero starting-from-scratch.**

app-kit is a personal product platform: the foundation under every app Geoff
builds from here on. It is not a framework for the public, not a startup, and
not an enterprise architecture. It is leverage — the place where a good idea
is implemented once and then exists in every app, where a new product starts
at the 80% mark instead of the 0% mark, and where the accumulated taste,
patterns, and hard-won decisions of past projects are machine-readable fuel
for the next one.

The proof of the vision is concrete: **Throughline, rebuilt on the kit, with
less code and nothing lost.**

---

## The questions this vision actually answers

The original brief asked for "a kit." Building toward that brief naively
produces a template — and templates rot. These are the bigger questions,
asked on the brief's behalf, that this repo is the answer to:

**1. What is the unit of reuse?**
Not code snippets. The unit of reuse is the *decision*. Code is the residue
of decisions, and code without its decision rots into cargo cult. So the kit
ships three things in priority order: decisions (ADRs, patterns,
constraints), then contracts (CUE schemas, Go interfaces), then
implementations (packages, components, scaffolds). The MCP assistant exists
to inject the decisions into every project, not just the code.

**2. Who is the user of this kit?**
The developer+agent pair. Every app from here on is built by Geoff working
with AI agents, and the kit is *context infrastructure* as much as code: CUE
schemas agents can validate against, an MCP server that answers "how does the
kit do X," component metadata that is queryable rather than scrollable, ADRs
written to be read by a model mid-task. A kit a model can't navigate is a
kit that slows development down. Design for machine legibility is not a
nice-to-have; it is the second-most-important property after correctness.

**3. How do improvements propagate?**
"Implement once, adopt everywhere" is a *versioning* problem, not a copying
problem. Apps depend on the kit as a versioned Go module; the kit maintains
upgrade notes per release; the MCP assistant audits any project for drift and
walks the upgrade. A great idea enters the kit by PR, ships as a version
bump, and reaches every app as a `go get -u` plus a guided migration — not as
twelve manual retrofits.

**4. What survives a dependency dying?**
The kit bets on young technology (Wails v3 alpha, Datastar 1.0, CUE 0.x).
The survival rule: **the app is a Go HTTP server serving hypermedia; that is
the invariant. Everything else is a shell.** If Wails stalls, the same
binary runs in Tauri-as-chrome, or a plain browser tab, or a future owned
webview. If Datastar dies, it is one vendored 34KB file with a documented
contract, not ten thousand lines of framework lock-in. Bet boldly at the
edges, keep the core sovereign.

**5. Where does determinism actually live?**
In the toolchain, not in slogans: pinned tool versions via `go.mod` tool
directives, a Tailwind standalone binary pinned by checksum (no Node,
anywhere, ever), vendored JS as `go:embed` artifacts, committed generated
code with CI checks that regeneration is clean, CUE validation at build time
and runtime, content-addressed data artifacts with pinned embedding-model
digests. A build of any kit app on a clean machine is two commands and
produces the same bits.

---

## The three pillars

### 1. The Kit — `appkit` (Go module)

The foundation library every app imports:

- **The platform contract.** Every app is one Go `http.Handler` tree serving
  templ-rendered HTML with Datastar reactivity over a loopback HTTP listener
  (`127.0.0.1`, ephemeral port, per-launch auth token). Shells — a Wails v3
  window, a browser tab, a cloud deployment, a future mobile shell — are
  thin chrome over that server. Native capabilities (windows, tray, dialogs,
  notifications, secrets, updates) live behind a capability interface with
  per-shell implementations and an explicit degradation matrix. This is the
  **Loopback Invariant**, and it is load-bearing: it is what makes web,
  macOS, Windows, and later iOS/Android one codebase, and it is forced by
  hard platform facts (Windows WebView2 cannot stream SSE through request
  interception; Android WebView drops intercepted POST bodies).
- **The UI layer.** Server-rendered templ components + Datastar v1 signals
  and SSE patches, in the Tao-of-Datastar shape: backend owns all state, one
  long-lived SSE stream per tab renders idempotent snapshots, commands are
  short POSTs, no optimistic UI. The kit's component library is informed by
  the TailwindPlus corpus and vendored best-of-breed (templUI, DatastarUI),
  unified under CUE-defined design tokens.
- **The spine.** Embedded NATS JetStream (in-process, no sockets) as the
  event/state backplane: KV watches drive re-renders, subjects fan out to
  SSE, and the same code path later points at a cloud NATS hub when device
  sync arrives. SQLite (pure-Go driver) as the durable store. Both behind
  small interfaces with trivial opt-outs for tiny tools.
- **Services.** Auth/session, config (CUE-validated), jobs and progress,
  logging, backup, secrets, updater wiring, packaging/signing tasks — the
  undifferentiated 80% of every app, done once, done well.

### 2. The Gallery — the living style guide

A kit app, built on the kit, that *is* the proof of the kit: every
component rendered live with knob controls generated from its typed
parameters, every design token visualized, every UI/UX pattern demonstrated
as a working page, every service exercised. It replaces Storybook (which
does not exist for server-rendered Go — a real gap the kit fills), serves as
the visual regression baseline, and is the first place any new idea lands.
When deciding how an app should look or behave, the Gallery is the catalog;
when the kit claims something works, the Gallery demonstrates it.

### 3. The Assistant — the MCP design server

A single Go binary (`appkit-mcp`, official MCP go-sdk, stdio + streamable
HTTP) installed into every project via a committed `.mcp.json`. It is the
kit's knowledge made ambient:

- **Component intelligence:** hybrid search (structural + BM25 + semantic)
  over the TailwindPlus corpus and the kit's own components — search-then-get
  tool shapes, never catalog dumps.
- **Design guidance:** UI/UX pattern selection ("this is a long-running-job
  screen; here is the kit pattern and the wiring"), design tokens, theming.
- **Adoption and drift:** audit a project for kit conventions, scaffold new
  apps, walk version upgrades.
- **The decisions themselves:** ADRs and patterns served as resources, so
  any agent in any project can ask *why* and get the real answer.

Behind the Assistant sits the data pipeline: the 336MB TailwindPlus JSON
dump becomes CUE-validated typed records → a canonical Parquet artifact
(sharded, diffable, under any upload limit) → one SQLite index file (FTS5 +
sqlite-vec + structural columns) with pinned-model embeddings. Rebuildable
from source with one command, queryable in microseconds, small enough to
ship inside the binary.

---

## The stack, and why it does not regress to the mean

| Layer | Bet | Status (June 2026) |
|---|---|---|
| Language | Go everywhere; CUE for every schema | Go stable; CUE v0.16.x, funded (CUE Labs/Sequoia) |
| Templates | templ (compile-time-checked) | v0.3.x, de facto standard, gomponents as fallback |
| Reactivity | Datastar (hypermedia, SSE, signals) | v1.0.2 stable, core frozen |
| Transport | Loopback HTTP + SSE, identical on all targets | Forced by platform facts; proven pattern |
| Desktop shell | Wails v3 (pinned alpha) | alpha.99, near-beta; mobile experimental (PR #5562) |
| Events/state | Embedded NATS JetStream | v2.14.x, ~16MB binary cost, leaf-node sync future |
| Store | SQLite via pure-Go driver | cgo-free, FTS5 + sqlite-vec capable |
| Data artifacts | Parquet (parquet-go) + SQLite indexes | pure Go, active |
| Assistant | MCP official go-sdk | v1.6.x, stability guarantee |
| CSS | Tailwind v4 standalone binary (no Node) | v4.3, checksum-pinned |

The enterprise mean is React+Node+Kubernetes+REST+OpenAPI. This stack
refuses each of those defaults *for reasons documented in ADRs*, and accepts
the cost: smaller ecosystems, younger dependencies, more problems solved
in-house. That trade is correct for exactly one situation — a single expert
owner with full-stack control, multi-decade horizons, and agents doing the
labor — which is this situation.

## Non-goals

- **Public consumption.** No docs-for-others, no API stability promises to
  outsiders, no genericity for use cases Geoff doesn't have. (Parts may be
  opened later; the licensed TailwindPlus data never.)
- **Multi-tenant SaaS framework.** Kit apps are personal-scale: single-user
  or small-circle. Cloud deployment is supported; "web-scale" is not chased.
- **JS ecosystem citizenship.** No React/Vue/Svelte adapters, no npm
  packages, no Node in any toolchain. JS appears only as vendored, pinned,
  embedded artifacts behind a documented island contract.
- **Backwards compatibility theater.** Until the kit stabilizes, breaking
  changes are fine — there is exactly one customer and an upgrade assistant.

## North-star acceptance tests

The vision is achieved when all four are true:

1. **The Throughline test.** Throughline is rebuilt on the kit — feature
   parity, less app-specific code, ships as web + macOS + Windows from one
   repo.
2. **The day-one test.** A brand-new app idea reaches a running, themed,
   signed, auto-updating walking skeleton (web + desktop) in under a day,
   mostly agent-driven.
3. **The propagation test.** An improvement landed in the kit reaches every
   kit app through a version bump and a guided, sub-hour upgrade.
4. **The ambient-knowledge test.** In any project, an agent with the
   Assistant connected answers "how do I build this screen the kit way?"
   with the correct pattern, components, and tokens — without a human
   explaining the kit.

## How this document is used

VISION.md is the north star: every ADR cites the principle it serves, every
roadmap phase advances at least one acceptance test, and anything that
contradicts this document either loses or changes it explicitly. The
decision register lives in `docs/DECISIONS.md`; the reasoning lives in
`docs/adr/`; the designs live in `docs/design/`. Read those next.
