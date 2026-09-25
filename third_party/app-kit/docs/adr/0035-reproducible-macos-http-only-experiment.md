# ADR-0035: Evaluate an explicit HTTP-only macOS host patch

- **Status:** Accepted
- **Type:** platform
- **Date:** 2026-09-14
- **Extends:** ADR-0026 and ADR-0033; supersedes no record

## Context

ADR-0026's containment revisit trigger fired at pinned Wails beta.21: the native
delegate lacks an origin veto and always installs the script-message bridge and
runtime. Unbound services and an HTTP fallback handler do not remove those paths.
The existing delegate also owns lifecycle callbacks; replacing it after creation
cannot protect the first authenticated navigation. Browser delivery stays independent.

## Decision

Evaluate one opt-in, macOS-only patch inside the existing isolated experiment.
`MacWindow.HTTPOnlyOrigin` sets a fixed exact origin before WebView creation. The
profile permits same-origin top-level navigation, refuses other origins/schemes,
subframes and page-created windows, and omits Wails script-message/scheme handlers
and runtime injection. The application's ordinary HTTP authorization remains required.
Framework asset endpoints are denied separately; no native binding replaces HTTP.

Keep the upstream module pin and checksums unchanged. A checked-in patch and explicit
preparation command verify inputs, copy the module into a fresh temporary directory,
apply the patch, and produce a temporary module file. Never mutate the module cache,
root dependency graph, installed tools or a consumer pin. Missing preparation must
fail compilation of the opt-in build rather than silently fall back to an unpatched
host. Ordinary native Gallery launch fails closed without this explicit profile;
the independently built server and browser path remain available.

Run the identical synthetic oracle on the original and patched binaries using actual
macOS WebViews. Each negative test needs evidence that its attempt executed, positive
bootstrap/navigation controls, and the required post-attempt outcome. Missing reports,
native failure or timeout are failures. A native shutdown receipt and separate verifier
are required: macOS termination need not return through Go code after `app.Run`.
Different outcomes establish only the cases measured, not a general sandbox proof.

## Consequences and limits

The local patch incurs maintenance and replay cost until an upstream option or a
different evaluated host meets the boundary. It is not a released adapter, upstream
contribution, new sub-kit or production-support claim. Navigation containment is not
a network firewall, hostile-local-process isolation or a per-window cookie store.
Application CSP, authentication, CSRF, secret handling and stream cancellation stand.
Actual Gallery task parity, keyboard/IME, permissions, multiple views, startup recovery
and applicable distribution evidence remain required. Windows and mobile stay deferred;
no signing, updater, installer or remote topology is selected here.
The strict frame refusal does not support Gallery's iframe-based private comparisons;
private inputs are disabled in this candidate. Keep their independent browser viewer.

## Alternatives rejected

- DOM click interception: does not govern script navigation, redirects or the bridge.
- Post-start delegate replacement: creates first-load and lifecycle-forwarding gaps.
- Keep unsafe native launch as the default: an explicit bounded baseline probe is
  sufficient for comparison; ordinary experiment users should not inherit it silently.
- Build a generic capability platform now: this experiment needs one narrow HTTP host.

## Revisit when

- An actual initial-load, redirect, bridge, frame, popup or lifecycle case fails.
- Wails supplies equivalent supported controls: test the same oracle before dropping
  the patch; a newer version number is not proof.
- A named consumer is ready for adoption: settle reviewed delivery and dependency
  ownership before promoting this temporary experiment.
- A selected native task needs embedded comparisons: evaluate a narrowly constrained
  frame policy and its own oracle before enabling that task; do not weaken this probe.
