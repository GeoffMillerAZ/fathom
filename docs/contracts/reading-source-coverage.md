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

## Executable scratch handoff (critic correction)

Frozen PROPOSED interface, not implemented tooling or run evidence. W3-2 leaves
real failing tests to the implementing reader. All earlier assertions remain
binding. No product/telemetry/API grant. Synthetic test success cannot close
missing raw inputs, policies or dispatch prerequisites. READY is for document
review only. Resource-windows is final FIX round2; another unresolved finding is
HOLD. The other seven reading drafts are FIX round1 and have one round remaining.

Exact layout: workspace/tmp/reading-source-coverage/checker.py (normalize/grade),
contracts.py (schema validation/declaration guard), test_checker.py (unittest
ContractTests), manifest.json, normalized.json, result.json and the three schemas
below. Fixture directory is fixtures/<exact-test-name>/ with input.json and
expected.json for EVERY assertion case in the earlier table. Implementing reader
creates these workspace-only files. Copy each JSON code block to its named file.

contracts.py implements only the closed JSON-Schema subset below (objects/
required/additionalProperties, arrays/items, enum/const, anyOf/null, minimum,
string pattern, date-time). Standard library only, no network schema fetch or
dependency. Check RFC3339 timestamps, unique artifact IDs, SHA256 and foreign
references, non-nil arrays, nonfinite numbers rejected. Quantity decimals use
canonical base10 strings; null upper bound is unbounded, never zero.
contracts.py guard parses the three Python files with ast without importing them,
requires checker normalize/grade entrypoints and ContractTests(unittest.TestCase)
with EACH exact method selected below declared once; checks the three schema
hashes pinned by the contract commit and fixtures for every case. Declaration
presence is never a substitute for running each assertion.

checker.py normalize --manifest PATH --output PATH validates manifest, reads ONLY
listed artifacts under this scratch root (reject absolute paths, traversal and
symlink escape), verifies raw hashes/lengths and uses only the named adapters.
SourceRef positions are half-open original byte offsets, record_index zero-based.
Every normalized row retains native lineage. Preserve identical distinct rows,
stack/frame/parent order and copy-overlap identity. Never synthesize missing facts.
Unreadable/absent native data appears as coverage gaps/nulls. No polling, capture,
network, provider, ingest, Compile/Sweep or live configuration mutation.

checker.py grade --manifest PATH --normalized PATH --output PATH revalidates
schemas, hashes, lineages, then recomputes all earlier criterion assertions from
native evidence, independent labels and bound policies. Declared PASS or approval
fields cannot supply evidence. Emit canonical UTF-8 JSON, sorted object keys;
criteria sorted by row_id,criterion_id,group_key and quantities by name. Every
authored clause and group appears, including missing required groups as INCOMPLETE.
Coverage includes all expected/read/malformed/torn/skipped rows and gaps.
complete=true needs independent accounting, not artifact presence. Pending policies
remain pending_policy_ids. Null/absent facts never become zero or empty success.
Unused group_key dimensions are empty strings. All result arrays are non-nil.

dispatch_state is BLOCKED for this draft interface; foreman approval plus real
input/prerequisite handoff is separate. Grade exit0 means valid computation,
including a FAIL/KILL/INCOMPLETE result, not measurement PASS. guard/validate/
normalize/grade exit1 means assertion/schema/identity/limit/refusal error; exit2
operational inability to read/write requested paths. Where output is writable,
errors have typed grader_errors kind and SourceRefs. TestCase exits nonzero on
any missing/weakened assertion. No new numerical bar or policy from this interface.

D-HANDOFF (W3-2, I38, L-R/parallel-lanes §3): recommend this exact standard-library
scratch layout, closed schemas and all tests; alternative ad-hoc reader protocol
is rejected because it leaves the handed contract unspecified. Proposed commands
below run from the clone root AFTER the implementing lane creates these files.
None ran in the author lane.

```sh
set -eu
cd "workspace/tmp/reading-source-coverage"
test -f checker.py
test -f contracts.py
test -f test_checker.py
test -f manifest.schema.json
test -f normalized.schema.json
test -f result.schema.json
python3 contracts.py guard
python3 -m unittest test_checker.ContractTests.TestCoveragePopulationIndependence
python3 -m unittest test_checker.ContractTests.TestCoverageParserAccounting
python3 -m unittest test_checker.ContractTests.TestCoverageFullJoinLedger
python3 -m unittest test_checker.ContractTests.TestCoverageLiveConsumerProjection
python3 -m unittest test_checker.ContractTests.TestCoverageFiveLanguageGrader
python3 contracts.py validate --kind manifest --input manifest.json
python3 checker.py normalize --manifest manifest.json --output normalized.json
python3 contracts.py validate --kind normalized --input normalized.json
python3 checker.py grade --manifest manifest.json --normalized normalized.json --output result.json
python3 contracts.py validate --kind result --input result.json
```

