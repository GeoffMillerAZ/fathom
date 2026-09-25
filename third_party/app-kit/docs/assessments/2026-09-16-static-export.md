# Static reading: plain-file delivery

[ADR-0071](../adr/0071-deliver-a-bounded-static-reading-reference.md) and the
[consumer guide](../design/static-content-delivery.md) define this bounded reference.
It reuses Gallery's reading/FAQ body and profile article, not the Gallery server.
This is first-party composition, not a port or fidelity claim for paid source.

## Delivered boundary

`apps/gallery/cmd/static-content` accepts explicit trusted compiled CSS, a fresh
absolute output directory and a canonical hosting prefix. It emits nine semantic
HTML documents: index, article and 404, each system/light/dark; one CSS file and a
sorted byte/hash manifest. Normal links are relative; error-page links/assets bind
the chosen prefix for recovery from an arbitrarily deep missing URL. Gallery-only
navigation and synthetic state forms are absent. No visitor script or font is required.

The dedicated CSS input disables implicit repository scanning and includes only the
reading templates and their documentation/callout primitives. Existing Gallery CSS
is unchanged. Header theme links have 44 px targets and visible focus; the skip link
lands on a focusable outlined main, and article links target focusable headings.

Existing output, symlinked paths, invalid prefix and malformed/oversized CSS refuse.
Outputs are exclusively created through a held `os.Root`; the manifest is last.
Partial I/O failures retain their new directory, never overwrite/delete user files.
CSS remains a trusted input, not an arbitrary-content sanitization promise. The new
command separates parse/argument exit 2 from accepted file/export/output failure 1.

## Retained preparation

Private parent: `/Volumes/m2ext/appkit-evidence/appkit-static-delivery.7yMDB3/`.
`prepared-4nKDl6` retains the first preparer failure: test variants report absolute
Go filenames, which were incorrectly prefixed twice. `prepared-VH8JPf` retains the
second failure: the isolated module cache lacked CUE-tool dependencies and external
lookup was disabled. The repaired preparer handles absolute filenames and uses only
the pre-existing local file-module proxy; it has no network fallback.

`prepared-Pfxrkm` succeeds with 215 retained local source/test/style/config files,
an allowlisted execution environment and before/after source checks. It copies and
verifies the already provisioned Tailwind 4.3.0 compiler against the repository's CUE
pin and invokes that copy directly, not a download-capable resolver. This is a
cooperative working-tree build, not hermetic, independent-cold or adversarial-race proof.

The three race-tested packages are `apps/gallery`, its new `cmd/static-content`, and
`appkit/ui/docs`: 577 passing test/subtest events, zero failed events. Four opt-in
analytics/private-store/live-shell tests skip because their separate target variables
are unset; none is counted as passed. Both trimpath exporter builds match, and two
fresh exports match byte-for-byte across all eleven files. No generated workspace
CSS, consumer, dependency pin, installed binary or global configuration changes.

| Prepared artifact | SHA-256 |
|---|---|
| Both exporter binaries | `c7ce174a5e040139da2d27dd53039b3a39c987796f4227e76059747cd6ebb12d` |
| Explicit CSS, 17,852 bytes | `34d078f7ada9aa8c9f61ad47afb6e6c3eb14033e1393bc26ddcd0c69a8a5c90f` |
| Site manifest | `3a0e1688670c51a5ebe46c7a9ae654b7a683d67998ef748ddb259e47cec98ccf` |
| Exported browser contract | `e2a3969b3f5514cffe4c5859702bd5b6556d7d2a09fef551b03e00579b54bd9f` |

The separate exporter-unit packets at `/Volumes/m2ext/appkit-evidence/appkit-static-export-proof.CpuWTd`
and `/Volumes/m2ext/appkit-evidence/appkit-static-export-followup.bXXrqA` retain earlier red/green
iterations. They precede the final header/focus refinements; their identities are not
substituted for this preparation's final compiled bytes.

## Browser acquisition boundary

The [maintained fixture](fixtures/static-export/README.md) serves copied export files
under `/reading/` with no Gallery/Go process. Its prospective CUE contract names six
390/1440 system/light/dark lanes, JavaScript disabled, native input/navigation,
exact asset accounting and local byte/navigation-timing ceilings. A deep missing URL
must actually return 404 with usable CSS and recovery; named 404 pages return 200 and
are only appearance checks. System fallback follows the configured browser preference,
not a fictional remembered theme. Native browser history is tested separately from
an in-page return link. Preflight review tightened link grammar and consumes every
declared outline/FAQ oracle; the browser runner retains its own exact executed revision.

### First browser run retained: link-oracle failure

`static-export-lSeBMv/` completed 90 document observations, 180 exact asset responses
and 126 captures on Chrome 152.0.7977.84. Six index checks fail: the independent
link allowlist omitted the legitimate FAQ destination `content-profile-title`.
The receipt is `be28bd28e0e5ca29b69c0e4caad4e9d9c35478ab1a4bc4600d112196a283b1c8`;
the executed harness is `2cc72b324504462c8a72eb051160a95d01a6ef3cb11fa697cc68e94a11f0a4f5`.
The run remains failed, despite its successful task actions and exact-byte accounting.
Its local maxima are 25,602 decoded bytes, 23.5 ms response end, 27.5 ms DCL and 31 ms load.
Chrome exits 0, its server closes and port 49154 refuses with `ECONNREFUSED`.

