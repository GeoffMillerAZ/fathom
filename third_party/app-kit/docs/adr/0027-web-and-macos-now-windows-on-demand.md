# ADR-0027: Prove web and macOS now; Windows follows a real use case

- **Status:** Accepted
- **Type:** platform
- **Date:** 2026-09-13
- **Supersedes:** ADR-0021's requirement that current G6 completion include Windows

## Context

The owner explicitly deferred all Windows work: no current use case or Windows test
host exists, while future Windows support remains desirable. Treating it as a current
completion prerequisite spends effort on a platform the owner cannot presently use.
This is an owner-directed scope change, not passing or waiving a failed platform test.

## Decision

Current G6 proves the same representative workflow in **web and macOS**. Windows is
a [future](../intent/futures.md#windows-desktop-delivery), not a current blocker or build order.
Do not seek a Windows machine, install a VM, implement Windows-only adapters/packaging,
or repeat cross-builds merely to satisfy current G6. Existing isolated source and dated
cross-build evidence stay intact; they do not establish Windows runtime support.

Surviving ADR-0021 commitments remain binding: portable artifacts require no kit runtime;
Go/templ with explicitly useful pinned Datastar remains the preferred operational profile;
server authority and local view state stay separate; active streams are bounded and
cancelled; ordinary HTTP and browser fallback stay useful. Wails remains a thin pinned
host around the shared handler, independent of domain code. Nonce/CSP, origin/auth,
secret, no-telemetry and applicable signing/update requirements remain. NATS is optional
for demonstrated need, Charm remains the terminal target, and native mobile stays future.

The complete ADR-0021 matrix still applies to **each currently claimed target**: pinned
versions, SSE/POST, isolation/auth, reconnect, multi-window consistency, focus/input,
shutdown, native availability/denial and applicable packaging/update failure recovery.
Web/macOS promotion is not granted by this record. ADR-0026's isolated experiment and
native containment blockers remain unchanged. A future Windows claim must pass the same
matrix on an actual authorized Windows host; successful cross-compilation is insufficient.

## Alternatives rejected

- Keep Windows as a completion dependency: no current owner workflow justifies it.
- Remove all Windows-compatible source: needless churn would discard a cheap option.
- Label cross-builds Windows support: confuses build evidence with usable delivery.

## Revisit when

- The owner identifies a current Windows consumer/task and an authorized test host.
- A proposed web/macOS change would gratuitously couple domain code to macOS; preserve
  the existing shared HTTP boundary without prebuilding a Windows abstraction.
