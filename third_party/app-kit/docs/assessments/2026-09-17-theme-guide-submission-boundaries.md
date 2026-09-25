# Theme-guide submission/loading boundary controls

Follow-up to the [idle/edit and Stop controls](2026-09-17-theme-guide-target-controls.md).
This prospective diagnostic separates an accepted but pending native Preview request
from its completed response/navigation. It changes neither the real handler nor the
CDP guard, source CUE, budgets, installed browser or consumer configuration.

## Fixed acquisition

Select four fresh-profile runs in this order: `pending-preview`, `completed-preview`,
`pending-preview`, `completed-preview`. Use the same exact source/runtime/build inputs,
wide Dashboard/light specimen, edited values, no-startup-window, phase observations on
and unknown-target metadata inspection off. Do not expand the sample to obtain a pass.
Stop this acquisition if inputs drift or owned cleanup cannot be verified.

Both controls use the existing authenticated fixture arm and native focus/Enter
submission. Require the exact POST body and a held fixture with `started`/`live` true
and `canceled`/`timed_out`/`delegated` false. Retain request identity, response count,
page loader/navigation count, time and guard/protocol state at this boundary.

- Pending: enumerate targets once while that POST has no response and the original
  document/preview and edits remain. Do not Stop, retry, export or reset. Later context
  closure may cancel the request; that is cleanup, not tested Stop behavior.
- Completed: the held marker is passive, with no Target enumeration. Wait beyond the
  fixture's existing 15-second timeout for actual candidate POST/navigation and the
  complete required asset closure, then enumerate once. `delegated` alone is not
  completion because the wrapper records it before invoking the actual handler.

Both retain the common initial navigation observation. Skip the usual final target
enumeration for these controls so each has exactly one additional boundary observation.
Unknown targets remain paused and fail the diagnostic. A refused boundary leaves later
work unexecuted. All twelve cancellation cells remain unexecuted; full producer and
completion flags stay false even when the bounded diagnostic succeeds.

## Interpretation fixed before execution

A pending failure means completed Preview navigation and Stop are unnecessary for
that observed failure. It does not prove that submission caused the target. A failure
only after completion still conflates elapsed time, delegation, response/navigation and
enumeration-triggered attachment. A clean passive interval records no observed attach
event, not absence of an unattached target. Two clean runs establish non-reproduction
under those inputs, not a safe target type, general exclusion or solved failure.

No guard exception, new target policy, network privilege or resumption follows from
these controls. Full cancellation, memory/cache and consumer acceptance remain open.

## Evidence

Implementation and scoped verification are prepared. **Browser acquisition was not
run**: the owner selected stabilization and demand-driven continuation under
[ADR-0072](../adr/0072-stabilize-and-resume-on-demand.md). Resume only through R2 in
the [handoff](../design/stabilization-and-resumption.md), not automatically.

Retained private packet root: `/Volumes/m2ext/appkit-evidence/appkit-theme-cancel.F2rbr0oq`.
The following exact receipt bytes were rechecked at handoff:

| Receipt directory | Verified scope | receipt.json SHA-256 |
|---|---|---|
| `owned-check-hiCL49` | Process-owner unit canaries, receipt-binding tests and owner syntax pass; synthetic only | `cef85bc0989105bc70be8d24ad0407288a7276ea9b2305d230dc24adc85b70cd` |
| `guard-check-XzBhie` | Guard unit canaries and probe syntax pass; no browser containment claim | `1084e154a4205a04effbed94da210f0a8f0958d65f20169cf30d92732f956678` |
| `quality-submit-boundaries` | Go1.26.3 race tests for qualityplan, MCP, cancellation fixture and Gallery; fixture build pass | `7f3aabd1526ef85274501d73c7701f9b19167930c2d4029d25d3701b659e83c4` |

Each receipt recorded unchanged retained inputs at acquisition. These are historical
checks of those inputs, not checks of subsequent documentation/MCP edits. Probe
`apps/styleguide/browser/cancellation.mjs` still hashes to
`ce08e3f7b0f0702b90d09349068ade7b48b2013238bc7c2c54b30d5566ff7daf`;
unchanged `cdp_guard.mjs` hashes to
`bc670b3cb8ac12c01d02de95e74840c7748a462a55eac13c1eaf6a7259302f34`.
Prepared fixture binary at `quality-submit-boundaries/theme-cancellation` is recorded
as `55ca7d7e7fd02679384f6fb5a1df0116b5eb881f17645deb43f6ac388a2fa6e0`.

The packet's `run-gallery.mjs`, `receipt-control.mjs` and retained input snapshots own
the private acquisition harness; check their receipt identities before resuming.
All12 cancellation cells, completed/pending browser observations and full acceptance
remain unexecuted. Earlier guard, cache and memory failures remain unresolved. No
consumer, production handler, installed browser or guard policy changed for this work.
