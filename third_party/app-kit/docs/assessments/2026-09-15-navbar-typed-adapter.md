# P09 typed responsive navbar: evidence and limits

## Result

Frozen P09 now has an exact `NavbarReference` and a typed `NewNavbar` sharing one
desktop/mobile primary-route model. Optional account/notification destinations are
app-owned GET navigation, never inferred mutations. [Guide](../design/responsive-navbar-adapter.md)
and [ADR-0057](../adr/0057-explicit-responsive-navbar-and-route-identity.md) define the contract.
This is the ninth experimental private typed adapter, not a certified target.
The latest explicit repair adds independently bound normal/actual200% runtime proof,
nine-file CLI delivery and two cold standalone builds, recorded below. Earlier failed
results remain historical evidence, not current repair verdicts or waived gates.

## Source, compiler and delivery

Private compiler root: `/Volumes/m2ext/appkit-navbar-proof.Sea12gFt/probe`.
Command-scoped TMPDIR is its parent, with an existing external Go cache and offline
module resolution. The first invocation refused a missing output directory before
writing; after explicitly creating the required empty 0700 directory, the probe passed.

`TestAdaptNavbarLive` passes both unformatted and formatted templ phases, exact
original/reference DOM comparison, typed DOM assertions and generated model race tests.
The original archive and private catalog hashes remain unchanged. The compiler receipt
SHA-256 is `d2d19352a2ef101db2a8351c1015afaedbb4ee4b0daf438669b5d45585072062`.
It binds the source, adapter rule, runtime, generated files and five rendered specimens.

Synthetic tests cover identity/shape/attribute refusal, exact replay, malformed URLs,
bounded labels and collections, duplicate/unknown route keys, no-current state, optional
controls, deep snapshots, escaping, cancellation and writer failure. Typed DOM tests
check both current-route presentations against original current/default class lists,
non-submit triggers, named landmarks, image semantics and independent instance IDs.

`TestAssetsAdaptNavbar*` passes with the explicit private root, including deterministic
JSON, five-file export, silent exact checking, existing-output refusal (2), and missing,
symlink and mutated-file drift (1). These are live in-process command-dispatch tests,
not an installed CLI or a standalone consumer build. No dependencies or pins changed.

## Browser screening and oracle review

The maintained [probe](fixtures/navbar-adapter/browser.mjs) uses the approved isolated
Playwright harness and installed Chrome, fresh profiles and synthetic loopback destinations.
It pins Elements 1.0.22, Inter and the existing Tailwind 4.3.0 binary. The original's
three remote image URLs remain blocked; synthetic typed images are explicit fixtures,
not a recovered source-image baseline.

Initial run `probe/browser-fuQtWT/receipt.json` has SHA-256
`5e8f1182a4c9c63342c2dcb89f65d74ad735f4ceabe3b74f99185198f60998b1`:
204 passes, four failures, zero runtime errors; inputs unchanged and owned processes closed.
Inspection traced all four geometry failures to the checker including the notification
link's clipped screen-reader name/hit-target spans as visible content. Phone/desktop
captures show the long route/account text fitting these samples. This is an oracle
false alarm, not evidence that a component repair occurred.

Independent review also identified insufficient external-traffic classification,
responsive-group exclusivity and line-fragment/vertical geometry checks. Strengthening
those checks is separate from changing the component; the initial receipt remains intact.
Follow-up results must name the new harness identity and demonstrate seeded failures.

Final run `probe/browser-Tn5991/receipt.json` has SHA-256
`2dfad48ffd46e6187ab4809b3a7dd65675a1af48fdbf1702c1e01ca1102dbe48`:
**238 passes, zero failures/errors**, on Chrome 152.0.7977.84 at 390×1000 and
1440×1000 CSS pixels, light/dark and reduced motion. Harness SHA-256:
`7388a1e05b53a4eb907081ba449af0aa043167ac88718256d50d60d977348c3e`.
All 34 retained outputs, including 20 PNGs, were independently rehashed and checked
as single-link 0600 files. Inputs remain unchanged; owned Chrome/listener are closed.

The run covers original/reference screenshot equality, keyboard disclosure/menu
navigation, Escape/focus return, four native synthetic GET destinations, omitted
controls, two instances and three remounts per lane. Actual visible text-node line
fragments are checked against their controls, viewport and open popover. The exact
clipped notification name is validated separately. The expected three source images
produce 24 named CSP refusals across eight source/reference lanes; no external request
reaches routing. This is deliberately not a source-image fidelity pass.

Three classifier canaries use synthetic facts. Two additional browser canaries mutate
the actual DOM: expose a hidden desktop group at 390px, and constrain a visible primary
link so its text overflows. The same collectors/classifiers refuse both defects and
pass after restoring the exact prior styles. No component, fixture size or tolerance
was changed to obtain a pass. Intermediate `browser-0GKIQH` (231/5) and `browser-vmUjkx`
(236/0, before DOM canaries) remain retained, not relabeled. Their receipt hashes are
`2060cfa41f5ea0484db7ee096d72e33f50f15a118abe1156ea4987fc63c5d4d7` and
`77ba40fdcc7b11e9ae28ed8da8ecaa27d9f3adedba87e0cd94da1bdd77b3ce10`.
The intermediate five failures concern the clipped-name classifier and its synthetic
canary's one-pixel tolerance boundary, not a changed navbar implementation.

