## VB-21 / M11b: fifty sampled receipts re-derived independently

Row (`docs/plans/verbs.md` wave E, op row): "fifty sampled receipts re-derived
independently at ≥ 95 %, and the envelope under 10 % of median payload in the
L1 replay, else the envelope is removed" {id: VB-21, needs: VB-20}.

### The sample

50 receipts, deterministic stride sample over 127 exported top-level Go
functions (`grep '^func [A-Z]'` on non-`_test.go` files) across the five
packages named in the row: `internal/semantic` (23), `internal/mcp` (14),
`internal/scenariobench` (83), `internal/analysis/serviceref` (5),
`cmd/fathom` (2). Sample index `i` (0..49) picked population index
`round(i*126/49)` — full-range coverage, all 5 packages represented; kind
assigned by a 10-slot repeating pattern (5 callers : 3 references : 2
symbol_location per 10), giving the exact required split of **25 callers /
15 references / 10 symbol_location**. Every receipt came from ONE live tool
call, unfiltered — exactly what an agent would get and could naively save:
`find_callers` for callers, `references` for references and for
symbol_location (`definitions[0]`, the documented alternate source for that
receipt kind). Daemon: `http://127.0.0.1:7575/rpc/agent`, service `fathom`,
index_commit `e1841718fa07af964a3d5dc5c9d6f68eb26516c5` for all 50 calls.
Harness: `scratchpad/vb21/build_receipts.py`, `scratchpad/vb21/tabulate.py`
(before) / `tabulate_after.py` (after). Full sample table (package, symbol,
payload/envelope bytes per receipt): `scratchpad/vb21/sample_table.md`.

### Run 1 (before the fix): 14/50 = 28.0%, one cause, zero exceptions

Verifier: `bin/fathom` built at `e184171` in a separate clone
(`scratchpad/pushclone`). `fathom receipt verify --repo <clone> <receipt>`,
once per receipt.

| Kind | n | reproduced | rate |
|---|---|---|---|
| callers | 25 | 2 | 8.0% |
| references | 15 | 2 | 13.3% |
| symbol_location | 10 | 10 | 100.0% |
| **overall** | **50** | **14** | **28.0%** |

Every one of the 36 non-reproductions (34 exit-1 `DIFFERENCE`s plus 2 that
first hit a transient operational exit 2 — shared-machine `GOMODCACHE`
write contention, unrelated to the `/Volumes/m2ext` TCC wedge — and on
retry came back the identical `DIFFERENCE`) had **the identical shape**:
`extra: []`, and every name/file in `missing` was either `Test*`-prefixed
or a `_test.go` file. Root cause: `internal/receiptverify`'s independent
oracle called `publicloc.LoadGoTruth`, whose `packages.Load` ran with
`Tests: false` — scoped to non-test packages by design, since it also
serves `cmd/loctruth`'s public-corpus ground truth and
`internal/indexbench`'s L0 index oracle. The live daemon's
`find_callers`/`references` answers are **not** scoped that way — they
correctly include `_test.go` callers and occurrences, because those are
real, indexed facts. A receipt built verbatim from a live answer for any
symbol with a test-file caller (almost every well-tested symbol in this
TDD repository) therefore claimed strictly more than the verifier's truth
set could ever contain, and `receipt verify` reported a spurious
`DIFFERENCE` on a receipt whose underlying claim was never wrong.
**Diagnosis: verifier defect** — not an index defect, not a sampling
defect. Full report: `scratchpad/vb21/report.md`.

### The fix

`internal/contextbench/publicloc` gained `LoadGoTruthTests` (`Tests: true`),
alongside the unchanged `LoadGoTruth` (`Tests: false`, kept for
`cmd/loctruth` and `internal/indexbench`'s L0 oracle — both deliberately
scoped to production code for different jobs). `internal/receiptverify`'s
two go/types load sites (`verifyCallers`, `verifyReferences`) now call
`LoadGoTruthTests`, so the independent oracle covers exactly the scope the
live answer it re-derives does — the contract VB-20 itself states
("re-derives a served claim … with go/types and git alone"). A
failing-first fixture module (a production function plus an internal
`_test.go` file that calls and references it, kept separate from the
shared fauxtown fixture so no other bench's goldens grow a test file to
accommodate it) pins both the callers and references case; all prior
goldens (including every `symbol_location` case, which never had this
problem — a definition site is never itself a test-scope question) still
pass. `internal/receiptverify/receiptverify_test.go`,
`internal/contextbench/publicloc/gotruth.go`.

### Run 2 (after the fix): 50/50 = 100.0%

Same 50 receipts (`scratchpad/vb21/receipts/*.json`, unchanged), same
`--repo` clone rebuilt at the fix commit, same index_commit
`e1841718fa07af964a3d5dc5c9d6f68eb26516c5` (`git rev-parse HEAD~1` in the
clone confirms it as the parent of the fix commit). Results:
`scratchpad/vb21/verify_results_after.tsv`, tabulated by
`scratchpad/vb21/tabulate_after.py` into
`scratchpad/vb21/tabulation_after.json`.

