# Runtime selection: precise defaults, strong alternatives

Accepted direction: [ADR-0021](../adr/0021-runtime-profiles-and-platform-proof.md),
[ADR-0027](../adr/0027-web-and-macos-now-windows-on-demand.md) (web/macOS now; Windows future), and
[ADR-0023](../adr/0023-profile-specific-conversion-and-client-compute.md). This is selection
guidance, not a claim that profile scaffolds or runtime adapters have shipped. Bounded
HTML syntax translation exists; [web/desktop parity](web-desktop-parity.md) defines ADR-0033's native/browser contract.
[Research](../assessments/2026-09-13-conversion-strategy.md) separates facts from inference.

## Start with the physical task

Record device, network latency/disconnection, data scale, input frequency, accessibility,
SEO, hosting, threat model, persistence and failure costs. Choose where computation,
DOM ownership and authoritative state belong. Technology names are not proof of speed,
stability or determinism; measure total delivery and maintenance cost on the same task.

| Workload | Recommended starting point | What would change the choice |
|---|---|---|
| Brochure, content, documentation | Go/templ build-time HTML + pinned CSS/assets; native links/forms; no hydration by default | content pipeline/CMS/localization complexity needs a proven static generator, or a named dynamic feature |
| Dashboard, settings, operational workflow | Go + templ; ordinary HTTP first, pinned Datastar for useful reactive/server projections | measured offline/RTT/input requirements defeat the server-owned interaction boundary |
| Heavy visualization/editor/workbench | client-owned interactive island with typed boundary; a full client app if most of the screen needs it | local complexity is small enough for ordinary HTML, or a measured simpler implementation wins |
| CPU-heavy browser analysis | worker first; compare JS and compiled Go/TinyGo Wasm on identical inputs | module startup/copy/heap cost exceeds useful compute saving |
| Backend fan-out/replay/work queues | NATS/JetStream when topology requires it; relational authority remains explicit | an in-process call or SQL transaction satisfies the need more simply |
| Native desktop | first-class browser tasks plus the same Go/HTTP workflow in the pinned Wails candidate | target conformance fails or an OS-essential task needs an explicit browser exception |

These strategies map onto existing catalog IDs (`static-web`, `go-web`, `portable`,
`react-web`) only where a particular entry already declares support. They introduce no
new profile strings or compatibility claims by implication. CLI remains ADR-0018.

## Static is a first-class target

Generate complete semantic pages with real content, canonical URLs, metadata, sitemap,
feed/search where useful, explicit redirects and a working 404. Preserve browser history,
deep links and navigation without a client router when files/routes suffice. Optimize
responsive images, intrinsic dimensions, font loading and caching; account for licenses.
Dynamic contact/search/account features declare their backend, privacy and failure path.
No fake form success or secret in a static bundle. Choose hash-based static CSP or
external scripts; never bake a reusable nonce into public cached HTML.

Go/templ can produce HTML during a build: a visitor needs neither Go nor a server.
For a content-heavy publishing workflow, a measured existing generator may be the better
consumer tool. Keep kit tokens, sections and verification portable instead of rebuilding
a CMS, asset pipeline, localization platform or universal site generator inside app-kit.

The experimental [static reading export](static-content-delivery.md) applies this
boundary to the first-party guide/FAQ and article: build-only Go, system/light/dark
files, explicit CSS, a byte manifest and base-path-aware404 recovery (ADR-0071).
It does not add a publishing platform or certify a consumer's hosting configuration.

## Server authority without sluggish local interaction

Let Go own authorization, domain validation, durable commands and authoritative queries.
Let the browser own focus, selection, disclosure, drafts and other immediate view state.
Datastar is an enhancement, not a reason for every keystroke or menu to require a request.
Server-generated projections need escaping, static expressions, stable IDs, cancellation,
request ordering and explicit resnapshot/recovery. One stream per active view/tab when
useful, with bounded queued bytes and cleanup; not one subscription per widget.

Measure compression flushing and proxy buffering end-to-end, not just handler timing.
Test slow/offline/reconnected tabs, stale responses and multi-window conflicts. Durable
commands need idempotency and user-visible pending/error states. A duplicate network
delivery must not duplicate an external side effect. Standard HTTP fallback or an
explicit useful degraded state is part of the feature, not an afterthought.