## Two cold standalone consumer builds

The maintained [rehearsal](fixtures/navbar-adapter/standalone.mjs) and
[consumer test](fixtures/navbar-adapter/consumer_test.go.txt) pass in
`/Volumes/m2ext/appkit-navbar-proof.Sea12gFt/navbar-standalone-9ZORuX`.
Receipt SHA-256: `9b6c4720060846e8367c6c2003438493dfee830186369b80525fc66deffec9c2`.
Harness SHA: `c3453435ed9f006b7c3b41f6d998fd9694b4b4513b725197a7a447e55a7ff9f4`.
The privately built CLI SHA is
`95112d0d2676030c8ab386be62849fcb795fb0d63f387e37661c5143af1342c6`.

All 26 commands produce their expected outcomes. Both fresh build/module-cache roots
generate templ, compile and execute `TestDeliveredNavbarConsumer`, verify public modules,
and inspect actual-used compiler/renderer/consumer module directories. No app-kit import,
fleet module, replace directive or sibling checkout supplies runtime code. macOS sandbox
canaries prove kit/pristine-delivery read denial and network denial against an independently
reachable loopback fixture. Withholding generated code makes each consumer build fail.
CLI delivery checks before and after are silent/matching; originals and pins are unchanged.

| Byte-identical output in both builds | Bytes | SHA-256 |
|---|---:|---|
| Generated navbar Go | 209,840 | `438c1467885e7243efbb2b39ceeb307eadb6ca504c980006168733887080af05` |
| Typed HTML | 6,283 | `66e7119f77b811685830833b26312bf038d94c0a41c66d7902524c185305cbdc` |
| Minimal HTML | 4,271 | `b2293f6e244dc53cb8ec8ccf896b9fc4f672fc46879cb28d60548c0b8e75d3e0` |
| Consumer test binary | 7,110,994 | `df5bb90408ce2c46e83aca69cc98bd5e53c429450bbcb1734be70e6c298e6a03` |

Main independently rehashed all eight output files and command logs. Go is 1.26.3
darwin/arm64 and Node v26.3.0. Public ZIPs come from an existing read-only local proxy
and are checked against pinned sums; live sumdb access is denied, not claimed verified.
This proves standalone synthetic rendering, not standalone CSS/runtime delivery,
real-app adoption or performance. The owned network-canary server is closed.

## Discovery and remaining gates

The shared catalog contains 34 records, nine typed adapter metadata entries and the
separate Elements translator. `reference.navbar-adapter` exposes experimental CLI usage,
private source requirements and app-owned seams. It declares no web profile, permanent
layer promotion, consumer adoption or certified target. Legacy 439/657 results are unchanged.

The five affected packages (`internal/designadapt`, `cmd/appkit`, `appkit/catalog`,
`cmd/appkit-mcp`, `apps/gallery`) pass race tests and vet. Shared discovery tests cover
exact metadata/revision, new guide/ADR retrieval, absent licensed stores, bounded search
and refusal of web/available/proven promotion. The initial integration test failures
identified missing gap wording and stale eight-adapter expectations; both are corrected.
The scoped Intent Doctor subset passes ten living-document budgets, all 57 ADR/index
and compact-register bodies, identical agent stanzas, 60 Unreleased lines and 359 relative
linked paths. This is not a full tool-version/host/skill-wiring or semantic intent audit.
The root `go test ./...` also passes after integration; opt-in private/runtime skips
are not promoted by that command. Private source/CLI/browser/standalone proof above
ran separately. P09 captures are retained privately, not yet imported into Gallery's
historical image-evidence catalog. Existing preview binaries were not replaced.

Original-image fidelity, assistive/physical input, full focus/contrast, runtime-failure
access, Datastar lifecycle, performance and standalone CSS/runtime or real-consumer
acceptance remain open. A bounded text/viewport check is not whole-page visual conformance.
No consumer, source archive, dependency, global configuration, deployment or commit changed.

## Actual 200% zoom, focus and runtime outage

The separate [diagnostic](fixtures/navbar-adapter/zoom.mjs) preserves the normal
238-pass harness, exact typed base, source/runtime/font bytes and five-file delivery.
Final retained failure: `probe/zoom-t4U6Hs/failure.json`, SHA-256
`d537109542046601845d0e53f45dbf6dea92efc74abe0138490cd1da731e412f`.
It completes **240 passing checks, 38 failed checks and zero operational errors**.
This is a failed quality gate, not successful fallback or target certification.
Harness SHA: `8b840c6c978d6a2ebc1820c76d9decd4eb063948b1ed12b39a30f25d28b1c2f3`;
local capture-helper SHA: `8fc2f4574c7806cc993ec3c4443355a3f5427d14934ce82aa5b4f4b04b6e2bd5`.

Chrome152.0.7977.84's Appearance setting is actually 200%, not CSS zoom or relabeled viewport
emulation. Outer windows are 1440/780 by 1000; measured CSS viewports are 720/390 by500,
devicePixelRatio2 and visualViewport.scale1, light/dark with reduced motion.
Keyboard input is browser-protocol input, not physical, touch or assistive input.

