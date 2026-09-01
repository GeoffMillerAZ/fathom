# Decision register

The at-a-glance index of every load-bearing decision. Each entry is binding
until its ADR is superseded. Full reasoning, alternatives, and revisit
triggers live in the linked ADRs. All versions/dates verified 2026-06-10.

| # | Decision | ADR |
|---|---|---|
| D1 | Decisions are recorded as ADRs | [0001](adr/0001-record-architecture-decisions.md) |
| D2 | Go + CUE are the only source languages; no npm toolchain, ever | [0002](adr/0002-go-and-cue-only-no-node.md), amended by [0017](adr/0017-multi-target-layered-kit.md) |
| D3 | Hypermedia-first UI on Datastar v1 | [0003](adr/0003-hypermedia-first-datastar.md) |
| D4 | templ for templates, compile-time checked | [0004](adr/0004-templ-templates.md) |
| D5 | The Loopback Invariant: every app is an HTTP server; shells are chrome | [0005](adr/0005-loopback-invariant.md) |
| D6 | Wails v3 (pinned alpha) is the desktop shell; fallbacks documented | [0006](adr/0006-wails-v3-desktop-shell.md) |
| D7 | CUE is the single source of truth for schemas, manifests, tokens | [0007](adr/0007-cue-single-source-of-truth.md) |
| D8 | Embedded NATS JetStream is the event/state spine | [0008](adr/0008-embedded-nats-spine.md) |
| D9 | SQLite (pure-Go driver) is the default durable store | [0009](adr/0009-sqlite-default-store.md) |
| D10 | TailwindPlus corpus: JSON → CUE → Parquet → SQLite index + embeddings | [0010](adr/0010-component-data-pipeline.md) |
| D11 | The Assistant: MCP server on the official go-sdk | [0011](adr/0011-mcp-design-assistant.md) |
| D12 | The Gallery: living style guide as a kit app | [0012](adr/0012-gallery-living-style-guide.md) |
| D13 | Monorepo, single Go module, semver propagation | [0013](adr/0013-monorepo-versioning.md) |
| D14 | Security baseline: loopback token, CSP, signed updates | [0014](adr/0014-security-baseline.md) |
| D15 | AI-first surface: the kit is built for developer+agent pairs | [0015](adr/0015-ai-first-surface.md) |
| D16 | Client view-router: zero-dep History-API router for static SPAs; not for hypermedia apps | [0016](adr/0016-client-view-router.md), superseded by [0017](adr/0017-multi-target-layered-kit.md) |
| D17 | Multi-target layered kit: take any layer and stop; the promotion bar scales with the layer | [0017](adr/0017-multi-target-layered-kit.md) |
| D18 | cli-kit: the terminal target — L0 tokens to lipgloss/fang, one command/output/config/tui discipline | [0018](adr/0018-terminal-target-cli-kit.md) |

## The decisions, with teeth

### D2 — Go + CUE only; no npm toolchain, ever
All logic in Go. All schemas/config/manifests/tokens in CUE. Versions: Go
≥1.26; CUE v0.16.1 (cuelang.org/go pinned exactly, upgrades treated like
compiler upgrades).

"No Node" was four rules wearing one slogan, and conflating them is what let
*"React apps can't use the kit"* pass as policy for a year. ADR-0017 §3
separates them permanently:

1. **Build hygiene — stands, verbatim.** CSS comes from the Tailwind v4.3
   standalone binary (Lightning CSS bundled, no Node), pinned by SHA-256 per
   platform in `internal/tailwind/manifest.cue` and **re-verified on every
   invocation**. No npm, no `node_modules`, no lockfile, no PostCSS, no
   `tailwind.config.js`. *This is what the rule protects: a clean-machine
   build that produces the same bits on any day.*
2. **No `package.json` in a kit repo — stands, and the term is now defined.**
   "A kit repo" means the app-kit monorepo, and any repo whose primary
   artifact is a Go binary or an `http.Handler` tree. It is a rule about
   *those* repos and binds no one else. Single sanctioned exception:
   Playwright inside the pinned visual-regression CI container (D12), config
   only under `apps/gallery/visreg/` — the one path excluded from the
   package.json-is-a-build-failure check.
3. **Kit-authored browser JS at escape-ladder rung L1 — amended by
   [ADR-0017](adr/0017-multi-target-layered-kit.md).** ADR-0002 §3's
   *"JavaScript is never written as source"* is retired because it was
   already false: `chat.js`, `overlay.js`, `router.js` and `motion.js` are
   hand-written. Permitted now, with conditions: ephemeral view manipulation
   only, no state outliving the element, dependency-free, one `window.*`
   global, no build step, `go:embed`-ed with `Script()`/`Handler()`, listed
   in `docs/ui-components.md`. **Third-party JS still enters only as a
   vendored, version-pinned, SHA-256-recorded artifact** (datastar.js ~34KB)
   — that half was always the real rule.
