# Free measurement contract: FCT-10 coverage feasibility

Status: T4-2 feasibility-only amendment for separate lane review; contract only, no reading results.
Base: `83226aece9b0f421381373f8dafc4a912d3888e5` (all production sources and fixtures pinned to this commit).

## Intent and scope

Governing text: `docs/plans/realignment.md` FCT-10 asks for foreign coverage for
five languages with blocks joined to symbols, references TQ-M3, and explicitly
needs no build of new functionality. `docs/plans/test-intelligence.md` TQ-M3
requires at least five languages; its governing design at
`docs/design/test-intelligence.md:168` specifies **Go plus four T3 formats**.
T4-2 preserves that original population: the five synthetic structural controls
may run only as labelled feasibility and close neither FCT-10 nor TQ-M3.
TQ-9..14w name the existing ingestion and join path.
`docs/design/test-intelligence.md` T3 permits the six formats and requires
real symbol ranges or an explicitly named file fallback. ADR-051 decisions 1–2
require joining existing assets, without another engine/store design. I38,
ADR-027, P7/P22 and principles T5 require complete inputs and unchanged criteria.
No applicable intent trigger is established by this contract. No compatibility
surface, API, policy, dependency or product implementation changes.

D1: Measure **synthetic format-control ingestion and symbol-join structure** on
five actual public source fixtures through current production APIs. T4-2 permits
these controls only as a labelled feasibility check at that evidence scope. Source
bytes are unchanged; report paths/line locations are adapted explicitly below.
This does not measure actual runtime coverage, native producer interoperability,
independent-oracle accuracy, whole-language accuracy, deployment or fleet health.
The alternative, treating parser-test success or Go-only joins as five-language
proof, is excluded by the issue and the five-language row. Unmodified parser
reports paired with unrelated sources are also not valid positive controls.

D2: The only reading-lane tracked output is
`docs/bench/readings/realignment/fct-10-foreign-coverage.md` (new at base).
The absent `docs/bench/readings/tests/coverage-languages.md` is not a prior result
and is not owned. This contract does not authorize a second reading, plan edits,
new tracked tests/fixtures, a daemon, a shared fleet store or any paid call.
Scratch runner, reports, DB, logs and binaries stay under
`workspace/tmp/free-measurements/`. Grounding: issue ownership and workspace rule.

D3: The five Python/JavaScript/Java/Ruby/C controls remain the feasibility
denominator, unchanged. They do not replace the original Go plus four T3
formats cohort, add Go to these trials, or select the original four formats.
Formats are not languages, and a file-level fallback is never counted as a symbol join. Each source has at least
one preregistered callable with real golden ranges. Goldens are local fixture
expectations, not outside oracles. The five were chosen before trials to pair
five native-format examples with five existing language fixtures, not after
observing a join outcome. No fallback language may replace a failure.

## Frozen inputs and deterministic construction

Use the base commit's bytes verified by SHA-256 below. Refuse drift before any
trial rather than silently refreshing fixtures, expected counts or code. Record
runner SHA-256, toolchain/OS/architecture, base/tree, exact commands and all
transformed report bytes/hashes in the reading. A different base needs review
of this manifest before trials. Goldens are read as expectations only; never
insert their symbols into the store.

Copy each source byte-for-byte to the scratch corpus path below. Do not copy
its surrounding `testdata` directory (the local connector excludes testdata),
resolve its imports or execute its program. The pipeline extracts the actual
source; unavailable imported libraries do not justify fabricated nodes/ranges.

| Language | Source under `internal/extractor/dialects/` | Scratch source path | Expected target name/kind/range | Report seed | Line shift | Expected parsed blocks/statements/covered |
|---|---|---|---|---|---|---|
| Python | `python/testdata/shapes.py` | `python/shapes.py` | `top`, PythonFunc, 13–17 | `coveragePySample` | +12 | 3 / 3 / 2 |
| JavaScript | `javascript/testdata/bindings.js` | `javascript/bindings.js` | `create`, JSFunc, 17–21 | `istanbulSample` | +16 | 2 / 2 / 1 |
| Java | `java/testdata/shapes.java` | `java/shapes.java` | `Widget.sum`, JavaMethod, 13–21 | `jacocoSample` | +12 | 2 / 2 / 1 |
| Ruby | `ruby/testdata/calls.rb` | `ruby/calls.rb` | `Pipeline.run`, RubyMethod, 6–16 | `TestParseLcov` local `in` | +5 | 3 / 3 / 2 |
| C | `c/testdata/shapes.c` | `c/shapes.c` | `add`, CFunc, 36–41 | `llvmCovSample` | +35 | 2 / 2 / 2 |

