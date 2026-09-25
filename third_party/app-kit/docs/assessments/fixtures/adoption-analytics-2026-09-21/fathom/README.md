# Fathom R5 guide and analytics — review-only packet

For the next integration, use the [separated guide-only rebase](../../fathom-guide-only-2026-09-22/README.md)
against Fathom `fa8c8ed8`. It excludes this packet's service-JSON change and narrows
the root dependency delta. This historical mixed packet and its evidence stay frozen.

This patch is a frozen, synthetic candidate against Fathom `c58adaa2` at
`/Volumes/m2ext/code/geoffmilleraz/sextant`. It is **not applied** to that
checkout. It contains only Fathom source/test/CUE/CSS-script changes and prospective
`go.mod`/`go.sum` deltas. It contains no mirrored app-kit tree, licensed assets,
live logs, real records, or deployment configuration.

## Exact dependency prerequisite

Fathom's current `third_party/app-kit/.vendor-commit` is
`bcfeb8e94ce4132e1198a69f4cf10dc6764bd353`; it lacks
`apps/styleguide`. The candidate was tested with a private in-tree app-kit
mirror stamped `f7ab0fbd3496e9dc3da8c97b6c4290f09a71ec50`, matching the
app-kit source revision for this packet. Integration requires a separately
reviewed, exact source archive from that revision into Fathom's in-tree mirror,
with the stamp and source closure verified, while excluding licensed/private
inputs. Neither this patch nor a sibling-checkout build supplies that archive.
Do not apply the patch alone and call the result deliverable.

The guide's embedded `ui/guide-baseline.cue` is app-owned: it matches the
mirrored app-kit's `cue/tokens/kit.cue` except for light `positive` at
`oklch(0.55 0.14 155)` instead of kit default `0.51`; `0.55` matches Fathom's
existing `cmd/fathom/ui/tokens/theme.css`. The original CUE is embedded and
the synthetic guide test checks exact export bytes. It is not a user theme
preference or a released kit preset.

## Patch and verification

[`theme-guide-and-analytics.patch`](theme-guide-and-analytics.patch) applies
relative to the Fathom root. It adds an opt-in loopback guide over fixed
Observatory models, a guide-only disabled-link preview, and synthetic traffic
canaries. The CSS builder now uses and hashes Fathom's app-owned guide baseline
as its token source, preserving the committed light positive value `0.55`
instead of silently taking the mirrored kit default `0.51`. It corrects registry/ledger/call-log false-zero display in the
candidate renderer. Service-scoped JSON retains its flat row fields while
adding `generated_at`, `call_log_note`, and `degraded_note` context fields;
that is an additive *candidate* API change, not an active deployment.

Read-only applicability check (executed, exit 0):

```sh
cd /Volumes/m2ext/code/geoffmilleraz/sextant
git apply --check /Volumes/m2ext/code/geoffmilleraz/app-kit/docs/assessments/fixtures/adoption-analytics-2026-09-21/fathom/theme-guide-and-analytics.patch
```

Focused test and build in the frozen, ignored candidate (executed, exit 0):

```sh
cd /Volumes/m2ext/code/geoffmilleraz/sextant/workspace/appkit-r5-candidate.7QEdhc/src
GOWORK=off GOMAXPROCS=2 GOCACHE=/Volumes/m2ext/code/geoffmilleraz/sextant/workspace/appkit-r5-candidate.7QEdhc/go-cache TMPDIR=/Volumes/m2ext/code/geoffmilleraz/sextant/workspace/appkit-r5-candidate.7QEdhc/tmp go test -mod=readonly ./cmd/fathom -run '^(TestAppkitTraffic|TestObservatoryThemeGuide|TestThemeGuideRejectsNonLoopbackBeforeListening)' -count=1
GOWORK=off GOMAXPROCS=2 GOCACHE=/Volumes/m2ext/code/geoffmilleraz/sextant/workspace/appkit-r5-candidate.7QEdhc/go-cache TMPDIR=/Volumes/m2ext/code/geoffmilleraz/sextant/workspace/appkit-r5-candidate.7QEdhc/tmp go build -mod=readonly ./cmd/fathom
GOWORK=off GOMAXPROCS=2 GOCACHE=/Volumes/m2ext/code/geoffmilleraz/sextant/workspace/appkit-r5-candidate.7QEdhc/go-cache TMPDIR=/Volumes/m2ext/code/geoffmilleraz/sextant/workspace/appkit-r5-candidate.7QEdhc/tmp bash scripts/ui-css_test.sh
```

