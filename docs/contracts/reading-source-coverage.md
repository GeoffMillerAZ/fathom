# FCT-10 independently labelled source-language coverage join

W4-3 draft for #279, wave1 pairing reading. Base
d9eef4794afdbc200d1b8697ce121d9d688fedaf. Later L-R owns only
docs/bench/readings/realignment/fct-10-foreign-coverage.md plus workspace scratch.
No raw independent reports supplied, no reading fabricated and no new API/checker
implementation file grant. No test, ingest, report parser or daemon ran here.

## Choices and governing text

| ID | Recommended choice / source | Alternative |
|---|---|---|
| D1 | Five distinct non-Go SOURCE languages, each independently labelled, format recorded separately. FCT-10, #279, CONTRACT-GAPS #279. | Five formats of one language. Rejected. |
| D2 | Parser CLI plus actual tests coverage --foreign join against commit-bound symbols. FCT-10/TQ-14, ADR-051 d1/2, I38/I43/I44. | Parse-only JSON/baseline unit tests as empirical proof. Rejected. |
| D3 | Every raw block disposition including module-level/range misses and ambiguous suffix paths. I38, actual JoinForeignBlocks behavior. | Unmatched as complete join coverage. Rejected: known-file out-of-range blocks omitted. |
| D4 | Pin independent Python/Java/JavaScript/Ruby/C++ reports via existing formats, preserve missing inputs. FCT-10, existing parsers. | Tiny authored unit fixtures called real independent reports. Rejected. |

TQ-M3 design says Go plus four T3 formats (test-intelligence §5); its plan says
at least5 languages. FCT-10 explicitly asks FOREIGN coverage for5 languages.
Use the stronger foreign population without rewriting the older row or claiming
TQ-M3 completed without its own reading/Go evidence. No new precision/recall bar.
CTR10..13 do not supply these populations. No future prebuilt or trigger established.

## Existing shapes, functions and consumers

internal/analysis/coverage/foreign.go ForeignBlock has File,StartLine,EndLine,
Statements,Covered,Tests. Inclusive line spans, current parser outputs line-level.
Tests normally nil, lcov TN optional, never invent per-test attribution.
ParseLcov(io.Reader), ParseCoberturaJaCoCo(io.Reader),
ParseCoveragePyIstanbul(io.Reader), ParseLLVMCov(io.Reader) each return
([]ForeignBlock,error), pure with supplied readers and no context parameter.

cmd/fathom/coverage_ingest.go parseForeignCoverage(format,io.Reader) dispatches
lcov/cobertura/jacoco/coverage-py/istanbul/llvm-cov. cmdCoverageIngest accepts
--format --commit --json REPORT, flags before REPORT, read cap256MiB. Output schema
fathom.coverage.ingest/v1 has format,file,commit,bound,files,blocks,statements,
covered,percent. --commit is operator text, not verification the producer tested
that commit. NO store write or symbol join. TestCmdCoverageIngestBindsCommit tests
formatting, not empirical binding.

JoinForeignBlocks(blocks []ForeignBlock,symbols []core.SymbolNode) ForeignJoinResult
in internal/analysis/coverage/join_foreign.go returns BySymbol []SymbolCoverage,
ByFile []ForeignFileCoverage,Unmatched []string. SymbolCoverage has
Symbol,File,Line,EndLine,Statements,Covered,Percent,Tests. ForeignFileCoverage has
File,Statements,Covered,Percent,Tests. core.SymbolNode has Service,Package,Name,
Kind,FilePath,Line,EndLine and Key().

Actual join: candidates are every symbol FilePath; exact or slash-boundary suffix,
longest path preferred. Callable kinds end Func/Method/Fn/Function. Every callable
in a file needs EndLine>0 for symbol joins, else entire file rolls up. A block must
fit wholly within [Line,EndLine]; nested ranges can receive the same block.
Known-file module-level/outside-range blocks are omitted, not Unmatched. Unmatched
is distinct unknown file paths, not omitted block counts. Equal-length suffix
conflicts require audit, not an invented deterministic winner.

cmd/fathom/tests_coverage.go loadForeignInputs parses --foreign format=path;
joinForeignInputs concatenates blocks then calls JoinForeignBlocks.
runTestsCoverage accepts --store --service --foreign --json and loads indexed
symbols. NO foreign --commit flag. foreignJSON emits inputs,by_symbol,by_file,
unmatched_files. Bind producer/index commits externally; invent no per-block field.

Reference tests, unrun: TestJoinForeignBlocksByRange,
TestJoinForeignBlocksByFile_NoRangedSymbols,
TestJoinForeignBlocksByFile_OneUnrangedSymbolDemotesWholeFile,
TestJoinForeignBlocksByRange_TestAttributionUnioned,
TestJoinForeignBlocksByRange_UnmatchedFileOmitted. These do not provide real reports.

