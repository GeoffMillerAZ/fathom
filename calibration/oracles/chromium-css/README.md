# Chromium CSS range companion (C-R7)

The pinned browser validates declaration/property ends, but this authored-proposal
population grades at tier C under C2-F1. Tier A requires browser enumeration under
[C-R7/P-T1, recorded in design §1.3](../../../docs/design/language-breadth.md#13-oracle-tiers-and-what-they-can-prove).
The primary oracle remains Lightning CSS 1.33.0. This companion changes only range
counts through the existing indexbench sidecar format, not definition/call/import
truth, extraction, matching rules or denominators.

`pin-mac-arm64.json` freezes Chrome for Testing headless shell 156.0.8078.47 and
SHA-256 of both the downloaded official archive and executable. Digests were
computed locally; the upstream manifest provides a versioned URL, not an upstream
checksum. The browser and its bundled license remain under the assigned lane cache,
never in Git. Chromium's bundled BSD-style license and third-party notices apply.

Set CSS_BROWSER_CACHE to a directory under the assigned lane cache, CSS_SUBJECTS
to the parent of the clean pinned clones, and CSS_OUT to an empty scratch directory.
The subject SHAs/scopes are in calibration/corpus.d/css.yaml. A sparse checkout may
retain a complete declared subpath, but no CSS file within it may be removed.

```sh
: "${CSS_BROWSER_CACHE:?set to assigned lane cache directory}"
: "${CSS_SUBJECTS:?set to pinned clone parent}"
: "${CSS_OUT:?set to empty scratch output directory}"
mkdir -p "$CSS_BROWSER_CACHE" "$CSS_OUT"
curl -fL https://storage.googleapis.com/chrome-for-testing-public/156.0.8078.47/mac-arm64/chrome-headless-shell-mac-arm64.zip -o "$CSS_BROWSER_CACHE/headless.zip"
printf '%s  %s\n' 2fd1c09a3e1ebde9e69e6aba92f1dd3f21ad247b55d9bf66e39213ff4205436d "$CSS_BROWSER_CACHE/headless.zip" | shasum -a 256 -c -
unzip -q "$CSS_BROWSER_CACHE/headless.zip" -d "$CSS_BROWSER_CACHE/156.0.8078.47"
CSS_BROWSER="$CSS_BROWSER_CACHE/156.0.8078.47/chrome-headless-shell-mac-arm64/chrome-headless-shell"
node --test calibration/oracles/chromium-css/*.test.cjs
node calibration/oracles/chromium-css/probe.cjs "$CSS_BROWSER" calibration/oracles/chromium-css/pin-mac-arm64.json "$CSS_BROWSER_CACHE/profiles" "$CSS_OUT/controls.json"
```

Run heavy phases serially under nice 10, after waiting for one-minute load <=9.
Install Lightning CSS 1.33.0 into scratch and set NODE_PATH as its README directs.
Then produce primary truth and companions from the same clean source manifests.
The primary JSON keeps its strict neutral schema. Each primary has a sibling
.source.json containing its byte hash and the source manifest; the companion
requires both to match before launching the browser:

```sh
node calibration/oracles/chromium-css/primary.cjs "$CSS_SUBJECTS/normalize-css" "$CSS_OUT/normalize.json"
node calibration/oracles/chromium-css/primary.cjs "$CSS_SUBJECTS/pico" "$CSS_OUT/pico.json" css
node calibration/oracles/chromium-css/primary.cjs "$CSS_SUBJECTS/gitea" "$CSS_OUT/gitea.json" web_src/css
node calibration/oracles/chromium-css/ranges.cjs "$CSS_BROWSER" calibration/oracles/chromium-css/pin-mac-arm64.json "$CSS_SUBJECTS/normalize-css" "$CSS_OUT/normalize.json" "$CSS_OUT/normalize.ranges.json" "$CSS_BROWSER_CACHE/profiles"
node calibration/oracles/chromium-css/ranges.cjs "$CSS_BROWSER" calibration/oracles/chromium-css/pin-mac-arm64.json "$CSS_SUBJECTS/pico" "$CSS_OUT/pico.json" "$CSS_OUT/pico.ranges.json" "$CSS_BROWSER_CACHE/profiles" css
node calibration/oracles/chromium-css/ranges.cjs "$CSS_BROWSER" calibration/oracles/chromium-css/pin-mac-arm64.json "$CSS_SUBJECTS/gitea" "$CSS_OUT/gitea.json" "$CSS_OUT/gitea.ranges.json" "$CSS_BROWSER_CACHE/profiles" web_src/css
```

Build cmd/indexbench into scratch. Run lang-oracle three times against the same
output reading, using subject names normalize.css, pico and gitea. Set -subject to
the repository root and -tool-oracle to the respective primary JSON. If a full
checkout contains CSS outside the declared subpath, use -only 'css/**' for Pico
or -only 'web_src/css/**' for Gitea to express the complete corpus scope; never
narrow it further. The sibling .ranges.json is loaded automatically. Only its range
counts join the reading. Retain the existing generated-source gate, all original
primary truth and the unchanged W2 import/name-reference rules.

## What is and is not attested

The source scanner proposes offsets only. CSS.setStyleTexts must accept the exact
native declaration range and return that same CSSStyle.range while an identical
body is written into a disposable in-memory stylesheet. Exact text is checked before
and after all edits. Source files are never edited. A full-source manifest binds
primary and companion to the same clean commit and file bytes before/after execution.
Source metadata lives in a sibling .source.json file and binds the exact primary bytes;
the primary neutral-oracle JSON schema is unchanged.
Wrong starts/ends are rejected in the pinned controls, including UTF-16 and CRLF.

At pinned revision 1d17a2e2ee147bb7d6ae2ab6abdb3a2510c17775,
[FindRuleByDeclarationsRange](https://chromium.googlesource.com/chromium/src/+/1d17a2e2ee147bb7d6ae2ab6abdb3a2510c17775/third_party/blink/renderer/core/inspector/inspector_style_sheet.cc)
compares both native offsets. SetStyleText rejects mismatch, and BuildObjectForStyle
returns range/cssText from the native declaration range. These are parser ranges,
not echoed guesses. Eligible selectors use the native end only when it points at
the original closing brace. Custom properties use their native CSSProperty.range.
Disabled, implicit or parse-failed properties do not attest definitions.

A parent containing nested rules has a declaration section ending before its full
rule ends; its whole-selector end is explicitly unmeasured. Keyframe whole-rule ends
are unmeasured too. Rejected or unsupported source remains in the primary oracle.
Diagnostics report accepted/rejected candidates and attested/unattested definitions
by kind; a small positive subset does not prove whole-language excellence. The raw
Pico oracle includes generated minified files the existing ingest already skips.

The transport uses CDP pipe descriptors only and a disposable profile. HTTP, HTTPS
and file requests from stylesheets are blocked. Unexpected protocol errors, source
changes or a 20-minute native timeout abort without writing a partial companion.
Profiles/processes are closed on success, failure and SIGTERM. Controls are instrument
proof only. The independent holdout pin stays in docs/bench/css-browser-holdout.yaml,
outside the canonical corpus, and must not be tuned after its first reading.

For a pooled audit, all subjects share the protocol's 20-minute actual-execution
budget. Set CSS_BROWSER_TIMEOUT_MS to the remaining positive milliseconds before
each invocation (maximum 1200000), subtract completed native invocation wall time,
and stop when no budget remains. Time waiting for the load gate is excluded.

## Reproduce the harness readings

Use an empty output reading for each pass, the complete pinned clones above, and
an indexbench binary built from this branch into scratch. Primary/companion names
must stay siblings. The commands below express the complete declared subpaths;
the actual audit used sparse clones containing those same complete subpaths.

```sh
: "${CSS_INDEXBENCH:?set to scratch indexbench binary}"
"$CSS_INDEXBENCH" lang-oracle -dialect css -subject "$CSS_SUBJECTS/normalize-css" -subject-name normalize.css -tool-oracle "$CSS_OUT/normalize.json" -out "$CSS_OUT/candidate-reading.json"
"$CSS_INDEXBENCH" lang-oracle -dialect css -subject "$CSS_SUBJECTS/pico" -subject-name pico -only 'css/**' -tool-oracle "$CSS_OUT/pico.json" -out "$CSS_OUT/candidate-reading.json"
"$CSS_INDEXBENCH" lang-oracle -dialect css -subject "$CSS_SUBJECTS/gitea" -subject-name gitea -only 'web_src/css/**' -tool-oracle "$CSS_OUT/gitea.json" -out "$CSS_OUT/candidate-reading.json"
"$CSS_INDEXBENCH" lang-retime -reading "$CSS_OUT/candidate-reading.json" -subject "normalize.css=$CSS_SUBJECTS/normalize-css" -subject "pico=$CSS_SUBJECTS/pico" -subject "gitea=$CSS_SUBJECTS/gitea" -runs 3 -write
```

For the original baseline, run the same lang-oracle commands before creating any
.ranges.json companion, writing a separate baseline-reading.json. Confirm that
its counts and metrics equal the existing canonical lang/css.json before admitting
range truth. The actual audit made this equality check; no other metric changed.

The independent Tachyons pin is now spent. Reproduction is a regression replay,
never permission to tune against it or copy it into the canonical shard:

```sh
node calibration/oracles/chromium-css/primary.cjs "$CSS_SUBJECTS/tachyons-css" "$CSS_OUT/tachyons.json" src
node calibration/oracles/chromium-css/ranges.cjs "$CSS_BROWSER" calibration/oracles/chromium-css/pin-mac-arm64.json "$CSS_SUBJECTS/tachyons-css" "$CSS_OUT/tachyons.json" "$CSS_OUT/tachyons.ranges.json" "$CSS_BROWSER_CACHE/profiles" src
"$CSS_INDEXBENCH" lang-oracle -dialect css -subject "$CSS_SUBJECTS/tachyons-css" -subject-name tachyons -only 'src/**' -tool-oracle "$CSS_OUT/tachyons.json" -out "$CSS_OUT/holdout-reading.json"
"$CSS_INDEXBENCH" lang-retime -reading "$CSS_OUT/holdout-reading.json" -subject "tachyons=$CSS_SUBJECTS/tachyons-css" -runs 3 -write
```

The measured candidate and holdout snapshots live outside canonical lang/css.json
under docs/bench/readings/css-browser*.json. Their range grade is tier C, so L1 is good; L2/L3 retain their measured
primary-family grades. These candidate readings do not
override the canonical parser's 120/212 ERROR/MISSING count. See the dated CSS
assessment and native grammar census before claiming support or full excellence.

## Review corrections and exclusion census

The definition population is Lightning CSS's. Chromium attests eligible ends and
does not enumerate the population. The following diagnostic command reads the
frozen primary, companion and source binding. It never changes range truth:

```sh
node calibration/oracles/chromium-css/exclusions.cjs --self-test
node calibration/oracles/chromium-css/exclusions.cjs "$CSS_SUBJECTS/normalize-css" "$CSS_OUT/normalize.json" "$CSS_OUT/normalize.ranges.json"
node calibration/oracles/chromium-css/exclusions.cjs "$CSS_SUBJECTS/pico" "$CSS_OUT/pico.json" "$CSS_OUT/pico.ranges.json" css
node calibration/oracles/chromium-css/exclusions.cjs "$CSS_SUBJECTS/gitea" "$CSS_OUT/gitea.json" "$CSS_OUT/gitea.ranges.json" web_src/css
```

Its source-proposal categories do not reconstruct unrecorded native error messages.
Before canonical adoption, an owner ruling must set any range-coverage floor. Before
pinning or reading another CSS holdout, implement and verify the Lightning CSS
`nesting` walk (O849). Tachyons was spent with the old walk, has no nesting, and
cannot discharge that obligation. It and open-props must never be reused.

For a timing repeat with per-trial loads, run `uptime` immediately before and after
each subject's existing `indexbench lang-oracle` command above. Repeat the complete
subject set three times into three fresh output reading files, using only the frozen
primary/companion files. Verify all subject identities, counts and metrics equal the
frozen reading; select the actual whole run at the median pooled CPU throughput.
Keep the loads and all three outputs in scratch, publishing only the aggregate
loads, throughput values and hashes. This reuses frozen truth to time extraction;
it does not rerun or tune a holdout oracle. The earlier `lang-retime -runs 3` audit
did not persist per-run loads, so its timing is not a controlled performance
comparison with the historical canonical reading.

## C2-F1 classification replay

Do not rerun the browser or edit the frozen primary. Make scratch classification
copies of the frozen primary, its .source.json sidecar and companion, changing
only companion tier A to C. Rerun the lang-oracle commands above through the
corrected harness. Preserve every range entry and all non-range counts/metrics,
then recompute grades from persisted subject families. Keep original artifact
hashes and append the classification-copy and reading hashes to the receipt.
This is a retrospective correction of a spent holdout, never a new validation.

The live first-party enumeration route is `Runtime.evaluate` over the same CDP
session, recursively walking `document.styleSheets[i].cssRules`. It can enumerate
CSSStyleRule (including nesting), CSSKeyframesRule.name, @property and custom
properties without DOM selector matching. No such enumeration was measured here.

Runnable classification-copy command (the original bundle stays unchanged):

```sh
python3 calibration/oracles/chromium-css/retier.py --self-test
python3 calibration/oracles/chromium-css/retier.py \
  --primary "$CSS_FROZEN/normalize.json" --out-dir "$CSS_CLASSIFIED"
```

Repeat for pico, gitea and the spent Tachyons bundle, then use CSS_CLASSIFIED
as CSS_OUT for the grading commands. The helper emits only counts and hashes.
The replay corrects the existing candidate files in place at
`docs/bench/readings/css-browser-2026-10-26.json` and
`docs/bench/readings/css-browser-holdout-2026-10-26.json`. The receipt records
the original reading hashes, timing and Fathom commit identities, classification
harness commit, frozen artifact hashes and command templates; Git history retains
the original reading bytes.