| Failed check group | Count | Observed limit |
|---|---:|---|
| Conservative focus screen | 24 | eight primary-link cases retain thin browser-auto cues extending beyond viewport edges; sixteen account-item cases use low-contrast background changes without qualifying outline/ring |
| Long-label geometry | 4 | the desktop long route escapes the fixed64px bar at720 CSS width, before and during account-menu inspection in both themes |
| Runtime-loss essential access | 10 | an explicit module HTTP503 leaves account destinations hidden at both widths and primary destinations hidden at390 CSS width |

Native desktop primary links and notification destinations still navigate by keyboard
under the outage. Sixteen exact module503 responses are named expected events, not
ignored network errors. Runtime-owned regions stay wholly hidden, rather than falsely
reporting action success. No account backend or external Projects destination is activated.

Focus measurement records pre-keyboard unfocused styles, actual induced outline or
inset/outset shadow candidates, composited translucent backdrops and clipping.
The mobile toggle's real inset ring is accepted. The two-pixel/3:1/24px conservative
screen is not a blanket WCAG violation classifier. Actual keyboard canaries accept
induced paint, reject a permanent ring after blur/baseline/refocus, reject absent or
clipped paint and verify two translucent layers. Exact inline styles are restored.
The local capture contract has Node rejection tests; no frozen shared helper changes.

Thirty-four PNGs and raw facts are retained. The initial `zoom-cFoEkK` run (63/11)
stopped when the shared1440-only capture helper rejected the narrower window;
receipt SHA `a0fefb30de8830bd0311cd854eb8ce3819a7d3726cf296656fc7168f7f7e3d0b`.
That is a harness limitation, not a component failure or completed lane. Intermediate
`zoom-iZG6pH` (237/38, zero errors) remains, SHA
`20698f1ac86c0496bad9419b8d1104591a25ff925bd38dd05f2fe6bdbd8f3cd9`;
the final run strengthens actual keyboard/permanent-ring and compositing canaries.
An independent reviewer rehashed all34 single-link0600 captures (1,520,899 bytes)
and all15 inputs, confirmed all four owned Chrome PIDs absent and checked cleanup.
All34 final captures match the intermediate capture hashes; main visually inspected
the identical light desktop long-label/account and dark narrow primary/account images.
Capture screening is not a visual-fidelity or whole-page accessibility pass.

These observations fire ADR-0057's first revisit condition. Any repair must
preserve exact original/base access, fix measured geometry/focus without taking over
Elements interaction, and provide explicit essential-destination access during runtime
failure. The base remains unchanged; the separate candidate below is not promotion.

## Explicit native-route and layout/focus repair candidate

[ADR-0058](../adr/0058-explicit-navbar-repair-and-native-route-access.md) chooses a
persistent native directory over runtime-driven visibility switching. A synthetic
Chrome experiment found that revealing undefined hidden/popover elements works until
registration hides the focused link and focus moves to BODY. The chosen companion
does not perform that hiding or introduce a readiness/focus-transfer state machine.

`NewNavbarWithRoutes(Navbar, summary)` wraps the exact base with selected geometry/focus
CSS and a native details/summary directory using the same validated/snapshotted routes.
The repair allows long desktop routes to grow/wrap and gives failing links contained
focus paint. A distinct CUE envelope binds base, rule, CSS, companion Go/templ and
wrapper. No CLI variant, permanent layer/profile or consumer pin changes.

Compiler proof: `/Volumes/m2ext/appkit-navbar-repair.RlCczDk0/probe-v2`, receipt SHA
`91171695b41fe40b9e2982d834520ac1ef508a1ba6777bc4a924ee5d3c796a61`;
candidate artifact SHA `77a5a32f909eeb1403be5eff43525d0f581d983d070e612ec6678b9b9532f581`.
Both templ formatting phases compile and run model/DOM tests with race checking.
Tests cover nested replay/refusal, validation, escaped summary, slice/pointer snapshots,
native route ordering/current state, absent options, cancellation and writer failure.
The separately replayed base artifact remains
`5559fb79f3d06d312df0a6a7bafe216815b5f34499b28a804862291d3f8cb642`.
Initial compiler failures caught a duplicate concatenated CUE package declaration and
an explicit templ import already emitted by the generator. They were corrected; the
first private `probe` directory and compiler diagnostic remain, without a pass receipt.

The [repair browser probe](fixtures/navbar-adapter/repair.mjs) binds the normal receipt,
the exact38-failure diagnostic, original measurement-function source and new candidate.
Receipt `probe-v2/browser-AUZKJm/receipt.json` SHA
`063d516aa637c9c78426da0e76b1df5f3f1844a88bfe6ef5206ce95719379c1a`:
**386 passed, zero failed, zero operational errors**, actual200% at720/390 CSS widths,
light/dark on the pinned Chrome152.0.7977.84 environment. Harness SHA
`343a81b51f5c7beeaca6abb75e29df8801155b9870f0af8c9e66a71206c42c24`.