Current Datastar documents a nonce-based CSP mode, but expression input remains unsafe
unless serialized/escaped appropriately. Test the exact runtime pin, selected browser
and CSP together. Do not assume an upstream documentation example proves compatibility.

## Client-heavy does not mean undisciplined

A rich editor, map, canvas, local dataset or direct-manipulation interface may legitimately
need a client renderer. Prefer a proven specialized implementation over recreating its
undo, IME, selection, clipboard, accessibility and virtualization in server templates.
Choose the smallest useful boundary; use a full client application when a patchwork of
islands would create more state/lifecycle glue. Popularity is not the decision criterion.

ADR-0023 permits a pinned client toolchain and correctness-bearing browser code in a
declared consumer profile, even when Go hosts it. This does not add npm/build tooling
to kit-core or make portable L1/L2 artifacts depend on that consumer's framework.

The private [P12 Go-hosted editor](client-editor-island.md), retrievable as
`get_kit_decision("client-editor-island")` or `kit://client-editor-island`, demonstrates
this boundary with scoped ownership/browser proof. Its guide links the private fixture
prerequisites and replay commands; it is not a catalog capability, public API or certified profile.
Its [pinned Datastar probe](../assessments/2026-09-16-client-editor-datastar.md) also
preserves the editor across an actual surrounding outer morph; direct replacement,
declarative expression compilation and general lifecycle integration remain unproven.
The [actual200% follow-up](../assessments/2026-09-16-client-editor-zoom.md) retains
large-document and morph tasks without changing editor/style bytes. It is scoped
Chrome evidence, not full accessibility or production readiness.

One renderer owns each DOM subtree. Datastar must not morph a client renderer's internals;
declare mount/update/dispose and versioned data/event boundaries. Avoid duplicate stores
claiming the same data. Signals are not an auth boundary. Use typed APIs and CUE-authored
data contracts where sharing earns its cost; no cross-framework component IR is implied.
Lazy-load costly features; cancel work, bound memory, virtualize only when measured,
move expensive computation off the main thread, and retain accessible table/text views.

If offline edits matter, define local persistence, quota/eviction handling, schema
migration, reauthentication, pending/conflicted status, idempotency and reconciliation.
Service-worker caching alone is not offline correctness. CRDTs and sync engines require
a named collaborative task; do not prebuild them for every app. A server-authoritative
baseline cannot be relabeled offline-capable because assets happen to be cached.

## Wasm and event runtimes have distinct jobs

Browser WebAssembly executes compute through the browser's native engine and host APIs.
It does not replace HTML/CSS/accessibility or remove the need for DOM interop. Go's
browser target and WASI host modules are different builds/ABIs, not interchangeable.
Benchmark download, compile/start, JS/Wasm boundary copies, memory, cancellation and
warm compute; record compiler/runtime/architecture. Workers still need messages to update DOM.

wasm3 is an interpreter for embedding Wasm in a host, not browser Wasm support. Its
maintenance posture requires review. For a Go-native plugin need, evaluate a pinned
pure-Go host such as wazero; use another runtime when its required features/platforms
win. Do not embed any interpreter just to run an ordinary webpage. No host selected yet.

NATS is event infrastructure, not a UI framework, automatic database consistency, or
offline sync. If selected, specify stream/consumer limits, backpressure, replay windows,
deduplication, acknowledgments, tenant/subject authorization and failure recovery.
Expose authorized browser projections, not privileged broker credentials. Exactly-once
messaging claims do not guarantee exactly-once external business effects.

## Deliberate performance experiments

For the initial conversion pilot, use a named mid-range device/browser, 390 and 1440 CSS
pixel widths, and 0/100/300 ms added RTT plus offline recovery. Proposed screening budgets:
static essential navigation uses 0 JS bytes; operational first-view JS ≤50 KiB compressed;
local input-to-next-paint p95 ≤100 ms; no task-blocking main-thread work >50 ms; heavy
client budgets are declared before choosing its runtime, not retrofitted to a result.
These are pilot design targets, not current measurements or universal kit guarantees.
Record CSS/fonts/images too; a tiny script beside megabytes of media is not a lean page.

Measure cold/warm first useful view, input/update latency, data/mark counts, retained heap,
stream fan-out and clean integration effort. Test 200% zoom, keyboard, dark/light and
missing/error states alongside performance. Compare to source-native and simpler options.
An exception names the requirement, measured tradeoff and revisit trigger. Fewer bytes
or dependencies alone cannot justify broken behavior or a worse task experience.
