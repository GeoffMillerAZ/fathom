# Fathom Observatory: shared frame and delivery checks

Owner-selected consumer: Fathom's dashboard/analytics. This first pass changes the
frame, not metric derivation or the whole analytical experience. Consumer source was
reviewed against HEAD `5908e15d8d2257ed8cfb1ae1f994826f23b70833`; concurrent harness
work was preserved. No live provider data, deployment, dependency or mirror edits.

## Adaptation

The existing role-token palette stays distinct in light/dark. MatDash-derived
`dashboard.RenderShell` supplies the desktop sidebar, top bar, native mobile Menu
and main landmark. Existing seven destinations, leads, explainers, banners, body
and generated/version footer remain. No licensed source was copied into Fathom.
One app-owned first skip link targets the focusable, bounded content container.
`PRODUCT.md`, `docs/app-kit.md`, `appkit.adoption.json` and matching agent routers
now record the selection, task, delivery identity, divergences and next acceptance.

## Integration failures caught before handoff

Workspace tests initially passed with new kit `RenderNav`. `GOWORK=off` refused
the build: the stamped mirror lacks `NavItem`, `NavGroup` and `RenderNav`.
Retained small app-owned route links through the existing `Shell.Nav` slot; a
generic navigation copy or undocumented mirror patch would defeat ADR-019.

The stamped Shell also lacks a focusable skip target. Its export failed 18 of
216 browser checks, all skip-focus checks, while geometry and page loads passed.
Used Shell's documented skip opt-out and an explicit app-owned `tabindex="-1"`
content target. Tests retain exactly one skip link/target/main/heading. Do not
remove either divergence until a reviewed stamped kit upgrade proves replacement.

## Evidence

| Check | Result | Boundary |
|---|---|---|
| Shared-shell contract against old page | Failed for missing shell/navigation/skip structure | Actual TDD red before frame adaptation |
| Observatory + preview policy tests, `-race` | Pass with workspace (1.794s) and `GOWORK=off` (1.756s) | Existing fleet/service/impact plus new route/focus/inert assertions |
| `task ui:css` replay | Pass; 52,485-byte app CSS and 6,753-byte token CSS stable | Existing sibling-based generation, not sibling-free rebuild |
| Static adoption checker | 13/13 pass | Selection, hashes, declarations and paired router text; not host invocation |
| Fathom config audit | 8 hold, 0 violated/unable | 120-line Claude router, pointers and owned-region drift; not model behavior |
| Final stamped-mirror Chrome preview | 216/216; zero page errors, blocked requests or detected section/outer overflow | Three pages × three widths × two themes, with before/after |
| Visual inspection | Before/after desktop; final desktop dark/light and phone light reviewed | Frame improved; content craft and accessibility certification remain open |

Chrome 152.0.7977.84 / isolated Playwright 1.63.0, fresh profile, widths 390/768/1440.
Browser assertions cover theme, landmarks, current destination, first Tab skip link,
Enter-to-content focus, keyboard native mobile disclosure, visible current mobile
link and retained unmeasured impact wording. Twenty-four screenshots retained.
The blue focus outline in desktop captures is intentional keyboard-test state.

## Isolation and identities

The opt-in consumer test exports only its fixed synthetic `alpha` model into an
existing empty `workspace/tmp` child, with six HTML pages and two CSS files.
Scripts/forms are disabled and CSP blocks network/scripts/submission. A local
server serves four exact GET/HEAD routes per theme, not a repository directory.
No operational handler, listener, provider or daemon model is started by the exporter.
Its inert transform is scoped to known renderers, not a hostile-HTML sanitizer.

Temporary artifacts (not release evidence):

- Before: `sextant/workspace/tmp/appkit-preview.YimAyZ`.
- Rejected mirror focus fixture: `sextant/workspace/tmp/appkit-preview-mirror.ldb2u2`.
- Final stamped mirror: `sextant/workspace/tmp/appkit-preview-final.m6CkpI`.
- Final workspace export: `sextant/workspace/tmp/appkit-preview-workspace.tv7SUe`.
- Browser harness/report/screenshots: `/tmp/appkit-browser-review.xBrDCp/fathom-smoke.mjs`, `fathom-smoke.json`, `fathom-screenshots/`.
- Harness SHA-256: `a02c6f8c7016d8b8af39b78886403f422db7bb3c7aa0ff1728de4d256b2be517`.
- Final report SHA-256: `24795d478319dbb87fccf676d6b55c8b530344f873a9de8524afb2206a266e14`.
- Final light overview HTML: `0fe5b98602c4a0a6e7f9a0208b252fcf5aea3f97966d16045b1e90e12a9a04bc`.
- Final dark overview HTML: `cb5a8c87a664f261f065915188092f81a3f3cb9747cfa7ca6e4fc300db3475d4`.
- Consumer renderer SHA-256: `df6d161791982dfc935ceb042cc1a86a320c151653e28b3bed420e715178fce4`.

