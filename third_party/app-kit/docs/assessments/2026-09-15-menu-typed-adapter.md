# P07 typed dropdown and app-owned action proof

## Result and boundary

The seventh private typed adapter turns the frozen dropdown into app-supplied links
and an optional explicit POST/CSRF action, retaining an unchanged constant reference.
Source/compiler/DOM replay, immutable delivery, CLI refusal/drift behavior and
**302 scoped Chrome checks pass**. Twenty captures include four byte-identical
original/reference open-state pairs. Two cold standalone render builds also reproduce.
Default v1 still fails actual200% first-item focus and anchored height. An explicitly
mounted repair now passes314 normal and210 zoom/resize/lifecycle checks; see the
follow-up below. No target is certified or consumer adopted.
[Guide](../design/menu-adapter.md) and [ADR-0049](../adr/0049-explicit-menu-actions-and-private-reference.md)
define the model and authority boundary. Legacy 439/657 syntax coverage is unchanged.

## Implementation

`internal/designadapt/menu.go` starts with the existing pinned Elements translator,
validates the frozen shape and extracts labels/classes/icon geometry at execution.
Tracked adapter code contains no purchased prose, utility-class list or icon path.
The generated `MenuReference()` preserves original parsed DOM, including placeholders.
`NewMenu` supplies bounded ordered links and optional POST with a hidden CSRF field,
non-submit trigger and disabled state. Inputs are copied; strings are escaped.
`NewMenuEnhanced` separately opts into scoped wrapping, viewport bounds, 44px targets
and visible focus. The original never receives those styles.

POST endpoints are same-origin paths without query/fragment, including empty `?`/`#`
delimiters. Encoded path characters retain path meaning. The app owns token issuance,
authorization, actual effects and recovery. No broker, auth service, browser state
machine or additional runtime dependency was introduced. Elements remains host-provided.

`assets-adapt-menu` has read-only JSON, new-directory export and exact replay check.
Six immutable 0600 files live in a 0700 directory; existing outputs are refused.
The shared catalog now has 32 entries and seven private adapter references. MCP and
Gallery expose the same source-free, experimental CLI metadata without licensed data;
there is no web-profile, preview or permanent reusable-layer promotion.

## Current retained evidence

Private root: `/Volumes/m2ext/appkit-menu-proof.Vyw3imOW/probe.b5g2px7k`.
Use command-scoped `TMPDIR=/Volumes/m2ext/appkit-menu-proof.Vyw3imOW`; the internal
Data volume was nearly full. No global temp/cache setting or source archive changed.

| Evidence | SHA-256 |
|---|---|
| Original P07 | `10ed82d0c337ad246f1ab8687d73cc0a7dc5d834bb7b3ec46a01bc7c5d5f144a` |
| `delivery/artifact.json` | `7917bf4253fe81b0ef97da581a39a92cc834a024b21c59656e5b36385f0a3ea0` |
| Composite adapter rule | `63f92b44d90aa2f1704df3f317238f67f978b387c962e11d3b2aa1285e8132ed` |
| Generated template | `336e61c82c3dfec467afa64013499f255becc98c452ca9e1d8345946ef4b56c3` |
| Generated model | `2b8b008eeb76824915e79d4db402f8478466965a5679dbdab094655e87b6d639` |
| Enhanced CSS | `3aca3fc896b42ae4d0ee9bbfafbba5b434e061837cedab884fd3d7568a5d957b` |
| `compile-receipt.json` | `90258d4ab9d513c43eb548ae057b2cfeb8de58e70d59c6b339fbf706e62081cc` |
| `browser-joAWxO/receipt.json` | `83ee58395928f68d0114907e522123bef9839e21f9a7630de6cb12a88be36bf0` |
| Browser harness | `b37403f0656b7133ac0828fbb83bf587b942eb514b7bc80b23389782537d9000` |

`TestAdaptMenuLive` rechecks original archive/catalog before/after, replays the
artifact, tests no-clobber/missing delivery, compiles before/after templ formatting,
compares the constant original DOM and checks typed links/button/form/token structure.
Its receipt binds all six rendered fixtures, source, artifact and runtime. It is
an explicit private-source test, not a default-test skip counted as a pass.

The actual CLI function/dispatcher live test passes with 17,123 JSON bytes and six
files, silent export/check success, refusal exit 2 and missing/drift exit 1. This is
in-process CLI coverage, not a separate installed-binary invocation claim.

## Browser task evidence