### Native adapters

| Adapter ID | Frozen normalization contract |
|---|---|
| native_foreign_reports | Current ParseLcov/ParseCoberturaJaCoCo/ParseCoveragePyIstanbul/ParseLLVMCov plus native report lineage/source/run attestation. |
| native_symbols | Pinned core.SymbolNode fields used by JoinForeignBlocks; no inferred EndLine. |
| block_ledger | Each native record/parsed block, candidate paths/actual multiplicity and independent expected mappings; known-file outside-range residue retained. |
| native_cli | Actual coverage ingest and tests coverage --foreign JSON/flags; no foreign --commit. |

All input roles must appear in manifest.inputs. Missing roles use a named absent
Artifact with null hash/size and named coverage, never a placeholder evidence file.
Adapter IDs are dispatch keys of checker.py, not product commands. Independent
labels/derived classifications are checked against native source refs, not trusted
booleans. Prerequisite and policy authority must resolve to immutable receipts;
pending fields cannot be promoted by the reader. Unsupported native conversion
or missing capture contract keeps the corresponding consumer BLOCKED.

### manifest.schema.json

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {"schema":{"const":"fathom.reading.scratch/manifest-v1"},"protocol":{"const":"reading-source-coverage"},"source_base":{"type":"string"},"preregistered_at":{"$ref":"#/$defs/Timestamp"},"window":{"type":"object","properties":{"from":{"anyOf":[{"$ref":"#/$defs/Timestamp"},{"type":"null"}]},"to":{"anyOf":[{"$ref":"#/$defs/Timestamp"},{"type":"null"}]}},"required":["from","to"],"additionalProperties":false},"artifacts":{"type":"array","items":{"$ref":"#/$defs/Artifact"}},"inputs":{"type":"array","items":{"$ref":"#/$defs/Input"}},"policies":{"type":"array","items":{"$ref":"#/$defs/Policy"}},"prerequisites":{"type":"array","items":{"$ref":"#/$defs/Prerequisite"}}},
  "required": ["schema","protocol","source_base","preregistered_at","window","artifacts","inputs","policies","prerequisites"],
  "additionalProperties": false,
  "$defs": {"Artifact":{"type":"object","properties":{"artifact_id":{"type":"string"},"relative_path":{"type":"string"},"sha256":{"anyOf":[{"$ref":"#/$defs/Hash"},{"type":"null"}]},"size_bytes":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"state":{"$ref":"#/$defs/ArtifactState"},"error":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["artifact_id","relative_path","sha256","size_bytes","state","error"],"additionalProperties":false},"ArtifactState":{"type":"string","enum":["available","absent","unreadable"]},"Hash":{"type":"string","pattern":"^[0-9a-f]{64}$"},"Input":{"type":"object","properties":{"role":{"type":"string","enum":["source_snapshot","native_report","producer_attestation","native_line_labels","symbol_range_labels","index_snapshot","parser_identity","parse_output","join_output","path_mapping"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"adapter":{"type":"string","enum":["native_foreign_reports","native_symbols","block_ledger","native_cli"]}},"required":["role","artifact_ids","adapter"],"additionalProperties":false},"Policy":{"type":"object","properties":{"policy_id":{"type":"string"},"status":{"type":"string","enum":["approved","pending","not_applicable"]},"authority_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"parameters_artifact_id":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["policy_id","status","authority_refs","parameters_artifact_id"],"additionalProperties":false},"Prerequisite":{"type":"object","properties":{"id":{"type":"string","enum":["five_real_populations","independent_labels","commit_bound_index"]},"state":{"type":"string","enum":["proven","missing","unproven"]},"evidence_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}}},"required":["id","state","evidence_refs"],"additionalProperties":false},"SourceRef":{"type":"object","properties":{"artifact_id":{"type":"string"},"record_index":{"type":"integer","minimum":0},"byte_start":{"type":"integer","minimum":0},"byte_end":{"type":"integer","minimum":0}},"required":["artifact_id","record_index","byte_start","byte_end"],"additionalProperties":false},"Timestamp":{"type":"string","format":"date-time"}}
}
```

### normalized.schema.json

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {"schema":{"const":"fathom.reading.scratch/normalized-v1"},"protocol":{"const":"reading-source-coverage"},"manifest_sha256":{"$ref":"#/$defs/Hash"},"coverage":{"type":"array","items":{"$ref":"#/$defs/Coverage"}},"records":{"type":"object","properties":{"populations":{"type":"array","items":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"population_id":{"type":"string"},"language":{"type":"string","enum":["python","java","javascript","ruby","cpp"]},"format":{"type":"string","enum":["lcov","cobertura","jacoco","coverage-py","istanbul","llvm-cov"]},"source_commit":{"type":"string"},"producer_commit":{"type":"string"},"index_commit":{"type":"string"},"parser_sha256":{"$ref":"#/$defs/Hash"},"producer_receipt_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"label_ids":{"type":"array","items":{"type":"string"}}},"required":["source","population_id","language","format","source_commit","producer_commit","index_commit","parser_sha256","producer_receipt_refs","label_ids"],"additionalProperties":false}},"symbols":{"type":"array","items":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"population_id":{"type":"string"},"service":{"type":"string"},"package":{"type":"string"},"name":{"type":"string"},"kind":{"type":"string"},"file_path":{"type":"string"},"line":{"type":"integer","minimum":0},"end_line":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"symbol_key":{"type":"string"}},"required":["source","population_id","service","package","name","kind","file_path","line","end_line","symbol_key"],"additionalProperties":false}},"blocks":{"type":"array","items":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"population_id":{"type":"string"},"block_id":{"type":"string"},"file":{"type":"string"},"start_line":{"type":"integer","minimum":0},"end_line":{"type":"integer","minimum":0},"statements":{"type":"integer","minimum":0},"covered":{"type":"integer","minimum":0},"tests":{"type":"array","items":{"type":"string"}},"candidate_paths":{"type":"array","items":{"type":"string"}},"resolved_path":{"anyOf":[{"type":"string"},{"type":"null"}]},"expected_symbols":{"type":"array","items":{"type":"string"}},"actual_symbols":{"type":"array","items":{"type":"string"}},"actual_file_rollup":{"anyOf":[{"type":"string"},{"type":"null"}]},"label_ids":{"type":"array","items":{"type":"string"}},"disposition":{"type":"string","enum":["symbol_match","file_rollup","unknown_file","outside_range","ambiguous_path","parser_loss","invalid_block","unknown_input"]}},"required":["source","population_id","block_id","file","start_line","end_line","statements","covered","tests","candidate_paths","resolved_path","expected_symbols","actual_symbols","actual_file_rollup","label_ids","disposition"],"additionalProperties":false}},"projections":{"type":"array","items":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"population_id":{"type":"string"},"kind":{"type":"string","enum":["parse","join"]},"exit_code":{"type":"integer","minimum":0},"manifest_source_commit":{"type":"string"},"by_symbol_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"by_file_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"unmatched_file_paths":{"type":"array","items":{"type":"string"}}},"required":["source","population_id","kind","exit_code","manifest_source_commit","by_symbol_refs","by_file_refs","unmatched_file_paths"],"additionalProperties":false}},"labels":{"type":"array","items":{"$ref":"#/$defs/Label"}}},"required":["populations","symbols","blocks","projections","labels"],"additionalProperties":false}},
  "required": ["schema","protocol","manifest_sha256","coverage","records"],
  "additionalProperties": false,
  "$defs": {"Coverage":{"type":"object","properties":{"input_role":{"type":"string","enum":["source_snapshot","native_report","producer_attestation","native_line_labels","symbol_range_labels","index_snapshot","parser_identity","parse_output","join_output","path_mapping"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"expected_records":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"read_records":{"type":"integer","minimum":0},"malformed_records":{"type":"integer","minimum":0},"torn_records":{"type":"integer","minimum":0},"skipped_records":{"type":"integer","minimum":0},"missing_intervals":{"type":"array","items":{"type":"object","properties":{"from":{"$ref":"#/$defs/Timestamp"},"to":{"$ref":"#/$defs/Timestamp"},"reason":{"type":"string"}},"required":["from","to","reason"],"additionalProperties":false}},"complete":{"type":"boolean"},"reason":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["input_role","artifact_ids","expected_records","read_records","malformed_records","torn_records","skipped_records","missing_intervals","complete","reason"],"additionalProperties":false},"Hash":{"type":"string","pattern":"^[0-9a-f]{64}$"},"Label":{"type":"object","properties":{"label_id":{"type":"string"},"sources":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"labeler_role":{"type":"string","enum":["native_oracle","independent_person","architect"]},"labelled_at":{"$ref":"#/$defs/Timestamp"},"outcome_blind":{"type":"boolean"},"value":{"type":"string"}},"required":["label_id","sources","labeler_role","labelled_at","outcome_blind","value"],"additionalProperties":false},"SourceRef":{"type":"object","properties":{"artifact_id":{"type":"string"},"record_index":{"type":"integer","minimum":0},"byte_start":{"type":"integer","minimum":0},"byte_end":{"type":"integer","minimum":0}},"required":["artifact_id","record_index","byte_start","byte_end"],"additionalProperties":false},"Timestamp":{"type":"string","format":"date-time"}}
}
```