Focus and geometry measurements are the frozen base functions, not relaxed thresholds.
The geometry caller selects the enhanced nav, while new checks separately open the
native directory and complete four GET destinations during exact module503 outages.
The enhanced failed-runtime regions still remain unavailable. The added directory is
the explicit alternate path, not a relabeling of those inert controls as working.
Late loading of the real pinned Elements module preserves the focused native Profile
link/open directory and completes its GET in all four lanes. Eighty-eight conservative
focus checks pass. All44 retained captures and raw observations remain private.
Main inspected light desktop long-route/account focus and dark narrow native navigation.

Independent review identified an uncovered long-label native directory. The expanded
run retains the earlier386 result and adds per-link keyboard/focus/Range containment
after native document scrolling. `probe-v2/browser-3hka4c/receipt.json`, SHA
`adf849f125c20154a4fd8162c18feb4af9d763f64811b2662f55483506790b51`, completes
**466 passed, zero failed/errors**, with the same candidate/compiler inputs. Final
harness SHA: `6db51ff8d5f291cfa78f15731a4cc9ce6c47be0eaa2f7d25ace994ed43b900f2`.
All20 long-link text-containment checks and108 scoped focus checks pass. The long
directory lane checks independently reachable/painted links, not an uninterrupted
five-link journey or GET results; typed lanes prove the actual destinations separately.
The reviewer rehashed all52 single-link0600 captures and23 inputs, and confirmed all
four owned Chrome PIDs absent and listener cleanup. Main additionally inspected the
dark narrow long-account route after document scrolling. Component inputs did not change.

The affected designadapt, MCP and Gallery packages pass race tests after integration.
The authored CSS scope/no-hidden-runtime checks run in ordinary Go tests; private
source/compiler and actual-browser evidence were separate explicit opt-in runs.

This is scoped candidate evidence, not complete accessibility, source-image fidelity,
performance, disabled-JavaScript, maximum-bound data, repeated/unselected-instance,
normal100% or standalone/consumer acceptance. The eight-file CLI variant remains
unimplemented. No base source, archive, dependency, global configuration, deployment,
existing preview, consumer or commit changed.

## Supplemental maximum-bound and repeated-instance fixtures

The new retained compiler output is
`/Volumes/m2ext/appkit-navbar-acceptance.hEhhhUJv/probe`, compile receipt SHA
`52cf59d6ba2950c8bef7ddc27032fef20d9d44069da3e1224b50523e739ba3a9`.
Both synthetic and private source pass unformatted/formatted templ generation and
model/DOM race tests. The exact candidate and all eight prior renders are unchanged.
The independent supplemental receipt `acceptance-fixtures.json`, SHA
`9e05ebf16a1e53fbc17fef7f9e2fc913f460e88797a31c47b789dd3ca9d96fcd`, binds:

- `repair-maximum.html`: twelve primary links, twelve account links and a notification;
  256-byte labels/summary, 64-byte identifiers/keys, 2048-byte URLs and 512-byte image
  alt text. The last primary link is current. Seventeen one-over-bound input cases fail
  construction rather than truncating data or emitting partial output.
- `repair-multiple.html`: two explicitly selected companions and one unchanged base
  component, with distinct authored disclosure IDs and independent current keys.
  Elements still assigns its menu IDs at runtime; a new test's initial assumption
  that those IDs were source-authored was corrected against the unchanged source.

This compiler result alone does not establish browser acceptance. The separate
normal-size browser result below retains unresolved keyboard-entry failures.

## Normal-size, no-script and isolation acceptance: still incomplete

The maintained [acceptance harness](fixtures/navbar-adapter/acceptance.mjs), SHA
`5517f72b46b986954ee6001d36388c583b38ff260a03ead9404f51f7f95ee590`, records
**523 passed, four failed, zero operational errors** in
`/Volumes/m2ext/appkit-navbar-acceptance.hEhhhUJv/probe/acceptance-5JN3xE/failure.json`,
SHA `47927d1d646fdab6f84178e53513ba670be043cfed94256be66daff3054bcc99`.
Chrome152.0.7977.84 runs at1440/390 CSS widths, height1000, DPR1 and visual scale1,
light/dark with reduced motion requested. The narrow headless outer window clamps to
500 pixels; this is normal CSS viewport emulation, not an actual390-pixel outer window
or a replacement for the separately retained actual200% proof.

Passing scope includes maximum primary/mobile routes, account items1–11 after arrow
navigation, all25 native-directory links after keyboard scrolling, and all25 full-length
URL query roundtrips through owned GET destinations. Two selected companions retain
independent disclosure/current state. Toggling repair CSS leaves the unselected base's
DOM, measured dimensions and representative computed styles identical; a selected
summary's minimum-size change proves that the stylesheet toggle actually took effect.

Four JavaScript-disabled contexts serve the same module-bearing HTML, retain undefined
Elements tags, request no runtime module, and complete Dashboard, Notifications, Profile
and Settings with native keyboard navigation. Their88 checks pass. No scriptless
behavior is inferred merely from omitting a script tag. These fixtures do not make the
original enhanced controls operational without their runtime.

