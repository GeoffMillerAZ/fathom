# RAC Settings native browser probe

Bounded synthetic-only consumer journey; see the [assessment](../../2026-09-14-resume-settings-native.md).
No app dependency, installed browser profile, provider or live credential store is used.
The existing reviewed Playwright entry is supplied explicitly, not downloaded by this probe.

From the RAC checkout, create a fresh empty system-temporary directory and start:

```sh
probe_meta_dir=$(mktemp -d -t appkit-settings-meta)
APPKIT_SETTINGS_PROBE_DIR="$probe_meta_dir" devbox run -- go test ./cmd/serve -run '^TestSettingsBrowserProbe$' -count=1 -v
```

The helper prints its exact bound loopback origin and writes `server.json`. It uses
temporary Google/discovery/registry fixtures and the real `guard(s.Handler(), boundAddr)`;
an outer allowlist rejects every route except Settings/assets and its credential POST.
Do not substitute ordinary `racli serve`. Lifetime is five minutes; SIGINT closes it.
No auth/profile/secret is required from the operator.

In another terminal, from app-kit, pass that exact origin, a fresh empty system-temp
output directory, the reviewed absolute Playwright `index.mjs`, and the RAC checkout:

```sh
probe_output_dir=$(mktemp -d -t appkit-settings-browser)
node docs/assessments/fixtures/resume-settings/browser.mjs \
  http://127.0.0.1:PORT "$probe_output_dir" /ABS/PLAYWRIGHT/index.mjs /ABS/resume_as_code
```

The script launches installed Chrome in fresh contexts, rejects unexpected origins/
routes, submits generated synthetic values, exercises JS-on/off at 390/768/1440px,
and checks invalid/retry/save/stale paths. Cookies, CSRF and POST bodies are not
recorded. Screenshots contain synthetic client IDs, never actual credentials.
The current exact oracle has 288 repeated checks, not 288 independent requirements:
the original 165 credential/save/recovery checks remain unchanged, and 104 additive
cross-state checks cover the exact eight app-owned routes, Settings current state,
shared skip/main identity, narrow overflow, and native Menu/Settings focus traversal.
Fourteen CSS-readiness checks and five negative-control checks complete that total.
RAC intentionally keeps one Synthwave identity across system color preferences.
Node-side bounded focus polling works even when document scripts are disabled;
native Tab/Enter follows the error link without scripted focus repair.

Before any page task, `browser-process.json` exclusively records the one CDP-reported
browser PID, its shell-free `ps -ww` command line, canonical temporary user-data
directory, PID-equal process group and exact `lstart,command` identity. The file is
mode 0600, contains no submitted values and lets the private parent runner distinguish
normal Chrome exit from an exact still-owned leftover; the probe itself kills nothing.
The receipt records that same browser's version, request/status counts, selected
source/tool hashes, diagnostics and capture hashes. This is selected evidence, not
full dependency/binary provenance.

The zero-horizontal-overflow oracle is not relaxed. `shellReadiness` waits at most
2,500ms for document load, exact active/readable `app.css`, and computed shell flex
layout before measuring the unchanged 0px threshold. The `nojs-390-invalid` state and
any observed overflow retain an exclusive geometry JSON file and screenshot. The
diagnostic records the original measurement first, then readiness, stylesheet/rule
availability, active element, viewport/body/document widths, bounded offending-element
geometry, a fixed 100ms observation and the first native Tab result. It records no
submitted values, and later settling cannot replace an earlier failing result.

Private attempt `MDdP6V` stopped before UI work because installed Chrome refused
`Browser.getBrowserCommandLine` without `--enable-automation`; `emQdz2` retained the
original 268/269 result and 125px post-reload observation. `IMWK5x` identified its
cause without relabeling either failure: at11.4ms CSS was absent, the document was
interactive, shell flex rendered block and overflow was125px; at87.1ms, before input,
CSS had49 rules, the document was complete and overflow was0px. Final frozen run
`/Volumes/m2ext/appkit-evidence/appkit-rac-shell-state.sX5bLw` passes288/288. Its browser receipt is
`6367a3bd33a01239b164f02ad6ce6b5cc39bae662b9498737d437927c7513261`; run report is
`8cdcc727302763fdbd34a8fe13efb94223445af96d511f60763117c96888f4da`.
Test-only missing-CSS and deliberate
128px styled-overflow controls prove refusal. No app code changed. Canonical joint
scope and limits remain in the [G2 shell acceptance matrix](../../2026-09-16-shell-acceptance-matrix.md).

Rebuild CSS before launching the helper, freeze source/CSS through the run, and
compare receipt hashes to current files. Never relabel a prior run as proving new bytes.
Under I17, acceptance also requires the private runner to retain the exact executed
harness, runner, configuration and selected consumer bytes before execution and verify
them afterward; command transcripts or hashes without those bytes are diagnostic only.
Do not run build commands inside private evidence packets under `data/design-assets`.
