# P04 typed responsive directory adaptation

Second private source-specific Go/templ candidate under ADR-0023, not a certified
target or public table replacement. The 439/657 literal syntax/compiler denominator,
five saved syntax attempts and original purchased sources remain unchanged.
[Frozen contract](fixtures/table-adapter/README.md).

## Implemented boundary

`appkit data assets-adapt-directory [-root PRIVATE_DIRECTORY]` verifies the frozen
application-ui archive and `html/lists/tables/07-with-stacked-columns-on-mobile.html`.
It emits private JSON only: template/model/defaults, exact source/syntax/rule/output
identities and a CUE-governed `not-verified` envelope. No writes, installs, compilation,
export, target registration or Gallery publication occur in the command.

Generated `Directory` and `DirectoryRow` provide bounded headings/column/action labels,
0..100 rows and app-owned destinations. `NewDirectoryNavigation` validates before output,
snapshots the row slice and uses templ escaping. The model has no app-kit runtime
dependency. One field supplies each mobile/desktop Title/Email and accessible row Name.
Zero rows keeps the header and empty body; it does not invent an empty-state workflow.

The explicit semantic mapping changes the inert Add button to a native same-window
anchor (`type=button` becomes `href=AddURL`) and each `Edit #` to its row destination.
All other classes, text whitespace, structure and attributes are retained. Defaults
leave every URL empty; the caller must supply routes. No account store, authorization,
creation/edit backend, sort/filter/page/export behavior or identity-theme remap is inferred.
The exact original remains independently available; this is not unmodified-source DOM.

## Original browser baseline

`prepare.go` reads the exact source, pinned Inter/license and cached Tailwind4.3.0.
It streams the executable into an owned private snapshot, verifies the pin before making
it executable, and builds identical CSS twice without downloads or cache modification.
Complete document hashes bind the font/license prefix; `css_sha256` binds compiled CSS
alone. These are chosen comparison conditions, not recovered portal defaults.

Actual isolated Playwright1.63.0/Chrome checks at390/1440 CSS pixels, light/dark:
**52/56 passed, four inherited quality failures, zero boundary/browser/server failures**.
All four failures are the44px target rule: Add is approximately85.09×36px; Edit25.39×17px.
Exact rows, responsive labels, hierarchy, no outer overflow, sampled contrast, loaded
InterVariable and first-Tab focus pass for the supplied six-record source fixture.
Main visually inspected mobile/light and desktop/dark. No full accessibility claim.
Original Add remains inert and Edit remains`#`; neither is counted as a working task.

Source browser receipt SHA-256:
`5e58f8234da877c0136294ee06cd20eeee9f6148543939a6bd338ce6cef4d7e1`.

## Compiler and refusal evidence

Synthetic source/shape canaries cover missing/extra rows/fields, changed row attributes
and spacing, mismatched duplicated data/labels, accessible-name drift, unsafe behavior
and forged identity. Generated runtime tests cover dynamic labels,0/1/6/100 rows,
escaping, limits, URLs, snapshotting, cancellation and writer errors before/after formatting.
The opt-in paid probe repeats adaptation/replay and compiles/renders before/after templ
formatting, comparing exact parsed DOM with only the declared action mappings. Planted
whitespace, mobile-value and column-scope changes are detected; completion markers ensure
the generated test actually executes. Source/catalog hashes remain unchanged.

Initial generation failed because a tag chunk began with a source-map comment, not the
tag itself. A second failure exposed repeated-row comparison including differing source
offset comments. The repair checks only those exact generated non-rendering annotations
separately; emitted comments and all rendered content stay intact. The full DOM oracle
was not normalized or weakened. Independent production/live-oracle review found no
substantive issue after both repairs; synthetic race tests and the actual paid probe pass.
The separate CLI probe passes deterministic JSON and unchanged archive/catalog checks.

## Source-styled target browser result

Final run: **88/104 checks passed; zero boundary or mapped-DOM/geometry failures**.
All32 Add/Edit activations pass across JS on/off,390/1440,light/dark,keyboard/pointer;
eight anchored accessible-name/destination checks also pass. These reach read-only
synthetic destination pages, not an account backend. Exact72 requests and unchanged
inputs pass. Both source and target use the same font/style bytes and local-only CSP.

Sixteen retained quality failures: four source44px checks, four target44px checks and
eight target Edit-focus checks (observed1px outline versus required2px). Sampled contrast
passes (minimum4.576:1), as does page containment for these six default records.
Non-action geometry and mapped DOM match. Desktop light/dark PNG pairs are byte-equal
in this run; mobile pairs differ because native block-anchor Add expands85.09→358px.
This declared-action visual delta remains recorded, not normalized away. Main visually
reviewed the mobile/dark target. Long/empty/partial-data browser proof is still separate.

The first harness run incorrectly assumed accessible-name punctuation spacing. Its red
receipt remains. The corrected probe retains exact DOM checks and verifies the full
anchored Edit/name label with browser whitespace around the comma allowed; it does not
replace accessible-name checking with URL-only selection. Repeated runs use new private
directories. Final receipt SHA-256:
`44ca23dd78562d4216ae971cb5a1b6955312dc218595fbfe6fe2e02eed0cc2a9`.

Private copies live in `data/design-assets/directory-typed.fudaVj`:54 payloads,
3,262,799 bytes,12 selected code identities and all four browser receipts. Every copied
payload matched its original. The manifest is not a complete worktree attestation:
`b4a62627a19282705116bdb60ee8639ddb28df42843aa80fdf2b87c02013fc89`.
Current artifact:`d49356678db9709574847f69871af18c86bb433290c64d6fc3c952f5f91805e7`.
The ignored private store needs separate backup; no original was overwritten or removed.

Root Go tests/vet, scoped race tests, CUE validation and MCP tool/resource document
parity pass. Scoped intent checks:15 files/164 relative links resolve, stanzas match,
root living-doc budgets pass and Unreleased is60 lines. This is not a full host/config
audit or the complete `task check` regeneration gate. No consumer pin was changed.

## Still open

The [separate enhanced companion](2026-09-14-directory-enhanced-adapter.md) now adds
explicit output/CLI choice and compiler fixtures for long/empty/partial states; its
own browser evidence is independent of the historical receipts above. Full
accessibility/zoom/performance and real consumer use remain separate gates.
No candidate quality, fidelity, compatibility or adoption badge is earned
by compilation. Static original controls are not application behavior.

P04 makes ADR-0034's second-adapter delivery review concrete. Its source inventory must
stabilize before factoring shared private export/check machinery; P02's existing CLI,
artifact and six-file delivery remain unchanged. P04 is currently JSON-only. This is
remaining G1/G4 implementation, not a new public vendor channel or completed goal.