Chrome 152.0.7977.84, Playwright 1.63.0, 390/1440 × 1000 CSS pixels, light/dark,
reduced-motion preference and the retained InterVariable test font. The host supplies
exact Elements 1.0.22 bytes and locally compiled Tailwind 4.3.0 CSS. No asset download,
owner profile, real account or consumer repo was used. Browser-protocol keyboard and
pointer input exercises native links/forms; this is not OS/physical input proof.

- Tab/Enter entry, arrow movement, Escape focus return, pointer open/outside dismissal.
- Original/reference/typed/enhanced and long-label variants; independent image settling
  before exact original/reference comparison, never threshold or mask relaxation.
- Focused enhanced items: 44px or taller, contained text/geometry, solid 2px inset
  outline. Sampled sRGB-converted outline contrast is 16.13:1 light and 14.67:1 dark.
  These checks cover the focused first item, not every component/state's contrast.
- Native link navigation, native POST with exact CSRF body and 303 result; deliberate
  403 rejection, same-scheme return and an explicit successful retry exactly once.
- Disabled POST is skipped by keyboard and a real pointer attempt causes neither
  navigation nor a request. Link-only output contains no form or hidden token.
- Wrong-token and cross-origin POST controls return 403. The receiver records only
  synthetic decisions, not real identity/authentication or external side effects.

All 302 checks pass, with no page errors. The report binds 268 allowed loopback
requests and 13 synthetic action observations (12 browser POSTs plus bad-token control).
Twenty image hashes were independently rechecked; the narrow long-label light menu
and desktop dark menu were visually inspected. Browser PID 20675 is absent and the
owned listener refuses connections. The owner's existing RAC preview remains live.

## Retained failed observations

Under prior root `probe.DpTjrriI`, `browser-NrAgOj` stopped after seven checks because
the menu legitimately covered the supposed outside-click button. The revised probe
uses the unobscured heading, never forced clicks or DOM focus injection.
`browser-0tAEog` recorded 272 passes/10 failures: immediate animation polling sampled
the source's 0.95 scale/transparency, and a numeric RGB parser misread OKLCH channels.
The final probe waits for three settled samples and independently stable screenshots;
the browser converts computed CSS colors to sRGB. Source/companion CSS was not changed
to mask those observations. Failed receipts/images remain, not accepted fidelity proof.

## Remaining gates

Default v1 actual200% zoom fails; the explicit repair's scoped follow-up passes.
Current evidence does not establish touch/screen readers, every
focus/contrast state, repeated typed-instance/remount isolation, Datastar lifecycle,
runtime-failure fallback or real consumer
integration. Original small controls remain source findings. No application gets
automatic auth, persistence, mutation retry or no-script menu behavior from this adapter.
Performance budgets and broader family/target certification remain open.

Focused default and live tests pass. The affected package race run passes for
`internal/designadapt`, `appkit/catalog`, `cmd/appkit`, `cmd/appkit-mcp` and `apps/gallery`.
No commits, dependency upgrades, global changes, deployment or publication occurred.

## Standalone consumer replay

`fixtures/menu-adapter/standalone.mjs` runs from
`/Volumes/m2ext/appkit-menu-proof.Vyw3imOW/menu-standalone-YsjEWH`.
Receipt SHA-256: `ee076b404185bc29dee829f06751c9b49a3dd1593966720dd720ff99d73486fc`.
Harness SHA-256: `b9edded867612793a4739eb1758932cbaf2d04e6e92f4e4088e71bd2dc98d0b9`.

The 24 completed commands use Go1.26.3, Node26.3.0, pinned templ0.3.1020 and two
independent empty build/module caches. Public ZIP inputs come from the existing local
cache; network denial is exercised against an independently reachable local canary.
Kit-source and pristine-delivery reads are denied inside the consumer build. Proxy
write denial is configured but has no independent attempted-write canary.

Only copied private template/model/defaults and an exact typed-render oracle enter
the app-owned consumer. Runtime module discovery finds the consumer and templ, no
app-kit import or replacement. Both runs execute the named render test, reject blank
app-owned routes, check links-only/disabled semantics and preserve the enhanced wrapper.
Withholding generated code must fail with undefined symbols. Original delivery checks
pass before/after; input, fixture, module and sum identities remain unchanged.

| Identical output in both runs | SHA-256 |
|---|---|
| Generated Go | `6da91ff962cd70120a0046ce0402b5bfc1b1a807437a660cb5423e11a5d7c168` |
| Source-styled render | `035a56108f5ca9526dae13d4fcc399e2c7f249741b23d6366d7622c3b890d839` |
| Enhanced render | `31a60ad6bd72f681e4d691f22939d8751ec0a77573aa1f780992cc0dd2d74010` |
| Consumer test binary | `635b67a14cf8ff7bcdffe97488e5e65d38e843ef5b3a493584e5ff87299690bc` |

