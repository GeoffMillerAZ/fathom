# P10 textarea transport, counts and retained newlines

Follow-up to [native focus](2026-09-15-catalyst-native-focus.md) under
[ADR-0044](../adr/0044-native-catalyst-settings-composition.md). This repairs the private
complete form, not a shared text-normalization API or a sixth certified/catalog target.

## Observed defects and representation contract

The recovery screenshots showed the same biography as30 code points in native mode
and29 in enhanced mode. A native URL-encoded form expands a line break to CRLF;
the textarea API exposes LF. Counting the raw transport therefore rejected valid
240-code-point multiline text. Separately, HTML parsing ignored a leading newline
immediately after the textarea start tag, losing the draft's first blank line.
The browser behavior is described by the [textarea standard](https://html.spec.whatwg.org/multipage/form-elements.html#the-textarea-element)
and [WHATWG form-normalization explanation](https://blog.whatwg.org/newline-normalizations-in-form-submission).

Validation and the visible counter now count the textarea API representation:
Unicode rune count minus CRLF-pair count. Bare CR, bare LF and combining/ZWJ/modifier
code points remain individually counted as specified; this is not grapheme counting.
The decoded submitted value is not rewritten. UTF-8, control and byte bounds still run
before validation; failed drafts and app-selected initial snapshots retain their bytes.

The view emits one extra LF immediately after the textarea start tag so the parser
can discard that LF without consuming the user's first newline. The normal templ
expression still escapes text. `SettingsValues` and `Value("bio")` remain unchanged.
The source adapter accepts only absent/soft wrapping; hard/unknown wrapping is refused
because it can introduce additional submission line breaks. The pinned source uses
the supported behavior. The optional controller applies the same count to initial
configuration, so a valid CRLF snapshot does not disable enhanced mounting.

These clarify the existing preserved-draft/240-code-point contract, without changing
authority, source assets, routes or persistence. Independent review required the
initial-config companion as well as server and displayed-count repair. No ADR added.

## Frozen browser oracle and retained failures

`newline-browser.mjs` exercises four synthetic cases across390/1440, light/dark,
native scripts-disabled and enhanced modes:240 and241 code points with a newline,
leading/trailing newlines, and240 code points containing three newlines/combining text.
Every case checks actual CRLF POST data, HTTP status, exact textarea value, displayed
counter and error state. The same runner is used before and after the implementation:
SHA256 `a8668b453a43f959bd5ab775f53edea2931200222d7d2e3753a814c117e0f8ef`.

| Directory under `data/design-assets/` | Result | Report SHA256 |
|---|---|---|
| `p10-newline-browser-LNtXRM` | 160 checks,52 failures on prior candidate | `07756980b9debd5cc234dd1d73357350bff43eb5d87c6da861f9458f29bca0de` |
| `p10-newline-browser-2U02aO` | All160 pass after repair | `9fa04195a2656840fa3cddfeb490ed7af66599e585812e8d9da2be1834435956` |
| `p10-recovery-browser-4LTE0Z` | Prior candidate fails new503 counter agreement at9 checks | `df6209739a362dfe2e3d09ebd3301275750f8c3e574d1d5f7d23feda8aee3be5` |
| `p10-recovery-browser-OsOkYL` | All122 pass after repair; eight owned processes exit0 | `56550fd0cea0e401181436f60100655156d5ba6f1b13c33781780371864207b8` |

The first diagnostic `p10-newline-browser-vj8lzW` had an incorrect counter selector;
its160 checks/64 failures are retained but are not the fixed-oracle comparison above.
The selector was corrected before the second red run or any implementation repair.
Recovery added counter equality and passive document-focus/visibility/autofocus facts;
its existing focus assertion and50ms observation barrier were not weakened.
No browser exceptions or forbidden traffic were reported in the passing runs.

## Exact replay and source-only checks

Private source packet remains `p10-source-ohWeRq`, reference SHA256
`94d3ab4c145e33bf29af750918429833d833fff3f9ac8820e1314c7f2c67f53d`.
Generated modules `p10-go-3034667623` and `p10-go-2965934993` match all sixteen files;
actual bytes and every recorded digest were independently checked, not just manifests.
Manifest SHA256: `dbfe7d318f55e610e72577e9aeb0b172c1400180435c713d589308d7ce583ad2`.
Binary SHA256: `5c3f033be5c0e3f0209ffef1f35e93542bd0330d2d60f59f17282fe3d038d950`.
Go1.26.3 darwin/arm64, templ v0.3.1020; no LLM or React build in generated replay.
Controller SHA256: `87580fe59bcd6a2d3dde234aa1337a5a3c8b0c1aa833183b7fce6a4b5f326ece`;
18,280 raw bytes and5,235 locally calculated gzip bytes, not served-transfer evidence.

`TestUnicodeLimitAndRetention`, view count/config tests, `TestCatalystTextareaLeadingNewlineSentinel`
and transformer wrapping refusals cover raw retention, limits and serialization.
JS tests include CRLF initial snapshots at240/241. Private compilation/HTTP/race/build
rehearsals pass; root race tests pass designadapt28.167s and MCP22.883s, with vet both.
Fourteen controller/focus/zoom-helper Node tests pass. Repository-wide `go test ./...`
and `go vet ./...` also pass with module proxies disabled; cached package results are
not fresh native-host runs. Full `task check` including token regeneration is not claimed.

The current binary additionally passes the unchanged regression runners, serially:

| Private directory | Checks | Report SHA256 |
|---|---:|---|
| `p10-native-browser-AziqYV` | 78 | `7ebeeb6afb5fb00074a018824fdbcbc9998d28d0d033f8a87abc556066cd51c8` |
| `p10-enhanced-browser-UmA3zu` | 81 | `ff899cd4e919379b654d798fd6ad52fac2fed5400c6ef1160391e2b79db61945` |
| `p10-seam-browser-k5uF6u` | 23 | `89af921bb1c925729fa117cafd560494ccebf516782ab0f3020c2220ade2f68e` |
| `p10-focus-browser-7OV3fb` | 656 | `67749c8d1387d9127889b6256e56be9d1f4593325ed23567ed5147bd0deb332b` |
| `p10-zoom-browser-NkCAKW` | 76 | `039fa8c14e2d090471d63a0a245adf903132282f869ba7765525920c0f91d4e1` |

Together with the newline/recovery suites:1,196 scoped assertions, no reported browser
errors. All24 native/enhanced/zoom PNGs match the preceding native-focus candidate's
corresponding images exactly. The new mobile dark503 screenshot was inspected: native
and enhanced counters now both show29, with the complete form/draft and focused retry
message preserved. This is not a whole-source fidelity or performance result.

## Remaining acceptance

The separate concurrent-autofocus observation failure remains unresolved; serial passes
do not establish an environmental cause. Full accessibility/physical input, paired source
fidelity, named performance budgets, private CLI/catalog delivery and consumer adoption
remain open. No dependency, consumer, global configuration, commit or publication change.
