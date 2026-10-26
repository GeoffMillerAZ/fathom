# C-R7 CSS browser range audit

Preregistered before the first corpus/browser measurement. C-R7 classifies a pinned
headless browser engine's own stylesheet ranges as tier A under P-T1. I54 still
requires a committed harness reading before any grade claim. No extractor, epoch,
matching rule, denominator or shared harness change is planned.

## Pins and controls

- Chrome for Testing headless shell 156.0.8078.47, mac-arm64, official stable
  manifest revision 1706539. Download its exact versioned archive from the official
  Chrome for Testing storage endpoint. Record SHA-256 of the archive and executable
  before running it. Both archive and extracted browser remain under the lane cache.
- The canonical CSS shard is unchanged: normalize.css v2.1.3 at dafaf9ee60ff76173954aca755c78a6912a0ee6c;
  pico v2.1.1 at 1039a4788d6abc368d5485ae6bac84a8f0e3096f, subpath css;
  gitea v28.1.0 at 4ebd5e319b6a9e629bfd7a04292a6fe85b08dee9, subpath web_src/css.
  Preserve all 212 discovered files and the primary Lightning CSS 1.33.0 oracle.
- One heavy phase at a time, nice 10, wait for one-minute load <=9. Disposable
  browser profile and all outputs stay in scratch. Disable background networking
  and block stylesheet network/file requests. Stop the initial full-corpus browser
  attempt after 20 minutes of actual execution; a timeout is a named route limit.

## Native attestation experiment

First probe the pinned browser's protocol and small synthetic stylesheets. The
usual matched-styles API depends on DOM selector matching, so it must not silently
restrict a complete-source oracle to easy matching selectors.

The candidate complete-source route uses CSS.setStyleTexts with an identical body.
An independent source scanner may propose a declaration-body range. It is never
truth by itself. The browser must accept the exact range and return CSSStyle.range
from its own parser, while CSS.getStyleSheetText remains byte-for-byte equal to the
input. Corrupted start/end ranges must be rejected by the same pinned browser.
A successful no-op uses a disposable in-memory stylesheet, not a subject-file edit.

Probe ordinary and comma-grouped selectors, comments/strings containing braces,
UTF-16 columns, CRLF, nested rules, empty blocks, custom properties and at-rules.
For an accepted declaration body, verify its exclusive end points at the original
closing brace before converting to the inclusive one-based closing line. A returned
CSSProperty.range may attest an individual custom property's end. Keyframe and
other kinds without native whole-definition attestation remain explicitly unmeasured.

## Corpus and grading discipline

If the probe succeeds, walk the complete frozen subject scope. Record every
candidate, native success/rejection, parse or transport failure, source mismatch,
and definition with or without native end attestation, by kind. Keep unsupported
source in the primary oracle and every original grading denominator. Never obtain
truth from Fathom output or substitute source, selectors or compiler versions.

Only independently attested ends may enter the existing .ranges.json companion.
Its scope must name eligible kinds and every coverage limit; a scanner's guess or
unmatched browser rule never acquires an end. Preserve primary definitions, calls
and import truth. Re-read css.json through the existing harness and report all
coverage counts alongside the resulting range agreement. Do not infer excellent
from synthetic probes or from a small attested subset.

An independent holdout must be pinned with criteria outside the canonical shard
before its first source/oracle read. Do not reuse the spent CSS holdout or tune
against a new one. Freeze the adapter before its one untuned reading. Run final
reading timing three times. The controller owns generated boards and plan boxes.

## Sources