The main agent independently rehashed all eight output files. This is synthetic
standalone rendering, not CSS compilation, Elements/browser behavior, release or
real consumer adoption. The CLI used for before/after checks was built from current
kit source separately; that does not weaken the consumer's source-isolation boundary.

## Actual zoom failure

`fixtures/menu-adapter/zoom.mjs` uses a fresh temporary Chrome profile and selects2
in Chrome's appearance setting. Runtime checks confirm1440 outer/720 inner CSS pixels,
DPR2 and visualViewport scale1. It reuses the exact normal-browser receipt, compiler
receipt, rendered fragments, source CSS, font and Elements pin; no fixture output is
silently regenerated or repaired.

Latest completed task run: `probe.b5g2px7k/zoom-WOUp0B/receipt.json`, SHA-256
`62dacffa31cf38e182a83a9bfa9050b6db55ceb4ce903e03a89c2efce0805944`.
Executed harness SHA: `1f259c8a51e92fc9ed503f32688eea37c9740c5838508c663e1f75999f940477`.
It records22 passes and one hard first-item-focus failure. Normal-length light-menu
items pass all four focus/wrapping checks and its protected POST completes. The valid
long-label menu then leaves activeElement on `EL-MENU`, while the first link has
tabindex0. Independent source inspection finds a deferred container-focus call and
immediate first-item selection in Elements, consistent with a focus race; CSS alone
is not the claimed cause of this observation.

The same captured geometry independently proves the height defect: menu top136px,
height468px, bottom604px in a500px-high viewport. The enhanced height limit uses the
whole viewport instead of available anchored space. The failure capture was visually
reviewed; the menu lacks first-item focus and its bottom action is below the viewport.
Inputs were rehashed unchanged even after failure; browser PID26529 is absent and the
owned listener refuses connections. Dark and later variants did not run after the
failure; no pass is claimed for them. Earlier diagnostic runs retain the same failure.

The harness now also records anchored containment as an independent assertion, so a
focus failure cannot hide it. That additional assertion has not executed yet: the next
run, `zoom-67hVZ6/receipt.json` (`12f95a563c912d3b2d956e5a914150403bb2a6e75d74a9bd4e76ebeeb9a80a1c`),
failed during Chrome startup with `No space left on device` and exited with zero checks.
Its ProcessSingleton message is not evidence of another live test browser; the log
records process26684 exiting21. No profile deletion or global changes were used.