Seed literals are in `internal/analysis/coverage/`:
`coveragepy_istanbul_test.go` (Python/JavaScript),
`cobertura_jacoco_test.go` (Java), `lcov_test.go` (Ruby), and
`llvmcov_test.go` (C). Extract the named Go raw string literal using Go's
`go/parser` and `strconv.Unquote`, scoped to the named declaration/function;
require exactly one match. Do not retype seed hits or harvest expected block
arrays as if they were observations.

Transform only source locations; preserve all hit counts, statement IDs,
columns and segment flags. These transformations create synthetic controls,
not genuine report/source pairs from a runtime:

- Python: replace `files` key `src/widget.py` with `python/shapes.py` and add
  12 to every executed/missing line. Keep meta/totals unchanged.
- JavaScript: replace the top-level report key and `path` with
  `javascript/bindings.js`; add 16 to every statement start/end line. Keep
  columns and `s` hit map unchanged, including the multiline second statement.
- Java: replace JaCoCo `sourcefile` name `Widget.java` with `java/shapes.java`;
  add 12 to each `line nr`. Package names and instruction counters unchanged.
  The existing parser reads the sourcefile name, not a package-joined path.
- Ruby: replace lcov `SF` with `ruby/calls.rb`; add 5 to every `DA` line and
  `FN` definition line. Retain TN, FN name, all counts and summary records.
- C: replace llvm-cov `filename` with `c/shapes.c`; add 35 to the first element
  (line) of every segment, including skipped/no-count/gap segments. Keep other
  elements unchanged: line 37 must OR-merge its two counted segments, while
  shifted lines 38 and 39 emit no blocks.

Serialize transformed JSON deterministically (sorted object keys, fixed indent,
final newline), XML with fixed UTF-8 encoding and final newline, and lcov with
original record order and final newline. Publish exact bytes rather than
claiming every serializer produces the same hash. Freeze them before ingest;
never use actual extracted ranges to move a control after a failure.

## Existing production path and prospective execution

No new product helper is needed. A scratch Go runner located within this
module can import `pkg/client` and `internal/analysis/coverage`. It opens a new
scratch SQLite DB using `client.NewConstantClient(client.ModeLocal, dbPath)`,
calls `Client.Ingest(ctx, client.ServiceRef{ID: "free-coverage", Path: corpus})`,
then `Client.ListAllSymbols(ctx, "free-coverage")`, and closes the client.
These are current production APIs in `pkg/client/client.go`, through the local
connector and fusion pipeline. Require nil ingest error, Discovered=5, Extracted=5, Skipped=0, ReadFailed=0,
Oversize=0, SkippedGenerated=0 and no unsupported languages, with all five source
paths accounted for; log the complete ingest report and symbol projections (file,
name, kind, line, end line and Key()). Do not populate the DB directly or
construct positive SymbolNodes. The DB is scratch storage, not a new store
schema or a daemon dependency.

Parse the five written reports with, respectively,
`coverage.ParseCoveragePyIstanbul`, `coverage.ParseCoveragePyIstanbul`,
`coverage.ParseCoberturaJaCoCo`, `coverage.ParseLcov`, `coverage.ParseLLVMCov`.
For every parsed slice call `coverage.JoinForeignBlocks(blocks, nodes)` with
the actual full indexed node set; do not filter away an unranged callable to
make a file range-eligible. These are the same parsers and join called by
`cmdCoverageIngest` / `runTestsCoverage` (TQ-9/TQ-14w). The latter opens the named
store and calls `Client.ListAllSymbols`; its CLI-level behavior is separately
covered by `TestTestsCoverageJoinsForeignBlocks`. The scratch API method does
not itself claim to have tested the CLI argument/size-cap path.

