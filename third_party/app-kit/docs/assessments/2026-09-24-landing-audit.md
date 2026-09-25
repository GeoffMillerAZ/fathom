# 2026-09-24 landing and workspace audit

This closes the authorized cleanup/landing pass without treating deferred quality
gates as passed. No consumer, service, dependency, global agent configuration or
deployment was changed.

## Landed on main

`origin/main` and local `main` both identify `1a01826c8658f2f27fa83fb1210bcf3586273224`.
The evidence-packet organization and truthful analytics-source-error Gallery fixture
are committed and pushed. Pre-push decision-index, Fathom verification and
quality-evidence hooks passed. The analytics error fixture also has the bounded
synthetic/browser evidence and explicit limits in
[`2026-09-23-analytics-source-error.md`](2026-09-23-analytics-source-error.md).

The external-volume root was cluttered with 241 `appkit-*` task folders (about
58.3 GiB). They were moved within `/Volumes/m2ext` under
`/Volumes/m2ext/appkit-evidence/`; 241 destinations and 178 literal evidence links
were checked, and no `appkit-*` task folder remains at the drive root. The task
receipts remain outside Git. The tracked documentation reference and CUE receipt pin
were updated and verified.

## Branch disposition

- `appkit-stat-font-radius-delivery-20260914` remains pushed at
  `bcfeb8e94ce4132e1198a69f4cf10dc6764bd353`. The stat reflow/wrapping and
  runtime font/radius behavior from that delivery are already represented in `main`.
  The only reviewed tree delta on those files is a light positive-token adjustment
  back to 0.55; current main intentionally keeps the later contrast-safe value. Do
  not merge the branch over main. Its original evidence remains in
  [`2026-09-14-stat-reflow.md`](2026-09-14-stat-reflow.md).
- `archive/stale-templ-artifacts-20260918` remains pushed at
  `4a17c7d1bf57727cc2964a930ea67ccb76ae0c87`. Its tip has 16 commits not in main,
  but its tree predates 156 later main commits and would remove over 259,000 tracked
  lines if merged as a branch. Component families such as table, kanban, callout,
  chart primitives and the client view router are present in the current tree in
  their subsequently evolved form. Preserve this branch as historical source only;
  any uniquely needed capability should be re-evaluated and ported just in time
  against current contracts, never wholesale merged.
- `fix/viz-bars-comma-grouped` is already an ancestor of main; it has no unlanded
  change.

Only `main` is checked out. The pre-existing `stash@{0}` containing stale templ
artifacts is preserved. No branches, stashes, or evidence packets were deleted.

## Limits and next use

The main worktree is clean and local main equals origin/main. Historical branch tips
remain available for provenance; their presence is not an outstanding merge task.
This audit does not certify all catalog entries, close G1-G6, establish human
analytics usability, or replace the demand-driven continuation contract. A future
consumer need should select and verify only the relevant capability and gates.
