# Actual Fathom read-only handler probe

Opt-in companion to the exported navigation/analytics fixtures. This mounts the
consumer's real `landingHandler`, `observatoryHandler`, `observatoryServicePage` and
`assetsHandler` on a temporary loopback server. It does not start the operational
daemon, RPC server, database, provider or mutation routes.

The test belongs to app-kit as `actual_handler_probe_test.go.txt`. `run.mjs` generates
a Go build overlay for a previously absent `_test.go` path in the explicitly supplied
Fathom checkout, then proves Go discovered it before execution. No file is added to
that checkout. `-mod=readonly`, disabled proxy/workspace/toolchain downloads and a private
compiler cache keep this offline; prerequisite failures are not passes. Optional
`APPKIT_FATHOM_HANDLER_CACHE` selects an existing private real directory for reuse;
the report names it and explicitly makes no cold-build claim.

## Run

Use only an owner-approved consumer checkout, existing isolated Playwright installation
and installed Chrome. From the app-kit root:

```sh
node docs/assessments/fixtures/fathom-handler/run.mjs \
  /absolute/approved/fathom-checkout \
  /absolute/approved/node_modules/playwright/index.mjs
```

The wrapper prints a new system-temporary evidence directory. It checks canonical Git
root/module identity and refuses to shadow an existing probe file. Reports bind selected
source/CSS/module/mirror hashes, fixture hashes, Git HEAD and dirty inventory before/after.
This is current-working-tree evidence, not an immutable build closure, release pin,
clean checkout or proof that concurrent unrelated files never changed.

### Unapplied reader candidate

The [candidate patch](call-log-read-errors.patch) and [five reader regressions](call_log_read_errors_test.go.txt)
address scanner-error classification only. Preparing a patch or running this mode is
not permission to apply it to Fathom. Prepare the source in an isolated temporary copy,
never in the consumer checkout. The expected base SHA256 is
`e56cf78ddc16160a0536e245b26f1d5c063a2b2ec0749764d4fb9986228ef197`;
the prepared candidate must be
`20052f0bf5cf364aaaf36f3d05a19f09295206cff8dbb4f467fd82ad93ec0292`.

```sh
node docs/assessments/fixtures/fathom-handler/run.mjs \
  /absolute/approved/fathom-checkout \
  /absolute/approved/node_modules/playwright/index.mjs \
  --reader-candidate /absolute/temporary/instrument_adoption.go
node --test docs/assessments/fixtures/fathom-handler/run_test.mjs
```

Candidate mode refuses different source/base bytes, copies the accepted candidate
into its private evidence directory and overlays that snapshot for compilation only.
Both virtual tests must be discovered, run and pass. Its receipt explicitly says
`unapplied-reader-candidate`, binds both real and overlaid inputs and checks the snapshot
and absent virtual files afterward. Eight synthetic refusal tests cover arguments,
tampering, symlinks, oversized candidates and existing probe files; they do not prove
every filesystem race or failure mode. This is not a general-purpose overlay facility.

[Retained evidence](../../2026-09-15-fathom-handler-read-failure.md) separates the failing
original reader from candidate unit and actual-handler/browser results. Freshness and
clipped chart/error content were separate defects; the reader probe cannot prove them.

## What runs

- Registry unavailable: exact synthetic list-services503, named degradation, no invented
  service link, unknown service404 and still-usable shared home/fleet navigation.
- Pathless service: alpha with one indexed symbol, unavailable corpus/configuration and
  unreadable call log, with actual home → fleet → service → fleet → home navigation.
- Real route HTTP status/content/body bounds, POST405 and unknown route/service404.
- Chrome light/dark at390/1440 ×1000, native skip focus, actual Tab/Enter, mobile Menu,
  current route, visible focus,44px navigational targets, overflow and named missingness.
- Explicit unknown/unavailable aggregate freshness; pathless grade-SVG/card bounds and
  unreadable-log text within its own card/clipping ancestors. Quality failures do not
  short-circuit the existing journeys/captures, but still fail the browser process.

The original consumer failed these added quality checks. The [retained red result](../../2026-09-15-fathom-handler-read-failure.md#freshness-and-local-containment-refusal)
records fourteen failures across eight completed journeys, not lost browser access.
The earlier navigation-only green receipt does not satisfy this expanded oracle.

### Unapplied display candidate

The [two-file patch](display-quality.patch) and [synthetic regressions](display_quality_test.go.txt)
fix coverage-aware freshness, diagnostic wrapping/alignment and compact grade geometry.
The [assessment](../../2026-09-15-fathom-display-candidate.md) records exact base/candidate
identities before authorization. For historical candidate replay, prepare the frozen
CSS source closure, apply the patch there, and run its unchanged `scripts/ui-css.sh`.
Generated CSS/theme must remain byte-identical; the runner freezes those identities.

```sh
node docs/assessments/fixtures/fathom-handler/run.mjs \
  /absolute/approved/fathom-checkout \
  /absolute/approved/node_modules/playwright/index.mjs \
  --display-candidate /absolute/private/prepared-tree
```

`--display-baseline` with the same arguments runs the new unit regressions against the
frozen original consumer and is expected RED, not a post-adoption check. Candidate mode
overlays snapshots of just the reviewed two sources/CSS plus virtual test files; it
does not apply the patch. Both modes retain the full local harness/config closure
before execution and compare it afterward. Current browser proof additionally checks
unscaled/in-bounds grade text, left-aligned diagnostic prose and320px resize/restore
in both themes, with two extra hashed captures per scenario. Original four journeys
and assertions remain intact. A final candidate pass is not adoption or authorization.

### Adopted display repair

[September16 adoption](../../2026-09-16-fathom-display-adoption.md) applies the exact
two renderer changes and durable regressions. Use the ordinary two-argument runner
against that checkout: `current-consumer` mode overlays only the absent handler test,
not production source. Run the consumer's durable display and Observatory tests
separately. Do not use candidate/baseline modes against adopted sources: their original
base identity checks must refuse. The adoption receipt retains both proof sets and
the unchanged generated CSS check; historical RED/candidate receipts remain intact.

The injected arm/narrate/session-ledger paths live in a fresh temporary directory.
`FATHOM_CALL_LOG` names an existing empty directory: its read fails as unreadable,
while its successful stat prevents fallback discovery of the user's real logs.
An HTTP transport allowlist permits only the two owned loopback servers; the fake
registry accepts only the exact list-services/tools-list/status operations consumed by
the real model. Browser routing permits only the selected read-only pages and stylesheet.
No synthetic prebuilt model or HTML replaces a real handler's output.

Reports and screenshots are new private local artifacts, not permanent backups. Go and
browser failures remain failures. Test execution must finish and receipts be inspected
before any completion claim; the presence of this harness proves nothing by itself.

## Limits

No full `newHTTPHandler`/serve-http middleware, authentication, wiring actions, credentials,
operational fleet, backend recovery transition, human task timing, physical keyboard,
assistive technology, native-host or full accessibility claim. Other navigation routes
remain visible but are not activated by this bounded fixture. Registry cache use here
does not prove invalidation or live outage recovery. Consumer UI/business source and
dependencies remain unchanged; no publish, deployment or global configuration operation.