All four failures are the maximum account menu's entry state: after opening/settling,
DOM focus is on EL-MENU rather than the first link. Later item checks pass. This differs
from the earlier small/long fixtures and needs a focused comparison of runtime focus
and input ordering; neither an arbitrary Home/wrap workaround nor an assumption that
the harness is wrong closes the failure. First-item task acceptance remains unproven.

The supplemental maximum-data contract permits native scrolling, then requires each
focused link's complete text and focus paint inside the viewport and clipping ancestors.
Earlier global-all-links geometry is retained as diagnostic data, not rewritten: every
item in a long scrolling menu need not be onscreen simultaneously. Main independently
reviewed that scope correction before the per-route run. The frozen focus and geometry
functions are unchanged, with the earlier three-stable-sample animation/opacity settling
condition restored. Rapid input during opening and reduced-motion conformance are not
established by these settled-state tasks.

Retained setup/sequence diagnostics under the same probe root:

| Directory | Pass/fail/errors | Interpretation |
|---|---|---|
| acceptance-UNuYpP | 3/1/1 | incorrect notification selector; not a product verdict |
| acceptance-MXJICz | 3/1/1 | notification selector matched both enhanced and native links |
| acceptance-Yxgj6p | 135/5/1 | global viewport assumption, narrow outer clamp and wrong disclosure-state wait |
| acceptance-7ubrK6 | 521/6/0 | first-item entry plus two transient strict-boundary comparisons |
| acceptance-kaclxl | 483/44/0 | changed arrow sequence overshot expected items; not evidence of44 UI defects |

The two transient comparisons exceeded a transformed menu boundary by exactly
0.0000152587890625 CSS pixels while text fitted. They do not establish visible clipping;
thresholds were not relaxed and no component CSS changed to address them.
Both reviewers independently rehashed the final17 inputs and12 private0600 captures;
all nine owned Chrome PIDs are absent and the listener closed. Main inspected repeated
desktop, maximum-label and final dark narrow no-script captures. The three affected Go
packages pass race tests. Original/candidate source, earlier receipts, consumers and
dependencies remain unchanged; CLI repair delivery and full target gates remain open.

## Explicit source-pinned runtime focus repair

The separate passive [focus trace](fixtures/navbar-adapter/focus-entry.mjs) retains
`/Volumes/m2ext/appkit-navbar-acceptance.hEhhhUJv/probe/focus-entry-HN0uaT/failure.json`,
SHA `096ae6ef38d319f04227ecabbda13a9ff7b0e5be7accf1a128c0ecb14f2d8951`.
Its9 passes/two failures are diagnostic, not acceptance: typed fixtures retain link0,
but maximum fixtures focus link0 and then EL-MENU while Elements' active index stays0.
The second focus stack reaches the runtime's deferred container-focus callback.
Home reaches link0 but is not an accepted workaround. Earlier small/long zoom proof
remains valid; it did not cover this maximum scrollable-menu state.

[ADR-0059](../adr/0059-guard-selected-navbar-runtime-focus.md) records the narrow repair.
`AdaptNavbarRuntime` verifies the frozen source and unchanged repair, then replaces
exactly one deferred-focus expression in the pinned original ESM. The callback captures
wrapper selection before scheduling. Selected menus receive container focus only while
connected, open and without an active item; unselected behavior remains unconditional.
No extra event listener, item selection, state owner or altered timeout is introduced.

The independent private replay is retained at
`/Volumes/m2ext/appkit-navbar-acceptance.hEhhhUJv/runtime.XZgbXqWk`:

| Input/output | SHA-256 |
|---|---|
| Original Elements ESM | `120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013` |
| Unchanged paired repair artifact | `77a5a32f909eeb1403be5eff43525d0f581d983d070e612ec6678b9b9532f581` |
| runtime-artifact.json | `2829e0cd5cae7c24db5db4be5e1c6baf2cb2f88b275f8e9f5f0aa1c7a05f1fbd` |
| elements-navbar.js,84812 bytes | `7c7184f9ea5ba3e4842f8624f500cb80d81c3c8621570715cfbd69ecdf1d7179` |
| runtime-receipt.json | `e8e4e06247e1ea3a95fc63dc931c3ca4dcf19a957e962500f5743db8262cfa5b` |

`TestNavbarRuntime*` passes exact-site/size/source refusals, private deterministic replay,
eight changed-envelope fields, schema drift, self-rehashed output and paired-CSS drift.
The actual replacement expression passes19 controlled timer-state cases in Node,
including selection/close/detach after scheduling. These isolated callback tests do not
establish real Elements pointer or lifecycle behavior.

The [runtime acceptance harness](fixtures/navbar-adapter/runtime-acceptance.mjs), SHA
`9d0272706369718a39b5571d77fd35289d1c6eb4a3619c7cd1f24bfaa43b8186`, derives a new
private runner from the frozen acceptance harness. Independent review confirms every
quality function and the entire task block are byte-identical; only declared runtime
input/provenance and output naming differ. It replays the patch against the original,
validates the paired artifact and records original versus served runtime separately.

`probe/runtime-acceptance-zMCwJh/receipt.json`, SHA
`68240dc6ed461404c936030dbc51cd72298a57019ca0485c0bfd4e1c057f42c7`, records
**527 passed, zero failed/errors** at the same1440/390 normal CSS viewports and themes.
All four first-item checks now observe an A element with qualifying focus paint and
complete text containment. The original25 native URL roundtrips, disabled-script tasks,
maximum fields and repeated/unselected CSS isolation checks still pass.