The reviewed `candidate.mod`/`candidate.sum` deltas were applied exactly to
the private candidate's ordinary root manifests; they are not adopted in
active Fathom. The focused test and build passed without `-modfile`, despite
macOS link-version warnings. Before that delta, `GOWORK=off go test
-mod=readonly ./cmd/fathom -run '^TestThemeGuideRejectsNonLoopbackBeforeListening$'`
failed at setup for missing `github.com/rivo/uniseg` `go.sum` data. The graph
still requires review with the exact mirror before active integration.
The synthetic CSS script test passed 11/11 checks. Its fixture proves the
app-owned token source is passed to generation, changes during generation
refuse publication, and a later source mutation makes `--check` report a stale
theme. It does not prove a full CSS rebuild with the pinned Tailwind compiler.
The separate [September 22 real-compiler follow-up](../../../2026-09-22-fathom-css-rebuild.md)
now passes build and `--check` on an isolated copy of this exact source closure:
the theme is byte-identical, with 720 additive bytes in application CSS. That receipt
preserves its first failed read and does not modify this frozen packet or prove
active integration/visual equivalence.

Earlier red findings that the candidate now has named canaries for: missing
registry rendered `services indexed: 0`; unreadable ledger displayed zero
sessions and ON/OFF arm counts; service-only JSON dropped call-log validity;
preview links led to unmounted `/observatory/...` routes; a synthetic
"complete" guide fixture used a non-protocol freshness string and falsely
displayed index freshness unknown. The candidate
separates call-log and ledger failures, preserves measured zero for readable
sources, carries source notes in service JSON, disables only preview links,
and uses the actual `fresh` protocol value with an `all fresh` canary.
These statements describe scoped source/tests, not browser certification.

## Source identities (SHA-256)

| Candidate path or artifact | SHA-256 |
|---|---|
| `cmd/fathom/main.go` | `7fd6419fdcb3f08035b805503da1480235aab327e70645d3b6f4dda1e692afdf` |
| `cmd/fathom/verbs.go` | `832551b490217ace18649da861388e5c17b04f5ee6476e04041edec3d1537832` |
| `cmd/fathom/observatory_render.go` | `bac649753bcc86de6ca3a738c12efb438ac98443bcf7ff907fc53cb7436fe0b8` |
| `cmd/fathom/appkit_styleguide.go` | `b76ce3f3cd8ad9cffeaa211ee8211bc0292c7ac56a542910df93398865f41afb` |
| `cmd/fathom/appkit_styleguide_test.go` | `3b409ba279385df5d1968148f2c476465db1b0e6928c506323ba32558bc4329e` |
| `cmd/fathom/appkit_analytics_acceptance_test.go` | `57e238446e30eb5f9f5d77dab9d2fd4d5c5347d64f0245644494b6f7f0814f97` |
| `cmd/fathom/ui/guide-baseline.cue` | `443461ce94d565b18ea6917631f45fed689a8248839533ec15b2dd328c5267bf` |
| `scripts/ui-css.sh` | `c7674ba638cd7f1d0d596faf092dc272c88c6a166297b2e6eff964e5283973d7` |
| `scripts/ui-css_test.sh` | `0eeef07407340afd14d47d218fb73046d967f46202e41b6fb4cef4863f437407` |
| `candidate.mod` → prospective `go.mod` | `d71e0b8146c9cc65f86c887fc8b8b2eb39bc35e664af7ab6ce264a98fb8c9db1` |
| `candidate.sum` → prospective `go.sum` | `56267941161457eb60a16b219fc425ec3de7a750294b34d10edde37250f52e79` |
| `theme-guide-and-analytics.patch` | `a841a76e64fa089ae5b8a4aed66a56756d84e5bcfb975b62e2ca431b4b7387e7` |

## Still required before integration or acceptance

Review the exact archive/module/pin, app-kit adoption metadata and Fathom
guidance/changelog together; repeat the normal `GOWORK=off` build after
integrating the reviewed graph and exact mirror into active Fathom. The
private normal build is evidence, not active integration. Rebuild/verify
generated CSS from the app-owned CUE source and the embedded closure. Run
scoped race/vet and the existing Observatory regressions on the applied source, plus a
controlled browser pass for both schemes, narrow/zoom, navigation boundary,
preview/reset/refusal/export and actual CSS loading. The subsequent
[browser acquisition](browser-results.md) passes13 assertions through export readiness,
but the native ZIP event times out; retain that failed gate and its diagnostics.
No full-suite/browser acceptance, human task-time comparison, second-app propagation,
consumer commit/push or deployment is established. G3/G4 remain open.

The separate [September22 native-export follow-up](../../../2026-09-22-fathom-native-export.md)
passes pointer/Enter downloads on this exact frozen executable. Reuse its retained
ZIP/event/cleanup oracle rather than repeating the diagnostic; the broader workflow,
active integration and acceptance requirements above remain. Historical failures are
unchanged, and the follow-up does not identify their cause.

The separate [September 22 scoped-regression follow-up](../../../2026-09-22-fathom-candidate-regressions.md)
passes race testing (36 top-level tests, 104 test/subtest passes, no skips) and vet
on an unchanged isolated copy of this frozen candidate. Source/mirror identity and
cleanup pass. This supplies the candidate prerequisite, not active-source verification
or the broader acceptance requirements above.
