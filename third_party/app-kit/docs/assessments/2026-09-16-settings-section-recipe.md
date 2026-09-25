# Experimental responsive settings-section recipe

G2 presentation reuse under [ADR-0064](../adr/0064-incubate-responsive-settings-sections.md).
The initial Gallery proof and later RAC adoption below are separate; neither establishes
complete account workflows or permanent promotion.

## Implementation

`form.SettingsSection` and `RenderSettingsSections` replace five repeated Gallery
Profile/Settings description/control sections. Shared presentation supplies escaped h3
headings and descriptions, heading associations, semantic roles, responsive columns,
separators and long-text wrapping. The caller supplies reviewed HTML content, labeled
controls, an h2 parent and document-wide unique IDs. Invalid/duplicate heading IDs and
blank titles return an error before any markup; an empty list emits nothing.

The app retains forms/actions, permissions, validation, persistence, concurrency,
drafts and recovery. Gallery preserves IDs, copy, controls, digest-error wiring and
state branches; Appearance stays independently composed. No new script or dependency.
Consumers scan `appkit/ui/form/*.go` and include theme CSS. Existing form APIs remain.

## Verification

- Unit tests cover parsed heading/section relationships, order/separators, escaped
  malicious text, reviewed slots, atomic refusal, invalid UTF-8 IDs, duplicate IDs,
  empty/blank input and no invented form/runtime authority; a public example compiles.
- Race tests pass for form, dashboard, catalog, Gallery and MCP; CUE validation and
  18 Node draft/config tests pass. Catalog now has35 records and70 selection cases.
  The added positive Go-web and negative React case run through library, MCP and
  Gallery JSON/HTML; exact source anchors, experimental maturity and empty consumers
  are checked. The pre-existing34-record assertion was updated to35, retaining the
  exact nine-private-adapter expectation and its full contract checks.
- A compiled, script-free fixture passes72 browser assertions at390/1440px in both
  schemes: complete long copy, real text Range bounds, stacking/columns, headings,
  labels, native Tab order, changed focus indicators, viewport bounds and effective
  theme colors. Playwright supplies only the virtual component route; actual Gallery
  serves the CSS. No form submission occurs in this layout probe.
- The unchanged real-route forms probe passes215 checks: native saves, validation,
  conflicts, retry, permissions, sign-in/help, themes and account traversal, including
  script-disabled sessions. This is synthetic process-memory data, not a consumer.
- Four component and22 actual-route captures are retained. Narrow-dark/desktop-light
  component and Settings screenshots were visually inspected; wrapping, section
  hierarchy and controls remain readable. Review is operator-attested, not inferred
  from screenshots or counted as a complete visual/accessibility certification.

## Retained evidence

Final root: `/Volumes/m2ext/appkit-evidence/appkit-settings-browser.Y1UdgRIJ/`.
`tested/` retains16 selected source/test/catalog/browser/config files before execution,
the compiled fixture and runner. `verification-receipt.json` binds commands, binary,
CSS and served catalog revision. `audit-receipt.json` checks those16 current hashes,
two reports,26 captures and a fresh byte-identical Go fixture export. Owned Gallery
PID15987 exited0 after SIGTERM; existing previews were not touched.

Earlier roots remain unchanged. `appkit-settings-recipe.TYyTXCCA` preserves the
before-source and failed34-record assertion. `appkit-settings-verified.hga7nIdm` retains
passing race/CUE/Node logs, then a failed comparison against preliminary CSS generated
before the browser harness was final. The fresh scan added only `top-1`, `bottom-1`
and `left-1` utilities. Final execution reused only those three successful command
results after checking source/log identities; it regenerated matching final CSS,
built the Gallery and verified exact served CSS. No earlier failure became a pass.

| Artifact | SHA-256 |
|---|---|
| Final verification receipt | `9a8128dd4ce6587e3d794283e1bf8aa8cbbbfae76b40287e6229a5dd38812601` |
| Final audit receipt | `9f438b8c3a370671785d9a8d57611517cebb0f44290c2d872eddc0fef301a67b` |
| Working-tree Gallery binary | `d4feb8240ab45607606b99c1fd39e23a29f5a1e29821d3b5a7e69f91fa922fff` |
| Regenerated/served CSS | `5fae6a9799233452c81039e8e67fec929e8ec6fb7513c0ee333b2af43e652b0f` |

Go1.26.3/Darwin-arm64, Chrome152.0.7977.84, Node26 and the existing approved isolated
Playwright were used with warm offline Go caches. This is selected-source working-tree
proof, not complete input closure, cold reproducibility, release delivery or installed
MCP refresh. No consumer, dependency, real data, global configuration or commit changed.
At this initial proof, full accessibility, physical/IME input, other engines, actual200%
zoom and two-consumer acceptance remained open. The compatibility surface is experimental.

## Resume as Code adoption follow-up

The owner-authorized RAC integration replaces three repeated Settings cards with the
same renderer. The exact kit `form.go`/`settings.go` files are vendored and scanned into
embedded CSS. RAC supplies the h2 parent and reviewed HTML slots; protected credential
POST, CSRF, validation, persistence, recovery and hosted refusal remain app-owned.
Root review compared retained before/after source and verified both vendored files
against the kit. The credential-form implementation and browser oracle were unchanged.

- Structural regression fails on the retained baseline and passes after integration.
- An initial race run caught the omitted form-package CSS-source expectation; the
  closed-source canary was repaired, not bypassed. Vendored serve/Google race suites,
  scoped vet and CSS rebuild/check then pass.
- The unchanged synthetic Chrome journey passes165/165 with zero errors, scripts on/off
  at390/768/1440px. Two offline vendored trimmed builds are byte-identical; static v2
  adoption checks pass37 with one whole-catalog check not applicable and zero failures.
- RAC's contract, selected capability/artifact pins and release notes record
  `RAC-SETTINGS-01` as returned evidence. Existing ADRs/invariants still govern; no
  consumer domain, agent configuration, dependency, deployment or global change.

Evidence root: `/Volumes/m2ext/rac-settings-sections.pi53Hs/`.
Root receipt SHA256 `6e5133a0f081afb2cc61324f47d9d25af7d63b736ca69b5f263e2e9cbcdcf73f`;
browser receipt `2cecddc91597ee48a3cfaa60c7081f26843369f65e00cc01b966f81eeb6a0c0b`;
binary `d4dd332f8e486a6a9aa8adb72bd547e5f17a897c3733c9bd6210ee05e1dcba6c`;
CSS `7a8e0ad74402b2c32bc97409e71477e1af758840f8aa5f6684ae132b457b8aba`.

ADR-0064's second-consumer revisit trigger has fired: the integration supports the
presentation-only boundary. Keep experimental maturity while permanent promotion and
versioned delivery receive explicit review; this dirty-working-tree vendor subset is
not a released pin, cold/clean export, OAuth, performance or full-accessibility proof.
No tests are rerun or historical failures relabeled merely by recording this handoff.