Independent Sol/Impeccable visual review inspected eight named captures: 390-system,
390-light and 390-dark index; 390-system FAQ; 1440-light index; 1440-dark article;
390-light deep 404; and 390-dark `toc-start-with-task`. It found no clipping, overflow,
readability or focus-outline blocker, but identified two affordance defects: no visible
current-theme treatment, and outline links resembling static text. The export-only
CSS now styles the existing `aria-current` theme link and underlines outline links.
Shared component rendering, Gallery CSS and exported HTML remain unchanged.

### Narrow oracle and visual corrections

`fragment-proof-yNvvxI/` freezes one exact standalone fragment canary, the old/new
harnesses and unchanged contract before execution: old code fails the valid FAQ link
assertion; corrected code passes it and rejects unknown or wrong-document fragments.
The first full-suite replay at `oracle-red-1tpGUz/` failed importing a newly added
presentation helper, not on the intended assertion. Its diagnosis explicitly rejects
that failure as fragment-regression proof. Agent console-only checks are supplemental;
the retained packet and later preparer's logs provide the replayable evidence.

`prepared-4wDE2G/` repeats the same 215-source preparation, three package race suites,
two matching binaries and two identical site exports. The exporter binary and HTML
identities remain unchanged; the visual-only CSS is now 18,209 bytes. Thirteen frozen
pure harness tests pass. The contract and numeric ceilings are unchanged.

| Revised artifact | SHA-256 |
|---|---|
| CSS | `bf0b8e787aaecf1e4fdafe4a7e3db724b3ea8a787fa5e03f7959aabfc5ff22ce` |
| Site manifest | `317301ea8b114aa59c4182611c3fee3c5a662ee1731782a4591fce4053f2352c` |
| Fragment/presentation harness | `ab75f7bea44e6022bab08bceb019e690fc3859929764220f291f0b8f3e8b0a4c` |

The separate `static-export-anrtgq/` run again retains 90 observations, 180 responses
and 126 captures. It fails six article checks: after opening both FAQs, the driver's
name-only Tab lookup follows the second identical link label, reaching
`#content-runtime-fit` rather than the declared first FAQ's `#content-profile-title`.
The stricter outcome correctly exposes this driver error. Its failed receipt is
`8c2bcc8b8748661d6d125f260be9ceb1934f1b8cb0a87830b2d656a0203f96ba`.
All selected-theme and underlined-outline checks pass; Chrome exits 0 and port 49475
refuses after cleanup. Four independent Sol-reviewed captures confirm both visual
repairs without new clipping, contrast, wrapping or focus defects: 390-light index,
390-system index, 1440-dark index and 390-dark `toc-start-with-task`.

The final harness selects the first FAQ's exact link through native Tab/Enter and
checks its visible label. Expected fragment/focus, contract, site HTML/CSS and all
budgets remain unchanged. This is a driver correction, not a relaxed task outcome.
Full static-starter standards, other hosts, accessibility, broader performance,
consumer adoption and permanent API promotion remain separate requirements.

### Final exact-site replay: bounded task passes

`static-export-EvUefV/` retains 90 passing document observations, 180 exact-body
responses and 126 captures across six script-disabled lanes. It exercises keyboard
skip/outline focus, all four ToC destinations, two independently open FAQs, the exact
FAQ article fragment, real browser Back/Forward, both onward reading sections,
page-preserving theme links, named error appearances and an actual deep 404/recovery.
Every observed page has 44 px visible controls, no horizontal overflow, expected theme
polarity and a visible selected appearance; each index has four underlined ToC links.
No forbidden request or script is observed. Every document and stylesheet binds to
the exact prepared site bytes, including non-root error recovery. The HTML/CSS,
contract and runtime inputs match the preceding failed acquisition; only the frozen
driver changes. These are automated local task checks, not a human reading trial.

| Final replay fact | Result |
|---|---|
| Executed harness | `21285bcb533c284ff620ce189dc370d4a2a2efee9479d2cb09a08ee2fafeefb2` |
| Browser receipt | `e59dce48c71b3a85a2f4ebab0b3238553dede11116c8ebf7ce634c9c8a25bc2f` |
| Maximum decoded document+CSS | 25,959 bytes / 131,072 ceiling |
| Maximum response end / DCL / load | 17.1 / 22.2 / 38.1 ms against 100 / 250 / 500 ms ceilings |
| Cleanup | Chrome exit 0, server closed, port 49567 `ECONNREFUSED` |

The finite plain-file reading task now has build, asset, browser and scoped visual
evidence. It does not complete all `converted-core-v1` standards, any consumer upgrade,
full accessibility or cold/warm/degraded/memory performance, nor change weighted goal
completion. Independent Impeccable review of the final `390-light-profile.png` and
`390-dark-profile.png` captures found the expected title focus visible and unclipped,
with no new visual blocker; this is not a full accessibility review.

## Ordinary macOS temporary-root follow-up

A later [analytics package run](2026-09-16-analytics-composition-safety.md) exposed
positive export tests passing macOS's `/var` alias where a canonical parent is
required. Only the two test fixtures now resolve that temporary-root alias; explicit
symlink-refusal cases and all export/runtime bytes remain unchanged. The four-package
rerun passes with the ordinary macOS test root. Earlier failures remain retained;
this test portability repair adds no new browser or delivery claim.
