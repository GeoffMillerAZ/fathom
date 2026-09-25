# Isolated Resume-as-Code analytics browser probe

[Receipt and limits](../../2026-09-14-resume-pipeline-uplift.md).
This is an optional local verification tool, not a RAC runtime/build dependency.
Use the already-approved isolated Playwright installation and fresh Chrome context;
do not install dependencies, use the signed-in profile, or launch normal RAC services.

## Inputs

Create distinct empty children of the system temporary directory for before/after
exports and the report. RAC's `TestExportAppkitPreview` emits five fictional states
(`empty`, `populated`, `outcomes`, `incomplete`, `terminal`), each with Today, Pipeline,
People and embedded CSS. It isolates all stores and strips scripts/forms.

From RAC, with `APPKIT_PREVIEW_DIR` set to an explicit empty system-temp child:

```sh
devbox run -- go test ./cmd/serve -run '^TestExportAppkitPreview$' -count=1 -v
```

For a before/after comparison, use a reviewed Go overlay for the baseline source/CSS;
do not overwrite the worktree. A before export is an observed baseline, not a passing
target. The private receipt packet includes this run's actual exports and overlay.

From app-kit:

```sh
node docs/assessments/fixtures/resume-pipeline/browser.mjs /absolute/before-export /absolute/after-export /absolute/empty-report
```

`APPKIT_PLAYWRIGHT_ENTRY` can identify an already-authorized Playwright module;
the default is the isolated `/tmp/appkit-browser-review.xBrDCp` installation.
Chrome comes from `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`.

## Contract

- Reads only explicit exported files into an exact GET/HEAD route map; no directory server.
- Loopback/Host checks, no-cache, restrictive CSP, no form submission, workers or downloads.
- Browser request/WS guards reject off-map traffic; unexpected errors fail the run.
- Keyboard operates native disclosures only, never application actions or forms.
- Tests 320/768/1440px and both system preferences; this is one retained Synthwave
  palette, not proof of two distinct themes.
- Geometry includes actual SVG label and table-cell bounds. Page overflow alone is insufficient.
- Writes `receipt.json` and screenshots to an empty temporary output; closes Chrome/servers.
- Exit 1 means an assertion or operational failure. Do not filter failing cases or label
  skipped exports successful. Receipts distinguish baseline observations and after assertions.

It does not test live data/actions, full accessibility, performance, released dependencies,
an installed native app, or the entire Pipeline/People route. Inspect screenshots as well.
