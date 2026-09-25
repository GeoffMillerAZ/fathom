# P06 private typed page navigation

Status: fourth private typed pilot candidate; zero certified targets. The frozen
439/657 syntax/compiler/DOM count, five saved syntax attempts and twelve pilot cases
are unchanged. [Contract](../design/page-navigation-adapter.md) and
[reproduction](fixtures/navigation-adapter/README.md) own usage and acceptance.

## Delivered capability

`appkit data assets-adapt-navigation` independently reads frozen P06, checks its exact
identity and reviewed shape, then generates CUE-validated Go/templ with source/rule/
output hashes. JSON is the default; explicit `-out`/`-check` reuse five-file private,
no-clobber delivery. Existing exports and the licensed original are never edited.

The bounded model accepts 2–12 ordered Key/Label/URL items and one CurrentKey, snapshots
the slice, escapes text and restricts operational destinations to canonical local paths.
`NewPageNavigationReference` preserves the original unbound select/placeholders.
`NewPageNavigationGET` supplies a named native select and explicit submit button;
the app maps submitted keys to allowlisted routes. `NewPageNavigationLinks` supplies
wrapping ordinary links for static delivery. These are page links, not ARIA tabs.
Operational wrapping/focus/44px changes are explicit adaptations, not source fidelity.

Generated code needs templ, not app-kit at runtime. No router/auth backend, new runtime,
dependency installation, global configuration, paid-source publication or consumer
mutation is introduced. Its source-free `reference.navigation-adapter` record now routes
agents and Gallery readers to the same private delivery guide. Discovery is experimental
CLI tooling, not certified web support; consumer pins remain unchanged.

## Evidence observed

| Gate | Result and boundary |
|---|---|
| Source and replay | Frozen application-ui P06: 2,483 bytes; archive/member SHA checked independently; repeat generation, envelope/default/source tampering and no partial output refusals pass |
| Private delivery | Exact five-file export/check, no-clobber and drift classification; original archive/catalog unchanged |
| Synthetic runtime | Bounds/paths/keys/current parity, safe escaping, caller mutation, cancellation and writer errors; namespace-aware original/reference DOM with all text retained |
| Standalone module | 14 preparation checks pass with own go.mod/go.sum and cached pins; missing-generation failure canary, raw/formatted templ tests, no app-kit dependency and unchanged immutable export |
| Native HTTP | Every key in light/dark maps to exact 303 destination; GET-only, Host, malformed/duplicate/unknown fields and routes fail as specified; standalone race/vet pass |
| Browser | 115 scoped checks pass, 23 screenshots, 104 local requests/106 responses; two original target failures remain separate |
| Fidelity | Source/reference exact parsed browser DOM, geometry and byte-identical screenshots at 390/1440 × light/dark using the same controlled CSS/font |
| Operational behavior | JS-disabled native select type-ahead → Tab/Enter → 303 → correct page; keyboard/pointer links, current state and browser Back; responsive visibility and actual Tab order |
| Usability samples | 44px operational targets, solid visible focus, sampled normal-text contrast minimum approximately 4.576:1, 256-character wrapping geometry; no full accessibility certification |
| True browser zoom | Fresh Chrome profile set to 200%; DPR2, 720px layout in a 1440px window, visual scale1; native keyboard route, no overflow, 1440×1000 capture and last-control containment pass |
| Maintainer regression | Full designadapt/appkit command tests and vet; scoped race tests; catalog/MCP package tests and artifact CUE vet pass; no full-repository test claim |

Main visual review inspected ordinary dark/narrow links, the light/wide original,
dark/narrow long labels and the full physical-width 200% capture. The fixture is a
component rehearsal, not a redesigned settings app or a deployed consumer.

## Limitations and retained failures

The source select is 40px tall in both narrow themes. It remains unchanged in Reference;
GET explicitly adds 44px height, and Links uses source-styled wrapping link targets.
Reference has no submitted choice or working placeholder destinations and is never
credited as usable navigation. Native selects may truncate long options; choose Links
when full labels must remain visible. Long-label browser checks mutate only the rendered
document; independent runtime tests cover model bounds/escaping, not every host layout.

