# CSS browser range audit and measured parser limit (C-R7)

Population: Lightning CSS definitions. The browser attests ends for 97.62% of eligible pairs and does not enumerate. The proposal-selected range grade is tier C under C2-F1, capped at good: range agreement is **1.0 on 134,456 of 137,732 paired canonical definitions (97.62% coverage)**. Other scored metrics remain 1.0. This does **not** establish full excellence: the unchanged grammar has ERROR/MISSING nodes in 120 of 212 eligible files (56.60%), above the design's 5% refusal line. Both named native grammar candidates reproduce that failure. No exhausted-parser ceiling or legacy exemption is claimed.

The [canonical candidate reading](../bench/readings/css-browser-2026-10-26.json) and [untuned holdout reading](../bench/readings/css-browser-holdout-2026-10-26.json) carry the C2-F1 range tier correction in place. The dated [receipt](data/2026-10-26-core-t1-css-browser.json) preserves their original SHA-256 hashes, measured-at and Fathom commit identities, timing notes, and the classification harness commit. Git history retains the original reading bytes. The canonical `lang/css.json` is not replaced while the parser gate is unresolved. Its earlier L1-good grade is not a new passing-parser claim. Generated boards and plan boxes remain controller-owned.

## Measured results

| Scope | Range agreement / coverage | Other result / limit |
|---|---|---|
| Canonical normalize.css, Pico and Gitea | 134,456/134,456 correct; 137,732 paired definitions total | Harness L1 good (range tier C), L2/L3 excellent; 45,956 name-reference pairs and 88 file edges unchanged; 0 faults, 120/212 files with parser errors. Original throughputs 2.07/2.06/1.91, median 2.06, lacked individual load records. Load-recorded repeat 2.15/2.10/2.13, median 2.13 MB/CPU-s. |
| Fresh Tachyons v4.9.1 `src` holdout | 2,498/2,498 correct; 2,508 paired definitions total | L1 good (range tier C), L2 excellent; L3 not attempted because all 56 imports lack physical literal targets in primary truth. 0 faults, 2/60 parser-error files; Original median 2.13 (2.17/2.01/2.13) lacked individual loads. Timing-only repeat 2.18/2.12/2.19, median 2.18 MB/CPU-s. No tuning after this reading. |
| Official native CSS v0.25.0 | 120/212 eligible files in error; 238/331 raw | Matches the unchanged WASM reading: Pico 118/119 eligible, Gitea 2/92, normalize 0/1. No timeouts. |
| Native css-strict fork | 120/212 eligible files in error; 238/331 raw | Same failures; changing the pin to this candidate would not pass the gate. No timeouts. |

The browser processed all 331 raw canonical files, including Pico's 119 minified files that the existing production ingest already skips. Neither corpus nor oracle scope was narrowed. The range companion contributes only independently attested ends; primary definitions, calls, imports, W2 rules and matching denominators remain unchanged. The native audit used about 125.6 seconds of its 1,200-second pooled budget. Complete-source hashes were checked before/after every invocation. Keyframe whole-rule ends and nested parents whose declaration section ends before their full rule remain unmeasured.

The 3,276 production-eligible exclusions are **2,837 selectors and 439 keyframes**
(normalize: 2 selectors; Pico: 2,832 selectors/434 keyframes; Gitea: 3 selectors/5
keyframes). There are no excluded custom properties. A source-bound retrospective
proposal census finds all 2,837 excluded selectors have one matching flat proposal,
zero match nested-parent proposals, zero have unmatched headers, and zero have
multiple matching proposals. They are flat proposals left unattested in the frozen
browser result. Exact native rejection messages were not retained, so this is a
proposal classification, not a reconstructed browser-error taxonomy. Keyframes have
no whole-rule native attestation in this route. The raw-file diagnostic totals include
minified files and must not be used as the production-eligible denominator.
Reproduce with [exclusions.cjs](../../calibration/oracles/chromium-css/exclusions.cjs).

