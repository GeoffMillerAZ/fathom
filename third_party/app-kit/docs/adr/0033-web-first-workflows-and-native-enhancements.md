# ADR-0033: Keep Wails applications usable as web applications

- **Status:** Accepted
- **Type:** platform
- **Date:** 2026-09-13
- **Extends:** ADR-0021/0026; ADR-0027 still defers Windows
- **Decision drivers:** reusable workflows, independent browser delivery, explicit native value

## Context

The owner wants Wails applications to remain functional as web apps where their tasks
permit. ADR-0021 separates domain logic from the shell and retains browser delivery.
Calling the browser only a fallback understates its product role. The isolated Wails
candidate has HTTP/build evidence, not completed native task or delivery proof.

## Decision

Ordinary-browser delivery is first-class for app-kit-owned and consumer-selected Wails
profiles. New work names core tasks and implements them through shared application
contracts, independently of a running Wails window or privileged bridge. Existing apps
move toward this boundary when the owner selects an actual migration task; this does
not authorize a fleet rewrite or change portable/client-only adoption requirements.

Native capabilities are explicit enhancements behind app-owned adapters. Each names
its permission, browser equivalent or honest unavailable/degraded behavior, recovery
and evidence. An OS-essential task records an exception, useful browser subset and
revisit trigger. Do not invent a remote device service merely to satisfy parity.

The Go operational profile shares its HTTP/application boundary. A browser-local
server is valid web delivery but runs independently of Wails. Remote HTTPS hosting is
a separate selected topology with identity, authorization, storage, proxy and deployment
tests. Loopback launch credentials are not cloud authentication. ADR-0023's client-owned
profiles remain valid; no new runtime, framework or capability-registry API is introduced.

Promotion requires the [parity contract](../design/web-desktop-parity.md): core tasks
pass separately in browser and macOS with identical deterministic fixtures; native-only
exceptions are explicit. No target inherits another target's pass. Windows/mobile,
offline sync, remote deployment and signing retain their separate scope and authority.

## Alternatives considered

- Native-first with a later web rewrite: duplicates application work and binds ordinary
  tasks to the host. Wins only for a documented OS-essential task.
- Identical features everywhere: misrepresents browser permissions and can force
  unnecessary infrastructure. Prefer equivalent outcomes with explicit exceptions.
- Mandatory cloud hosting: imposes operations and identity costs on local tools
  without a demonstrated remote-use requirement.

## Consequences

G6 owns the initial Gallery parity proof; consumer adoption names topology and
exceptions. Independent browser/server builds and native tests are promotion conditions,
not current support claims. No public API or dependency changes in this decision.

## Revisit when

- A named core task requires OS access with no useful browser outcome.
- Domain authority leaks into a native adapter or server builds import native bindings.
- A selected remote topology fails authentication, transport or performance requirements.
- The owner selects an existing Wails app for migration, or a required parity test fails.
