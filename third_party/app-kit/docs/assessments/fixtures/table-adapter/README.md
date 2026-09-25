# P04 responsive directory/table pilot

Private source-specific experiment under ADR-0023. This contract precedes adaptation
and browser checks. It does not promote a public table API or a verified target.

## Frozen source and scene

P04 is `application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8`,
`html/lists/tables/07-with-stacked-columns-on-mobile.html`, SHA-256
`5d00e2755dcf060cceafa4c2dba4f15452719e54f5cd40646ab3155c89dc8288`.
The owner reviews a directory at a desk and checks a record on a phone. Preserve both
source light/dark schemes and the stacked narrow presentation. No palette remapping.
Existing `appkit/ui/table` remains the public first-party table; this candidate is
an independent licensed composition, not a replacement or generic table language.

## Source-native baseline first

- Verify exact archive/member bytes; retain the original unchanged and separately.
- Use cached checksum-pinned Tailwind 4.3.0 and the already pinned InterVariable/license.
  These are chosen comparison conditions, not recovered portal settings.
- Build CSS twice with identical inputs and compare bytes. No network fallback/install.
- Capture 390/1440 CSS pixels, light/dark, fonts ready, zero app JavaScript.
- Record visible row/header/stacked-label semantics, overflow, sampled contrast, action
  size and keyboard focus. Keep source failures; do not silently repair the baseline.
- Original Add user is an inert button; Edit links are `#`. These are not functioning
  management actions. Do not click them and claim a completed application task.
- Keep private HTML/screenshots out of tracked/distributable source; save first-party
  harnesses and non-content evidence separately. Document budget: 1 MiB per HTML page.

## Typed boundary and use

`appkit data assets-adapt-directory [-root PRIVATE_DIRECTORY] [-enhanced]` verifies the exact
original and defaults to private JSON with template/model/defaults and replay identities.
Explicit `-out NEW_DIRECTORY` exports immutable private sources; `-check DIRECTORY`
compares the selected variant to fresh original replay. They are mutually exclusive,
silent on success, and never build, install dependencies or publish a catalog target.
Keep emitted sources private in an app-owned package; use the artifact's pinned templ
requirement and scan its template with the app's pinned Tailwind4 build. Supply the
source stylesheet and deliberately selected fonts; this is not token-themed kit CSS.
Decode `defaults` into generated `Directory`, provide `AddURL` and every row's `EditURL`,
then call `NewDirectoryNavigation(data)`. Empty default URLs intentionally fail.
Base delivery has five files; `-enhanced` selects eight, including the companion.
Always repeat `-enhanced` when checking an enhanced export; cross-variant checks fail.
P02's separate six-file delivery and default output contracts are unchanged.
MCP exposes this guide as `get_kit_decision("directory-adapter")` and
`kit://directory-adapter`; rebuild/restart the server to gain that new document ID.
The shared catalog's `reference.directory-adapter` is an experimental CLI-tool
reference (`profile=cli`, `kind=reference`), not a certified web component. Its exact
record lists usage and private-source requirements without opening licensed assets.
[Dated evidence](../../2026-09-14-directory-typed-adapter.md) distinguishes compiler,
source/target browser results and remaining gates. No host registration is implied.

Frozen composition: heading/introduction, Name/Title/Email/Role and Edit labels, and
bounded typed rows. One data field supplies each repeated desktop/mobile value.
Snapshot caller-owned slices. Validate text, counts and destinations before rendering;
all configurable text uses templ escaping. No raw HTML, app-kit runtime dependency,
invented directory backend, auth, sorting, filtering, pagination or export engine.

Originals and literal syntax conversion stay available. A separately named navigation
adaptation may map the inert Add button to a native same-window link, bind Edit links
to app-owned destinations, and preserve accessible row-specific action names. That is
an intentional semantic mapping, not exact unmodified-source DOM equivalence.
Fixtures use only local demonstration routes and synthetic records, never real users.

Keep exact structure/classes/text whitespace except declared field/action mappings.
Do not repair small targets or truncate/wrapping behavior in the source-faithful pass.
Any quality-enhanced variant is explicit, separately rendered and independently tested;
it must preserve all data and column/header associations rather than hide overflow.

## Opt-in usability companion

