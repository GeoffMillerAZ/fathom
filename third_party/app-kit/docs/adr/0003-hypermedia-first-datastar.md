# ADR-0003: Build all UI hypermedia-first on Datastar

- **Status:** Accepted
- **Date:** 2026-06-10
- **Decision drivers:** backend state authority; no Node ever (D2); machine
  legibility for agents (D15); determinism over a multi-decade horizon; one
  codebase across web/desktop/mobile via the Loopback Invariant (D5)

## Context

Every kit app is a Go HTTP server serving HTML over loopback — that is the
survival invariant from VISION.md and ADR-0005. The UI layer must therefore
be something a Go server can drive completely, that an agent can read and
write as plain Go + HTML, and that does not drag a JavaScript toolchain into
a repo that has banned Node permanently (ADR-0002).

The relevant facts, dated and versioned:

- **Datastar reached stable v1.0.0 on 2026-04-16** after an 8-RC run from
  July 2025; v1.0.2 (2026-06-02) is current as of today. The author declares
  the core finished ("done like dinner") and frozen against further churn;
  post-v1 roadmap items (Stellar CSS, Project Darkstar/WebTransport) are
  additive, not core rewrites
  (https://github.com/starfederation/datastar/releases,
  https://data-star.dev/essays/v1_and_beyond).
- **The runtime is one 34KB JS file** (11.76 KiB brotli, measured from the
  official v1.0.2 CDN bundle) that evaluates declarative `data-*` attributes
  with reactive signals (https://data-star.dev/). Small enough to vendor and
  `go:embed`; documented enough to maintain a fork if the project dies.
- **The server drives the DOM over SSE.** All backend actions
  (`@get/@post/...`) go through `fetch()` with in-library
  `text/event-stream` parsing — never `EventSource` — and responses stream
  `datastar-patch-elements` / `datastar-patch-signals` events. Morphing
  preserves element state (focus, scroll, input values). Reconnection retry
  is built in and configurable (1000ms base, 2x scaler, 30s max wait, 10
  retries) (https://data-star.dev/reference/actions, verified 2026-06-10).
- **The official Go SDK is mature and in lockstep:**
  `github.com/starfederation/datastar-go` v1.2.2 released 2026-06-02, the
  same day as core v1.0.2. API: `NewSSE`, `PatchElements` (+ templ helpers),
  `MarshalAndPatchSignals`, `ReadSignals`
  (https://github.com/starfederation/datastar-go).
- **The recommended architecture matches our priors.** "The Tao of Datastar"
  prescribes CQRS: backend owns all state, one long-lived GET SSE stream per
  tab renders idempotent "fat-morph" snapshots, commands are short POSTs,
  signals are only for ephemeral client interaction, optimistic UI is
  explicitly discouraged. Brotli on repetitive snapshots reaches ~200:1
  (https://data-star.dev/guide/the_tao_of_datastar). The author (Delaney
  Gillilan, ex-HTMX contributor) works at Synadia — the NATS company — and
  the Go+NATS+Datastar pairing is an established pattern with a maintained
  reference template (zangster300/northstar, v0.16.0 tagged 2026-06-03).
- **Two permanent costs are known up front.** (1) Datastar expressions are
  evaluated via the `Function` constructor, so CSP must include
  `'unsafe-eval'` — there is no nonce path
  (https://data-star.dev/reference/security). (2) Datastar Pro (Inspector
  debugger, extra attributes/actions) is a $349 lifetime solo license whose
  terms forbid embedding in open-source projects
  (https://data-star.dev/pro, verified 2026-06-10).

## Decision

We build every kit UI hypermedia-first on **Datastar v1.0.2** with
**datastar-go v1.2.2**, vendored as a pinned, checksummed, `go:embed`-ed
34KB artifact — never loaded from a CDN, never built by a toolchain.

The **Tao-of-Datastar CQRS shape is the kit default**, non-negotiable unless
a pattern in `../design/ui-architecture-patterns.md` says otherwise:

1. The backend owns all state. The DOM is a projection, never a store.
2. One long-lived GET SSE stream per tab renders idempotent fat-morph
   snapshots; the NATS KV-watch → re-render → patch loop (ADR-0008) is the
   engine behind it.
3. Commands are short POSTs that mutate server state and return little or
   nothing; the stream delivers the new truth.
4. Signals exist only for ephemeral client interaction (form binding, open
   menus, highlight index). Anything worth keeping goes through a command.
5. **No optimistic UI.** On loopback the round trip is sub-millisecond;
   showing anything but server truth is a latency hack we don't need.

High-interactivity surfaces (canvas, editors, maps) do not get a framework;
they get the escape-hatch ladder defined in
`../design/ui-architecture-patterns.md` — at most a vendored JS island behind a
documented signals-bridge contract.

**Pro boundary:** we own one Datastar Pro Solo license, bought for the
Inspector (the signals/SSE debugger — the dev-time observability story).
Pro code is a privately vendored, optional overlay. It never enters any
layer of the kit that could ever be open-sourced; the license forbids it,
and the OSS-able core must stay clean by construction, not by audit.

**Accepted cost, recorded here, detailed elsewhere:** the kit's default CSP
is `script-src 'self' 'unsafe-eval'`. This is a permanent Datastar
requirement and an accepted trade; the full security treatment lives in
ADR-0014 (`0014-security-baseline.md`).

This decision does NOT cover: the templating layer (ADR-0004), the event
spine that feeds the render loop (ADR-0008), transport/shell questions
(ADR-0005, ADR-0006), or per-pattern UI wiring (the design doc owns that).

## Alternatives considered

- **HTMX (+ Alpine.js for client state).** The most popular hypermedia
  library, vastly larger community, more answers on every search. Lost
  because Datastar subsumes both halves in one smaller library: SSE is the
  native transport rather than a GET-only bolt-on extension, signals replace
  Alpine entirely, and morphing is the default patch mode — one library, one
  mental model, less total JS. The public switch literature (Everyday
  Superpowers, "Why I switched from HTMX to Datastar", HN thread Oct 2025)
  matches our own read. Real cost of the winner: Datastar's ecosystem is a
  fraction of HTMX's (~4.5k stars, one primary author funded by Pro sales),
  and v1.0 is only ~8 weeks old — we are trading community depth for
  architectural fit and accepting bus-factor risk.
- **SPA frameworks (React/Vue/Svelte).** The largest component ecosystems in
  existence, and the code agents have seen most. Lost on three independent
  grounds, any one of which is fatal here: (1) D2 — every one of them
  requires the Node toolchain we have banned; (2) state authority — an SPA
  duplicates state on the client, and the duplicate inevitably drifts from
  the Go truth, creating exactly the two-codebase problem the kit exists to
  kill; (3) determinism — JS toolchain churn over a multi-decade horizon is
  a maintenance tax with no upside for personal-scale apps. Real cost: we
  forfeit the npm component universe and must source components our own way
  (see `../design/ui-architecture-patterns.md` §4).
- **Server-side islands frameworks (Astro-class partial hydration).** The
  pitch — mostly-static HTML with hydrated islands — sounds adjacent to our
  ladder. Lost because the entire category is built on Node toolchains and
  JS-authored islands (D2 violation), and its islands own client state in a
  framework runtime, violating state authority. We keep the good idea —
  isolated interactive islands — as L2 of our ladder: vendored web
  components with a signals bridge, no hydration framework.
- **Plain server-rendered pages + hand-rolled vanilla JS.** Maximal
  sovereignty, zero dependencies. Lost because we would end up rewriting
  Datastar badly: fetch-SSE parsing, exponential retry, request
  cancellation, and state-preserving DOM morphing are each genuinely hard,
  and a 34KB vendored file with a documented wire contract is strictly
  cheaper to own than a homegrown equivalent. This option remains the
  disaster-recovery floor: if Datastar dies, the patch-event contract is
  small enough to reimplement or fork.

## Consequences

- **Easier:** the whole UI is Go + templ + attributes — compiler-checked,
  grep-able, and agent-legible end to end; reconnection and multi-tab
  convergence fall out of idempotent snapshots instead of being features we
  build; web and desktop are literally the same rendering path (D5); the
  deterministic-build story stays two commands with zero JS toolchain.
- **Harder:** every interaction round-trips to the server — free on
  loopback, but remote web deployments need HTTP/2 (browsers cap ~6
  connections per origin on HTTP/1.1) and sane latency; rich interactivity
  requires the discipline of the escape ladder instead of `npm install`;
  component sourcing is adapt-and-own instead of import (mitigated by the
  TailwindPlus corpus, templUI, and DatastarUI — see the design doc).
- **Risk: bus factor.** One primary author, Pro-funded. *Tripwire:* a
  confirmed security or regression issue in core goes unfixed for 6 months
  → we activate the fork plan (we already vendor the file and document the
  patch contract).
- **Risk: "frozen core" is a promise, not a track record.** v1.0.2 still
  changed behavior (checkbox `data-bind` default, abort-controller scoping).
  *Tripwire:* any 1.0.x release breaks a kit app → upgrades get the
  CUE/compiler treatment: pinned exactly, release notes read, Gallery
  visual-regression run before adoption.
- **Risk: `'unsafe-eval'` forecloses strict-CSP web deployments.**
  Accepted; irrelevant inside shells we control. *Tripwire:* a kit app must
  ship under a strict third-party CSP → that app takes the documented
  deviation path in ADR-0014.
- **Risk: no-optimistic-UI meets real latency someday.** Mobile/offline
  will not have sub-ms round trips. *Tripwire:* mobile phase perceived-
  latency tests fail → we put local-first Go behind the same SSE interface
  (a kit problem, not a Datastar deviation).

## Revisit when

- Datastar ships a 2.x or Project Darkstar (WebTransport) changes the
  transport story — re-evaluate, don't auto-upgrade.
- Core maintenance stalls: no release or fix within 6 months of a confirmed
  security/regression issue.
- Pro license terms change in a way that threatens the private-overlay
  arrangement, or table-stakes features start landing Pro-only.
- The mobile phase measures perceived latency that idempotent snapshots
  plus loading indicators cannot hide.
