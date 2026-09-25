# Analytics usability comparison protocol

Status: [runnable pair and automated task checks pass](../../2026-09-16-analytics-comparison-pair.md);
human results remain pending. Recheck the recorded artifact identities before a trial.
Both versions share the same synthetic dataset and task semantics. The older private packet
`data/design-assets/gallery-analytics-quality.PWc0oB/manifest.json` preserves selected
before/after materials and receipts, but the assessment explicitly does not claim a
full source, binary, or dependency closure. Original failures remain evidence. See the
[retained quality assessment](../../2026-09-14-gallery-analytics-quality.md) and
[journey contract](../../../design/analytics-journey.md).

Use neutral artifact labels A/B, a fresh browser context and the same starting route,
viewport/theme and dataset for each run. Serve synthetic loopback routes only. Complete
both steps on each artifact; do not compare different steps as if they were the same task.

## Artifact preparation

The retained packet contains no runnable historical A/B pair. Prepare a controlled
**reconstructed-composition** comparison, not a claimed replay of the historical app:
freeze one current Gallery runtime/data/shared-shell source closure, then create two
independent copies. In A only, substitute the packet's
`selected-source/before/apps/gallery/analytics_templates.go`; B uses the current
template. Current data-view fields cover the retained template without an API adapter.
Rebuild each CSS output from its own sources with the same pinned compiler and scan
configuration; do not overlay historical compiled CSS onto the current shared shell.

Before recruitment, retain both complete source/configuration closures, build commands,
tool identities, binaries and generated CSS. Verify the moderator answer key on both
actual outputs, including return state and exported coverage semantics. Retain A's known
control-size/quality failures; do not weaken the improved target's quality oracle to
make the baseline pass. Label the experiment's scope: prior versus current analytics
composition under one contemporary runtime, not the entire historical product upgrade.
If the old template cannot compile or preserve the shared task semantics, stop artifact
preparation and review the mismatch; do not silently redesign A. The linked assessment
records executed preparation separately from pending human usability results.

### Reuse or replay the prepared pair

The private pair is `/Volumes/m2ext/appkit-evidence/appkit-analytics-pair.u19A9Z/prepared`.
`prepare.mjs <absolute-config.json> <fresh-output>` prepares a new pair, using explicit
tool/cache paths, pinned template/compiler/preparer hashes, and a current first-party
snapshot. Reusing the retained pair avoids another build and preserves this experiment.
Changing its source requires new preparation and verification; do not edit it in place.

Run the automated fixed task with the approved isolated harness and a fresh output:

```sh
node docs/assessments/fixtures/analytics-usability/run-pair.mjs \
  /Volumes/m2ext/appkit-evidence/appkit-analytics-pair.u19A9Z/prepared \
  /Users/geoff/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs \
  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' \
  /Volumes/m2ext/appkit-evidence/appkit-analytics-pair.u19A9Z/new-browser-run
```

The runner refuses an existing output directory, verifies binary/CSS hashes, freezes
the executed scripts/configuration, starts loopback servers and closes them afterward.
It requires Playwright 1.62.1; it does not install dependencies or use a signed-in profile.
For exact historical replay, use the final assessment's frozen `run-pair.mjs` and its
sibling `verify-pair.mjs`, not potentially changed maintained scripts.

For a moderated trial, first verify identities against `prepared/receipt.json`, then
start `prepared/A/gallery` and `prepared/B/gallery` in separate terminals with
`-addr 127.0.0.1:0 -design-assets ''` and `APPKIT_DESIGN_ASSETS=''`. Each prints its
own loopback origin. Start at `/analytics?theme=light` in a fresh isolated context with
the same fixed viewport for both. Do not navigate unrelated Gallery pages. Stop both
processes with Ctrl+C afterward and check port refusal. These staging instructions
are not evidence that a moderated trial or its cleanup has occurred.

## Participant prompts

1. Select August, API services, no comparison and partial data. What counts are shown
   for Gateway and Search, and what can you conclude from them?
2. Inspect Gateway's detail, return to your selection, then export it. Does the export
   contain the same selected services and values? Show the evidence for your answer.

## Moderator answer key (do not show before the trial)

The selected fixture has Gateway4 and Search2. Both are partial/lower-bound counts,
not complete-month totals or evidence of reliability rates/causes. Returning from
Gateway retains all four selections. The overview CSV contains two current-series
records with those service/value pairs and their partial coverage context; it must
not be the one-service detail export. Recheck these expectations against both pinned
artifacts before recruitment; a mismatch is a preparation failure, not a user error.

## Observation and limits

Start timing when the task card is revealed; stop when the participant gives the final
answer and identifies the supporting page or CSV. Record manual elapsed seconds and
each wrong selection, value, interpretation, detail, return-context, or export action.
Record assistance verbatim; assistance does not erase errors. Mark abandonment with
elapsed time and last completed step.

Counterbalance A/B order across participants when possible. With one participant,
record the fixed order and learning/carry-over limitation; do not call it counterbalanced.
Record participant/trial count, order, artifact
identities, browser, and environment. An N=1 observation is descriptive only: make no
population, accessibility, statistical-equivalence, or speedup claim. No post-hoc pass
threshold is permitted.