`-enhanced` emits `p04-directory-enhanced-v1`: the unchanged base artifact under `base`,
plus independently hashed companion template/model/CSS and rule identity. Without the
flag (or with `-enhanced=false`), the original JSON envelope is unchanged. Both variants
remain `not-verified`; generation/replay is not browser quality or app adoption proof.

In the same private generated package, keep `base.templ`/`base.model` as
`directory.templ`/`directory.go`; add companion `templ`/`model` as
`directory_enhanced.templ`/`directory_enhanced.go`. Compile both templates with the
pinned templ tool. Load companion `css` after the source stylesheet. Call
`NewDirectoryEnhanced(data)` for the scoped wrapper or `NewDirectoryNavigation(data)`
for the source-styled composition. Never append raw CSS into unescaped user content.

The companion preserves the purchased palette and every cell/label; it adds minimum
44px links, a 2px row-link focus outline, readable column space and long-content wrapping. It is not a token
remap. App-owned `Introduction` explains an empty or partial result; rows and Add/Edit
destinations remain app-owned. No new state engine, persistence or account service.

The opt-in private compiler fixture is reproducible with
`APPKIT_PROBE_CONVERSION_KIT_ROOT=/absolute/app-kit APPKIT_PROBE_DIRECTORY_RETAIN=1 go test ./internal/designadapt -run '^TestAdaptDirectoryEnhancedLive$' -v -count=1`.
It retains a new private temp directory containing four JSON scenarios and source-styled
and enhanced renderings before/after formatting, with a hash/size inventory. Retention
does not back up the files. Run the separate browser harness against this directory and
the existing controlled source reference; never build inside `data/design-assets`.
From the kit root, run `node docs/assessments/fixtures/table-adapter/browser-enhanced.mjs /absolute/reference-temp /absolute/enhanced-temp`.
The harness uses the already approved isolated Playwright/Chrome setup; it installs
nothing. [Enhanced evidence](../../2026-09-14-directory-enhanced-adapter.md) names the
exact matrix, source-style failures, visual corrections and remaining boundaries.

The separate [native directory journey](../directory-journey/README.md) demonstrates
app-owned filter/page/detail/Back and all-matching CSV through a fresh enhanced export,
including partial/unavailable states and independent data checks. Its
[dated proof](../../2026-09-16-directory-journey.md) is synthetic, not real adoption,
a shared backend, or full target certification.

## Acceptance before any promotion

Repeat generation, compile/render before and after templ formatting, compare DOM with
only the declared mappings, and plant lost mobile labels, wrong repeated values,
changed attributes/whitespace and unsafe URLs to prove refusal/detection.
Browser target checks include baseline, zero/partial/long rows, long labels, 390/1440,
light/dark, JS on/off native keyboard/pointer navigation, visible focus, no page overflow,
44px actions and 4.5:1 normal/3:1 large-text contrast. Record inherited failures separately.
Ordinary short roles/emails must remain readable, not compress into vertical text or
unnecessary lines merely to pass an overflow test. Hostile long values may wrap.
Source/target images use the same fixture/fonts/engine with no changed goldens or masks.
Actual zoom/assistive-input, performance and real consumer use remain explicit gates.

## Immutable private delivery

[ADR-0036](../../../adr/0036-share-private-delivery-assurance.md) resolves the second-adapter
review: share only internal filesystem safeguards, keeping source replay, inventories
and error identities distinct. Export refuses every existing destination, including
empty directories. Its existing real parent must be operator-controlled. It creates
0700/0600 directory/files and leaves failed partial output for inspection, never cleanup
or repair. `artifact.json` is last, not a crash-atomic marker or certification.

Checks require original archive access, all exact bytes/names/modes and bounded regular
single-link files. Missing/extra/modified/linked entries fail. macOS/Linux use no-follow,
nonblocking reads and identity checks; other platforms are unsupported. Exit 0 means
matching/exported, 1 observed drift, 2 invalid request or could-not-check. No private
body is printed during these operations. Compile a COPY outside the input store; apps
own their Go/templ/Tailwind/font pins, routes and data. Neither export includes compiled
Go, source CSS/fonts, a Go module, dependencies or a public vendor-channel entry.

No target badge follows from export, replay or a compiled typed candidate. Rendering
gates and real consumer integration remain distinct from delivery integrity.
[Delivery evidence](../../2026-09-14-directory-private-delivery.md) distinguishes checks,
copied-source compilation and standalone-module proof.
