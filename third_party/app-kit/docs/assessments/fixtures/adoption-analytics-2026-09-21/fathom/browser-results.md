# Fathom guide: retained failed browser task

This is synthetic local evidence, not active adoption or a browser pass. No signed-in
profile, portal, live logs or deployed daemon was used. Existing Gallery/node listeners
were left alone; only the private guide on `127.0.0.1:18891` belonged to this probe.

## Subject

- Source/normal-manifest candidate: [review packet](README.md), exact app-kit `f7ab0fb`.
- Final probe binary SHA-256: `3bd8187a5c4181eb86d33e8151b17cb7e669faed2a7defaa16c364aa1a00499a`.
- Embedded app CSS: `0e0b5e5408c5e2484f0f2adcdba6a7c9491214d3528051ef7b43300dc52acaa1`.
- Embedded token CSS: `e5b8a4aa803dd81d37bc18414c3415f486283d8e4a03a67d305b43b6fe82c291`.
- Browser: installed Chrome `153.0.8010.53`, fresh Playwright-owned headless context.
- Private captures: `/Volumes/m2ext/code/geoffmilleraz/sextant/workspace/appkit-r5-candidate.7QEdhc/`.

## Acquisitions, preserved separately

| Record | Result |
|---|---|
| [First smoke](browser-smoke-1.json) | Four assertions pass; `getByLabel` selector times out. An independent accessibility snapshot identifies the control as combobox “App specimen”; corrected probe uses that role/name. Baseline screenshot also exposed a synthetic freshness value outside the app's protocol; candidate fixture was corrected to `fresh`, with a canary. |
| [Startup failure](browser-smoke-2.json) | Connection refused before any task assertions. The candidate had not yet printed its listening address. This is a failed acquisition, not a UI pass; readiness is now checked before browser launch. |
| [Corrected ready-server smoke](browser-smoke-3.json) | Thirteen assertions pass through preview, light/dark selection, qualified partial data, a388px preview with388px document width, invalid-token refusal, reset and enabled acknowledged export. The native ZIP event times out after10s; overall **fail**. |
| [Export-only diagnostic](export-diagnostic-1.json) | Valid native form,64-character reviewed digest, preview POST200. Export click completes but no download/alert or export POST response is observed during8s. It is not a task pass or proof of a particular browser defect. |

A second bounded agent probe also checked an enabled submit button associated with
the valid POST form, with no export request/requestfailed/response during its2s
window. Its private script is `export-narrow-probe.mjs`; this is supporting diagnosis,
not an independent successful download. The `form.action` JavaScript property is
shadowed by named action controls; no evidence yet shows that causes native failure.

Screenshots inspected: `browser-smoke-1/baseline-light.png` and
`browser-smoke-3/partial-dark-narrow.png`. The latter shows readable, stacked lower-bound
and unavailable states without document overflow; it does not establish full UI,
screen-reader, zoom, font-fidelity, memory or cancellation acceptance.

## Reproduction

Only against the reviewed private executable and a free loopback port. Start the
synthetic `theme-guide -addr 127.0.0.1:18891` command, confirm its listening message,
then run from app-kit with a **new** evidence directory:

```sh
node docs/assessments/fixtures/adoption-analytics-2026-09-21/fathom/browser-smoke.mjs /Users/geoff/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' http://127.0.0.1:18891 /ABSOLUTE/NEW/EVIDENCE/DIRECTORY
```

`export-diagnostic.mjs` takes the same arguments for the smaller diagnostic. It emits
observations, not a pass verdict. Both use fresh contexts, restrict page requests to
the selected loopback origin, close their browser, and refuse an existing output
directory. Stop only the test-owned guide process afterwards. Do not terminate an
unrelated listener to acquire the port.

Next: capture one decisive native input/submit observation without changing security
policy or the actual production export handler. Preserve these failures and the
HTTP ZIP tests; neither substitutes for the other. No further broad acquisition is
authorized by this receipt alone.

Separate [September22 follow-up](../../../2026-09-22-fathom-native-export.md):
native pointer/Enter export passes on the same frozen executable, with exact ZIP,
event and owned-process cleanup evidence. Different launch transport and the smaller
radius workflow do not establish the old cause or rewrite these failures. Active
integration and full-guide acceptance remain open.
