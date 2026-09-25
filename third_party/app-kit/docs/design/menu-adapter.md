# P07 typed menu and explicit app actions

Status: experimental private adapter, not a certified web component. Unchanged v1
fails actual 200% focus/containment; the explicit repair passes314 normal and210
zoom/resize/lifecycle checks in [retained evidence](../assessments/2026-09-15-menu-typed-adapter.md).
Two cold standalone base render builds reproduce. Full target, performance and
consumer acceptance remain open; selecting the repair is not certification.
[ADR-0049](../adr/0049-explicit-menu-actions-and-private-reference.md) owns the
reference/action boundary; [ADR-0036](../adr/0036-share-private-delivery-assurance.md)
owns immutable private export/check.

## Use and ownership

`appkit data assets-adapt-menu -root PRIVATE_DIRECTORY` emits private JSON.
Add mutually exclusive `-out NEW_PRIVATE_DIRECTORY` or `-check PRIVATE_DIRECTORY`
for six-file export or exact current-source comparison. Existing output is never
overwritten, repaired or deleted. Success is silent; exit 1 means observed drift,
2 invalid request/source or could-not-check. A failed export remains for inspection.

The six files are `menu.templ`, `menu.go`, `menu.css`, `defaults.json`, `USAGE.md`
and `artifact.json`. They remain licensed private derivatives. Compile a copy in
an app-owned module using pinned templ v0.3.1020; do not edit immutable delivery.
Generated Go needs templ and the standard library, not app-kit at runtime.

Add `-repair` to explicitly select the eight-file variant under [ADR-0051](../adr/0051-explicit-private-menu-repair-delivery.md).
Its `p07-menu-repair-v1` envelope nests the unchanged base and binds module/CSS hashes
and reviewed gap/edge defaults (8/16px). It adds `menu_repair.mjs` and
`menu_repair.css` with variant-specific usage/artifact files. `-repair -check` checks
only this variant; base and repair inventories reject cross-checks. Default JSON
and all six base files remain unchanged. Nothing is upgraded or overwritten.

Use `NewMenuEnhanced`; load source CSS, menu.css, then menu_repair.css. Import
`mountMenuRepair`, await `customElements.whenDefined("el-menu")`, and explicitly
mount each connected selected wrapper. Retain its disposer and call it before
removal/remount. No auto-bootstrap, global mounting or test handle is delivered.
Source-styled `NewMenu` and `MenuReference` remain unmounted. Spacing changes need
consumer evidence; the app still supplies the pinned runtime and failure fallback.

The app supplies source-scanned Tailwind 4 CSS, fonts and Elements **1.0.22**, whose
module/tarball hashes are in the artifact. No dependency or runtime is installed
by conversion/export. Theme, CSP and browser support require integration checks.
Elements uses local positioning styles; do not assume a script/style CSP permits
it just because a static page rendered. No inline event handlers are generated.

| Entry point | Contract |
|---|---|
| `MenuReference()` | Constant original markup, including placeholder links/POST; inspect with form submission denied |
| `NewMenu(Menu)` | Source-styled explicit app links and optional protected POST; adds non-submit trigger and hidden CSRF field |
| `NewMenuEnhanced(Menu)` | Same action model plus scoped target-size, wrapping, viewport and focus CSS; load `menu.css` after source CSS |

A person opens an account/action menu, chooses a destination or submits one explicit
action, and receives an app-owned result/recovery page. Preserve the purchased light
and dark styling. The kit does not invent the account, sign-out semantics or backend.

## Typed data

`Menu` contains `Label`, ordered `Links []MenuLink`, and optional `Submit *MenuSubmit`.
Each link has `Label` and `URL`. A submit has `Label`, `Action`, `CSRFName`,
`CSRFToken` and `Disabled`. Zero to twelve links are allowed; at least one link or
submit is required. A disabled submit still requires valid endpoint/token data.

The trigger label is bounded to 160 bytes and action labels to 256; all are nonblank
UTF-8 without control characters. URLs are bounded to 2048 bytes. Links accept
root-relative paths or explicit HTTPS without userinfo. POST action is a same-origin
absolute local path without query or fragment. Reject malformed escapes, encoded
controls, backslashes and protocol-relative URLs. Input slices and submit data are
snapshotted; labels are escaped text, never HTML or executable expressions.

CSRF names use a bounded ASCII identifier and reject form named-property hazards.
Tokens are nonblank UTF-8, at most 4096 bytes, without controls; neither is logged
in errors. Defaults extract source labels but leave every destination/token field
blank, deliberately failing real construction until the app supplies them.

