# app-kit — Roadmap

Gates, not deadlines. One owner, agent labor, no calendar. A phase is done when
its gate — an observable, falsifiable exit criterion — is green, and not
before. Each phase names the [VISION](VISION.md) north-star acceptance tests
it advances (#1 Throughline, #2 day-one, #3 propagation, #4 ambient
knowledge). Decisions cited as D1–D15 are binding per
[docs/DECISIONS.md](docs/DECISIONS.md); this roadmap sequences them, it never
overrides them.

Operating rules:

- **One phase in flight at a time.** Continuous tracks (below) are exempt.
- **Spikes precede structure.** Nothing in Phase 1+ is built until Phase 0
  has either confirmed the bet or triggered its documented fallback.
- **Fallbacks are pre-committed.** Every major bet has kill criteria (last
  section) naming the evidence that triggers the fallback already written
  into its ADR. We do not improvise retreats.
- **Gates produce artifacts.** Every gate closes with a committed artifact
  (spike README, conformance test, parity matrix, tagged release) — a gate
  that lives only in memory is not a gate.

---

## Phase 0 — Spikes & proofs

De-risk the four bets that everything downstream stands on. Each spike is a
small standalone program under `spikes/`, kept in-repo permanently with a
README recording setup, results per platform, and the verdict. Spikes are
allowed to be ugly; they are not allowed to be ambiguous.

Advances: nothing directly; gates everything. A failed spike here is the
cheapest failure we will ever buy.

### Spike 0.1 — SSE over loopback in real shells (the Loopback Invariant's empirical gate)

