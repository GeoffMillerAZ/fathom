# Synthetic real-handler browser acceptance

Declared before execution, 2026-09-21. This is a bounded continuation of the
[Status-read repair](README.md), not a consumer migration or G3/G4 completion.

## Fixed boundary and oracle

- Real Conjure HTTP handler, compiled templ, embedded CSS and Datastar; fake
  Status/Doctor/Runs/snapshot/source/clock dependencies. Random loopback origin,
  synthetic token, read-only route allowlist. No operational CLI, provider, engine,
  real HOME, installed binary, dependency, vendor, daemon or global changes.
- Fresh Chrome profile via existing isolated Playwright; no signed-in profile,
  external pages, new product dependencies or mocked browser DOM/timers.
- Two lanes: 1440x1000 light and 390x1000 dark, reduced motion. Actual five-second
  polling must move failed → three clean units → failed/stale → five clean units
  → valid empty → orphan. A second connection during failure must start unknown.
- First failure must not claim empty/converged; hostile detail stays text and
  wraps. Stale retains all six prior data fragments with a last-good timestamp;
  patch roots remain unique. Recovery clears the error. Orphan is not converged.
- Check stylesheet load, explicit theme, viewport overflow, one reachable keyboard
  control, script errors, requests outside the origin and stream cleanup. Capture
  first-failure/stale/recovered screenshots; inspect actual images before a visual
  claim. This is not a full accessibility audit or screen-reader evidence.
- Browser process and fixture stop on success/failure; fixture lifetime cap 150s.
  Keep exact source, fixture, browser harness, binary and embedded-asset identities.
  Keep failed runs. No human A/B, performance-budget or whole-dashboard pass inferred.

Budget: one initial two-lane run, then at most one correction replay for each
directly demonstrated in-scope defect. Do not expand to a theme/viewport matrix or
upgrade shared dependencies to make this pass. Stop and hand off unrelated findings.

## Invocation

The opt-in `TestDashboardBrowserFixture` is preserved separately from the historical
13-file repair patch. Build `./internal/webui` with `go test -c -o <private-binary>`
in the reviewed Conjure checkout. Launch with `APPKIT_CONJURE_BROWSER=1` and
`-test.run '^TestDashboardBrowserFixture$' -test.v -test.timeout 3m`.
The test prints `APPKIT_FIXTURE_URL=http://127.0.0.1:<random>`.

Run `node browser-smoke.mjs <playwright-module> <chrome-binary> <printed-origin>
<new-absolute-output-directory>`. The script refuses an existing evidence directory,
writes JSON/screenshots and stops only its own fixture. It requires no package install.
Retained results and exact paths belong in the assessment, not an unqualified pass
claim in a project plan.