[C-R7 and C2-F1, recorded in design §1.3](../design/language-breadth.md#13-oracle-tiers-and-what-they-can-prove), permit the pinned engine as an oracle candidate but require its own enumerated population for a tier-A grade. This authored-proposal population is graded tier C. The old W2 primary-family labels are historical; their tension with P-T1 is recorded for owner disposition before adoption. Chrome for Testing headless shell 156.0.8078.47/mac-arm64, actual revision `1d17a2e2ee147bb7d6ae2ab6abdb3a2510c17775`, is pinned/checksummed in [pin-mac-arm64.json](../../calibration/oracles/chromium-css/pin-mac-arm64.json). Checksums are locally computed, not claimed as upstream-published hashes. Binary/archive remain in cache. The [protocol](../bench/css-browser-protocol.md) and [portable recipe](../../calibration/oracles/chromium-css/README.md) document native positive/negative controls, exact declaration-range validation, unchanged stylesheet text, source binding and cleanup. Scanner proposals never become truth without native acceptance.

The holdout pin was committed before first source access in `docs/bench/css-browser-holdout.yaml`, outside the canonical shard. Its 56 extensionless imports have no literal physical target, although appending `.css` finds 56 files. The unchanged Lightning CSS adapter records literal bundler reads and checks physical existence; no extension-appending resolver was substituted after observing the holdout. The older spent open-props holdout's withdrawn `@nest` traversal limit (O849) remains open. Tachyons was read with the unchanged Lightning CSS rule walk. It has no nesting and did not test or discharge that obligation. This was a protocol error: the required nesting walk should have preceded another holdout. Both holdouts remain spent and must never be reused. Implement and verify traversal of `nesting` rules before pinning or reading any new CSS holdout. Neither holdout may become a tuning corpus.

## Named candidates and X-R4 source searches

Searched 2026-10-26 UTC. Registry metadata was checked directly at `registry.npmjs.org` for PostCSS, vscode-css-languageservice and tree-sitter-css-strict; the newer strict snapshots were fetched and their compilation inputs compared. A negative catalog lookup is not proof of absence; unmeasured interfaces remain live.

| Required source searched | Candidate / version or pin | Checked result / next work |
|---|---|---|
| Own reference implementation / enumeration | [Chromium CDP CSS domain](https://chromedevtools.github.io/devtools-protocol/tot/CSS/), headless shell 156.0.8078.47 | Native declaration/property ranges measured above. Exact source-range validation confirmed in pinned Blink source and controls. Whole keyframe/parent ends remain unavailable through this route. **LIVE UNMEASURED** browser-enumeration route: `Runtime.evaluate` recursively walks `document.styleSheets[i].cssRules`, including nested CSSStyleRule, CSSKeyframesRule.name, @property and custom-property declarations. It must establish its own complete denominator and preserve source identity before any tier-A grade. |
| Own toolchain parser | Lightning CSS 1.33.0 | Existing primary oracle emits starts, not complete ends. Import truth on extensionless Tachyons input remains unattested. |
| Sourcegraph SCIP catalog | [SCIP indexers](https://github.com/scip-code/scip#readme), [Sourcegraph recommendations](https://sourcegraph.com/docs/code-navigation/writing-an-indexer#sourcegraph-recommended-indexers) | No CSS producer listed by these lookups. GitHub/web search `scip-css`, `scip-scss`, `scip-sass` found no maintained drop-in producer; does not prove none exists. |
| Sourcegraph LSIF catalog / lineage | Search `sourcegraph lsif indexers CSS`; [historical indexer-list path](https://github.com/sourcegraph/sourcegraph/blob/main/doc/code_intelligence/references/indexers.md) | Historical URL not retrievable in this audit; current recommendations contain no CSS producer. Recorded as a lookup limit, not a failed producer run. |
| Language server | [Microsoft vscode-css-languageservice](https://github.com/microsoft/vscode-css-languageservice), npm 6.3.10 | Definitions/references/parser interface exists; no full-source independent end-range measurement performed. Not used to claim a higher tier or exhausted route. |
| Registry parsers / AST dumpers | PostCSS 8.5.31 (registry); Universal Ctags 6.2.1 | PostCSS exposes source ends, but is tier C; ctags lacks usable full ends. Neither supplies a tier-A excellent-range reading. |
| Registry / production grammar | [Official tree-sitter-css v0.25.0](https://github.com/tree-sitter/tree-sitter-css/tree/dda5cfc5722c429eaba1c910ca32c2c0c5bb1a3f), MIT, commit `dda5cfc572` | Latest official tag/HEAD checked. Native 0.25.2 runtime reproduces 120/212 errors, so no newer official pin fixes them. |
| Registry / maintained grammar fork | [css-strict 0.1.0](https://github.com/meloncholera/tree-sitter-css-strict/tree/64d13f2f5b076233a631a5b922a73edcd7f7d45d), MIT, commit `64d13f2f5b` | Native ABI 15 parses the full frozen scope; same 120/212 errors. Archive/library hashes in receipt. |
| Alternative grammar runtime | [WillBooster fork](https://github.com/WillBooster/tree-sitter-css/tree/1bb2c737dd41dcb2577c3b5219a14ec91a4f7eaf), MIT, commit `1bb2c737dd` | Custom ABI `0x5742000b` is incompatible with upstream runtime. Source-read rejection as a drop-in, not a native parse result. |
| Newer strict publication / HEAD | npm tree-sitter-css-strict 0.1.1, gitHead `0c2f458ef1`; current HEAD `7b938c7dcc` | All five compilation input files are byte-identical to the measured 64d13f2f snapshot; no different parse result is implied by republishing. Exact archive/input hashes recorded. Repository package/tree-sitter metadata still says 0.1.0. Full commits are recorded in the receipt. |
| Runtime observation / tracing | [DevTools CSS coverage](https://developer.chrome.com/docs/devtools/css/reference), browser 156.0.8078.47 | Loaded/used-rule coverage is conditional on execution and DOM matches. It cannot replace complete-source ends. The no-op native range route above avoids that selection. |

## Concrete next trigger

A narrowly scoped `@-moz-document` header pre-pass is the next production candidate; it has not been measured or adopted. Before adoption, prove that it removes no graded definition/reference/import, drops pooled parse errors below 5%, and preserves original coordinates. The brief permits such a lossless pre-pass, but it is an **ENGINE** change: CSS and Tailwind share the grammar path, and HTML/Vue/Astro inject CSS styles. All affected canonical and regression readings must be regraded under K29, with a separately preregistered fresh holdout for any tuned rule. First implement and verify the Lightning CSS oracle's `nesting` rule walk (O849), then pin a new holdout before its first read. Before canonical adoption, measure the browser CSSOM population route or retain range tier C, resolve the W2/P-T1 primary-tier tension with the owner, and obtain a coverage-floor ruling if a conditional subset remains. A coverage floor cannot promote an authored population to tier A. The owner directed this browser audit to finish first and the shared pre-pass to follow in a separately serialized ENGINE wave. C2-F1 also fixes the shared grading harness to persist a range-specific downgrade; ENGINE is flagged for serialized landing, with no extractor epoch or production parser change.

Governing row: `LB-Q-css-rest`. No full-excellence or exhaustive-ceiling claim is made while these follow-ups remain open.

Timing correction: original per-run loads were not persisted, so the 2.06 result
cannot establish a regression against historical 2.61 MB/CPU-s under different
conditions. The three full extraction repeats reused only frozen oracle files,
changed no truth, and reproduced every original subject count and metric.
Canonical launch/end loads (normalize/Pico/Gitea) were run1 4.034/4.034,
4.034/4.017,4.017/4.017; run2 4.017/4.017,4.017/4.482,4.482/4.482; run3
4.482/4.482,4.482/4.845,4.845/4.845. Holdout timing-only runs were
4.845/4.845,4.845/4.845,4.845/4.777. All ran serially under nice10 on the
same Apple M2. The median is one actual whole run; no oracle was rerun or tuned.

C2-F1 correction: the prior 1.0/A range grades are withdrawn. Replays use
unchanged frozen primary bytes and range entries with only their tier corrected
to C. No browser/oracle rerun, population change or tuning is permitted. The
spent Tachyons replay corrects classification retrospectively; it is not fresh
validation. Original artifact hashes remain historical in the receipt.

C2-F1 replay verification: all four frozen bundles reproduce every count and
numeric metric. Range tier is C, so both L1 grades are good. Historical code,
source and timing identities are preserved in the dated receipt. Twelve
companion-reading scratch copies (22 subjects) regrade without any count,
metric, tier, grade or subject change; originals remain untouched. This is a
harness compatibility check, with no subject extraction or oracle execution.
