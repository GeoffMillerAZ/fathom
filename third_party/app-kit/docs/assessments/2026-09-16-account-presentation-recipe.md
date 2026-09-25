# Experimental account presentation recipe

G2 progress under [ADR-0063](../adr/0063-incubate-account-presentation-recipe.md).
This is shared presentation plus a Gallery integration, not a complete account
framework, consumer adoption or permanent component promotion.

## What changed

`dashboard.AccountDestinations` supplies optional Profile/Settings/Help/About entries
to the existing typed navigation. A fresh group retains the caller's labels, URLs,
icons and route-match policy; unsupported entries are omitted. `RenderNav` still owns
escaping, unsafe-destination disabling, current selection, focus and target treatment.
Raw fields are not sanitized: custom task links keep app-owned URL policy.

`dashboard.RenderNotice` presents escaped status/error/conflict text. The app determines
the outcome and message; an unknown kind uses attention treatment. No handler, save,
authorization, draft recovery, storage, new client script or dependency was exported.

Gallery replaces duplicated sidebar/account-menu definitions and its inline notice
template. The account disclosure now includes About, shares the Settings label/current
indicator and uses the existing 44px navigation targets. Fixed quick/task/help routes,
identity, permissions, forms, version handling and synthetic stores remain Gallery-owned.
Existing RenderShell/RenderNav APIs, theme defaults and draft script bytes are unchanged.

## Verification

- Dashboard tests cover optional/default/custom destinations, fresh slices, unsafe links,
  escaping, current matching, all notice kinds and an external compiled group example.
- A six-route Gallery regression compares desktop/mobile/account-menu destination and
  current-state agreement. It fails against both retained old rendering files through
  a Go overlay: About and account-menu current indicators are missing. The new source
  passes; a separate four-outcome regression preserves escaped feedback semantics.
- Race tests pass for `appkit/ui/dashboard`, `appkit/catalog`, `apps/gallery` and
  `cmd/appkit-mcp`. Catalog discovery keeps experimental maturity, no consumer claims
  and real exported anchors; existing 68-case cross-surface selection tests pass.
- Catalog CUE validation and 18 draft-controller/browser-configuration Node tests pass.
- The retained final browser probe passes **215 checks**, zero failures: existing native
  save/validation/conflict/retry/permission/load/sign-in/help/theme workflows plus account
  navigation in both schemes, at390/1440px, with scripts enabled and disabled. Pointer
  opens the native disclosure; Tab reaches all four links with the expected current
  marker, height at least44px and bounds inside the viewport. No changed-input backends
  or scripted field state stand in for native POST recovery.
- Twenty-two PNGs are retained. Desktop-light and narrow-dark enhanced menus plus the
  narrow-dark no-script menu were visually inspected across the initial/final runs.
  Full screenshots, actual keyboard traversal and geometry are separate observations.

The final executable embeds regenerated pinned-Tailwind CSS via a Go build overlay;
served CSS is byte-identical to that output. The checkout's existing CSS was not
overwritten. Exact Gallery JSON exposes the new source anchors at catalog revision
`fcccbfd9d39766fe8fcee7901ac18fa8e6c8971bf1e00122ed92872275f53ae0`.
MCP source/protocol tests pass; the installed MCP process was not rebuilt or restarted.

## Retained evidence and limits

Private root: `/Volumes/m2ext/appkit-shell-recipe.7PB31Ag0/`.
`final-tested/` preserves nine selected source/test/catalog/browser/config files before
the final run; subsequent hashes agree with the checkout. `before/` preserves the old
rendering and catalog, and `before-overlay.json` replays the negative control. Earlier
179-check evidence remains separate from the final215-check report and22 captures.

`test-command-receipt.json` and its eight stdout/stderr logs retain a final rerun of
the four-package race gate, CUE validation,18 Node tests and expected old-source failure.
All exit expectations pass; the nine selected source hashes match before and after.
Both owned test Gallery processes exited0 after SIGTERM. Visual review above is
operator-attested, not an assertion inferred from PNG identity or the browser report.

| Artifact | SHA-256 |
|---|---|
| `receipt.json` | `b64871ba0d015bdf17130c1bda73399af999c7ec6b3869adc0ae44d5ba71db41` |
| `gallery-final` | `0ff97a35d4b5d0caf8e800e9dbd7cbdc12d087ba575a5f1d68e88bd051d40793` |
| Regenerated and served `app.css` | `aea85d20d289480fc716d54391c2cc8761ee1d6be89d541823c53da4152fb85d` |

This is a warm working-tree build, not a cold/source-closure or released-pin proof.
The selected source snapshots do not attest every transitive build input. Chrome152,
Node26 and the previously authorized isolated Playwright installation used synthetic
sessions only. No consumer, dependency, shared preview, global config or real data changed.
No claim of complete contrast/accessibility, physical/IME input, 200% zoom, native host,
human task performance or two-app acceptance follows; those G2 gates remain open.

## Adoption

Opt in to the small helpers through the existing Go import and dashboard CSS scan;
the [shell guide](../design/application-shell.md) and [UI API guide](../ui-components.md)
describe the boundary. Keep a real app's routes, support policy, forms and persistence.
Do not copy Gallery's synthetic authority or non-secret draft policy into sensitive forms.
No consumer is auto-upgraded. ADR-0063 retains the two-consumer promotion and kill gates.
