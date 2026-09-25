# ADR-0021: Runtime choices apply to explicit consumer profiles

- **Status:** Partly superseded by ADR-0027 (current target scope only)
- **Type:** platform
- **Date:** 2026-09-13
- **Supersedes:** ADR-0003, ADR-0005, ADR-0006, ADR-0008; ADR-0014's CSP permanence clause

## Context

The original runtime decisions predate real layered and terminal adoption. Universal
wording contradicts ADR-0017/0018 and delays useful UI work behind an unbuilt spine.
The owner wants Go/CUE/Charm excellence and web/desktop reach across different use cases.
Wails' beta trigger fired: its [status](https://v3.wails.io/status/) now describes desktop
beta compatibility and experimental mobile (checked 2026-09-13).

## Decision

Restate the surviving runtime commitments with explicit boundaries:

- **Portable artifacts:** tokens/CSS and optional browser JS may serve any consumer
  framework. No Go server, Datastar, NATS or desktop dependency is implied.
- **Go web:** Go + templ with pinned Datastar is the preferred dynamic operational
  profile; ordinary HTML suffices for static/request-response behavior. Business state
  is server-authoritative. Use short mutation commands and idempotent HTTP/SSE
  projections when streaming is needed; view-only state stays local.
- **Streaming:** one shared stream per active view/tab is the default when required,
  with cancellation, bounded updates, reconnection and resnapshot semantics. Optimistic
  durable mutation is not the default; admit it only with a measured need and explicit
  reconciliation/error contract. Do not open a stream merely to render a settings page.
- **Desktop:** prefer pinned Wails v3 as a thin host for the Go/HTTP profile. Preserve
  one http.Handler tree and real loopback HTTP for dynamic traffic; cloud uses a
  configured bind and real auth. App domain code never calls shell-specific JS/IPC.
  Native capabilities have host adapters and explicit unavailable/browser behavior.
- **Security:** retain ADR-0014's token-bootstrap, origin/auth, escaping, secret and
  signing/update requirements where applicable, including no telemetry. Retire its
  claim that unsafe-eval is permanent: current Datastar [security docs](https://data-star.dev/reference/security)
  describe a nonce CSP path (2026-09-13). Validate it at the chosen pin and remove
  unsafe-eval where that proof passes; documentation alone is not a migration.
- **Events/storage:** NATS/JetStream is an optional profile for demonstrated fan-out,
  replay or distribution needs, not the prerequisite for an app or Gallery. Keep
  application state authority and durable relational storage explicit (ADR-0009).
  If selected, pin dependencies, bound retention, validate subjects/config in CUE,
  prefer in-process transport for local embedding and authorized server projections to
  browsers. Do not build an abstract bus plus multiple backends before a consumer needs it.
- **Terminal:** ADR-0018's Go/CUE/Cobra/Charm profile stands outside the HTTP contract.
- **Mobile:** responsive web is the present design target; native support is a future
  that must earn real-device proof. Shared source does not mean one binary for all OSes.

## Evidence before platform support

G6 must record exact runtime/SDK/browser/OS versions and demonstrate streaming delivery,
POST bodies, reconnect/resnapshot, multi-window consistency, authentication/isolation,
focus/input, shutdown, native feature availability and packaging/update failure recovery
on browser, macOS and Windows. Distinguish platform build tools from cgo-free core builds.
Signed delivery needs signature verification and rollback/recovery proof, not checksums
alone. No shell, updater or alternate fallback implementation is claimed today.

Browser delivery remains the fallback. Evaluate another shell only when Wails fails a
required test or maintenance blocks shipping; do not maintain speculative wrappers.
Re-pin against beta only during the tested adapter work; this ADR changes no dependency.

## Alternatives rejected

- Every application embeds NATS or adopts Wails: no demonstrated benefit for many consumers.
- Switch all apps to Wails service bindings: would change the shared HTTP/domain boundary.
- Pre-build mobile and every renderer adapter: unproven complexity and duplicated authority.

## Revisit when

- A required desktop conformance case fails at the selected Wails pin.
- A measured offline/latency/CSP constraint defeats the chosen web profile.
- A real workload needs distributed events, native mobile or accelerated client rendering:
  apply the corresponding future's promotion condition.