The official [Chrome for Testing manifest](https://googlechromelabs.github.io/chrome-for-testing/)
provides the versioned artifact. The [DevTools CSS domain](https://chromedevtools.github.io/devtools-protocol/tot/CSS/)
defines the range and edit APIs. An upstream source inspection found exact
FindRuleByDeclarationsRange validation before SetStyleText at the pinned binary's
revision 1d17a2e2ee147bb7d6ae2ab6abdb3a2510c17775; the pinned binary's
positive/negative controls establish that behavior for this audit's actual pin.


## Instrument result and source identity

The committed [controls receipt](../assessments/data/2026-10-26-core-t1-css-browser-controls.json)
records the pinned binary's outcomes, not a corpus or language grade. Both corrupted
start and end controls were rejected. Ordinary/comma, Unicode/CRLF, empty and inactive
media declaration ranges were accepted without changing source. A nested parent's
full body was rejected: the native API exposes its declaration section, not its entire
rule. Its whole-selector end remains unattested. No candidate scanner end is admitted
for a rejected range.

At the [pinned Chromium source](https://chromium.googlesource.com/chromium/src/+/1d17a2e2ee147bb7d6ae2ab6abdb3a2510c17775/third_party/blink/renderer/core/inspector/inspector_style_sheet.cc),
FindRuleByDeclarationsRange compares both native declaration start and end;
SetStyleText rejects a mismatch. BuildObjectForStyle returns range/cssText from
that same native declaration range. This explains the positive and negative controls.

A shared complete-source manifest must bind the unchanged Lightning CSS primary
oracle and browser companion to the same clean subject commit and file bytes,
with matching pre/post hashes. A changed source or file list invalidates the run.
The raw Pico source includes 119 generated minified files the existing ingest gate
already skips. Keep those in both oracle passes and disclose the gate unchanged;
212 is the expected ingested-file count, not a reason to remove raw source files.

The strict neutral loader accepts no added metadata field. Source manifests and the
primary-byte hash therefore live in a sibling .source.json file; the primary schema
and the existing harness remain unchanged. The initial metadata-in-primary attempt
was rejected before any baseline reading and supplies no grade evidence.

## Bounded grammar follow-up before any full-excellence claim

The native-range audit leaves the existing CSS parse error count unchanged:
120 of 212 production-eligible canonical files. This exceeds the design's 5%
refusal threshold, despite excellent harness metric buckets. No legacy exemption
is assumed. The Tachyons holdout has been consumed once and is excluded from
candidate selection or tuning.

Before implementing a pre-pass, compare the current official grammar with named
existing candidates using `calibration/oracles/css-grammar/gate.py`:

- tree-sitter/tree-sitter-css v0.25.0, commit
  `dda5cfc5722c429eaba1c910ca32c2c0c5bb1a3f`, MIT; latest official release/HEAD
  checked at audit. This is the native control for the current WASM reading.
- meloncholera/tree-sitter-css-strict commit
  `64d13f2f5b076233a631a5b922a73edcd7f7d45d`, MIT; stricter URL/comment grammar,
  unmeasured before this registration.
- WillBooster/tree-sitter-css commit
  `1bb2c737dd41dcb2577c3b5219a14ec91a4f7eaf`, MIT; custom ABI `0x5742000b`
  is incompatible with the upstream runtime, so it is not a drop-in candidate.

Fetch/checksum pinned source archives into scratch and read each licence before
compilation. Build generated parser/scanner C serially at nice 10 after load <= 9.
Use the already installed tree-sitter 0.25.2 Python runtime; do not install globally.
Parse every raw file in the three frozen source sidecars with a two-second file
limit, preserving source bytes and clean checkout hashes before/after. Report all
raw files and, separately, the existing `*.min.css` production exclusion. Do not
invent a new exclusion. Official-control disagreement must be explained before
comparing the fork. Commit only counts, hashes and repo-relative summaries, never
the subject file list or source content (X-R3).

A candidate must fall below 5% pooled production-eligible files in error. Passing
this parse census is not a language grade or adoption: a grammar change still
needs ENGINE/K29 impact analysis, the actual Fathom reading and all affected
readings before integration. Failure is a named candidate result, not proof that
no further grammar or lossless pre-pass can work.