Use two fresh scratch DBs in two complete trials, same frozen input bytes,
serial execution. This is deterministic conformance, not a latency sample;
no clock, percentile, power calculation or statistical accuracy claim applies.
Raw rows from both trials are published. Compare canonical projections of
nodes/blocks/joins, excluding ingest timestamps and duration fields. Sort nodes
by file/line/kind/name/key, blocks by file/start/end/statements/covered, joins by
file/line/key, and report all entries, including ByFile and Unmatched. No mean
or aggregate may hide a language's failed or missing join.

Run the scratch runner as `go run -race ./workspace/tmp/free-measurements/runner`
under `bash scripts/gate-lease.sh`, with TMPDIR/GOTMPDIR pointing into the scratch
directory after sourcing the prescribed lane environment. Set FATHOM_HOME and
FATHOM_STATE_DIR to fresh directories under that scratch root; unset inherited
FATHOM_VERIFY_LEDGER, FATHOM_INTENT_LEDGER, FATHOM_SESSION_LEDGER and
FATHOM_HOOKS_LOG overrides without reading their values. The existing
`internal/fathomhome` relocation isolates global ignore files and grammar caches;
retain the machine-wide gate lock. Give every store path explicitly. Do not use deployment, MCP, a live store, a model, network
fetches or native coverage producers. This is a prospective command; the
contract author has not run the measurement.

## Named assertions and controls

These are scratch-runner assertions, not new tracked Go tests. On any mismatch,
retain the actual data and report the failing assertion; never fix expected
values after observing them.

- `InputsFrozen`: all manifest hashes and copied source hashes match before
  each trial; all five reports exist, are readable and below 256 MiB. Report
  seeds and transformations are exactly those above.
- `SourcesReallyIngested`: five distinct source paths yield actual indexed
  symbols. Each target exactly matches name, kind and golden start/end; every
  callable in its file has EndLine > 0. Missing targets or range fallback are
  observed failures, never repaired by injecting golden nodes.
- `FormatsParsed`: per-language block/statement/covered counts equal the table.
  Python lines 13/14/15 have covered 1/0/1; JavaScript spans 17–17 and 19–21
  have 1/0; Java lines 13/14 have 1/0; Ruby lines 6/7/10 have 1/0/1; C lines
  36/37 have 1/1. Each block names exactly its scratch source path. Tests arrays
  are empty/nil for these seeds; a hit count is not per-test attribution.
- `FiveActualSymbolJoins`: each result has exactly one BySymbol row, its key
  obtained from that trial's actual target node, and the row has the table's
  statement/covered totals and the target's real range. ByFile and Unmatched
  are empty. Percent is 100*covered/statements (compare within 1e-9, report raw
  counts too). All five distinct languages must satisfy this assertion.
- `UnmatchedNeverGuessed`: on separate control copies, rewrite every parsed
  block File to `absent/<language>.missing`, call the existing join with the
  original actual nodes, require no BySymbol/ByFile rows and exactly that
  distinct unmatched path. These controls never enter the five-language count.
- `UnrangedDemotesFile`: on separate copies of actual nodes, set only the
  target's EndLine to 0. Require no BySymbol for that source, one ByFile row
  with its unchanged totals, and no unmatched paths. This deliberate synthetic
  mutation proves fallback behavior, never that the extractor lacks ranges.
