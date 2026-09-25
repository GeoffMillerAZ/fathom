# Actual-handler recovery probe

Separate opt-in G2 acceptance fixture. The original two-scenario probe remains
unchanged. This probe tests the current consumer, not a renderer/reader candidate.

```sh
APPKIT_FATHOM_RECOVERY_EVIDENCE_ROOT=/Volumes/m2ext \
APPKIT_FATHOM_HANDLER_CACHE=/absolute/approved/existing/private/go-cache \
node docs/assessments/fixtures/fathom-handler/recovery_run.mjs \
  /absolute/approved/fathom-checkout \
  /absolute/approved/playwright/index.mjs
```

Both paths must identify existing approved inputs. Nothing is installed. The cache
variable is optional; without it a private compiler cache is created under the
evidence directory. Evidence defaults to the system temporary directory; choosing
an existing external-volume parent also keeps process-local Go/Chrome scratch there.

The preparer retains all local harness/test/configuration bytes and selected consumer
source before invoking the retained runner. Each attempt gets a new private directory;
never overwrite an earlier failed attempt. The runner uses offline modules,
`GOWORK=off` and read-only module resolution. Only two absent test files are overlaid:
the unchanged original helper source and the new recovery test. Only the new test runs.
Selected source/HEAD/dirty-inventory checks are not a complete build-closure proof.
Node tests refuse direct unfrozen invocation. The runner first proves that refusal,
then supplies the retained root and checks test/browser/contract hashes before tests.

The frozen contract and SHA-bound three-row JSONL declare the exact long service name,
three all-time calls and two agent calls. An existing owned directory makes log reads
fail without selecting the consumer's missing-path fallback. After actual keyboard
journeys, the fixture observes a freshly filled cache, swaps only its owned input, and
requires immediate stale data followed by exact recovered data after the native30s TTL.
The swap holds the existing cache mutex only across the renames; it never resets the
cache or changes its clock, model, expiry, handlers, source, or browser contexts.

Control routes are test-only capabilities: exact loopback origin, token, methods,
paths, content type and bounded body; one transition only. They are not product APIs
or UI recovery actions. Ordinary user journeys use native links, disclosure and keys.
Four independent390/1440×1000 light/dark contexts disable page JavaScript and admit
only the owned landing/fleet/service/CSS routes. Twenty phase captures are required
for a pass; failed acquisitions retain whatever observations/captures were reached.

Receipts retain refusal tests, command exits, browser output, exact asset identities,
source stability and owned listener cleanup. Fixed timestamps do not prove freshness
or rolling-window values. No live daemon, real logs, consumer edits, dependency change,
deployment, global configuration, full accessibility or full G2 completion is implied.

Current result and gaps: [shell acceptance assessment](../../2026-09-16-fathom-shell-acceptance.md).