| Kind | n | reproduced | rate |
|---|---|---|---|
| callers | 25 | 25 | 100.0% |
| references | 15 | 15 | 100.0% |
| symbol_location | 10 | 10 | 100.0% |
| **overall** | **50** | **50** | **100.0%** |

### Remaining misses: none, after three transient operational retries

The first full pass of run 2 landed 47/50 clean on the first try, with 3
receipts (`#18` `ScenarioCellKey`/references, `#24`
`BorrowedScenarioCellsOf`/references, `#43` `RemainingPlanRows`/callers)
hitting exit 2 (`cannot run`, not a `DIFFERENCE`) — never a verifier
disagreement, always an operational failure inside the independent
oracle's own `go/packages.Load` while writing to a shared `GOMODCACHE`
under `/Volumes/m2ext` (`go: writing go.mod cache: open .../*.tmp:
operation not permitted`, once also `JSON decoding failed: unexpected
EOF` on a `go vet` intermediate file). `#18` and `#24` reproduced clean on
a same-conditions retry once a concurrent `go vet ./...` from an unrelated
sibling session (visible in `ps aux` at the time) finished and stopped
contending for the cache. `#43` needed one more step: it kept failing
identically across two retries even with no visible contention, which
matches this machine's known `/Volumes/m2ext` TCC-wedge symptom (writes,
not just reads, from a spawned Go toolchain subprocess going EPERM while
a plain shell `touch`/`echo` to the identical path succeeds in the same
second) rather than genuine contention; `tccutil reset
SystemPolicyRemovableVolumes com.mitchellh.ghostty` followed by one more
retry reproduced it clean. **Diagnosis for all three: operational
(shared-machine cache contention / TCC wedge), not verifier, not index,
not sampling** — none required a code change, and all three are
`RemainingPlanRows`/`BorrowedScenarioCellsOf`/`ScenarioCellKey` claims
that were never in doubt once the environment let the oracle load. Even
without pursuing the retries, the row's own accounting (exit-2 counted as
a non-reproduction) would have read 48/50 = 96.0%, already past the ≥ 95 %
bar; the retries were run anyway because a clean, unambiguous 50/50 is a
stronger receipt than a threshold squeaked past on operational noise.

### Envelope size

| | bytes |
|---|---|
| payload, median | 1,294 |
| payload, mean | 1,671 |
| payload, min–max | 445 – 4,684 |
| envelope, median | 98 |
| envelope, mean | 98 |
| envelope, min–max | 98 – 99 |
| **envelope / median payload** | **7.57%** |
| median of per-answer ratios | 7.57% |
| mean of per-answer ratios | 8.13% |

The envelope (`{"resolution":{"typed":N,"heuristic":N},"index_commit":"<40
hex>"}`) is a near-fixed ~98–99 bytes regardless of answer size — it varies
only by 1 byte when a resolution count crosses a digit boundary. On this
50-sample live draw it sits at 7.57% of the median payload, comfortably
under the 10% bar (median-of-corpus reading, per the row's own wording).
This corroborates the earlier 12-call reading
(`scratchpad/vb21-envelope-reading.md`: 6.2% of the real-corpus median with
that reading's corrected denominator) on a larger, deterministic sample —
same conclusion, same ballpark. **Caveat, unchanged by the fix**: the
envelope is a much larger fraction of the smallest individual answers —
`NegotiateProtocolVersion` (445B payload) carries it at 22.0%,
`LoadFakeStream` (470B) at 20.9%, `Join` (454B) at 21.6% — so a per-answer
(rather than median-of-corpus) reading of "under 10%" would fail for the
smallest answers. Not re-measured in run 2: the fix touches only which
receipts *reproduce*, never what a receipt or its envelope *contains* —
envelope/payload bytes are unchanged from run 1.

### Verdict

VB-21 holds, on a corrected instrument. Run 1's 14/50 (28.0%) was never a
finding about the daemon's `find_callers`/`references` answers or about
receipts as a mechanism — it was a defect in `receipt verify`'s own
independent oracle, which scoped its `go/packages.Load` to non-test files
while the live answers it was checking correctly include test-file
callers and occurrences. `docs/plans/verbs.md`'s M11b sample is exactly
what caught it: a live 50-receipt draw against real, well-tested code,
not the smaller golden fixtures, which never exercised a test-file
caller/reference case (`internal/receiptverify`'s own goldens all
predate this fixture). With `LoadGoTruthTests` in place, all 50 sampled
receipts (25 callers, 15 references, 10 symbol_location, stride-sampled
across all five named packages) re-derive independently and reproduce —
**50/50 = 100%**, clear of the ≥ 95 % bar. The envelope holds too: 98 B
median against a 1,294 B median payload is 7.57% (median-of-corpus,
matching the row's own wording; 8.13% mean of per-answer ratios),
consistent with the earlier 12-call reading (6.2%) and clear of the 10%
bar — with the same caveat as before, unaffected by this fix, that the
smallest individual answers (`NegotiateProtocolVersion`, `LoadFakeStream`,
`Join`) carry the envelope at 20–22% each. Both clauses of the row's
contract are met; the row flips to done.