Main independently rehashed24 inputs and12 single-link0600 captures, confirmed all nine
owned Chrome PIDs absent and the listener closed, and inspected dark narrow repeated
instances and desktop maximum native-link focus. These stress images use synthetic
labels/assets, not source-image fidelity or a polished consumer composition.
The affected designadapt/MCP/Gallery packages pass race tests. No existing runtime,
base/repair output, consumer, dependency, preview, commit or deployment changed.

The new derivative still needs its own actual200% and broader lifecycle/target matrix;
the earlier466 zoom passes cannot be transferred to it. Runtime-bearing standalone/CLI
delivery and consumer adoption remain open. Catalog counts stay34 records, nine typed
adapters and zero certified targets; G1–G6 remain active.

### Actual runtime pointer, lifecycle and isolation proof

The separate [lifecycle harness](fixtures/navbar-adapter/runtime-lifecycle.mjs), SHA
`3879d208e711810aadb6eb1744710be20954ebba2a0ccfdb83175dfa0535b93a`, records
**16 passed, zero failed/errors** in `runtime.XZgbXqWk/lifecycle-DBISeb/receipt.json`,
SHA `e4ef09a8380be51e6f6a5cc57be6ea612713e70d02808c3123e87ac4ad5fd628`.
Installed Chrome152.0.7977.84 uses a fresh private profile at1440x1000, with pinned
fonts and settled animations. The source stylesheet's exact font URL is served.

Selected small and maximum fixtures retain keyboard entry at item0, ArrowDown to1,
Home to0 and Escape to the trigger. Actual protocol pointer input opens the menu with
no active item and leaves container focus. Synchronous public-API close/removal before
the deferred callback cannot steal sentinel focus; these two are programmatic lifecycle
probes, not physical input, rapid-user-input or full remount proof.

Unselected keyboard and pointer state matches the original runtime. The short fixture
has scrollHeight/clientHeight80/80. A separately declared synthetic isolation fixture
repeats short account links and supplies identical external eight-rem height/overflow
CSS to both modules; its440/128 measurements prove actual scrolling. Both runtimes
retain the original scrollable keyboard behavior: DOM focus on EL-MENU, active item0.
This intentional non-selection check is not a claim that the original defect is fixed
globally. It does not change the frozen all-bounds fixture or its527 acceptance checks.

Main independently rehashed all17 final inputs and confirmed the owned PID absent and
listener cleanup. No source/runtime/consumer changed during these runs. Earlier fixture
diagnostics remain under the same runtime root; their failures are not passes:

| Directory | Pass/fail/errors | SHA-256 / limitation |
|---|---|---|
| lifecycle-NwIxKV |15/0/27| `8b5c43401fa886bac51373a737ff84d3cda5fb50cc9c091d35c8d5d344bba183`; local asset route mismatches and pointer timeout |
| lifecycle-6zhpyQ |14/1/1| `25fcd1313c3b97ce0cb2318a4149e7d3254a8bbf4df237c6a42ae401f1604eda`; all-bounds unselected source was not scrollable and its pointer trigger was obstructed |
| lifecycle-J9R0m3 |15/1/0| `955408ff066e772358a118dc7dfeeb229c265b2c8a41e0fd769d0372c9f3cbf8`; repeated short rows fit below the declared28-rem maximum, failing scrollability precondition |

The final synthetic height correction was reviewed independently before replay. No
pointer assertion was dropped or force-clicked; no product or frozen oracle changed.
All earlier owned PIDs are absent and listeners closed. At this checkpoint, broader
lifecycle, actual200%, assistive-technology, performance and delivery/consumer gates
remained open. The following results close only their explicitly named gaps.

### Runtime-bound actual 200% replay

The separate [runtime zoom harness](fixtures/navbar-adapter/runtime-zoom.mjs), SHA
`e3d9a204bfcca37a8b59ffac7da91d66e1b0dcf2247075ebf378f9e2ef2c4b85`, derives a private
runner from the frozen repair suite. Its entire task block and measurement functions
remain unchanged. Only declared runtime/provenance/output selection changes; the old
repair input is read-only. The generated runner SHA is
`c9bb84b7641ea751d337d5e8e43709a9761f89b028197c684694047d36e4069a`.

`runtime.XZgbXqWk/runtime-zoom-xHaMHl/receipt.json`, SHA
`ef85abc90d63e79ed4b47d1ce1920d0870862308cd755e6252c542b77311c533`, records
**466 passed, zero failed/errors** on Chrome152.0.7977.84 at actual200% Appearance zoom.
Outer1440/780 by1000 windows yield720/390 by500 CSS viewports, DPR2 and visual scale1,
in light/dark lanes. These are this runtime's own focus/geometry/outage/late-load tasks,
not a transfer of the older original-runtime466 result or a claim about all zoom levels.

