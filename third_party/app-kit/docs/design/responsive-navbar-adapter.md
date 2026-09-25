# Private P09 responsive-navbar adapter

[ADR-0057](../adr/0057-explicit-responsive-navbar-and-route-identity.md) defines the
source-specific route contract. This is experimental tooling, not a certified web component.

## Source and private delivery

Frozen archive `application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8`,
member `html/navigation/navbars/04-simple-with-menu-button-on-left.html`, source SHA
`79cdbf1cd163a13cb77cc96c4cf8046a088ad8f18757658408372d437400b9d7`.
The original stays independently retrievable through the private asset tools.

```sh
appkit data assets-adapt-navbar -root PRIVATE_DIRECTORY
appkit data assets-adapt-navbar -root PRIVATE_DIRECTORY -out NEW_PRIVATE_DIRECTORY
appkit data assets-adapt-navbar -root PRIVATE_DIRECTORY -check PRIVATE_DIRECTORY
```

Default output is private JSON. Export writes five immutable files: navbar.templ,
navbar.go, defaults.json, USAGE.md and artifact.json. Output/check are mutually exclusive
and silent on success. Exit 1 is observed drift; exit 2 is invalid input or could-not-check.
Existing files are never overwritten or repaired. Source replay precedes filesystem I/O.
Original access, artifact hashes and bounded single-link regular-file checks remain required.

Compile a COPY in an app-owned private module with templ v0.3.1020. Do not import the
internal adapter package. Generated runtime Go needs only templ and the standard library.
Scan navbar.templ for Tailwind 4 CSS. Supply fonts, approved images and the exact
Elements 1.0.22 module identified by the artifact. No dependencies are installed here.

## App-owned data contract

`NavbarReference()` is the constant original with inert placeholders and external imagery.
`NewNavbar(Navbar)` validates, snapshots and escapes the app's presentation data:

| Field | Responsibility |
|---|---|
| IDPrefix | document-unique ASCII identifier, 1–64 bytes, starts with a letter |
| Label / MobileLabel | nonblank navigation-landmark and disclosure-control names |
| Brand | explicit LightURL / DarkURL and meaningful Alt; no inferred home link |
| Links | 1–12 ordered Key / Label / URL records, keys unique within this model |
| CurrentKey | exact existing key, or empty to mark no primary destination current |
| Notifications | optional Label / URL for a real GET destination, not a provider |
| Account | optional Label / ImageURL and 1–12 Label / URL destinations |

Desktop and mobile share one Links slice and the same current-route decision. Both
current presentations carry `aria-current="page"` and their original source styles.
The constructor copies all caller-owned slices and optional objects before rendering.
The app owns the document-wide namespace; no global counter or auth state is introduced.

Account.ImageURL is decorative with empty alt because Account.Label names its button.
Nil optional fields remove controls. Empty account menus are refused. Defaults leave
IDPrefix, Label, image URLs and route keys/URLs unbound and optional actions nil.
Handle constructor errors before writing response headers. No guessed sign-out action,
account backend, POST/CSRF issuer or notification state is supplied; sensitive mutations
must not occur through GET. Link labels must describe the actual destination.

Text is bounded nonblank control-free UTF-8: labels 256 bytes, brand alt 512 bytes.
URLs are root-relative or explicit HTTPS, at most 2,048 bytes, without userinfo,
protocol-relative forms, malformed escapes, decoded controls or backslashes.
URL syntax validation does not establish destination authorization or image rights.

## Interaction and evidence boundaries

Elements owns the mobile disclosure and account menu. Triggers are non-submit controls.
Keep these subtrees under one owner; no replacement state machine, Datastar integration
or automatic runtime installation is included. Provide essential destinations elsewhere
when runtime loading fails. Responsive layout, focus and meaningful image delivery need
checks in the actual consumer, including its surrounding forms and other nav instances.

`TestNavbar*` exercises synthetic source refusals, exact replay, typed validation,
original/reference DOM and paired current-route semantics before/after templ formatting.
`TestAdaptNavbarLive` opts into the frozen paid source; `TestAssetsAdaptNavbarLive` checks
actual CLI dispatch, private export and drift. Default skips are not live-source proof.
The maintained [browser probe](../assessments/fixtures/navbar-adapter/browser.mjs) binds
the compile receipt, pinned runtime/font/CSS tool and retained renders. It tests real
keyboard/pointer navigation, narrow/wide layouts, optional controls and repeated instances.
Read the [assessment](../assessments/2026-09-15-navbar-typed-adapter.md) for current outcomes.

The assessment records 238 scoped browser checks and two identical cold synthetic
consumer builds with source/network denial. Neither replaces consumer acceptance.
The separate actual200% base diagnostic retains failures: long desktop labels escape
the fixed-height bar, primary/account links fail conservative focus screening, and
module HTTP503 hides account destinations plus mobile primary links. These fire
ADR-0057's review trigger; the original base still has no runtime fallback. Original-image
fidelity, full accessibility, performance, standalone CSS/runtime delivery and real
consumer adoption remain separate gates. The catalog record
`reference.navbar-adapter` is source-free CLI guidance, not certified target availability.

