# Compact command views

Status: accepted profile for GT13a; implementation/qualification tracked in the
[GT work order](../plans/governed-command-tools.md). Extends [GT-C4](governed-command-tools.md#gt-c4-economical-and-faithful-output)
under 0039's existing W1 budget. No installation, new dependency, generic reducer,
model call, persistent store or exported library API.
[Scoped qualification](../evidence/command-tools/2026-10-24-compact/README.md) records
executed behavior, measured byte reduction, added encoding cost and remaining limits.

## CV-1: reduce syntax, preserve the selected observation

`command-guard inspect|git-inspect --format compact` is explicit opt-in. Existing
`json`/`text`, request/control contracts, scans, source selection, disclosure checks,
status codes and resource limits retain their semantics except the INC-21 cap repair:
inspection failure reports now respect their accepted effective output cap, including
newline; if the fixed report cannot fit, exit 2 writes no bytes. Compact is escaped JSON,
never unescaped terminal text. Required failures stay ordinary privacy-bounded reports.
No model belongs in deciding which diagnostics to omit: omit none at this layer.

Replace repeated object field names with fixed positional tuples only after all
selected-source checks succeed. Preserve every report header field, row value, order,
coordinate, source identifier, scanner witness, count, truncation and omission state.
Do not summarize text, collapse rows, suppress repetitions, remove progress or infer
cleanliness. Existing projections remain partial where their reports say so.

Choose the shorter of the tuple JSON candidate and the ordinary minified JSON report,
preferring ordinary JSON on a tie. `compact` therefore accepts either self-identifying
wire version below. Never promise that every invocation uses tuples or saves bytes.
Failure and small-report fallback are useful behavior, not an error or silent bypass.

## CV-2: frozen tuple grammar

Inspection tuples use version `command-guard.compact-report/1`,
`rowFormat: "file-line-text/1"`, and `rows: [[file, line, text], ...]`.
All other fields equal `command-guard.report/1` for the selected projection.

Git tuples use version `command-guard.compact-git-report/1` and exactly one layout:

| operation / rowFormat | tuple positions in order | reconstructed kind |
|---|---|---|
| git-status / status/1 | kind, path, xy | kind is carried explicitly |
| git-object-diff / object-diff/1 | path, beforeOid, afterOid, before, after, beforeOmittedLines, afterOmittedLines | change |
| git-history / history/1 | oid, parentIds, author, committer, message, messageOmittedBytes | commit |

All other Git header fields equal `command-guard.git-report/1`. Each row has exactly
the declared tuple arity. Empty optional strings, empty lists and zero omission counts
reconstruct as absent fields under the existing `omitempty` wire semantics. Empty
blob sides and root-commit parent lists are valid, not unknown. Before/after contain
the same selected line views, not a patch. No additional Git content is retrieved.

Only successful `no-findings`/`ok` reports are tuple candidates. If a header is outside
the closed compact schema or a row contains a kind/field outside its exact layout,
retain the ordinary JSON report; never drop an unfamiliar future field.
The version and rowFormat jointly identify the grammar.
Future layout changes require a new version. Native JSON escaping remains mandatory.

The CUE contract must close all tuple variants, their operation/layout pairings and
header fields. It rejects missing/extra tuple elements and wrong position types.
Actual native output is checked against the applicable ordinary or compact definition.
Go owns behavior, scanning, encoding choice and output-cap accounting.

## CV-3: caps and checked detail

Apply the same effective source, result and final output limits, including the trailing
newline. Compare encoded candidates deterministically while selecting a bounded prefix.
The minimum of two monotonically growing encodings remains monotonic; qualify this
at count digit boundaries and representation crossovers rather than assuming it.
Existing raw projection caps may still limit candidates before encoding; compact is no permission
to read more source or relax a result budget. It may fit more already-checked candidate
rows into the output cap than ordinary JSON. Report that projection's actual counts.
Git additionally charges its normalized selected-row representation to the total
scan budget. Before scanning that representation, compact prefix feasibility must
satisfy result/raw projection, final output including scanner reserve/newline, and
normalized representation plus already-selected source bytes. Never shrink or retry
after a finding to evade it. Ordinary json/text retains its existing projection logic.
Bound repeated
encoding work over at most 1000 candidate rows; test maximal feasible prefixes.

Tuple file/line values retain the same request-ordinal and line coordinates. For a
detail call, retain the original file pins/control scope and rerun the normal guarded
request. Git reports retain their resolved commit identities and selected paths; a
fresh `HEAD` request can observe another revision. Existing Git line/message ceilings
still apply. This slice adds no raw/detail bypass, pagination authority or claim that
all content omitted by the original profile can now be retrieved.

Never cache a successful report or scan bit as permission to disclose. GT13's exact
observation cache remains unimplemented until its current-authority, disclosure,
scanner identity, freshness and invalidation profile is independently qualified.
Persistent cache/key management is not a prerequisite for syntax reduction.

## CV-4: predeclared baseline and acceptance

Use owned synthetic data, <=64 MiB total, no credentials/provider calls. Freeze fixture
and oracle identities before the final trial. Compare ordinary and compact at generous
caps, independently decode tuples to ordinary rows/headers, then compare full JSON
semantics. Tests also compare constrained caps where compact can return a longer prefix.

| corpus/task | independent acceptance |
|---|---|
| 400 distinct inspection lines with a known marker at line 289 | exact marker/file/line, all 400 row values and counts reconstructed; >=15% byte reduction |
| 30 changed/untracked synthetic paths | exact kind/path/xy set/order and live-observation qualifiers; >=15% byte reduction |
| added/deleted/changed text blobs and root/non-root commit history | exact selected sides, object/parent identities, authors/messages and omitted-line/byte counts |
| empty/single-row/Unicode/quotes/tab/CRLF inputs | same decoded observation; compact never larger than ordinary JSON for the same projection |
| hidden finding beyond a view cap, denied path, bad digest, invalid controls/scanner/metadata | same fixed failure/exit; no source tuple, body, path or digest leaks |
| tight byte/result/total caps, digit/fallback crossings and short/failing writer | maximal feasible prefixes checked exhaustively, bounded output including newline, truthful counts, no post-finding shrink/retry or claim of atomic delivery |

Preserve every known task answer and diagnostic; zero missed required diagnostic is
the acceptance bar. Retain exact bytes for both formats, exit statuses, output-byte
counts, source/fixture identities, native platform and command identity. Report bytes
as bytes, not estimated or measured tokens. Record local encoding latency/allocations
with a bounded benchmark and its host/limits; do not call it production p95, task cost
or end-to-end improvement. GT17 still needs retries, human effort and real accepted
task economics before adopting a format by default.

The initial 40-path native fixture hit GT6's existing 32-status-row `result_limit`.
Retain that negative. The successful task baseline uses 30 paths within the admitted
profile; do not raise source limits or reinterpret a held report as compact success.

Required verification: focused adversarial/race tests, CUE positive/negative shapes,
native entrypoint and independent decoding, guide-only adoption trial, independent
security review, full `task validate` and strict plan proof before a completion claim.
Cross-builds retain compilation-only labels. No Windows/Linux native claim follows.

## CV-5: owner decisions and rejections

- Keep compact layout local to command-guard. app-kit owns reusable CLI presentation;
  its current status-table/NDJSON primitive does not own this source-data grammar.
  Retain AK-CLI-02's stdlib divergence and record the contribution-loop outcome.
- Reject repository-defined reducers, summarizers and automatic RTK filter execution:
  these can hide required evidence. Revisit only with exact trusted reducer scope and
  an independent diagnostic-retention oracle; no such need is established here.
- Reject claiming syntax savings as fewer tokens, lower cost, improved judgment or
  faster reviews. Revisit after a pinned tokenizer/real accepted-task baseline.
- Reject making GT9/10 pass through fake native authentication or a Boolean prompt.
  They retain actual issuer-bound presence, durable state and host interception proof.

Revisit when a tuple cannot reconstruct the selected ordinary observation, a diagnostic
is hidden, metadata leaks on a failure, a format/cap changes disclosure checks, encoding
cost exceeds the declared task budget, a real consumer needs a different layout, or
0039's 2026-11-21/next-release review arrives. Keep full GT13 and the overall goal open
until their remaining acceptance is met. Prose/HITLMAN remains the flagship under 0038.