Two early browser runs failed because arrow/Home/popup key sequences did not change
the native select in headless macOS Chrome. A bounded key probe found native type-ahead
does work. The final test uses real type-ahead, Tab and Enter, not `selectOption` or a
scripted value assignment; destination/current assertions were unchanged. OS-popup
arrow navigation remains unverified, not silently credited by the successful task.
Back restores the route; a native select may retain a pending choice through history.
The app's heading/current route remains authoritative until a new submission.

Versions: Go1.26.3 darwin/arm64, templ0.3.1020, Tailwind4.3.0, Playwright1.63.0 and
Chrome152.0.7977.84. Inter4.1 and Tailwind are controlled reference choices, not proof
of the portal's exact original environment. CSS including embedded font/license is
484,717 bytes; no product performance budget, cold install or binary reproducibility
claim is inferred from these cached tests. Screen readers, touch, broad browser/host
coverage, full contrast, real app adoption and family-wide target gates remain open.

## Private retained capture

`data/design-assets/navigation-typed.FGyrmV` retains the immutable delivery, source,
raw/formatted standalone modules, test binary, controlled CSS/font license, raw/formatted
test logs, final browser report/screenshots and earlier failure reports. The retained
final harness is `implementation/browser-final.mjs`; the earlier harness snapshot is
kept separately. This ignored directory is private and not backed up by Git.

| Identity | SHA256 |
|---|---|
| Artifact JSON | `8ab0e692df0b1521097ac04f3847c68ddf3c5e9acaca13c13e8aa0289d78bea6` |
| Final `browser-YOPTyP/report.json` | `2abd2c775278ce90f0e81bb2c877ff3c23690dd672e8f200484e88df9aa995d3` |
| Preview test binary | `8648b4bbec543cbc0c64871cba1703658e13af87e2f76b7192be9eb790248f65` |
| CSS with font/license | `8f9f11836291945e9b08fb92f82e94a6a2704ca9fa0c49ea92388aa5cd615cf3` |

Intent context: I7/I8/I12/I13/I17, ADR0023/0034/0036. No new ADR, fired architecture
trigger, future promotion or breaking surface; scope and conversion guide now count
four candidates. The existing unreleased adapter entries were folded into one current
delta; no released block or substantive goal was removed. G1–G6 remain active, with
Windows and CA/GGB adoption still owner-deferred. Fathom's paused work stays untouched.

## Intent hygiene (scoped doctor)

| Check | Result |
|---|---|
| Router/living-doc budgets | PASS: router77, scope150, incidents149; all owned living files within their budgets |
| Relative-link existence | PASS across the ten root intent files and five selected design/assessment/index guides |
| Unreleased shape/budget | PASS: Release impact retained, current block below60 lines; no released entries changed |
| Other doctor/host checks | SKIP: no complete structural/semantic audit, installed-version or native-host invocation claim |

## Source-free discovery follow-up

The catalog now has 28 entries: one additive P06 record, with the prior 27 records and
catalog schema unchanged by deep comparison against the saved pre-edit CUE document.
Its global revision is `d76d9a56d1210f6c1c3b0274e07f38efa4707ed19781d514726d7597ee93520b`.
Metadata includes frozen source identity, private CLI usage, CSS scans, app responsibilities
and evidence gaps, but no licensed source bodies or generated artifacts. It remains
`experimental`, CLI-only, with no web layers, consumer claims or live preview declaration.

MCP search/exact-get/resource and Gallery search/detail/API tests cover all four private
adapters without a licensed store, bounded summaries and incompatible-profile exclusion.
The actual MCP protocol roundtrip compares the new guide and resource byte-for-byte to
the local file. Catalog, MCP, Gallery and terminal package suites pass. These are in-process
checks, not installed-host activation or a new browser certification claim. Focused race
tests and scoped vet also pass. The terminal command independently returns the same P06
ID/revision as one bounded result. Review corrected an overly broad "unverified" summary
to distinguish existing scoped evidence from still-unearned target certification.

The saved before-input is `data/design-assets/navigation-discovery.uRCmW8/catalog-before.cue`.
Fathom/RAC pins were not changed. Fathom's historical pinned-checker receipt remains
evidence for its selected snapshot, not a pass against this new catalog revision. A
current checker must report revision drift until a separately reviewed pin refresh;
unchanged selected records do not waive the strict revision check.

Next: remaining frozen pilot cases and real consumer acceptance. Do not treat a private
preview or discoverable record as adoption.
