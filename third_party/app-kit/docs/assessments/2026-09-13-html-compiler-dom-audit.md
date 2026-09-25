# Official HTML compiler and DOM diagnostic — 2026-09-13

All **439 syntax-accepted inputs compile and render the same namespace-aware DOM as
their originals**, before and after templ formatting. This is 439/657 (66.8%) of the
frozen official HTML inventory, not 66.8% of the entire purchased collection and not
visual, interaction or target-compatibility certification. No converter rule, grammar
limit or fidelity assertion changed to obtain these results.

## Measured coverage

| Official HTML archive | Inventory | Syntax accepted | Compiler/DOM passed in both phases | Other inputs |
|---|---:|---:|---:|---:|
| Application UI | 364 | 260 | 260 | 104 |
| Ecommerce | 114 | 60 | 60 | 54 |
| Marketing | 179 | 119 | 119 | 60 |
| Total | **657** | **439** | **439** | **218** |

The other 218 inputs remain 217 grammar refusals and one source-size refusal from the
[full syntax audit](2026-09-13-html-syntax-audit.md); they were not compiler-tested or
silently removed from the denominator. Accepted cases have zero failed/unproven batches.
The five previously saved/compiled pilot artifacts are not added again to this count.
React/Vue variants, full templates, Catalyst and MatDash remain outside this denominator.

## What ran

The opt-in [driver](../../internal/designconvert/coverage_dom_live_test.go) reads the
immutable syntax report, verifies its SHA-256, rereads every accepted original with
exact member hash/size checks, then regenerates identical rule/output identities.
The [fixture helper](../../internal/designconvert/coverage_dom_fixture_test.go) admits
only the existing literal templ grammar and groups at most 32 cases per batch.

Each of 14 batches runs pinned templ generation, separate compile-only and rendered-DOM
tests, then templ formatting and the same generation/compile/render checks again.
Comparison retains node types, namespaces, text, comments and exact attribute arrays;
classes, ordering and whitespace are not normalized away. Only the component declaration
is renamed to avoid duplicate names in a batch; original HTML and generated literal
bodies are unchanged. This adaptation is explicit in the metadata report.

Generated sources, compiler caches and temporary directories live under one private,
test-owned scratch tree, cleaned after the successful run. External Go cache helpers
are disabled; persisted Go settings cannot re-enable them. Dependencies must already
exist locally: pinned tools run with downloads disabled and modules read-only.
No vendor project or JavaScript ran. Subprocess diagnostics are discarded so compiler
errors cannot print paid source. Only a metadata report is retained; both original and
saved-attempt catalogs have the same before/after hashes. No derivative was published.

## Reproduce and inspect

```sh
APPKIT_PROBE_HTML_DOM_KIT_ROOT=/absolute/path/to/app-kit \
  go test ./internal/designconvert -run '^TestHTMLInventoryCompileDOMLive$' -count=1 -v -timeout 15m
```

The exact private syntax report must already exist. Missing opt-in configuration is a
skip, not a corpus pass. Changed source/rule/output identities fail; regenerate and
review the syntax baseline rather than bypassing its frozen hash. Optional
`APPKIT_PROBE_HTML_DOM_REPORT=/absolute/new-file.json` writes metadata only, mode 0600,
with exclusive creation. Default invocation saves no report or catalog entry.

Observed: Go 1.26.3 darwin/arm64, templ 0.3.1020, x/net 0.54.0; **166.21 seconds**
test time including original reads and translation. This is local diagnostic duration,
not browser rendering performance or measured integration payback.

- Input-set SHA-256: `d57b19cec955cc89c094e304ab7bb6d39ec1188fc88b0d6cb4c99f50ec0580b0`.
- Syntax-report SHA-256: `35067bfc3e591bfbecb2dd6ea20038b28159a97a02cf57fdda934004eddba79c`.
- Rule SHA-256: `0db161e2e0e567ae491661614e0115f4042c3ea76b188f867c8242e44e90eb7c`.
- Driver SHA-256: `85526c55fdfd2a8f12a56aa06db22535b59bb32c1764fffa668f3a7c8dc98248`.
- Fixture-helper SHA-256: `f85d1b4a4cfb1b2e24aae6c869c791c18e9d56727abeff0719c5a5ec9b4e1edd`.

The 208,908-byte metadata report is retained privately at
`data/design-assets/coverage/fc3b7b06c22ac01ed140651034776d5ab05c8d4ccb3f42b81a8b56d3a1d50e66.json`;
its filename is its SHA-256. It has 439 source-bound rows, output hashes and both-phase
dispositions; no source/generated bodies. Every record remains `not-verified` at the
report/translation level. This diagnostic does not create target receipts.

## Failure controls and evidence limits

Independent review caught a harness false-pass risk before the corpus run: Go's
`t.Run` returns true even for filtered-out subtests. A batch now counts only when its
full check actually completed; filtered batches remain unproven and fail a full audit.
A real subprocess-filter canary covers this behavior. Synthetic text/dark-class
mutations are detected by the unchanged DOM oracle. Bounds/executable-input refusals
and cache-environment isolation also have regression tests.

Independent converter/MCP race suites passed (21.641s/21.749s), with live/private
probes explicitly disabled; those tests are separate from the actual corpus run above.
The in-app browser connection was retried and remained unavailable. No graphical,
CSS/font/asset, accessibility, focus, interaction, lifecycle or native-host pass is
claimed. Saved pilot counts and verified-target discovery are unchanged.

Next: earn source-native rendered baselines and representative interaction-family
proof under the [conversion pipeline](../design/conversion-pipeline.md), preserving
the original-first fallback. Compiler success supports that work; it does not waive it.
