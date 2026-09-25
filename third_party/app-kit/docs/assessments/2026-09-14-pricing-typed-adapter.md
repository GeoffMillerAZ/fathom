# P02 typed Go/templ adaptation

Implements one bounded ADR-0023 family adapter, not a general HTML/React compiler or
certified target. The original, existing syntax attempts and Gallery comparison stay
unchanged. Other eleven frozen cases and the 439/657 syntax denominator are unaffected.

## Implemented contract

`appkit data assets-adapt-pricing [-root PRIVATE_DIRECTORY]` verifies the exact frozen
marketing archive/member, emits private JSON and performs no writes, installs, builds
or catalog publication. `internal/designadapt` binds source, syntax toolchain, adapter,
model/template/defaults and CSS identities; CUE validates a versioned envelope with
only `verification:not-verified`. Replay rejects changed source/rule/content/evidence.

The generated private package needs templ, not an app-kit runtime dependency. It offers
plain typed copy, app-formatted price/currency, 1..12 bounded features and one native
same-window link. Defaults deliberately have no usable destination. `NewPricing`
validates text/URL before output, snapshots the feature slice and preserves the source
composition. All configurable text goes through templ escaping; no raw HTML, JS,
checkout, auth, payment computation or invented backend is provided.

`NewPricingEnhanced` separately wraps the validated component and opts into the supplied
scoped CSS: minimum44px CTA, flex wrapping and long-content containment. Its CSS must be
explicitly included after source CSS. It retains vendor colors, classes, typography
and focus rules; it is not pixel-identical or a semantic kit-token adaptation.
[Probe contract and reuse instructions](fixtures/pricing-adapter/README.md).

## Compiler and runtime evidence

Synthetic tests cover exact DOM, field/structure refusals, safe URL/data escaping,
UTF-8/control/size limits, feature counts, snapshotting and compiler formatting.
The opt-in paid test verifies the archive/member, repeats generation, exercises
tamper refusals and compiles/renders before/after `go tool templ fmt`. Its baseline
compares every namespace, attribute, comment and text node, permitting only the one
declared CTA `#`→`/pricing/access`. A planted whitespace change proves detection;
explicit completion markers prevent a filtered-out runtime test from becoming a pass.
The separate CLI probe verifies deterministic JSON and unchanged source/index bytes.

The first typed loop failed exact DOM despite compilation: templ inserted whitespace
inside control-flow boundaries. A comment-only repair remained red. The final emitter
uses a small typed Go rendering loop plus a compiled templ feature component and the
source's exact whitespace separator. Synthetic and paid exact DOM checks then passed
before/after formatting. No whitespace normalization or changed golden was accepted.

## Faithful browser baseline

Existing isolated Playwright1.63.0/Chrome, local HTTP, prepared Tailwind4.3.0/Inter,
390/1440 CSS pixels, light/dark. These are chosen conditions, not recovered portal
typography. Source and target use the same explicit local demonstration destination.

Initial report: **58/72 passed,14 failed,0 operational/boundary errors**. All four
original/target PNG pairs are byte-identical; sampled absolute contrast, keyboard
focus and JS-on/off native keyboard/pointer navigation pass. Source and target CTA
height is36px, below the44px contract. Long permitted price text causes516px/375px
outer overflow at390/1440. These quality failures do not invalidate the faithful
conversion proof, but do prevent quality promotion. The source was not patched.

Red receipt SHA-256:
`b7e06c903cc73399076c92f59f8540991ed39e36345bf23aac201629bbfd560e`.
Private raw output initially retained in system temp `tmp.rulPpeir5q/browser-output`.
The red harness remains unchanged; enhanced output has separate checks/receipts.

## Enhanced browser result and preserved failures

First enhanced run:62/66, with a48px desktop overflow. The broad max-inline-size rule
overrode vendor max-width caps and visibly collapsed the text column. The repair
limits that maximum to price/currency spans; vendor column widths remain authoritative.
The failed receipt stays intact; added source/target width and heading-metric checks
guard against another apparently contained but visually broken composition.

Final enhanced result: **74/74 passed**, including48 target,24 navigation and2 boundary
checks; zero browser/server errors. Eight baseline/long viewport/theme cases meet
44x44 CTA, no-overflow and sampled absolute-contrast budgets (minimum4.576:1). All
16 JS-on/off × light/dark ×390/1440 ×keyboard/pointer journeys navigate natively.
Four baseline cases preserve source element widths and H2/H3/H4 typography metrics.
Root visually reviewed desktop dark, mobile light and extreme-content mobile dark.
The companion CSS is635 bytes. No app JS or network assets were introduced.

A fresh faithful rerun retained57/72, not58/72: one light/mobile image pair differed
at26 pixels despite equal DOM. A separate identical-document immediate/settled probe
reproduced exactly those26 pixels and bounds, with equal DOM/geometry/style hashes;
this is nondeterministic raster timing evidence, not a source/target content change.
The original byte-equality check and both failing reports remain; no tolerance, mask
or golden was changed. Initial four exact pairs are historical, not universal proof.

Final enhanced receipt SHA-256:
`4694de09c09d92b97f3e4e05ef99b6a7f13e8948635278a810f7ae553e00c946`.
Current artifact:`5e42940677809bdf55613c9a712254febd12d0a69b1b8c683a5de86011c88a5f`.
Current CSS:`d846ddefc1e930b8b141a5aaf3a925a0d6ee05963cab064fbab4a8feb12ef7d9`.
All private snapshots, screenshots and diagnostics are retained under
`data/design-assets/pricing-typed.JpO2uS`; originals remain independently available.
Manifest:`def35e3187d2b257d9f51f5ab04c763f292e0c080b027bc94fe6f18adac81804`,
90 payloads/13,130,727 bytes,12 captured source identities and5 linked receipts.

## Remaining acceptance

No target badge or catalog count is promoted. Scoped tests, race tests, full root
Go tests/vet, CUE validation and MCP authored-guide/resource parity pass. Root intent
structure/link checks pass;17 legacy ADR size exceptions remain unchanged.
The [private delivery follow-up](2026-09-14-pricing-private-delivery.md) adds opt-in export/check
and two isolated synthetic consumer builds, without changing the browser evidence above.
Both variants still need the full applicable target matrix and actual app use,
real zoom/assistive-input checks and declared deployment/performance evidence. A
source-faithful image match is not a portal baseline, blanket accessibility approval,
production pricing workflow or proof of conversion investment payback.
