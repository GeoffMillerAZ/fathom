# Gallery quality-work browser probe

This is bounded, local synthetic Chrome proof for Gallery's `/quality` list and
`/quality/{id}` detail. It is not accessibility certification, browser parity,
consumer acceptance, readiness scoring, gate execution, or permission to promote a
component. It uses authored queue data and follows no evidence paths.

The runner owns the Gallery process on `127.0.0.1:0`, disables design assets,
uses a fresh Playwright context with JavaScript disabled, reduced motion and blocked
service workers, and admits only exact loopback GETs for quality HTML and
`/static/app.css`. It does not install dependencies or use a signed-in profile.

First build the exact candidate and its CSS. Do not run until their paths are approved:

```sh
APPKIT_PLAYWRIGHT_MODULE=/Users/geoff/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs \
  node docs/assessments/fixtures/gallery-quality/probe.mjs \
  /absolute/private/evidence-parent \
  /absolute/private/gallery-binary \
  /absolute/private/app.css
```

The probe checks core/all/on-demand scope, an exact capability match, a known empty
selection, and unknown/stale/duplicate-parameter reset paths. Native Tab/Enter tasks
apply a filter, open detail, return with filter/theme intact, and open one evidence
disclosure. Detail must expose seven standards and nine gates with statuses, reasons
where authored, and next steps, without a readiness score or certification promotion.

At 320 and 1440 CSS pixels in light and dark, it checks horizontal overflow and every
visible standalone main-content target's own box (at least 44 by 44 CSS pixels). The
320-pixel lanes additionally render the complete ID list plus the long client-heavy
and on-demand-candidate details. It retains eight list/detail captures, every visited
HTML response, exact served CSS,
the binary, frozen harness, run config, logs, request ledger, Chrome version, hashes,
and all failed outcomes. Preserve failed evidence directories; never weaken the oracle
to make a candidate pass. A separate visual review should inspect two to four
captures and report scoped issues without making a blanket accessibility claim.

Preparation-only validation (does not launch Gallery or Chrome):

```sh
node --check docs/assessments/fixtures/gallery-quality/probe.mjs
```
