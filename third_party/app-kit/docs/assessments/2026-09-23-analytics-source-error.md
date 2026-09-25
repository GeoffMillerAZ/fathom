# Analytics source failure and recovery

## Selected task and prospective acceptance

R1/R4 owner-funded core maintenance, baseline `ac37d04`. The Gallery can show
complete, missing, stale, partial and valid-empty fixtures but rejects `state=error`
as an invalid filter. Add a distinct app-owned source-failure demonstration under
I4/I7/I11/I12/I15/I17 and ADR-0069's missing-critical-task revisit condition.
The existing analytics experience contract already requires this distinction;
no new shared API/state enum, engine, dependency or consumer migration is proposed.

Accept `state=error` in the Gallery query and existing source-condition controls.
The journey builder must refuse without a dataset. Overview, detail and every
comparison/recipe CSV route must intercept the failed selection before using a
builder or rendering/encoding data, even if a test builder would return poisoned
populated data. Return503 HTML without an attachment; GET/HEAD keep their normal
security headers and HEAD has no body. Invalid queries, paths and methods retain
their400/404/405 precedence. The separate signed fixture is unchanged.

Show a readable synthetic source-failure explanation, not zero, missing observations
or a successful empty result. No totals, differences, charts, numeric tables, detail
or export links. Preserve reporting month, comparison, group and theme as context.
Native “Load the complete fixture” recovery changes only state; recipe comparison
remains its existing canonical `none`. Source failure takes precedence over an empty
group; after recovery that group still shows its valid-empty result. App data and
authorization remain app-owned; no live service, save or network source is added.

Freeze independently authored tests and run against the retained baseline and candidate,
then Gallery/analytics regressions and scoped vet. Implementation and tests are authored
in parallel; the archived baseline stays unchanged. HTTP tests cover all groups and export formats, source
builder refusal, recovery, unchanged measured-zero/header-only controls and hostile
query refusal. Existing count and CSV agreement oracles remain unchanged.

Browser budget: one candidate acquisition, two fixed lanes: 1440×1000/light and
390×844/dark, on comparison and recipe surfaces. Use source-collected real handler
responses/CSS, no application scripts, native Tab/Enter recovery and browser Back,
exact complete responses, no unexpected requests/downloads, visible focus, no page
overflow and inspected captures. Direct CSV-error navigation must stay an HTML error,
not download successful data. HTML budget remains 32 KiB. At most one diagnosed harness
correction with the original preserved; no dropdown-key permutations or larger matrix.

Private evidence: `/Volumes/m2ext/appkit-evidence/analytics-source-error-2026-09-24/`.
The earlier prospect packet remains under `appkit-evidence/appkit-analytics-source-error.8phHtZ8D/`.
No consumer, dependency, installed configuration, service, release or live-data change.
This task alone does not close G3/G4/core, human A/B or full accessibility/performance.

## Results

Baseline RED: the archived source rejects `state=error` as HTTP 400, and the recipe
route returns HTTP 400 without reaching its poisoned builder. Candidate focused routes
and recovery tests pass. The isolated candidate passes `go test ./apps/gallery
./appkit/ui/analytics ./internal/qualityplan -count=1`. Scoped `go vet` exited successfully
once; later attempts hit macOS `Operation not permitted` while reading the module-cache
ziphash and dependency sources. A direct mounted-worktree package run was separately blocked reading
`data/design-assets/go.mod`; replay in the private local source copy passed all three
packages.

One bounded browser acquisition passed 146 checks across four overview/recipe × wide/light
and narrow/dark journeys. Exact handler HTML/CSS and completed responses matched; native
Tab/Enter recovery and Back passed; no unexpected requests or downloads occurred; Chrome
exit and output drains were observed; input hashes were unchanged. Captures were visually
reviewed. The first harness run found only a category-label expectation mismatch; its
146-check failed report and original harness are retained beside the corrected passing
run. Browser candidate archive SHA-256:
`93d8ea4d33d0cad571716864871ee4b448c4f4e970264b979c5aeea33bb2a1b4`. The final
verified source archive and its checksum are retained in the private evidence folder.

Evidence is synthetic Gallery behavior, not production-data correctness, consumer adoption,
human analytics effectiveness, or complete G3/core/accessibility/performance acceptance.