The app must issue/check CSRF tokens, authenticate, authorize, bound/parse requests,
avoid duplicate effects and show meaningful success/error/retry states. Disabling
an item is not an authorization boundary. Do not put a POST-bearing menu inside
another form. Use ordinary GET links for navigation, never for sensitive mutations.

Elements owns local menu focus, selection, Escape and dismissal. No second renderer
may morph its internals without separate lifecycle proof. Navigation/remount resets
state; no persistent selection, automatic retry or mutation replay is supplied.
The required runtime is not a no-script solution: keep essential destinations
available through an app-owned alternative if runtime loading fails.

## Evidence requirements

The frozen P07 archive/member/SHA is unchanged. The converter starts with the pinned
Elements translator, refuses unknown shape/bindings, and extracts paid copy/classes
only from verified source at execution. No vendor source is pasted into tracked code.
`MenuReference()` must preserve original parsed DOM before/after templ formatting.
Typed mutations are checked separately rather than normalized away as fidelity.

`TestMenuSyntheticReplayAndCompiler` covers replay and both compiler phases with
synthetic source. Model tests cover escaping, URL/CSRF refusals, exact bounds,
snapshot isolation, link-only/submit-only/disabled shapes, cancellation and I/O errors.
`TestAdaptMenuLive` is opt-in via `APPKIT_PROBE_CONVERSION_KIT_ROOT`; it verifies the
actual archive, constant-reference DOM, typed bindings and six-file private delivery.
Set `APPKIT_PROBE_MENU_OUTPUT` to an empty private directory below the command's
temporary root to retain rendered fragments and a hash-bound compiler receipt.

The [browser probe](../assessments/fixtures/menu-adapter/browser.mjs) accepts kit root,
retained probe root and independently checked compile-receipt SHA-256. Use a task-owned
command-scoped `TMPDIR` containing the probe; it downloads/installs nothing and uses
fresh Chrome profiles. Current light/dark 390/1440-width evidence covers menu input,
POST/rejection/retry, focused-item paint and long labels. Add `--repair` to the
normal probe or `--repair KIT` to `zoom.mjs` for the explicitly hash-bound companion.
Use `--repair-delivery PRIVATE_DELIVERY ARTIFACT_SHA256` to test the actual eight-file
export; the delivered module/CSS also pass all210 zoom/resize/lifecycle checks.
The separate zoom probe uses Chrome's real 200% setting; base fails and repair passes.
`standalone.mjs` proves two cold synthetic render builds with source/network denial,
not CSS/runtime/browser adoption. Passing generation alone is not passing those checks.
Datastar lifecycle, touch/screen readers, full performance and real-consumer adoption
remain required before broader target claims. No catalog target is promoted here.

## Confirmed zoom failures and repair boundary

At 1440 outer/720 inner CSS pixels and DPR2, opening the valid long-label specimen
leaves focus on `el-menu` rather than its first item. The pinned runtime contains a
deferred container-focus call competing with immediate first-item selection. Treat
that source finding separately from the directly measured geometry defect: the menu
starts at136px and extends to604px in a500px-high viewport. Its current height cap
subtracts margins from the entire viewport, not the available anchored space.

ADR-0049's runtime/focus revisit condition has fired. Repair must preserve original
markup/runtime references, app-owned actions and Elements' keyboard ownership. Do not
replace anchored placement with a fixed overlay or duplicate its state machine merely
to pass. Earn first-item entry, every-item visible focus, long-label wrapping, scrolling,
resize/reposition and disabled-action behavior together; re-run normal comparisons.
An enhancement or runtime patch must be explicit, identity-bound and independently
reviewed before updating private delivery. Existing output must never be overwritten.

[ADR-0050](../adr/0050-explicit-menu-geometry-and-focus-repair.md) admits the internal
`menu_repair.mjs` / `menu_repair.css` candidate, not a v1 export change. Explicit
`mountMenuRepair(root, {gap: 8, edge: 16})` returns an idempotent disposer; the gap
must match the frozen source CSS or the app's deliberate spacing override. It sizes
available above/below space and reconciles only Elements' existing selected item.
The host owns disposal before removal/remount. Node tests cover geometry, cancellation,
focus refusal and isolation. The314/210 browser follow-up covers explicit repair,
every-item zoom/resize, upward placement, multiple instances and queued-work disposal.
Lifecycle injections are separate from keyboard tasks; no Datastar or consumer proof
is inferred. The historical v1 failure remains reproducible and visible.