The Loopback Invariant (D5, [docs/adr/0005-loopback-invariant.md](docs/adr/0005-loopback-invariant.md))
rests on dated platform facts: WebView2's request interception buffers entire
response bodies (upstream MicrosoftEdge/WebView2Feedback#3519, open since
2023-05-28, confirmed unfixable by the Wails maintainer 2026-05-02 in
wailsapp/wails#5047 — https://github.com/wailsapp/wails/issues/5047), while a
real `127.0.0.1` listener is the documented community workaround. The facts
say loopback SSE works; we ship nothing until we have watched it work.

**Build:** a minimal Wails v3 app pinned at v3.0.0-alpha.99 (released
2026-06-10 — https://github.com/wailsapp/wails/releases). One Go
`http.Handler`: templ page, Datastar v1.0.2 + datastar-go v1.2.2 counter fed
by a single long-lived SSE stream, served from `127.0.0.1:0` with the D14
bearer-token→cookie exchange. The window loads the loopback URL.

**Test matrix:** macOS WKWebView via loopback; Windows WebView2 via loopback;
plain browser tab (Chrome + Safari); and — informational only — the
`wails://` asset transport on macOS, to confirm the documented streaming
behavior firsthand (macOS streams per-Write via WKURLSchemeTask; Windows
buffers — per Wails source, verified 2026-06-10).

**Updater dry-run:** the same spike app runs an `app.Updater` check-and-verify
dry-run (check against a throwaway GitHub release, download, SHA256SUMS
verification — no install), so the updater bet is probed at the pin instead of
first being met in Phase 1.

**Pass:** on both OSes via loopback: first SSE patch renders < 1s after
server publish; stream survives 30 minutes idle with heartbeats; killing and
restarting the server re-renders a correct snapshot via Datastar's built-in
retry; two windows on one server converge.

**Fail → fallback trigger:** loopback SSE fails inside WebView2 or WKWebView
→ immediately run the same Go binary under a minimal Tauri v2 wrapper
(tauri-v2.11.2, 2026-05-16 — https://github.com/tauri-apps/tauri/releases)
using the documented sidecar pattern
(https://v2.tauri.app/develop/sidecar/). If SSE works there, D6 flips to its
documented fallback (Tauri-as-chrome) and D5 stands. If SSE fails in *every*
shell (it should not — loopback is a plain HTTP origin), the browser tab
becomes the interim desktop tier and D5's Consequences section gets amended
before any further building.

**Permanence:** this spike graduates into `appkit/shell/conformance` — a test
suite re-run on every Wails pin bump, forever.

### Spike 0.2 — Embedded NATS KV-watch → SSE round trip

D8's canonical loop, end to end: nats-server v2.14.2 (2026-06-02 —
https://github.com/nats-io/nats-server/releases) embedded with
`DontListen: true` + `nats.InProcessServer` (the wiring the official docs get
wrong; copy northstar's — https://github.com/zangster300/northstar), a
JetStream KV bucket, KV put → watch fires → templ fragment re-render →
Datastar SSE patch.

**Pass:** put→pixel under 50ms locally (our threshold; research measured the
in-process transport as effectively free); stripped binary ≤ 25MB and idle
RSS ≤ 30MB (research baseline: 16MB / ~19MB measured 2026-06-10 on Go 1.26 /
darwin-arm64 — comfortable margin); JetStream file store survives a `kill -9`
mid-write with no corruption on restart; two SSE tabs converge from one KV
write.

**Fail → fallback trigger:** thresholds blown or file-store corruption →
D8's opt-out inverts: the channel-backed `appkit/bus` implementation becomes
the default, NATS the opt-in, and ADR 0008 is amended. The interface is the
same either way — that is the point of `appkit/bus`.

### Spike 0.3 — CUE → gengotypes → templ token pipeline hello-world

D7's codegen chain on one real schema: a `tokens.cue` design-token schema in
the Go-representable subset → `cue exp gengotypes` (CUE v0.16.1, 2026-04-08 —
https://github.com/cue-lang/cue/releases; gengotypes is explicitly
experimental) → committed Go types → a small Go generator (driving
cuelang.org/go directly, not `cue cmd`) emitting Tailwind v4.3 `@theme` CSS
and DTCG 2025.10 JSON → a templ component styled by the generated classes.

**Pass:** one command regenerates everything; a second run is byte-identical;
a deliberately invalid token value fails the build with an error a human can
locate in under a minute (CUE diagnostics are the known weak spot — we gate
on usability, not existence); the CI regeneration-cleanliness check works.

**Fail → fallback trigger:** gengotypes output unusable for the token schema
→ handwrite the Go types, keep CUE as the authoring + runtime-validation
layer (cuecontext is stable), keep codegen for CSS/JSON only. D7 survives in
amended form; the single-source-of-truth property is preserved by the CI
check that handwritten types still validate against the schema.

### Spike 0.4 — D10 pipeline dry-run on a 10-component slice

The full D10 pipeline shape on a thin vertical slice: 10 components (~120
variants) of the TailwindPlus JSON → CUE-validated typed records → one zstd
Parquet shard (parquet-go v0.30.1, 2026-05-18 —
https://github.com/parquet-go/parquet-go/releases) → one SQLite index via
ncruces/go-sqlite3 v0.34.x (v0.34.4, 2026-06-04 —
https://github.com/ncruces/go-sqlite3/releases): structural columns,
contentless FTS5, sqlite-vec vec0 `float32[768]` through the official ncruces
bindings (https://github.com/asg017/sqlite-vec-go-bindings), zstd-compressed
code blobs. Embeddings from local Ollama qwen3-embedding-0.6B at 768 dims,
model digest pinned.

**Pass:** typed fields survive JSON→Parquet→SQLite without loss (golden
diff); hybrid RRF query (per the sqlite-vec author's documented pattern —
https://alexgarcia.xyz/blog/2024/sqlite-vec-hybrid-search/index.html) ranks a
known component top-3 for 10 golden queries; with Ollama stopped, queries
degrade to BM25+structural and still return; rebuild is deterministic apart
from embedding floats (assert model digest, dims, and vector count instead).

**Fail → fallback trigger:** sqlite-vec-via-wazero fails → brute-force dot
products in plain Go over float32 blobs stored in ordinary SQLite columns
(~8k vectors is sub-millisecond either way; the vec0 virtual table is a
convenience, not a requirement). FTS5 + structural unaffected.

### Gate G0 (exit)

All four spikes merged under `spikes/` with verdict READMEs; every triggered
fallback recorded as an ADR amendment **before** Phase 1 begins. No verdict,
no foundation.

---

## Phase 1 — Foundation

The platform contract made real, plus the walking-skeleton app that proves
it. Advances **#2 (day-one test)** — this phase builds the skeleton the
day-one test will later scaffold automatically.

Deliverables:

- **The Kit's core packages:** `appkit/app` (lifecycle, `app.cue` manifest
  loading and validation), `appkit/web/{server,sse,middleware}` + `appkit/auth`
  (loopback listener, token→cookie exchange per D14, SSE helpers, heartbeat
  lifecycle for browser-tab mode), `appkit/shell` (capability interface; Wails
  v3 implementation; browser-tab implementation; the conformance suite from
  Spike 0.1), `appkit/bus` (D8), `appkit/store` (D9; pre-upgrade snapshot +
  restore helpers, exercised by the walking skeleton), `appkit/config`.
- **Walking-skeleton app:** one screen, one KV-backed live counter, one
  long-running job with progress, a tray icon, a native dialog — every
  capability the skeleton uses exercised once (windows, dialogs, tray,
  secrets, updater) through the interface, never through shell JS bindings
  (D5). Notifications, Clipboard, FileScopes, DeepLinks, and PowerEvents are
  deferred to their first consumer — the Gallery exercisers in Phase 3 or the
  Throughline mapping in Phase 4.
- **Ship pipeline:** `wails3 setup signing` + notarization tasks, DMG + NSIS
  or MSIX packaging, built-in `app.Updater` with the GitHub Releases provider
  and SHA256SUMS verification (Wails v3 updater guide, verified 2026-06-10 —
  https://github.com/wailsapp/wails/blob/master/docs/src/content/docs/guides/updater.mdx).
- **Determinism rig (D2/D7):** pinned tool directives, checksum-pinned
  Tailwind v4.3 standalone binary, `templ generate -check`, CUE regeneration
  check, committed generated code — all enforced in CI.

### Gate G1 (exit)

On a clean machine: `git clone` + one task command produce a signed,
notarized macOS DMG, a signed Windows installer, and the web binary — two
commands total (the VISION determinism claim, demonstrated). Install on both
OSes; tag a v0.1.x patch; both installs auto-update through the verified
updater. Pre-committed fallback: if `app.Updater` is broken at the pin, G1
passes with the browser-tier degraded updater (check + notify) and a tracked
re-test on the next pin bump. The same binary serves a browser tab and shuts down cleanly when the
tab closes (heartbeat). All capability calls route through `appkit/shell` —
verified by grep-able absence of Wails imports outside that package.

---

## Phase 2 — Data & Assistant

The D10 pipeline at full corpus and the Assistant's first useful life.
Advances **#4 (ambient-knowledge test)**.

Deliverables:

- **Full-corpus pipeline:** 336MB TailwindPlus JSON (657 components ×
  ~12 variants) → CUE-validated records → Parquet shards ≤ ~15MB
  (section/group sharding per D10) → one SQLite index with full-corpus
  embeddings (~8k vectors ≈ 24MB at 768 dims — brute force stays sub-ms).
- **`appkit-mcp` v1** (D11; design in
  [docs/design/mcp-design-assistant.md](docs/design/mcp-design-assistant.md)):
  official go-sdk pinned ≥ v1.6.1 (2026-05-22 —
  https://github.com/modelcontextprotocol/go-sdk/releases); stdio default,
  `-http` flag. Tool families shipped in v1: **components**
  (search/get/preview, search-then-get, paginated with `has_more`) and
  **decisions** (ADRs as resources + get-by-id). Patterns + tokens tools
  follow in Phase 3; adoption tools in Phase 5.
- **Retire-parity with the existing `tailwindplus` server:** a written
  checklist of every tool and real usage the current server provides; the
  Assistant matches or consciously supersedes each item.

### Gate G2 (exit)

In a fresh project containing only a committed `.mcp.json`: an agent
retrieves a named TailwindPlus component (correct framework/version/mode
parameters) and quotes a kit ADR, with no human explanation. Hybrid search
ranks the known-item top-3 for a committed set of 20 golden queries; the same
suite passes BM25-only with the embedder down. The old `tailwindplus` server
is removed from every `.mcp.json` — retirement is the gate, parity is the
proof.

---

## Phase 3 — Gallery v1

The living style guide (D12) becomes the kit's proof surface. Advances **#2**
(the Gallery is the catalog the day-one app is themed from) and **#4** (every
Gallery pattern page becomes Assistant-servable guidance).

Deliverables:

- The Gallery as a kit app — dogfood #1: live component catalog with knob
  controls generated from typed templ parameters; token visualization driven
  by the Spike 0.3 pipeline; service exercisers (bus, store, jobs, dialogs).
- First vendored components (templUI selections, DatastarUI reference
  material) adapted to kit tokens — vendored into the Kit once, per D13.
- **First patterns**, demonstrated as working pages: long-running job with
  progress; list+detail with live updates; form+validation round-trip. (The
  escape-hatch ladder for heavy interactivity is specified in
  [docs/design/ui-architecture-patterns.md](docs/design/ui-architecture-patterns.md).)
- Visual regression: Playwright snapshot baseline over Gallery pages, run in
  CI.

### Gate G3 (exit)

Every component and pattern the Kit ships has a Gallery page — enforced by a
registry check that fails CI on undocumented components. Playwright snapshot
suite green on macOS + Linux CI; a deliberate one-pixel token change is
caught by the suite (tested once, kept as the canary procedure). The Gallery
itself ships as web + signed desktop via the Phase 1 pipeline — the Gallery
is also the packaging regression test.

---

## Phase 4 — Throughline rebuild

North-star test **#1**, run as a protocol:
[docs/design/throughline-rebuild.md](docs/design/throughline-rebuild.md).
The rebuild is the Kit's requirements probe — every gap Throughline exposes
becomes a kit backlog item, built in the Kit when reusable, and the strangler
sequencing keeps that feedback flowing screen by screen instead of arriving
all at once at the end.

### Gate G4 (exit)

Exactly the acceptance criteria in the design doc, summarized: parity matrix
all green (or explicitly waived with recorded rationale); app-specific LOC
under the inventoried baseline by the target set post-inventory; ships web +
macOS + Windows from one repo through the standard two-command build, signed
and auto-updating; data migrated with verified fidelity; the old Throughline
archived after the daily-drive period passes. Advances **#1** to done.

---

## Phase 5 — Propagation

Improvements must travel. Advances **#3 (propagation test)** and closes the
loop on **#2**.

Deliverables:

- Assistant **adoption family** (D11): `audit_project` (drift report against
  kit conventions and current version), `scaffold_app` (new app from `app.cue`
  manifest), `upgrade_guide` (guided migration from upgrade notes per minor,
  D13).
- Upgrade notes discipline: every kit minor ships them; the Assistant serves
  them.
- The `CLAUDE.md` pack for kit apps (D15).
- A real improvement — something learned during the Throughline rebuild —
  lands in the Kit by PR and propagates.

### Gate G5 (exit)

One kit improvement: PR → tagged minor → both existing apps (the Gallery,
Throughline) upgraded via Assistant-guided migration, each in under an hour
wall-clock, zero manual retrofits — test #3 observed, not asserted. And the
day-one test measured end-to-end: a scratch app idea reaches a running,
themed, signed, auto-updating walking skeleton (web + desktop) in under a
day, mostly agent-driven, using `scaffold_app` + Gallery + Assistant. Both
measurements recorded in the repo.

---

## Continuous tracks (run alongside all phases)

- **Shell watch (D6).** Track Wails v3 beta promotion; on every pin bump,
  re-run the `appkit/shell/conformance` suite from Spike 0.1. Track mobile
  PR #5562 (experimental iOS+Android, same `main.go`, opened 2026-06-10 —
  https://github.com/wailsapp/wails/pull/5562): watched, not bet on; the
  tripwire to *start* a mobile phase is that PR merging and surviving two
  releases. Until then, mobile tier-1 is PWA (D6). Smoke-test the Tauri
  fallback wrapper at every kit minor release so the escape hatch stays
  oiled.
- **Version bumps.** Datastar: core frozen at 1.0.x, expect patch-level only
  (v1.0.2, 2026-06-02). CUE: quarterly minors treated like compiler upgrades
  (read notes, regenerate, diff — D7); v0.17 will re-baseline JSON Schema
  output naming (v0.17.0-alpha.2, 2026-06-03). templ (v0.3.1020, 2026-05-10)
  and Tailwind standalone (v4.3.0, 2026-05-08): re-pin with checksums. MCP:
  exactly one expected spec-driven go-sdk bump after the 2026-07-28 revision
  finalizes (release candidate as of June 2026 —
  https://blog.modelcontextprotocol.io/posts/2026-07-28-release-candidate/);
  budget it, don't fear it.
- **Data refresh.** Re-run the D10 pipeline when the TailwindPlus corpus
  updates; re-embed changed records only (content-addressed, digest-pinned).
- **Sync experiment (deferred).** NATS leaf-node device sync stays a
  prototype until the open offline-resync bugs are resolved and our own
  resync tests pass (nats-io/nats-server#6468, #7530 — open as of
  2026-06-10), per D8.
- **Cloud deployment (deferred).** Real auth design + deploy pipeline,
  deferred. Tripwire: the first kit app needing hosted web or PWA mobile
  tier-1. Until then, `-tags server` builds bind loopback only.

---

## Kill criteria — the evidence that triggers each documented fallback

Pre-committed, so a bad week can't argue its way past them.

| Bet | Kill / demote evidence | Fallback (already documented) | Real cost of falling back |
|---|---|---|---|
| **Wails v3 alpha (D6)** | Spike 0.1 fails on any platform with no fix after two pin bumps; OR no upstream release for 6 months; OR a pinned-alpha regression blocks shipping and cannot be pinned around | Tauri v2 + Go sidecar wrapper (kept tested, continuous track); interim: browser-tab tier | Rust toolchain tax, dual-binary signing/notarization, sidecar lifecycle edge cases; the updater gap — Wails's `app.Updater` doesn't carry over, so the update flow rebuilds on Tauri's updater or ships the degraded check-and-notify tier — and no Tauri path to mobile (sidecars impossible on iOS: tauri-apps/tauri#9774, discussions#11454, verified 2026-06-10) |
| **Datastar (D3)** | Upstream abandoned AND a webview/browser change breaks the vendored runtime with no patch we can carry | Fork-and-own the vendored ~34KB file (it is one file with a documented contract, by design) | We become maintainers of a small JS runtime — bounded, but ours |
| **Embedded NATS (D8)** | Spike 0.2 thresholds blown; OR JetStream file-store corruption on real laptop sleep/kill in daily use | Channel-backed `appkit/bus` becomes default; NATS demoted to opt-in | Lose persistence/replay/fan-out for free; device-sync future needs a new story |
| **CUE gengotypes (D7)** | Two consecutive CUE minors break codegen beyond a day's repair each (it is explicitly experimental) | Handwritten Go types + runtime cuecontext validation; codegen retained for CSS/JSON emission | Manual type/schema sync, guarded by CI validation — drudgery, not danger |
| **CUE itself (D7)** | CUE Labs collapses AND the pinned cuelang.org/go stops compiling against supported Go versions | Stay on the last good pin (vendored, no registry build-dependency) while migrating schemas to JSON Schema exports | Slow-burn by construction; the pin buys years of runway |
| **sqlite-vec / ncruces (D9/D10)** | Bindings unmaintained AND broken by a required SQLite/wazero upgrade; OR iOS (mobile phase) makes wazero-interpreter cost unacceptable | Plain-Go brute-force vectors over float32 blobs (~8k vectors, sub-ms); mobile plan-B per D9: modernc.org/sqlite or cgo | Lose vec0 SQL ergonomics; hand-rolled ranking joins |
| **MCP go-sdk (D11)** | None realistic — v1.x carries a hard compatibility guarantee (since v1.0.0, 2025-09-30) | n/a; budget the one post-2026-07-28 spec bump | A dependency-bump cycle per spec revision |
| **Mobile (deferred bet)** | (Inverse tripwire) PR #5562 merges and survives two releases → open a mobile phase proposal | Until then: PWA tier; if PWA insufficient before Wails mobile lands: gomobile-as-library + owned webview over the same loopback server (D5 makes this cheap) | PWA: no background sync on iOS, manual install; owned shell: we write thin Swift/Kotlin chrome |

The pattern across every row is D5: because each app is a Go HTTP server and
shells are chrome, every fallback above is a swap at the edge, not a rewrite
at the core. That is the Loopback Invariant earning its keep.