### result.schema.json

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {"schema":{"const":"fathom.reading.scratch/result-v1"},"protocol":{"const":"reading-source-coverage"},"manifest_sha256":{"$ref":"#/$defs/Hash"},"normalized_sha256":{"$ref":"#/$defs/Hash"},"state":{"$ref":"#/$defs/EvidenceState"},"dispatch_state":{"const":"BLOCKED"},"criteria":{"type":"array","items":{"$ref":"#/$defs/Criterion"}},"coverage":{"type":"array","items":{"$ref":"#/$defs/Coverage"}},"unknown_inputs":{"type":"array","items":{"$ref":"#/$defs/Unknown"}},"pending_policy_ids":{"type":"array","items":{"type":"string"}},"grader_errors":{"type":"array","items":{"type":"object","properties":{"kind":{"type":"string","enum":["schema","identity","limit","refusal","decode","coverage"]},"source_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"message":{"type":"string"}},"required":["kind","source_refs","message"],"additionalProperties":false}}},
  "required": ["schema","protocol","manifest_sha256","normalized_sha256","state","dispatch_state","criteria","coverage","unknown_inputs","pending_policy_ids","grader_errors"],
  "additionalProperties": false,
  "$defs": {"Coverage":{"type":"object","properties":{"input_role":{"type":"string","enum":["source_snapshot","native_report","producer_attestation","native_line_labels","symbol_range_labels","index_snapshot","parser_identity","parse_output","join_output","path_mapping"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"expected_records":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"read_records":{"type":"integer","minimum":0},"malformed_records":{"type":"integer","minimum":0},"torn_records":{"type":"integer","minimum":0},"skipped_records":{"type":"integer","minimum":0},"missing_intervals":{"type":"array","items":{"type":"object","properties":{"from":{"$ref":"#/$defs/Timestamp"},"to":{"$ref":"#/$defs/Timestamp"},"reason":{"type":"string"}},"required":["from","to","reason"],"additionalProperties":false}},"complete":{"type":"boolean"},"reason":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["input_role","artifact_ids","expected_records","read_records","malformed_records","torn_records","skipped_records","missing_intervals","complete","reason"],"additionalProperties":false},"Criterion":{"type":"object","properties":{"row_id":{"type":"string","enum":["FCT-10"]},"criterion_id":{"type":"string","enum":["five_language_population","native_report_binding","parser_accounting","independent_range_join","positive_symbol_joins","complete_projection"]},"group_key":{"type":"object","properties":{"service_alias":{"type":"string"},"population_id":{"type":"string"},"dialect_id":{"type":"string"},"utc_day":{"type":"string"},"channel":{"type":"string"}},"required":["service_alias","population_id","dialect_id","utc_day","channel"],"additionalProperties":false},"state":{"$ref":"#/$defs/EvidenceState"},"quantities":{"type":"array","items":{"$ref":"#/$defs/Quantity"}},"evidence_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"unknowns":{"type":"array","items":{"$ref":"#/$defs/Unknown"}}},"required":["row_id","criterion_id","group_key","state","quantities","evidence_refs","unknowns"],"additionalProperties":false},"EvidenceState":{"type":"string","enum":["PASS","FAIL","KILL","INCOMPLETE","INCONCLUSIVE","HOLD"]},"Hash":{"type":"string","pattern":"^[0-9a-f]{64}$"},"Quantity":{"type":"object","properties":{"name":{"type":"string"},"unit":{"type":"string"},"value":{"anyOf":[{"type":"string"},{"type":"null"}]},"lower_bound":{"anyOf":[{"type":"string"},{"type":"null"}]},"upper_bound":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["name","unit","value","lower_bound","upper_bound"],"additionalProperties":false},"SourceRef":{"type":"object","properties":{"artifact_id":{"type":"string"},"record_index":{"type":"integer","minimum":0},"byte_start":{"type":"integer","minimum":0},"byte_end":{"type":"integer","minimum":0}},"required":["artifact_id","record_index","byte_start","byte_end"],"additionalProperties":false},"Timestamp":{"type":"string","format":"date-time"},"Unknown":{"type":"object","properties":{"input_role":{"type":"string","enum":["source_snapshot","native_report","producer_attestation","native_line_labels","symbol_range_labels","index_snapshot","parser_identity","parse_output","join_output","path_mapping"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"reason":{"type":"string"}},"required":["input_role","artifact_ids","reason"],"additionalProperties":false}}
}
```

### Pinned scratch schema digests

SHA256 of each JSON code block plus one terminal newline. contracts.py guard
compares exact file bytes to these constants, not a newly generated policy.

- manifest.schema.json: 323e066d2eaae587b9b59a5ce261682a69bdb2481f272eb9e0c662023341b22c
- normalized.schema.json: 12102f00d37081d44a736cf9a12442fbacc15c01d60a582716f2a1adf39cd7b2
- result.schema.json: b173c0557ccba56ca058be0c9a4b8a5a9a24c0784be0fc85949352d38a26d3b3