The [private evidence packet](../assessments/2026-09-15-navbar-evidence-retention.md)
retains114 original/reference, failed zoom, runtime repair and delivered-repair captures.
Gallery's catalog link opens native capture/stack/full-size inspection; MCP returns the
same byte-free historical metadata. Larger gate inventories require ADR-0061 readers.
Interaction remains failed and all full target gates remain incomplete.

## Explicit runtime-bearing repair delivery

[ADR-0060](../adr/0060-deliver-explicit-navbar-runtime-repair.md) adds a distinct
nine-file private variant. Supply the exact original Elements ESM at an absolute path:

```sh
appkit data assets-adapt-navbar -root PRIVATE_DIRECTORY -repair -runtime ABSOLUTE_ORIGINAL_ESM
appkit data assets-adapt-navbar -root PRIVATE_DIRECTORY -repair -runtime ABSOLUTE_ORIGINAL_ESM -out NEW_PRIVATE_DIRECTORY
appkit data assets-adapt-navbar -root PRIVATE_DIRECTORY -repair -runtime ABSOLUTE_ORIGINAL_ESM -check PRIVATE_DIRECTORY
```

Repair requires the runtime input; a runtime flag without repair is refused. No module
is searched for, downloaded or installed implicitly. The reader rejects links, special
files, oversized inputs and anything other than the exact pinned original bytes.
Do not supply the delivered derivative as the original. Base defaults/exports remain
unchanged and base/repair variants reject cross-checks instead of silently upgrading.

The inventory is navbar.templ, navbar.go, defaults.json, navbar_repair.css,
navbar_routes.go, navbar_routes.templ, elements-navbar.js, USAGE.md and artifact.json.
The three base inputs and both nested repair/runtime artifacts retain their existing
identities. The outer envelope separately binds the delivery rule/schema. Source replay
precedes output access; fixed names/private modes/no-overwrite and exit0/1/2 stay intact.

[ADR-0058](../adr/0058-explicit-navbar-repair-and-native-route-access.md) admits a
separate `NewNavbarWithRoutes(Navbar, summary)` companion and scoped CSS. It retains
the exact base and uses the same validated/snapshotted app route model. Summary is
nonblank escaped control-free UTF-8, at most256 bytes; the app chooses its visible
summary and navigation-landmark name. No auth, sign-out or notification backend appears.

The companion adds `data-appkit-navbar-repair`, the unchanged NewNavbar output and
a native details/summary directory. Load navbar_repair.css after the original CSS.
Header/route groups may grow and wrap; failing links receive contained focus paint.
Do not wrap NavbarReference or unrelated navbars to select this styling implicitly.

The native directory remains available during healthy, missing and late runtime
loading. It never auto-hides on custom-element registration. A separate subtree has
native browser ownership; Elements retains the enhanced menu/disclosure. App authors
must assess the extra entrypoint's density and wording against their task.

The internal envelope binds exact base, companion Go/templ, stylesheet, rule/schema
and wrapper identities. Compiler/model/DOM and466 actual200% browser checks pass for
the recorded candidate. Its original normal-size acceptance retains523 passes and four
maximum-menu entry failures: Elements' delayed container focus steals first-link focus.

[ADR-0059](../adr/0059-guard-selected-navbar-runtime-focus.md) adds an independent
runtime derivative, generated only from the exact Elements1.0.22 bytes and paired repair.
It guards the existing delayed focus for selected wrappers, without adding listeners or
changing unselected behavior. Runtime/repair/output/rule identities replay independently;
do not load both original and derivative modules in one document. The original module,
base and repair artifacts stay unchanged. The frozen normal-size suite passes527 checks
with this explicitly selected runtime, including all four first-item failures, no-script
tasks and instance isolation. Its own actual200% replay also passes466 checks; this is
a separate runtime-bound result, not a transfer of the older original-runtime evidence.
Sixteen separate real-runtime checks cover selected keyboard/pointer entry, programmatic
close/removal and unchanged unselected behavior in short/synthetic scrolling menus.

Compile a private copy of both templates and scan both for source Tailwind4 CSS. Load
source CSS first and repair CSS last. Serve elements-navbar.js once as an explicit
module instead of the original Elements module; there is no extra mount listener.
The app supplies fonts/images and routes. Do not use the inert reference as application
navigation or silently expose its demonstration actions/assets.

The exact nine-file export also passes two cold, source/network-denied synthetic
consumer builds with identical generated Go, typed/minimal HTML and binaries. This
is separate from the normal100% standalone browser run:144 checks bind template-only
CSS builds, received CSS/runtime bytes, narrow/wide light/dark tasks and complete
viewport captures. The assessment retains earlier failed setup/capture evidence.
CLI availability does not certify a target. Broader lifecycle, other delivery/browser
matrices, consumer acceptance and full target gates remain distinct; see the assessment.