4. ***"React apps can't use the kit"* — deleted. It was never true.**
   ADR-0002's own Boundaries paragraph permits consuming artifacts built with
   Node upstream; the kit forbids *running* that toolchain. A React or Vite
   app importing `theme.css` and `motion.css` is a compliant consumer of
   generated artifacts. See D17.

### D3 — Hypermedia-first UI on Datastar
Datastar v1.0.2 + datastar-go v1.2.2 (official SDK). The Tao-of-Datastar
shape is the kit default: backend owns all state; one long-lived GET SSE
stream per tab renders idempotent "fat-morph" snapshots; commands are short
POSTs; signals only for ephemeral client interaction; **no optimistic UI**.
HTMX rejected (Datastar subsumes it + Alpine in less weight). SPA frameworks
rejected (toolchain, drift from Go-maximalism). Escape-hatch ladder for
heavy interactivity is defined in `design/ui-architecture-patterns.md` —
vendored JS islands (web components) bridged to signals, never a framework.
Datastar Pro ($349 solo, lifetime) bought for the Inspector; Pro code stays
out of anything ever open-sourced (license forbids OSS embedding).

### D4 — templ templates
templ v0.3.1020+. Compile-time-checked components, context-aware escaping.
CLI pinned via `go.mod` tool directive; `templ generate -check` (or
generate+diff) in CI; `*_templ.go` committed but marked linguist-generated.
Dev loop: `templ generate --watch --proxy` + wgo. gomponents v1.3.0 is the
documented no-codegen fallback; html/template rejected (runtime field
resolution); gostar rejected (unmaintained).

### D5 — The Loopback Invariant (load-bearing)
Every kit app is one Go `http.Handler` tree served by a real
`net/http` listener on `127.0.0.1:0` with a per-launch random bearer token
exchanged for a session cookie on first navigation. Shells point a webview
(or tab) at that URL. **No app code may depend on shell-specific JS
bindings.** Forced by hard facts: WebView2 request interception buffers
entire responses (SSE impossible on Windows through asset handlers; upstream
MicrosoftEdge/WebView2Feedback#3519, open since 2023, confirmed by Wails
maintainer 2026-05); Android WebView drops intercepted fetch() POST bodies;
Wails asset server 501s WebSockets. Consequences: web/desktop/mobile/cloud
are literally one binary (Wails server-mode confirms the shape); shell
failure is a swap, not a rewrite; "browser tab" is a permanently supported
zero-install tier (with server heartbeat lifecycle).

### D6 — Wails v3 desktop shell
Wails v3 pinned at ≥ v3.0.0-alpha.99 (2026-06-10; near-daily releases,
"nearly ready" for beta). Used for: window/menu/tray chrome, dialogs,
packaging (DMG/NSIS/MSIX), `wails3 setup signing` + notarization, built-in
verified `app.Updater` (GitHub Releases provider + SHA256SUMS). The webview
loads the loopback URL (D5) — the asset-server transport is used for static
chrome only, never SSE. Mobile: experimental PR #5562 (opened 2026-06-10,
iOS+Android, same main.go) is tracked, not bet on; mobile tier-1 today is
PWA; fallback native path is gomobile-as-library + owned webview. Desktop
fallback if v3 stalls: Tauri v2 + Go sidecar (kept as a tested wrapper), or
plain browser tab. Alpha churn is accepted cost; pin + budget upgrades.

### D7 — CUE single source of truth
Every schema is authored once, in CUE, in a Go-representable subset (no
load-bearing disjunctions): app manifest (`app.cue`: identity, capabilities,
nav, theme), config schemas, design tokens (emitting DTCG 2025.10 JSON +
Tailwind `@theme` CSS), component/pattern metadata, KV bucket schemas and
NATS subject hierarchies. Codegen via `cue exp gengotypes` (experimental —
output committed, regeneration CI-checked) driven by a small Go program
using cuelang.org/go directly (not `cue cmd`; its v2 redesign is unsettled).
Runtime validation via embedded cuecontext (stable, concurrency-safe).
Vendor CUE module deps; don't build-depend on the beta Central Registry.