## Frozen five-population protocol

Freeze five independent populations before join outcomes:

| Language | Native report / existing format | Independent inputs |
|---|---|---|
| Python | coverage.py JSON / coverage-py | native executed/missing lines and independent source-function ranges |
| Java | JaCoCo XML / jacoco | source/class identity, native method/line counters, independent source ranges |
| JavaScript | Istanbul JSON / istanbul | original JS source, statement/line map, source ranges, generated mapping retained |
| Ruby | lcov / lcov | producer/export adapter pin, native line labels, independent source ranges |
| C++ | llvm-cov JSON / llvm-cov | object/source/compiler pin, native region/line labels, independent source ranges |

This is a language/report protocol, not a claim the reports exist. EACH needs:
opaque repository ID+commit, source hashes, native producer/compiler/export version
and binary hash, run command/exit/clean state/times, raw report hash/size,
report-to-run attestation, independent line/block and symbol-range labels,
label timing/role, ingest/index commit/watermark/grammar/parser hashes,
path mapping and full symbol snapshot. Actual cells are MISSING today.
Private local mapping retained; publication has no private names/paths.

Labels derive from native report plus independent source/tool/person oracle before
fathom join, never copy join results into expected labels. Keep all report files
and blocks. Format does not establish language. Generated JS never counts as TS;
generated/header/shared source needs pinned mapping or stays unknown. No producer
tool run is commissioned here.

## Exact consumer invocation and grader

Later reader captures full exits/stdout/stderr separately for each population:

    fathom coverage ingest --format FORMAT --commit SHA --json REPORT
    fathom tests coverage --store SNAPSHOT_DB --service ALIAS --foreign FORMAT=REPORT --json

Manifest values, explicit service, flags before REPORT. Second command uses exact
complete index/source/parser snapshot at producer commit, no failed extraction or
partial index silently passed off as full.

Scratch checker accounts every raw record and parsed block, resolution candidates,
chosen path, independent expected symbol(s), actual symbol/file result, disposition:
symbol_match,file_rollup,unknown_file,outside_range,ambiguous_path,parser_loss,
invalid_block,unknown_input. These are local report labels, no new exported enums.
Record source+offset identity; reconcile raw/normalized/output counts, preserving
nested-symbol multiplicity rather than summing symbol totals as raw-block totals.

Check exact line/statement/covered semantics against native oracle, exact symbol/
range mappings against independent labels. File fallback never promoted to symbol.
Publish matches/mismatches, unmatched FILES and BLOCKS, outside-range residue,
unknowns and denominators per language. Coverage percent is not join correctness.
No per-test edges where Tests absent. Full trace retained.

No new percentage threshold: report exact agreement/differences. A correct
five-language join claim requires real bound inputs and independently checked
agreement for all five with actual positive symbol-range joins. File-only output
cannot establish that clause. Known wrong mapping => FAIL. Missing populations/
binding/labels/index or unresolved ambiguity => INCOMPLETE. Keep honest fallback
and unmatched residue visible. No support-level promotion, certificates or boxes.

## Exact prospective scratch-checker tests

Implementing lane writes these before real grading, critic compares each assertion.
W3-2: none observed RED/GREEN here.

| Test | Exact assertions |
|---|---|
| TestCoveragePopulationIndependence | five distinct source languages, not formats; generated JS not TS; missing native/source/run/label identity incomplete; copied join labels rejected; producer/index commit mismatch unknown. |
| TestCoverageParserAccounting | every raw block accounted; inclusive ranges/statement semantics exact; malformed/oversize/truncated/unreadable not empty PASS; optional TN retained; absent Tests not invented; --commit text not run attestation. |
| TestCoverageFullJoinLedger | exact/boundary-suffix/longest path checked; equal-length conflict ambiguous; one missing callable EndLine demotes whole file; no-callable file rollup; outside-range residue named with empty Unmatched; unknown file counts blocks+paths; nested multiplicity retained. |
| TestCoverageLiveConsumerProjection | actual pinned binary/explicit service/store, flags before path; parse-only fails empirical proof; by_symbol/by_file/unmatched_files reconcile oracle+trace; no missing projection fabricated. |
| TestCoverageFiveLanguageGrader | all five positive symbol joins plus independent agreement needed; one wrong map FAIL; missing labels/reports/binding/ambiguity incomplete; file-only not symbol; no invented precision threshold/per-test edges/TQ-M3 promotion. |

## Decision models

None. Native labels and deterministic checker, provider none, no paid calls.
