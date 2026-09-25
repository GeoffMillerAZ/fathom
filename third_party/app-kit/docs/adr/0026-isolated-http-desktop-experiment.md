# ADR-0026: Isolate the native-host experiment and preserve authenticated HTML workflows

- **Status:** Accepted
- **Type:** platform
- **Date:** 2026-09-13
- **Partly supersedes:** ADR-0013's one-module rule for this unpublished spike;
  ADR-0014 section 4's mandatory custom header for ordinary HTML forms
- **Extends:** ADR-0021

## Context

G6 needs an actual native host around the existing Gallery HTTP workflow. Wails
introduces native build dependencies; adding an experimental pin to the root module
would charge every consumer before the candidate passes its target matrix.
ADR-0013's cgo/dependency-isolation trigger applies. The Gallery's existing ordinary
HTML POST uses session-bound CSRF tokens and origin checks, not a JavaScript header.
Forcing a custom header would remove the useful no-JavaScript command path.

## Decision

Use `spikes/wails-gallery` as an unpublished, explicitly tested nested module for the
G6 candidate, pinned to Wails `v3.0.0-beta.21`. It imports the existing Gallery handler;
application routes, data and rendering do not depend on Wails. Root-module consumers
inherit neither this pin nor its native build prerequisites. The local module replace
is an experiment input, not released adoption or a clean-consumer-build result.
No independently versioned kit, public API or new consumer profile is introduced.

Bind one real `127.0.0.1:0` HTTP listener before opening a window. A fresh 256-bit
single-use launch token, expiring after 120 seconds, exchanges for a host-only,
HttpOnly, SameSite=Strict cookie and a clean redirect. Validate exact Host and peer;
refuse foreign origins and ambiguous credentials. Never log the launch URL or token.
All application and static responses require host authorization; an optional health
response contains no application data. Native services remain unbound in the spike.

For ordinary HTML commands, allow a server-validated, unpredictable session-bound
CSRF form token instead of `X-AppKit-Cmd`. Require same-origin mutation requests,
strict cookies, bounded parsing, and the application's authorization/version checks.
The alternative is not permission to accept a cookie alone or trust submitted signals.
Custom-header clients remain valid; each selected command path needs refusal tests.
All other surviving ADR-0014 requirements and ADR-0021's full target matrix stand.

Use synthetic data only and disable private source inputs. Keep the native candidate
out of normal kit builds until it earns promotion. A successful compile is not a
navigation boundary, browser task, multi-window result, signed package or support claim.
Native bridge exposure and external-navigation control require explicit investigation;
an unproven boundary blocks production promotion, not a truthful synthetic experiment.

## Consequences

The separate module requires an explicit test/build command; root `go test ./...`
does not cover it. Record exact checks and failures in its guide. Keep one root release
stream; reconsider module placement only when an adapter is ready for consumers.
No mobile runtime, event bus, generic capability framework, updater or OS installation
is authorized by this experiment. Browser delivery remains a required G6 outcome.

## Alternatives rejected

- Put the candidate in the root module now: exposes unproven native dependency churn.
  Wins if promotion evidence and measured dependency cost justify it.
- Rewrite forms around native IPC or mandatory JS: changes the shared HTTP workflow.
- Claim click interception is a native navigation guard: scripts are not that boundary.

## Revisit when

- The selected Wails pin cannot enforce a required navigation/bridge or lifecycle boundary.
- The same HTTP task fails on a claimed browser/macOS/Windows target.
- The adapter is ready for a named consumer: settle dependency/distribution placement.
- A missing/foreign CSRF token or ambiguous host credential reaches a mutation handler.