- `OutOfRangeDoesNotBecomeCoverage`: move control block spans beyond the
  maximum actual callable EndLine in the corresponding file (max+10, preserving
  each block's width). Require no BySymbol/ByFile rows and no unmatched file:
  known file/outside symbol is different from unknown file. Report omitted
  spans explicitly; never silently subtract them from positive denominators.
- `Repeatable`: both fresh-store canonical projections are byte-identical.

Existing regression proofs, with exact scope: `TestParseLcov`,
`TestParseCoberturaJaCoCo_JaCoCo`, `TestParseCoveragePyIstanbul` and its Istanbul
case, `TestParseLLVMCov` hold the seed normalization expectations;
`TestJoinForeignBlocksByRange`,
`TestJoinForeignBlocksByFile_OneUnrangedSymbolDemotesWholeFile` and
`TestJoinForeignBlocksNamesUnmatchedFiles` cover join behavior.
`TestTestsCoverageJoinsForeignBlocks` covers CLI joins over Go fixtures only.
None alone is the five-language measurement.

Specific package proof commands (serial, under the gate lease; never full gate):

```sh
bash scripts/gate-lease.sh go test -race ./internal/analysis/coverage -run 'Test(ParseLcov|ParseCoberturaJaCoCo|ParseCoveragePyIstanbul|ParseLLVMCov|JoinForeignBlocks)' -count=1
bash scripts/gate-lease.sh go test -race ./cmd/fathom -run '^Test(CmdCoverageIngest.*|BoundedReader_CapEnforced|TestsCoverageJoinsForeignBlocks)$' -count=1
```

## Reporting and verdict law

The reading must lead with **synthetic format controls; actual source extraction
and scratch-store symbol joins; runtime coverage and oracle accuracy unmeasured**.
Publish source/report hashes, exact transformed bytes, runner source/hash,
commands, exit codes, raw ingest/node/block/join/control rows for both trials,
and a five-row table of targets/ranges/counts/fallback/unmatched/assertion
outcomes. Raw trial failures and missing inputs stay visible. Existing summary
fields in a format seed are fixture metadata, never observed test-run evidence.

The original threshold remains at least five languages (Go plus four T3 formats)
with blocks joined to symbols. The five controls cannot satisfy that population.
Report their assertion outcomes separately as **feasibility structural controls**:
all assertions passing in both trials establishes feasibility only; a refuted
assertion is a feasibility failure; drift, missing inputs or incomplete trials
leave feasibility inconclusive. Preserve failures and missingness together.
Neither a successful probe nor a successful formal feasibility reading is an
original-cohort result. Existing critic scratch SQLite/race probes are only
runnability evidence; this contract records no completed reading. A later
approved report lane must run this fixed method fresh with two new stores and
publish both trials. Probe receipts do not substitute for those trials.

The eventual report must explicitly label itself **feasibility**, report the
structural controls separately, name the original input gaps below, and state
**FCT-10 and TQ-M3 remain open**. It must end with this exact line:

FCT-10 verdict: INCONCLUSIVE (original cohort unread)

This partial report does not discharge the original measurement, default-on or
support claims, native accuracy, runtime coverage, outside-oracle accuracy,
deployment or fleet health. No plan checkoff is authorized, even if a lexical
verdict pattern matches. This lane edits no boxes and claims no daemon proof.
No post-data threshold, cohort or method change is authorized.

## Original cohort input inventory

Inventory basis: the pinned base above, `docs/design/test-intelligence.md` T3 and
section 5 line 168, `docs/plans/test-intelligence.md` TQ-9..14w/TQ-M3, and
`docs/plans/realignment.md` FCT-10. The design supplies the population but does
not choose the four formats or their language/source/report pairs. Searches of
`docs/bench/` and `docs/contracts/` for TQ-M3, FCT-10 and coverage-languages
found no original-cohort preregistration beyond these governing requirements;
the present contract registers the separate feasibility controls only.
`docs/bench/readings/tests/coverage-languages.md` is absent at base, so there is
no original reading there to reuse. This is not evidence that parser or Go
examples are absent.

| Required original input | Available record or fixture | Exact remaining gap |
|---|---|---|
| Frozen original cohort registration | Design line 168: Go plus four T3 formats, at least five languages; TQ-M3 and FCT-10 remain unchecked | No frozen original roster selecting the four formats, their distinct languages, source/report paths, target symbols/ranges, byte hashes and report-to-source/commit provenance. The five controls above register a different feasibility population. No new four-format selection is made here. |
| Go source/profile pair | **available-as-example**: `cmd/fathom/review_test.go` `reviewFixtureSrc` defines `pkg/a.go`, function `A` at lines 3–5. `cmd/fathom/tests_coverage_test.go` `testsCoverageFixture` ingests it; `writeCoverProfile` constructs `cover.out` with a `mode: set` block using the extracted range; `TestTestsCoverage_JSONReportsJoinedSymbol_BarePathDefaultsUnit` joins it. `internal/analysis/coverage/profile_test.go` `TestParseProfile_Set` is a parser example. | No Go pair selected and frozen for the original reading, with exact source/profile bytes, hashes and binding/provenance. The CLI example generates the profile from extracted ranges; it is not a registered, frozen original measurement pair. |
| Four T3 format/language/source-report pairs | All six parser seeds exist (listed next); the five source fixtures and goldens in the immutable manifest exist | No governing selection of which four formats and languages, nor frozen corresponding source/report pairs and their source locations, hashes, provenance and expected joins. Existing seeds alone do not identify the original four; synthetic path/line adaptation does not supply that registration. |
| Commit association and reading receipts | T3 and TQ-9 require a report commit or command-line binding; an unbound report never gates. `cmd/fathom/coverage_ingest_test.go` `TestCmdCoverageIngestBindsCommit` supplies regression evidence | No original-cohort report binding/provenance or original trials/results is registered in the examined records. A source commit alone does not prove a report was produced from that source. |

The available T3 seeds are in `internal/analysis/coverage/` at the base and
hashes in the immutable manifest:

| Format | Exact seed symbol/local and original source location |
|---|---|
| lcov | `lcov_test.go`, `TestParseLcov` local `in`: `src/widget.py` |
| Cobertura | `cobertura_jacoco_test.go`, `coberturaSample`: `src/widget.py` |
| JaCoCo | `cobertura_jacoco_test.go`, `jacocoSample`: package `com/widget`, sourcefile `Widget.java` |
| coverage.py | `coveragepy_istanbul_test.go`, `coveragePySample`: `src/widget.py` |
| Istanbul | `coveragepy_istanbul_test.go`, `istanbulSample`: `/repo/src/widget.js` (fixture report path) |
| llvm-cov | `llvmcov_test.go`, `llvmCovSample`: `src/widget.c` |

`cmd/fathom/tests_coverage_foreign_test.go` `foreignCoverageFixture` and
`TestTestsCoverageJoinsForeignBlocks` are also available-as-example: lcov
`cov.info` joins `pkg/a.go` function `A`; coverage.py `cov.json` names
`pkg/consts.go` and demonstrates file fallback; the unmatched report entry is
preserved. These are reports over Go fixtures, not four other languages. The
additional Go/example files are pinned to the same base for this inventory:

| Path | SHA-256 |
|---|---|
| `cmd/fathom/review_test.go` | `59d1b4f8e4fe97ad81ca9c623fd81d272707039604cafc58b379f7fb520b1252` |
| `cmd/fathom/tests_coverage_test.go` | `61cec388d37bbb9558566d4c26a83a0c6a3911cb5a48814fa80a09bbcc7bd689` |
| `cmd/fathom/tests_coverage_foreign_test.go` | `e183b416022d0ccc9ae96d98095103bfc55e2448e1477983a60bd3ace86fd05c` |
| `cmd/fathom/coverage_ingest_test.go` | `aae2c71b10a0f1614c3b415a6d83e61b7dba9b99d32d6e403f34beaea6eae781` |
| `internal/analysis/coverage/profile_test.go` | `a8af73df666626ebf7630dc611e09377722e6309d690c69a29c93985efc823a2` |

These are literal missing registration/input/receipt items, not a new demand
for native producers or independent oracles. T3/TQ-M3 do not prescribe a new
native-producer/oracle accuracy bar. Native producer interoperability and runtime
accuracy remain **unmeasured**, separate from the missing original inputs.
Nothing here promotes synthetic hits to execution truth or changes the original
cohort or its bar after probes exist.

## Remaining inventory: CHG-M2

**DEFERRED under T4-3**, pending the integrating helper's frozen method on
`origin/main`. Run only after that method lands. Do not dispatch
`docs/bench/readings/realignment/chg-8-cold-latency.md` under this contract.
The adjacent CHG-2's 500 ms cap remains unchanged. No replacement workload,
clock, trial count or statistic is registered here; no actionable ruling request,
placeholder reading or result is created.

## Decision models

None: deterministic fixture ingestion, range matching and counts use existing
production APIs; no judgment site or paid call is needed.

## Immutable input manifest

Paths below are repository-relative. SHA-256 is over the exact file bytes at
base; the commit pins the rest of the production implementation transitively.

| Path | SHA-256 |
|---|---|
| `docs/plans/realignment.md` | `2392810232371350c4319a06a28fc9d0a814e00a90e3b0a55d8f3fea3f0cda54` |
| `docs/plans/test-intelligence.md` | `dc841711976406fe12a40077bf573e79814eeda68b3bd4a0693a8abba16791c8` |
| `docs/design/test-intelligence.md` | `13c91bd4ce4472df2813f8da4c10a701b59981555952ed49ff02439995b9d10d` |
| `pkg/client/client.go` | `ef58f04c1e631d3690ea658aa7ccc2242dfc4e19c19870cd24ef92ca30bd6b74` |
| `internal/analysis/coverage/join_foreign.go` | `50eb5134fa326913b15bf2c8e1a3c3efce82e2ae9ba4ba9405e6e45fdcca092b` |
| `cmd/fathom/coverage_ingest.go` | `9f9415d829ce1824c786b06209be231df432e77aaa66b13a1535d004c7b745cd` |
| `cmd/fathom/tests_coverage.go` | `3efa20f074e0b91f292a226aac56db99e5451cbc778d9f507ab3a6d2901915bb` |
| `internal/analysis/coverage/coveragepy_istanbul_test.go` | `945b00c686662c024e865121b44d4518dda402dc0e72b701dc100c71de21193b` |
| `internal/analysis/coverage/cobertura_jacoco_test.go` | `9fc485e927793eb12f0491c98a9756acaffa78c48790424981dd4acdee5fbcd2` |
| `internal/analysis/coverage/lcov_test.go` | `e7edd75d601c2a614ce2f50beb257edd3d2a3a2f8d4de8768280b8911e9849bf` |
| `internal/analysis/coverage/llvmcov_test.go` | `6a88bd985308fd3c049f68aa24b234f200a67723826c0a299ecd3f8e508ea606` |
| `internal/extractor/dialects/python/testdata/shapes.py` | `a2a6243afe55d7d89bba43418061e31ef1a1f76af730721ecfa7fcc529134308` |
| `internal/extractor/dialects/python/testdata/shapes.py.golden.json` | `f15ac834ee65ec34bbba64fcbf986b46512536c03be98bc72f9363add390f90d` |
| `internal/extractor/dialects/javascript/testdata/bindings.js` | `05732e3c4dce18143c3537fb550ca7cb7ee4bbba8db59b54540528420626026e` |
| `internal/extractor/dialects/javascript/testdata/bindings.js.golden.json` | `b2d156efbe8b2d6e82dfc31373b2d524aa10428a87c536fcb7619df7dfd632a7` |
| `internal/extractor/dialects/java/testdata/shapes.java` | `07f549be82864141ac5b106e1e7035e30574a1393d576946c4538c104d934ea9` |
| `internal/extractor/dialects/java/testdata/shapes.java.golden.json` | `513e002582ae9a2d6725920783c5ac64154593fdcade3d2405b0344117a8478b` |
| `internal/extractor/dialects/ruby/testdata/calls.rb` | `6282795432999e81b7d87c60dc8cc81e084aa316c8cc88070be2367e3c2943de` |
| `internal/extractor/dialects/ruby/testdata/calls.rb.golden.json` | `67ab82888eac9f2510f421a93b85821b491325df01d062e4353e9d7293a6dc59` |
| `internal/extractor/dialects/c/testdata/shapes.c` | `589187d168b9975ee4763a439ce769c432cfd378cadf68595058dbd9e63b4656` |
| `internal/extractor/dialects/c/testdata/shapes.c.golden.json` | `e057a842141be1dee76bee7c23a35fa13a59605921a110af23c85701fa3c6056` |