INC-41 and the [repair boundary](../design/menu-adapter.md#confirmed-zoom-failures-and-repair-boundary)
keep both defects visible. The normal302-check receipt and standalone builds are valid
within their scopes, not evidence that actual zoom or all menu interactions pass.

Historical operational handoff: the Fathom continuity-note write also failed with ENOSPC at
`/Users/geoff/.fathom/threads.jsonl`; its older brief is not this slice's current state.
Use this assessment and INC-41. Keep the owner's RAC preview PID88496 at
`http://127.0.0.1:56589/theme-guide`; main verified it alive after the failed launch.
All new scratch paths use command-scoped external TMPDIR/GOCACHE. Do not remove the
older retained hero disk-full evidence or user data to recover space without authority.

## Initial internal repair candidate, before delivery

[ADR-0050](../adr/0050-explicit-menu-geometry-and-focus-repair.md) records the scoped
companion's boundary. `internal/designadapt/menu_repair.mjs` measures available space
above/below the trigger, leaving placement to Elements, and restores only its existing
sole enabled tabindex0 item when the still-open menu container retains focus. It never
handles keyboard/pointer actions or submits data. Scoped resize/style observers and
one frame follow size, scroll and runtime position-only changes. An idempotent disposer
cancels pending work, removes listeners and restores owned styles/markers.

`node --test internal/designadapt/menu_repair_test.mjs` passes21 synthetic tests,
including position-only updates added after independent review. This is fake-DOM and
pure-geometry evidence, not a browser fix. The maintainer task now runs this suite.
The old six-file export, generated Go, source/runtime pins and catalog count do not
change. `zoom.mjs ... --repair KIT` explicitly labels the candidate and hashes the
companion module/CSS/bootstrap while preserving the original failing assertions.
That variant is syntax-checked but has not run in Chrome; no new pixel/task pass exists.

| Candidate input | SHA-256 |
|---|---|
| Module | `3de94e91ab8e79a84de18780d63c429d99c02c85f6b898b90d0b9bd5eec47935` |
| CSS | `2f98b4af12b271d74b1ec551dabb40e243ab82395e1c0c5f8e657c94118c2cd6` |
| Tests | `6f708b6cc84393521ef7fa1a8425d98e41fcdca35a69f7d08e26aa5961f35b27` |

A further isolated hello-page launch with external profile/temp/crash directories and
command-scoped redirect/disable flags still failed before page load with ENOSPC;
Chrome PID27378 exited21 and was confirmed absent. The external probe root
`/Volumes/m2ext/appkit-chrome-probe.0O418fGKBB` was retained. O21 records Fathom symbol
lookup failures against its internal-volume database. No global workaround was applied.

Read-only inventory found an inactive, cache-only969MiB Go build cache at
`/private/tmp/appkit-clean-build.LnJhBh/build-cache`; no open-file/process reference
was found. At that stage relocation approval and browser verification were pending;
the later authorized relocation and verification are recorded below.

After the companion change, `TestAdaptMenuLive` passes both pinned compiler phases,
original/reference DOM and immutable delivery checks. Targeted MCP decision/status/
index/guidance and document protocol tests also pass. These are explicit executed
checks; the earlier `Menu|KitDocs` filter selected no MCP tests and is not counted.
The21 Node tests, selected links/budgets, new ADR shape and JavaScript syntax pass.
At this earlier stage no browser repair pass, full maintainer run, consumer change
or G1–G6 completion was claimed. Review confirms the companion is absent from v1 exports and
the repair-mode browser harness preserves the original assertions.

## Browser-proven explicit repair follow-up

The owner authorized relocating only the inactive Go cache. All7,031 files were
hash/byte verified before removing originals; copy and receipt remain at
`/Volumes/m2ext/appkit-cache-relocation.1AhLVW/`. Sibling files and preview PID88496
were preserved. Internal space was already about15GiB before that relocation;
do not attribute all recovered space to it. Chrome now launches, Fathom resolves
`AdaptMenu`, and continuity notes can be written. No global configuration changed.

The companion module/CSS hashes above remain unchanged. Chrome152.0.7977.84 and
Playwright1.63.0 use the same source, runtime, font and compiler-bound fragments.

| Retained receipt below `probe.b5g2px7k` | Result | SHA-256 |
|---|---|---|
| `zoom-hNfyMj/receipt.json` | unchanged v1:23 pass,2 fail (focus and containment) | `7cdfe1fd07f8487c5e22ff4a60f3dad8ad566205800a31ee3229e58efd59f1ea` |
| `zoom-V0k8ZY/receipt.json` | initial explicit repair:136 pass,0 fail | `999f96da3b022a7353f140c63baeca2d2ddb5e8407e8f45c854f9bb379029f4c` |
| `browser-KTBdJj/receipt.json` | normal repair:314 pass,0 fail | `bae1c06dfb6f8cb3ee07e667c9a540877e6e02a82be03d790eff8a41a0f3b18d` |
| `zoom-d5n5oy/receipt.json` | expanded repair:210 pass,0 fail | `3e0fd0ba4543df812de7c06a22b20241dee8c412ecf90b8fc729f84f8b162560` |

Normal mode preserves all302 task/assertions and adds12 original/reference/source-styled
isolation checks. Four original/reference open-state image pairs remain byte-identical.
Actual zoom confirms1440 outer/720 inner pixels, DPR2 and visualViewport scale1.
Every enabled item stays focusable, readable and contained through resize to390 inner
pixels and back; native links and protected POST/303 complete in both themes.

A real host layout shift moves the trigger from72px to292px. Elements flips the
268px menu above it to16–284px, preserving the8px gap and16px edge. Two explicitly
mounted instances retain independent selection and sizing; an unmounted reference
on the same page retains its measured styling before/after other-instance disposal.
Duplicate mounts are refused. Actual browser lifecycle fault injection enqueues focus
and resize work, then disposes before callbacks, verifies exact host marker/value/
important-priority restoration, closes pending work and remounts/removes one instance
while the other remains interactive. These injection checks are distinct from the
keyboard/pointer task checks; no physical-input or Datastar-morph proof is inferred.

The expanded zoom harness hash is
`d31267e61c5e46196ff417a78e9b2c720fcab48a08906838be56139afce350e9`.
Its test-only bootstrap exposes disposer handles; that handle is not product code
or proposed consumer delivery. All reports retain input hashes and cleanup proof.
Main visually inspected light/dark long-menu captures and the dark upward-placement
capture. Browser hosts/listeners closed; the owner's existing preview stays live.

Retained fixture failure: `zoom-7pmAa8/receipt.json`, SHA
`fa0763850221c8de0d8ca27fda254eb37d8c0f81b6348db5a4b2650d2d7eb317`,
records87 passes and a failed upward-placement assertion. The test put margin on a
`display:contents` wrapper, so the trigger never moved. The corrected probe moves
the real host layout container and first asserts its220px displacement; no product
code, runtime bytes or containment threshold was relaxed.

[ADR-0051](../adr/0051-explicit-private-menu-repair-delivery.md) records the additive
eight-file private repair-delivery contract. Browser proof alone does not establish
delivery replay, real-consumer adoption, full accessibility/performance or G1–G6 completion.

## Delivered repair and actual CLI proof

`assets-adapt-menu -repair` now selects `p07-menu-repair-v1`: unchanged nested base,
bounded repair module/CSS with hashes, implementation identity and8/16px mount defaults.
The eight-file private inventory is template/model/base CSS/defaults, repair module/CSS,
explicit usage and artifact. The app supplies its own bootstrap and retains disposal;
the test-only browser handle is never exported. Default six-file delivery is unchanged.

Private proof root: `/Volumes/m2ext/appkit-menu-repair-delivery.geXhmN`.

| Artifact | SHA-256 |
|---|---|
| Actual built CLI | `d0a424b0fa64edc6be3d8c330a418510f43520966cfd1bd703f91474944b6819` |
| Repair implementation rule | `e73a688b5ce90f5e106db5d034c75b572876c3b8cb051bc00c4eed73258cd5c0` |
| `delivery/artifact.json` and identical `replay/artifact.json` | `9d18b4403ec554c1f1f40f76776dba730f5d9a8a16f0bb4228c9ab2d80754d5e` |
| `receipt.json`, nine actual CLI invocations | `ec8c6f9f38915f9f7bb80e28491555a9bd4114d0c3e755e5341b0d23900d239a` |
| Delivered runtime `probe.b5g2px7k/zoom-JdhTxt/receipt.json` | `4c23f608878410158522afef51132cd2e2eec13b333324fe02a9f05365321554` |

The actual CLI checks the previously retained six-file artifact, exports and checks
the new selection, refuses overwrite with exit2, refuses both cross-variant checks
and missing delivery with exit1, and produces an identical second eight-file export.
All modes, source/default bytes and nested base match. Source archive/catalog,
go.mod/go.sum, original delivery and first repaired delivery remain unchanged.
The receipt also independently rehashes30 normal/zoom captures and binds the exact
exported module/CSS to both successful browser suites.

`zoom.mjs ... --repair-delivery PRIVATE_DELIVERY ARTIFACT_SHA256` reads the actual
eight-file inventory with externally supplied artifact identity, exact nested base,
private modes and per-file hash checks. It serves the exported module/CSS, not the
internal checkout copies. All210 existing zoom/resize/lifecycle checks pass again;
input rechecks and host/listener shutdown pass. Current harness SHA:
`eface789339149c3299740f8a691949f9db52fb254b2ecee0f2182697a0befc7`.
This is direct delivered-companion proof under a synthetic test host, not a real-app
bootstrap, standalone CSS build or consumer migration.

All10 delivered-runtime captures were independently rehashed;9 are byte-identical
to the internal-companion run. `dark-shifted-anchor.png` differs at817 pixels with
maximum channel delta1, within x624–1391/y287–540. Both captures were visually
inspected; the cause is unproven and no tolerance/baseline update was accepted.
This does not invalidate the measured task/geometry results or the four exact normal
original/reference pairs; complete cross-run pixel reproducibility remains unproven.

Focused menu/repair race tests and the explicitly enabled private-source CLI race
test pass (base JSON17,123 bytes; repair23,050 bytes). The full catalog race suite
and Gallery race suite plus selected MCP document/guidance/protocol race tests pass. A missing secondary
decisions-index row failed first, then was fixed; a no-tests catalog filter was not
counted and was replaced by the full package run. Node21 tests remain passing.
Selected intent structural checks pass:10 root living docs within budget,338 relative
file links resolve, new ADR shape,60-line Unreleased block and identical intent stanzas.
This is not a full intent-doctor, host-configuration or architectural-truth audit.
No dependency change, consumer mutation, commit, publish or deployment occurred.