### D8 — Embedded NATS JetStream spine
nats-server v2.14.x embedded in-process: `DontListen: true` +
`nats.InProcessServer` (net.Pipe, no sockets). JetStream KV for reactive
state (KV watch → re-render → SSE patch is the canonical loop); subjects
for fan-out; measured cost ~16MB binary / ~19MB RSS — acceptable as default.
The kit's event API is a small Go interface: `appkit/bus` with the NATS
implementation as default and a channel-backed implementation as the
opt-out for trivial tools. Listen mode (in-process | leaf-node | external
URL) is one CUE config knob, making device-sync a config change later —
gated on passing offline/reconnect resync tests (open bugs #6468, #7530).

### D9 — SQLite default store
ncruces/go-sqlite3 v0.34.x (SQLite via wazero WASM: cgo-free,
cross-compilable, supports FTS5 and sqlite-vec). SQLite = durable
relational truth; JetStream KV = reactive/ephemeral state; Parquet = bulk
analytical artifacts. Division of labor documented per pattern. iOS caveat
noted (wazero interpreter fallback, no JIT) — revisit at mobile phase;
modernc or cgo are the mobile plan-B.

### D10 — Component data pipeline
Build-time Go tool: 336MB TailwindPlus JSON (657 components ×
~12 variants; `Section.Group.Category.Name` taxonomy) → CUE-validated typed
records → canonical zstd Parquet artifact, sharded by section/group so no
shard exceeds ~15MB (solves 20MB upload limits; diffable; durable) → one
generated SQLite index: structural columns + contentless FTS5 (BM25) +
sqlite-vec vec0 (float32[768], brute-force — corpus is ~8k vectors,
sub-ms) + zstd-compressed code blobs. Embeddings: local Ollama
qwen3-embedding-0.6B at 768 dims, model digest pinned in the artifact
manifest; Voyage (voyage-code-3) as optional flagged alternative. Query
time degrades gracefully to BM25+structural when no embedder is reachable.
DuckDB allowed as a dev-time query convenience; ships nowhere (cgo +
experimental VSS). The existing `tailwindplus` MCP server keeps running off
the JSON until the Assistant supersedes it; the downloader gets
reimplemented in-kit later, emitting the typed form directly.

### D11 — The Assistant (MCP)
`appkit-mcp`: one Go binary on the official `modelcontextprotocol/go-sdk`
≥ v1.6.1 (v1.x stability guarantee; GitHub's server migrated to it; spec
2025-11-25, one upgrade cycle expected after 2026-07-28 spec lands). stdio
by default, `-http` flag for StreamableHTTPHandler. Converged
component-assistant tool shape (per shadcn/Figma/Storybook precedent):
search-then-get, paginated lists with `has_more`, never bulk source dumps.
Tool families: components (search/get/preview), patterns & guidance
(list/get, scenario→pattern selection), tokens/theme, kit adoption
(`audit_project`/`scaffold_app`/`upgrade_guide`), decisions (ADRs as
resources). Distributed via `go install` + committed `.mcp.json` per
project.

### D12 — The Gallery
A kit app (dogfood #1) serving: live component catalog with knob controls
generated from typed templ parameters, token visualization, working pattern
demos, service exercisers. Replaces Storybook (no mature Go equivalent
exists — the kit fills the gap natively, no Node). It is the visual
regression baseline (Playwright snapshots) and the acceptance harness for
"the kit works."

### D13 — Monorepo, single module, semver
This repo is a monorepo: `appkit` (library), `cmd/appkit` (CLI),
`cmd/appkit-mcp` (Assistant), Gallery app, data pipeline, docs. One Go
module, tagged semver. Apps consume via `go.mod` (private repo: GOPRIVATE).
Kit components are imported, not copied; third-party material (templUI,
TailwindPlus) is vendored *into the kit* once, adapted to kit tokens.
Upgrade notes per minor; the Assistant's `audit_project`/`upgrade_guide`
tools do the propagation. Pre-1.0: breaking changes allowed, one customer.

### D14 — Security baseline
Loopback listener: random per-launch bearer token → cookie exchange;
non-loopback binds refused unless explicitly configured for cloud deploy
(then real auth). CSP default `script-src 'self' 'unsafe-eval'` ('unsafe-eval'
is a permanent Datastar requirement — Function constructor; accepted,
documented). data-ignore fencing for untrusted DOM. Secrets in OS keychain
via capability interface. Updates only via signature-verified updater.
MCP HTTP mode keeps SDK origin/DNS-rebinding protections on. No telemetry.

### D15 — AI-first surface
The kit treats agents as first-class users: all metadata machine-readable
(CUE), all knowledge queryable (MCP), ADRs written for mid-task model
consumption, a shipped `CLAUDE.md` pack for kit apps, scaffold output that
is agent-legible by construction. When a design choice trades human
convenience against agent legibility, agent legibility wins (the human has
agents).

### D16 — Client view-router for static-client surfaces
`appkit/ui/router` embeds a single, dependency-free, build-step-free
`router.js` (180 lines, vanilla ES5) that implements a History-API client
router for static SPAs (GitHub Pages, CDN-only deploys). Exposed as
`Script() string` and `Handler() http.Handler`. The boundary is hard: this
router exists ONLY for static-client surfaces with no server router. Kit
apps with a real HTTP server use real URL navigation (D3, anti-pattern 5.4)
— a client router on top of a hypermedia app is always wrong. The
GitHub Pages recipe: ship a `404.html` byte-copy of `index.html`, add
`<base href="/">`, use relative asset paths. Deep paths return HTTP 404
status while still rendering the correct view.

### D17 — Multi-target layered kit
The kit publishes artifacts **in layers**, and a consumer takes any prefix and
stops. **Stopping is a complete adoption, not a partial one.** L0 CUE tokens ·
L1 portable CSS (`theme.css`, `motion.css`) · L2 portable browser JS
(`router`, `overlay`, `chat`, `motion`) · **L3a** var-themed Go renderers
(`ui/viz`: zero `class=`, colour only via `var(--ak-color-*)`, no Tailwind
scanner needed) · **L3b** utility-themed Go renderers (the rest of `ui/*`:
the consumer must run `appkit css` with an `@source` glob over the kit) · L4
templ · L5 discovery (MCP + docs).

"Guide, do not dictate" is written as three constraints on **the kit**, not
as exhortations to apps: never require a consumer to run Node or emit an
artifact only a bundler can consume; never emit a literal colour (every
colour reaches the page as `var(--ak-color-<role>)` or a role utility); never
exceed one class of specificity, use `!important` outside an accessibility
floor, or ship preflight.

The promotion bar is indexed to the layer — a hazard entry needs one
occurrence plus its measured evidence; an L0 token, an L1 class, an L2 script
or an L3a renderer needs one consumer plus its fixture; **an L3b/L4 component
still needs two consumers (D13, unchanged)**. Promote at the lowest layer
that carries it. A promotion that leaves the original in place is a fork. An
artifact with zero consumers for two releases is demoted or deleted, and the
removal is recorded. Divergence is the system working: it carries a
`DIVERGENCE:` header naming what was declined and why, and two divergences
with the same reason are a kit bug with a queue position.

Adoption is measured by layer-weighted reach — six of eight consumers at L1
today — **never** by component-import counts. Distribution for non-Go
consumers is `appkit vendor -out DIR --include …`, which writes a `kit.lock`
(module version + SHA-256 per file); `--check` exits 1 on drift. Provenance
is adopted as the sibling package `appkit/ui/vizframe` — one predicate at
three strengths (`Validate`/`Render`/`MustRender`), zero changes to `ui/viz`.
Adding `Source` to the fifteen `viz.*Options` structs is **rejected** and is
named in the ADR because it is the change that will be proposed as the small
one.

### D18 — cli-kit: the terminal target
The terminal joins the layer model as a target consuming L0: `appkit/cli/…`
on cobra v1.10.2 + the charm.land v2 line (fang, lipgloss, bubbletea,
colorprofile — exact pins in the ADR). `cli/theme` carries a **generated**
palette (all 23 `--ak-color-*` roles, OKLCH→sRGB, byte-identity-gated like
`theme.css`) feeding lipgloss styles and fang's help chrome, so a CLI and a
web page draw one token source; terminal light/dark follows the terminal
background. `cli/command` = fang chrome + the fleet exit-code convention
(0/1/2); `cli/output` = NO_COLOR-correct profiles, human/JSON/NDJSON, a
status table whose **plain rendering is byte-compatible** with the doctor
format (styled is enhancement over identical geometry); `cli/config` =
layered CUE config with per-layer digests and `Explain` (project walk
bounded at the git root); `cli/tui` = a thin bubbletea runner + teatest
harness. bubbles/huh/glamour/log join only on consumer demand. `cmd/appkit`
stays hand-rolled until its own migration (named drift, revisit in the ADR).
First consumer: env-kit's `bosun`. crush informed patterns only (FSL — no
code). Loopback (D5) does not govern CLI binaries.

## Naming

- **the Kit** — the Go library (`appkit` module)
- **the Gallery** — the living style guide app
- **the Assistant** — `appkit-mcp`, the MCP design server
- **the Loopback Invariant** — D5, cited by name everywhere
- **app manifest** — `app.cue`
