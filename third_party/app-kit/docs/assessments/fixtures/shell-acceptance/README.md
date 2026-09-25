# Current Gallery shell replay

Opt-in synthetic acquisition for the [G2 reconciliation](../../2026-09-16-shell-acceptance-matrix.md).
Run only against the authorized checkout with an already-approved Playwright installation,
installed Chrome and an existing private Go compiler cache. Nothing is installed.

```sh
APPKIT_SHELL_ACCEPTANCE_EVIDENCE_ROOT=/Volumes/m2ext \
node docs/assessments/fixtures/shell-acceptance/run.mjs \
  /absolute/app-kit-checkout \
  /absolute/approved/playwright/index.mjs \
  /absolute/private/go-cache
```

Each invocation creates a new private evidence directory and freezes the runner,
configuration, existing browser probes and selected Go/source inputs before execution.
It generates private CSS, overlays only the Gallery's embedded CSS at build time,
builds an isolated Gallery binary and exports the existing settings-section fixture.
The owned ephemeral loopback server has no licensed design-assets directory enabled.
Existing previews, consumer files and installed binaries remain untouched.

The existing `shell_forms.mjs` exercises actual synthetic Gallery routes and saves;
`settings_sections.mjs` separately tests long-text component layout with actual served
CSS. That virtual component route is not an application save/backend. Both reports
and screenshots are retained; failures do not inherit older passing results.

Offline module resolution, bounded child execution, exact binary/generated/served CSS
identity, selected-source stability and owned listener cleanup support the receipt.
This is a selected working-tree source boundary, not complete transitive input closure,
cold reproducibility, a release, consumer adoption, full accessibility or G2 completion.
No production account, provider request, real credential or live business data is used.