Main reviewed the derivation and independently rehashed29 inputs and52 single-link0600
PNGs; all four owned Chrome PIDs are absent and the listener closed. Visual inspection
covered light wide first-account focus and dark narrow long-account focus: painted
focus and wrapped labels remain visible. This is not original-image fidelity or a
complete accessibility audit. Initial `zoom-runner-V1Aswc` stopped before browser work
at the old temporary-root precondition; it has no acceptance receipt and remains retained.

## Explicit nine-file repair delivery and standalone replay

[ADR-0060](../adr/0060-deliver-explicit-navbar-runtime-repair.md) replaces the prospective
eight-file proposal with a complete runtime-bearing variant, not an existing export.
`assets-adapt-navbar -repair -runtime ABSOLUTE_ORIGINAL_ESM` selects private JSON,
`-out` or `-check`. Runtime-without-repair and repair-without-runtime fail before output.
The reader accepts only the exact original pinned ESM through a bounded single-link
regular-file/no-follow open; no module lookup, download or dependency installation occurs.

The separate CUE envelope nests the unchanged repair and runtime artifacts. Its nine
fixed files include both templates/models, unchanged defaults, repair CSS, derivative
ESM, usage guidance and artifact.json last. Default five-file delivery remains identical.
Both variants refuse cross-checks; no-clobber, private modes and drift exit1 versus
invalid/could-not-check exit2 remain intact. Full source/repair/runtime replay precedes I/O.

Private delivery root:
`/Volumes/m2ext/appkit-navbar-acceptance.hEhhhUJv/delivery.oQKi8cBO`.

| Artifact | SHA-256 |
|---|---|
| delivery-receipt.json | `1192e4761ebd57eb6f1baadaf894da9b4bec73fd3bbde28161c75e2615c2972b` |
| repair/artifact.json | `67f015c1efa6c41b2acb38d56ac00ecc24b4c7d406729655d8330d425a5a6137` |
| Independently built appkit CLI | `9d01461e68b8d25eaa1c58384f667566cf2c8ed431aa9b16f91edaa036659a06` |

Explicit private `TestAssetsAdaptNavbarLive` and `TestAssetsAdaptNavbarRepairLive`
pass alongside dispatch/refusal tests with race checking. They verify deterministic
JSON, unchanged base bytes, nine0600 files, no-clobber, missing/extra/drift and both
cross-variant refusals. Supplying the derivative as the original fails without output.
`TestNavbarRepairDelivery*` additionally covers symlink/hardlink/FIFO/oversize reads,
changed nested fields and self-rehashed output. The built CLI's exact check is silent.

The maintained [repair rehearsal](fixtures/navbar-adapter/repair-standalone.mjs), SHA
`54bb2553a6daa028b8a795b69f53a13c618d97137450589ffb77eecb35de283b`, and
[synthetic consumer](fixtures/navbar-adapter/repair-consumer_test.go.txt), SHA
`57b8ab81d7235ca9d2c772a39a4dc48e337f423c2fa4aa94b57b534eae04360f`, pass in
`/Volumes/m2ext/appkit-navbar-acceptance.hEhhhUJv/navbar-repair-standalone-ueWgaW`.
Receipt SHA: `1354f1976e650bd3fd5c889c9720cb45e01f065ce5fcded554377e0367a6dc4d`.

All28 bounded commands have expected outcomes across two fresh consumer/compiler caches.
Both templates compile; typed/minimal renders equal the frozen repair compiler oracle,
while unbound defaults, empty summary and unwanted optional destinations are rejected.
Generated base Go209840 bytes, companion Go7633 bytes, typed HTML6713 bytes, minimal
HTML4550 bytes and test binary7112162 bytes are byte-identical between cold builds.
Binary SHA: `abad5475842bc5bc81eb45fa5987fa08978467a8c6afb424e167ba23526d7d41`.

macOS sandbox canaries prove kit, pristine-delivery, compiler-oracle and network denial;
withholding generated companion code fails each build. Actual-used module directories
contain no app-kit, fleet modules or replacements. Public ZIPs use the existing
`/Users/geoff/.dev-cache/go/modules/cache/download` proxy with pinned sums. Its write
denial is configured, not exercised against the owner's cache; no live sumdb proof or
complete transitive-graph claim is made. Go1.26.3 darwin/arm64 and Nodev26.3.0 are recorded.

Main reviewed both new fixtures and rehashed84 outputs, delivered copies and command
logs. Delivery checks before/after are silent, original inputs remain unchanged and
the owned canary closed. CSS/runtime copies are byte-bound here but not browser-served:
standalone rendering is not CSS/runtime integration or real consumer adoption.
Broader lifecycle, source-image fidelity, accessibility, performance and full target
gates remain open. No consumer, dependency, source archive, existing preview, global
configuration, deployment or commit changed. Counts stay34 records/nine typed/zero certified.

## Exported CSS and runtime browser delivery

The separate [delivery-browser harness](fixtures/navbar-adapter/repair-delivery-browser.mjs)
uses only the two cold-built typed/minimal renders, bound directly to their standalone
receipt and compiler oracle. CSS compilation scans only private copies of the two
exported templates, with implicit source scanning disabled. The unchanged delivered
repair CSS/runtime and pinned Inter are served explicitly. No original HTML or sibling
source supplies fallback classes, component code or runtime bytes.

