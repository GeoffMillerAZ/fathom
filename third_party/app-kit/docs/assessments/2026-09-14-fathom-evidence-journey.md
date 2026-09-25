# Fathom: evidence journeys and truthful response cost

G3 follow-up to [observation triage](2026-09-14-fathom-observation-triage.md).
This changes app-owned presentation and synthetic tests, not collection, metric models,
route handlers, dependencies or the paused session's unrelated work.

## User task and defect

From Home, inspect the evidence behind prevention or friction, distinguish an unavailable
source from a measured zero, inspect the separately labelled weekly instruments, and
return with the original value, coverage and theme intact. Weekly ledger totals are
not the same time grain or source as the current fleet snapshot or Impact catch count.
The fixture explicitly says it does not read the production ledger.

Review found that Impact ignored `CallLogNote`: response cost could become `~0 tok`
and `0 B`, or display stale populated totals, ratios and tool rows after their source
failed. Generic absence text hid the actual diagnostic. The existing observation-only
browser checks did not cover these dependent panels. Fathom records this as INC-19.

The fix carries source availability into the cost renderer and per-tool outcomes.
Unknown cost, ratio and rows receive the exact escaped reason; independent schema tax
and trend remain usable. Readable empty logs and measured positive costs stay distinct.
Poisoned nonempty rows cannot override the failed source. No wire/model fields changed.

The task also exposed 17px-high evidence links. Home actions and the session-inspection
action now compose the pinned kit's `form.RenderButton` as native ghost-style anchors.
App-owned direct-anchor styling establishes 44px targets, rather than crediting parent
padding as the link's hit area. Existing routes, labels, themes and native focus remain.
Tailwind Plus empty-state references and existing kit primitives were consulted;
no licensed source body entered Fathom.

## Fixed evidence and results

Ten synthetic scenarios: unknown, partial, complete zero/positive, registry unavailable,
registry partial, call-log unavailable, simultaneous registry/call-log failure, and
measured zero/positive catches. Catch fixtures are exactly 0 or 3 with a named two-session
ledger; the weekly fixture independently contains two sessions, zero catches and one
edit session. Existing vectors and refusal policies remain unchanged.

| Gate | Observed result |
|---|---|
| Behavioral Go regression | RED on stale populated cost/outcome data; GREEN after repair, with empty-source, escaping, independent-schema and readable-zero/positive canaries |
| Browser before | 5,560 checks; **276 failures**: 240 evidence-target failures, 12 missing cost reasons, 12 missing ratio reasons, 12 missing outcome reasons |
| Same browser assertions after | **5,560/5,560 pass**; 240 aggregate page cases, 144 service-detail/return journeys, 12 absent-registry refusals, 120 Home/Impact/weekly/Home journeys |
| Reviewed replay | Same assertions pass, with four extra failure-state captures: 32 screenshots, 2,400 loopback requests, no page errors or unexpected requests |
| Scoped consumer checks | Offline `GOWORK=off` Observatory/Landing/Appkit/CostPanel tests with `-race`, plus package `go vet`, pass |
| CSS generation | Workspace `task ui:css` passes twice with identical output; tokens unchanged, CSS grows 75 B from one generated selector |
| Static selection | Exact previously retained checker `52185ef4…`: 16/16 pass after changing only the CSS artifact hash; catalog/kit/module pins unchanged |

Chrome152.0.7977.84 / Playwright1.63.0, macOS arm64, 320/768/1440 ×1000, light/dark,
DPR1, reduced motion. Keyboard tasks focus their intended control then use real Enter
navigation; visible outline/ring, target bounds, exact destination, values, source text
and return theme are asserted. They are not an exhaustive Tab-order or physical-input test.
The same read-only fixture server refuses arbitrary services, operational routes and
mutation methods. It invokes no production handler, cache, provider or daemon.

Prespecified limits remain HTML163,840 B, CSS102,400 B, DOM2,500 and local
DOMContentLoaded1,500ms. Before/after maxima: HTML17,557→17,675 B, CSS52,776→52,851 B,
DOM194→195; first paired runs both reached approximately44.8ms DCL. Reviewed replay
reached32.4ms. Script-paced task median/p95 were144.3/165.2ms before and138.7/159.0ms after.
These single local samples establish neither human speedup nor production performance.
All 120 baseline journeys navigated, but had target violations; do not call that a
120-person task failure rate. Human task-time/error and update-latency acceptance stay open.

Visual review inspected phone light/combined-failure and dark/call-log-failure Impact.
Named failures and action controls remain readable without page overflow. Broader visual
polish is not complete: the independent schema-tax `0 B` can wrap its unit at phone width.
Full metric contrast, unit/glyph geometry, true200% zoom, screen readers and touch remain
separate acceptance work. These results do not certify the whole app or complete G3.

## Retained identities and replay

All local artifacts below live under `sextant/workspace/tmp/`, ignored and not a backup:

- Before: `appkit-evidence-export-before.k09FqB`, `appkit-evidence-browser-before.MQ2L2h`.
- After: `appkit-evidence-export-after.RLXZ2n`, `appkit-evidence-browser-after.tjrlS5`.
- Reviewed replay: `appkit-evidence-browser-reviewed.PRNNop`.
- Source/CSS/selection backups, build logs and final snapshots: `appkit-impact-repair.yjHgLg`.

Source snapshots use `.go.txt` so retained evidence cannot become accidental Go packages;
the copies match the reviewed source bytes. All148 exported inputs still match the
browser's recorded byte counts and hashes.

| Identity | SHA256 |
|---|---|
| Baseline report | `3301fe718945e2fc6ecd94b15b28819934d367e65f8c46d42cd8b049e1a8cb88` |
| First passing report | `c8d205daeb4483783c5919cc5c81597df8bc4c7a59f472939b61fbf110c1f56a` |
| Reviewed report | `bb0181511867e17fa7418d73e86a0e998e482b17bead6464ea178014d8556303` |
| Reviewed harness | `b0adadfd362be77311d3b4cd59f29479b9f4b6d2851962d2e26ebbcd7e609beb` |
| Renderer | `309c86dd3df035a0a22a3271359498daa9e28837d90cbe4cbbdd26b25f50b34c` |
| Exporter | `d52bd1563aca15203f86828e2703971d23152bb9e3d5904fe71f907aba7715dd` |
| CSS | `2a23e1e9317faee5c028924aa0fa18ce47b54ce8791385821b67abedf248f222` |

Replay `TestExportAppkitObservationPreview` with an empty workspace/tmp destination in
`APPKIT_OBSERVATION_PREVIEW_DIR`, then the [browser harness](fixtures/fathom-observations/browser.mjs)
with that export, a new empty report directory and the approved isolated Playwright module.
The current exporter produces128 HTML +20 CSS files. Do not reuse a nonempty destination.

Standalone CSS generation (`GOWORK=off task ui:css`) fails because Fathom lacks checksum
entries for the kit CLI's indexing dependencies. The workspace generator succeeds;
no `go get`, dependency edit or hidden fallback turns the standalone failure into a pass.
The static receipt uses Fathom's selected older catalog, not current app-kit catalog
compatibility. Tracked-only clean-clone delivery, release and host/production proof remain
open. No commit, push, deployment, global configuration or live-data mutation occurred.

Return to the kit: [experience-system](../design/experience-system.md#analytics-excellence)
now requires source availability to invalidate every dependent projection, with empty
and poisoned-input canaries. Existing intent/agent routers already reach that guidance;
no new MCP server or competing policy was added. G1–G6 remain active.
