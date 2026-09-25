# Static-content starter acceptance

This fixture tests a complete Gallery reading task without JavaScript. It does not
certify the catalog, prove a deployed static website, or promote a public page API.
The app-owned composition uses existing documentation/navigation/callout primitives;
its routes and content policy are not a new site generator.

## Prospective task contract

At 390 and 1440 CSS pixels, light/dark and JavaScript disabled, read the guide,
follow its outline, open independent FAQ disclosures, follow a real local reading
destination, and return with native browser navigation. Exercise explicitly synthetic
empty and unavailable states plus an unknown destination. Empty is not an error;
unavailable is not empty; a missing destination is a 404 with a useful return path.
All routes retain theme context. Invalid inputs fail before partial content renders.

Inspect visible keyboard focus, 44px standalone controls, readable long labels,
single main landmark, heading/outline identity and narrow wrapping. No external
requests, client code, web fonts, account data, persistence or source library is needed.
Any test-only fixture operation must remain distinguishable from product behavior.

`contract.cue` owns the local numeric screening limits. Export it before acquisition:

```sh
go tool cue export docs/assessments/fixtures/static-content/contract.cue --out json
```

Each document navigation has its own resource/timing observation. Account for every
delivered response, require actual body bytes, and fail on missing timings or request
denials. Cache is disabled; numeric results are local samples, not a complete cold,
warm, degraded-network, input-latency or memory gate. Task failure cannot be hidden by
fast timings. Preserve failed receipts and numeric ceilings through repairs.

Use the already installed Chrome and isolated Playwright harness, never a signed-in
profile. The runner must freeze its executed code, contract, exact binary/CSS and
configuration before launch, bind served CSS to those bytes, retain all failures and
captures, and prove owned process shutdown and listener refusal. Existing previews,
consumer repositories, installed binaries and dependencies remain unchanged.

## Reproduce or resume locally

The checked-in scripts are private test tooling for this Mac, not portable consumer
commands. `prepare.mjs` pins the local Go executable and module-cache location;
`probe.mjs` checks the approved Playwright entry hash and installed Chrome path.
Review those paths before running on another host; do not silently substitute tools.

1. Run `node prepare.mjs EXISTING_PRIVATE_PARENT EXISTING_GO_CACHE` from this fixture
   directory, with both arguments absolute. It prints a new private evidence root.
   Require `prepare-receipt.json` to report `passed: true`; preserve failed roots.
2. Run `node --test probe_test.mjs` before acquiring browser evidence.
3. Set `APPKIT_PLAYWRIGHT_MODULE` to the reviewed absolute module entry. Run
   `node probe.mjs EVIDENCE_PARENT BINARY CSS CONTRACT_JSON`, with absolute paths to
   the prepared `gallery`, `app.css` and `contract.json`. The runner retains a new
   exclusive snapshot of its executed inputs and writes `receipt.json` there.
4. Inspect every failure, observation and capture, plus process exits and listener
   refusal. A zero exit does not replace visual review or the separate delivery gate.

For a harness-only repair, reuse the exact prepared binary/CSS/contract. Compare
identities with the failed run and retain both versions; do not rebuild unrelated
application changes into a supposed replay. `select_diagnostic.mjs` is separate
native-input diagnosis, never an acceptance substitute. Its first trace established
premature Enter submission; it did not establish a general browser limitation.
Freeze intermediate Go regression-test bytes before repairing them as well. A
failure log plus hash is diagnostic-only when the executed test revision is missing.

## Separate delivery requirement

Pure deterministic rendering and request-time browser proof do not establish a
deployed, server-free directory. A test-only render/export replay may demonstrate
that boundary without introducing a permanent exporter API. Static-host routes,
metadata, theme URLs, recovery/status behavior and complete asset closure need their
own proof before claiming the full static-web delivery profile. This fixture's
initial Gallery proof is `go-web` and makes no broader claim.
The separate [plain-file export assessment](../../2026-09-16-static-export.md) now
provides bounded generic-static-host evidence; it does not retroactively widen this
Gallery fixture's result or certify a deployed consumer.