The matrix fixes light/dark,390/1440 normal100% viewports; typed account/mobile keyboard
and pointer GET tasks, current/default state distinction, focus/containment/loaded Inter,
minimal optional-field absence and native directory access. Narrow light representatives
add JavaScript-disabled, HTTP503 and late real-module arrival tasks. These are scoped
synthetic interactions, not actual app adoption, performance, assistive input or200% proof.

Outputs are new private children of `/Volumes/m2ext/appkit-navbar-acceptance.hEhhhUJv`:

| Suffix | Result | Receipt SHA-256 / limitation |
|---|---|---|
| navbar-repair-delivery-browser-ZZvPs1 |110/6/1| `dddba976c8299e69427414cc40c991db73894681af710a22665588ccedd6c10c`; fixture canvas CSS omitted and missing-CSS document also selected failed runtime |
| navbar-repair-delivery-browser-pyuuUO |2/0/1| `a7bc884d60116654c867a40d3f450627a4290e092e2f71a29e9537359a5f2209`; computed-style collector queried hidden mobile links before opening disclosure |
| navbar-repair-delivery-browser-opGkWU |135/0/0 task checks| `d5a772b48262c5954a84d5e194d91a7537865c2754431a9db8c9658c20bd1c43`; insufficient capture-coverage assertion |
| navbar-repair-delivery-browser-cAKSAG |135/0/0 task checks| `5c12fe441571657f22b6bd183edf0776e370011a010cd037d4739ad0f7c2c736`; strengthened received-body checks, same failed capture evidence |

The first failed run retained its script hash, not a copy before editing. Its initial
harness is not retained as a file; this replay gap remains explicit under INC-43.
Subsequent runs save exclusive0600 harness/scan snapshots before browser work. Main
compared those snapshots: the hidden-link correction opens/settles mobile disclosure
before measurement and starts fresh before keyboard tasks; no state/style threshold changes.

Main rehashed30 inputs and17 outputs for cAKSAG, including eight PNGs and both CSS
builds. CSS SHA `b6e7207a0fb090a9bf917e14cd21ce0380e105f44598ad72d66b1c7d92cc9b7d`
matches across builds. All26 browser-received source-CSS bodies,26 repair-CSS bodies
and24 runtime bodies match their exact pins, with reads joined before context closure.
Six owned Chrome PIDs are absent and the listener closed. Missing repair CSS fails a
formerly broken primary-link focus check; restoring the delivered CSS passes the same
oracle. Runtime503 is detected. Omitting route-template scan is a structural contract
canary, not observed missing styles: that template currently contains no utility classes.

Visual review nevertheless rejected all eight116px-high cAKSAG captures: capturing the
in-flow `#specimen` box clips the account popover, including Settings, below that box.
The original draft used page capture; narrowing the capture to address sparse-pixel
checks was not an acceptable substitute for coverage. This is an INC-28 recurrence,
not a component layout failure, and135 task passes do not make those images valid.
The reviewed follow-up requires exact measured viewport PNG dimensions, required
open-menu/target containment and unchanged nonblank thresholds within a declared task
region. Component CSS, templates, runtime and focus/geometry task oracles stay fixed.
The follow-up result must be recorded independently before capture acceptance.

### Capture-corrected delivery result

`navbar-repair-delivery-browser-1vrArf/receipt.json`, SHA
`bc8190f608f7d53d172650481cf7ebce34b5390f318ca4c110c1151af131b7a0`, records
**144 passed, zero failed/errors**. Harness SHA:
`752369888f7718b547046baeb33ec994c0c30a1e35b40e9d736dfd8ece6a697c`.
All eight PNGs are exact390/1440 by900 measured viewport captures at DPR1/scale1.
Required visible selected-nav, menu and link rectangles fit inside the image, with
Settings explicitly present in every open-menu capture. Declared task regions are
full-width by152 for open menus and by116 for minimal specimens; unchanged0.001
dark/light fractions and opacity checks apply there. Global pixel counts remain visible.
Truncated-dimension and menu-outside-image canaries fail as required. Neither component
layout nor the viewport was changed to manufacture capture height.

The response-drain helper is now available from cleanup and deadline-bounded; a seeded
resolved-response canary exercises it. This does not prove every forced cleanup failure.
All26 received source-CSS,26 repair-CSS and24 runtime bodies still match their pins.
Main rehashed30 inputs/17 outputs, inspected light wide/dark narrow account views and
light narrow/dark wide minimal views, and confirmed all six owned PIDs absent. The
listener is closed. Exact copied CSS/templates/runtime and frozen focus/geometry
functions remain unchanged; prior cropped captures are not retroactively accepted.

This closes the named normal100% synthetic standalone CSS/runtime delivery gap, not
full target certification, a browser-independent or200% delivery matrix, arbitrary
Datastar lifecycle, performance, physical/assistive input or real consumer adoption.
MCP/Gallery race checks and the scoped documentation/link/budget checks pass. Intent
I17, INC-43 and INC-28's recurrence carry the evidence-retention/capture requirements.
No original, consumer, dependency, existing preview, commit, deployment or global
configuration changed;34 catalog records/nine typed candidates/zero certified remain.