## Return and next acceptance

This validates the adoption contract's existing distinction between declarations,
effective resolution and runtime proof. The exact mirror remains at `d228711`;
neither new generic navigation nor the shared grid fix was propagated into it.
Current shared kit already fixes the skip focusability; explicit upgrade remains work.

At the first-pass boundary, next work was meaningful empty configuration/exploration
states, narrow chart labels and populated/stale/partial/error tasks (follow-up below). Browser
semantics, contrast/target sizes, live mutation policy, performance, installed-daemon
and fresh consumer-host tests were not completed here. G1–G6 remain open.

## Analytical gap and caption follow-up

Missing configuration now uses a named kit callout: installed layers, guards and
bindings are unknown, not confirmed absent. Missing comparable exploration has one
escaped explanation and an explicit session-evidence link, not an empty card or a
zero-savings claim. Measured exploration and configuration rows retain their values.
The fixed synthetic story renderer makes this next step testable without live ledgers.

The service-share figure uses the existing mirrored `vizframe` API: readable HTML
prose, `7 of 10` denominator, local call-log source and generated snapshot time surround
the unchanged `70%` gauge. Unreadable logs suppress this measured figure. No widget
is nested around its already-owned surface, and no mirror bytes or stamp changed.

Returned upstream (INC-14): `Gauge` now wraps long captions by a conservative,
width-aware grapheme budget using already-pinned `uniseg`; combining marks/ZWJ emoji
are not split. An escaped complete description survives visual line breaks. Ordinary
one-line output stays compact. Prefer HTML figure prose for long explanations; this
algorithm is not a font-metrics engine or universal unclipping guarantee.

| Follow-up check | Result | Boundary |
|---|---|---|
| Empty/configuration/source-frame tests | Red before, green after | Measured values, absent versus zero, unsafe reason and unreadable-log canaries |
| Observatory + inert-preview tests, race | Workspace 1.940s; stamped mirror 2.635s | No live providers/handlers; mirror contains the selected APIs |
| Full app-kit Go suite, vet, viz race | Pass | Upstream source, not propagated mirror or runtime certification |
| Adoption and consumer config | 15/15; 8 hold, 0 violated/unable | Five selected artifact hashes; static declarations only |
| Chrome follow-up | 96/96; no page/console errors or rejected requests | Six width/theme cases plus eighteen caption-glyph bounds |
| Visual review | Phone light/dark service, phone light/desktop dark impact, glyph specimens | Whole journey, contrast and all fonts remain unproven |

Keyboard Enter opens session evidence and returns to impact through desktop/mobile
navigation. The harness originally sampled the URL before navigation settled; waiting
for the expected destination fixed all twelve false negatives without changing the app.
Caption bounds cover 80/160/320px gauges, ASCII, CJK, emoji, combining and ZWJ sequences
under this Chrome/macOS system font. Cluster integrity has separate red→green Go tests.
The standalone sheet is a geometry fixture, not a recommended prose composition.

Follow-up artifacts (supersede the renderer/CSS/preview identities above, not the
historical first-pass result; thirteen new screenshots):

- Stamped export: `sextant/workspace/tmp/appkit-analytics.Hdr2RH`; eight HTML + two CSS files.
- Same isolated harness directory: `fathom-analytics-smoke.mjs`, `fathom-analytics-smoke.json`, `analytics-screenshots/`.
- Harness: `4b3a7a35d0a9be66abbd02683fe54867e885b4975f6feb19f43f28fa70edfee4`; report: `4a4d0b24f2f003589a5ec3640a3485a9cce66aec645c5861411edad928116e49`.
- Consumer renderer: `f1b30bcf34c61c9c25ad564d5d5817a85490bf2f74d2c3c47a904418f7799666`.
- Generated CSS: `b251b4d809ad65726c89fe0f41cd7b082bec304884a6089dc6530c28226f16e8` (52,527 bytes).
- Light service HTML: `436242845f97cb8a7f9c7cf6b05de2a4faba8eecc700e64a0b09eeb144751af3`.
- Dark service HTML: `22a862fa0486104034311ad86074b7a88cc72a6542bb199e8fd4da330be2dc5d`.
- Upstream Gauge: `e8de4be49bee18f14533c68362acc68fafc36c3429ba0f9c1de4d1361e0ae9e5`.

Remaining: populated/stale/partial/error task journeys, quantitative usability and
performance, accessibility review, standalone CSS regeneration and stamped upgrades.
This focused follow-up does not complete G3/G4 or change any runtime support claim.
