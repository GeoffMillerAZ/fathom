# Per-language surface acceptance: independent truth and answer extent

Status: DRAFT for #183. Contract only, based on source at
`8fd89510675fe8416338d348a81883680cac9e98`. Separate critic LAND and wave approval
are required before implementation dispatch. This document supplies neither an
executable adapter nor oracle evidence, a reading, fixtures, a plan or a completed
#183. Wave brief V2 item 7 assigns this shared contract to the wave, not the
language track. Subsequent source grants are serial and explicit.

## Intent before design

The [intent router](../intent/README.md) routes this additive measurement design
to G9/G15 in [goals](../intent/scope/goals-1.md), P2/P3/P14/P17 in
[principles](../intent/principles.md), and I1/I38/I54/I62 in
[invariants](../intent/invariants.md). Determinism, independent evidence, visible
blindness and computed claims govern. Enforcement remains at each invariant's
stated rung. A contract is not new enforcement. ADR-027 prohibits healthier
reports after input loss. ADR-032 and [bench-pyramid §3.1](../design/bench-pyramid.md#31-l0--the-index-oracle-bench)
put index correctness in free L0. ADR-048's support levels remain separate from
this surface measurement. ADR-053 names the three existing default-surface tools
measured here. Records: [ADR.md](../../ADR.md), identified by ADR number.

The [non-goals](../intent/scope/non-goals.md) prohibit an IDE language service,
new tool surface and workflow engine. [Bench preparation §4–5](../design/bench-prep.md)
requires reuse, truth-derived strata and C4/C6 independence. No new runner,
store schema, general oracle registry, evaluator engine, provider or model is
introduced. The MCP compatibility surface in [scope](../intent/scope.md) is
observed, not changed. F22 in [futures](../intent/futures/f11-f22.md) keeps a
cross-language port ledger deferred. No translation or cross-language identity
mapping is needed. Existing per-subject identities preserve that boundary for
free. No newly fired revisit trigger is established by source inspection alone:
in particular, there is no new L0 reading below 0.95 or three measured deferrals.

## Decisions and existing foundations

| ID | Decision and governing source |
|---|---|
| D1 | Reuse `cmd/indexbench` and `internal/indexbench` L0, not a second harness. ADR-032; bench-prep §4. The sampler and surface grader belong to the shared indexbench foundation, not a dialect owner. |
| D2 | Freeze independent truth before ingest. Never discover questions, decide coverage, map truth endpoints or repair extents from the tested index. `publicloc/gotruth.go` independence contract; bench-prep §5 C6 and §6; ADR-027/033. |
| D3 | Grade the registered `tools/call` result, including dispatch budgeting. Existing `indexbench.Dispatcher`/`ReplayGolden` and `mcp.Server.HandleRequest` are the seam. Backend-only equality cannot prove surface acceptance. Bench-pyramid E1 and §3.1. |
| D4 | Keep identity, full-definition extent, displayed window, call relation and coverage separate. Existing `core.SymbolNode`, `OracleDef`, SCIP `Occurrence` and L0 unadjudicated accounting supply the foundations, not interchangeable truths. ADR-027/048; extent rules below. |
| D5 | Retain every intended language/stratum and every selected query, including missing truth and capped answers. I38 and L0 `ToolScore.Population`/`UnadjudicatedQueries` require visible denominators. No blanket oracle-blind exemption is imported from language grading. |
| D6 | Freeze method and report numbers now, never infer a new acceptance bar. ADR-032's exact governed readings and bench-prep C4 retain their own 0.95 precision/recall scope. They do not give all non-Go definitions, extents or MCP strata that bar. Numeric #183 acceptance is Q1 below. |

Source anchors, with their present limits:

- [oracle.go](../../internal/indexbench/oracle.go): `IndexOracleReport`, `Subject`,
  `ToolScore`, deterministic stride sampling, bounded truth closure and misses.
  [gotruth.go](../../internal/contextbench/publicloc/gotruth.go): `GoTruth`,
  `LoadGoTruth`, `LoadGoTruthTests`. Existing by-name maps merge packages and lack
  definition extents. They cannot by themselves grade overloads or exact locations.
- [compare.go](../../cmd/indexbench/compare.go): `cmdCompare` uses a scratch client,
  Go-module truth and `sampleDeclaredNames`. Its fathom arm calls L0 backends,
  while `ProbeReferencesViaMCP` probes competitor reference tools. It does not
  already implement this three-tool, non-Go surface reading.
- [sciporacle.go](../../cmd/indexbench/sciporacle.go): `cmdScipOracle` calls
  `ImportIndex(raw, nodes)` with tested nodes. [scip_oracle.go](../../internal/indexbench/scip_oracle.go)
  scores mapped edge pairs. Useful importer diagnostics, not an independent
  definition/extent/caller graph for this contract.
- [langbench/scip.go](../../internal/langbench/scip.go): `SCIPDefs` reads raw
  definitions; `ScoreSCIPWith` and its `EnclosingFallback`/mapped-node paths are
  measurements, not independent truth inputs. [ctags.go](../../internal/langbench/ctags.go)
  provides `OracleDef`/`ParseCtagsJSON`; [score.go](../../internal/langbench/score.go)
  uses relaxed name/line pairing and end-line tolerance. Preserve those language
  readings. Neither tolerance nor mapped fallback establishes exact #183 extent.
- [replay.go](../../internal/indexbench/replay.go): `Dispatcher`/`ReplayGolden`
  already use JSON-RPC `tools/call`. Reuse the dispatcher and decoding seam,
  L0 counts and existing report conventions. L1 payload goldens remain L1 proof.

## Independent truth handoff

Present population: Python, Java, C#, C, Rust, PHP, Dart, Go, TypeScript and
JavaScript, as named by #183. This roster is not a claim that ten independent
truth graphs are available. Existing Go/Python/TypeScript L0/SCIP foundations are
starting points, not proof of parity with each other or with SCIPlabel.

The truth producer hands the sampler one immutable, subject-bound input, before
any client ingest or store inspection. It contains these required facts:

| Component | Required content |
|---|---|
| Provenance | Public source URL, exact subject commit, tool source/release and immutable revision or binary digest, actual version output, licence at the pin, arguments, build/configuration and dependency-lock digests, producer/adaptation revision, raw-output SHA-256 and source-manifest SHA-256. No floating tool name suffices. Local artifacts stay under `workspace/`. |
| Source set | Independently enumerated root-relative files with byte hashes, language, product/test/generated classification and exclusions with reasons. Record build-selected files separately from all tracked candidates. Missing, unreadable, inactive, external and failed units remain counted. |
| Definitions | Producer-native symbol ID scoped to subject and, for locals, document. Name, owner/qualified spelling, kind, package/unit, declaration site, independent extents and per-field availability. Distinct overloads/locations stay distinct. |
| Relations | Independently resolved caller→callee with occurrence site, relation kind and resolution status. Duplicate sites and duplicate pairs are different counts. Imports, type/value references and possible dispatch targets are not automatically direct calls. |
| Coverage | Per file/unit and fact family: complete, partial or unknown, with cause and counts. Closed negative coverage must say which definitions, source regions, target domains and call kinds were exhaustively checked. Successful process exit alone proves no negative coverage. |

These are one bounded handoff in `internal/indexbench`, not new store/core/public
API types. **A small additive truth/extent and per-query report shape is necessary**
because `GoTruth`/`EdgeSet`/`OracleDef` cannot retain producer identities, independent
full extents, coverage and capped/unknown query dispositions together. Reuse their
sufficient projections and `OracleRun` provenance vocabulary; do not mutate old
reading schemas or encode new IDs as `file:line` or sentinel-joined strings.
Any prospective exported additions use named closed enums, explicit JSON names,
structured subject/definition/site IDs, initialized `[]`, typed errors, copied
inputs and pure calls without context unless I/O is involved. This is no door-API
or schema grant.

Go uses the existing go/types loader/walk and native AST positions, extended at
that existing source to preserve definition identity/site/extent before its
by-name projection. Non-Go truth consumes pinned raw SCIP plus independent
compiler/native syntax evidence when SCIP does not supply caller ownership,
full definitions or call-site semantics. Reuse raw `SCIPDefs` decoding/kind
normalization where its assumptions are proven for the pin. Do not obtain truth
from `ImportIndex(..., testedNodes)`, `ScoreSCIPWith(..., testedNodes)`, extracted
`Calls`, dialect captures, stored edges or the product's name resolver.

A callable reference in SCIP is not necessarily a written call. The adapter
must show the producer's call-role guarantee or independently identify a call
at that source site. Attribute it to the narrowest independently proven callable
body containing the site. Closure calls follow the producer's documented owner
rule, matching the existing Go enclosing-function rule where applicable.
Executing top level may use an independently evidenced module/initializer owner
(ADR-048 K18), never the nearest unrelated method. A pure namespace is not a
callable. Dynamic dispatch, unresolved targets, missing callers and missing body
ranges remain named unknown relations. Ctags may independently grade definition
fields it actually provides, not resolved calls or absent overload identities.
A parser using the tested dialect's own query is not independent definition truth.

Existing [language readings](../bench/readings/lang/) identify these producer
leads. These are recorded versions at the inspected source commit, not new
availability, independent graph or full-extent attestations:

| Reading ID | Recorded semantic producer/version | Other recorded definition evidence |
|---|---|---|
| python | scip-python 0.6.6 | universal-ctags 6.2.1 |
| java | scip-java 0.12.3 | javaparser 3.28.2 |
| csharp | scip-dotnet 0.1.0-SNAPSHOT | roslyn-semantic/syntax, Microsoft.CodeAnalysis.CSharp 5.9.0 |
| c | scip-clang 0.4.0 | clang-preprocessor (exact version in reading) |
| rust | rust-analyzer 0.0.0, revision 03fcb77246 | no separate definition producer recorded |
| php | scip-php 0.0.1 | php-parser 5.7.0.0 |
| dart | scip-dart 1.7.0 | no separate definition producer recorded |
| go | scip-go 0.2.7 | universal-ctags 6.2.1; this contract reuses native go/types L0 |
| typescript | scip-typescript 0.4.0 | no separate definition producer recorded |
| javascript | scip-typescript 0.4.0 | no separate definition producer recorded |

No per-language adapter, runnable tool or raw corpus is declared available here.
A version string, especially SNAPSHOT or a shortened revision, is not the
immutable producer pin required above. The implementing handoff must supply
actual source URLs, full revisions/digests and source/output bindings for each.
No new producer research or native collection is performed in this draft.
Unavailable evidence leaves the applicable language/stratum ungraded. It does
not prove a ceiling or justify silently selecting a different population.

## Identity, coordinates and extent

The immutable source manifest defines the namespace. Paths are slash-separated,
root-relative and case-preserving. Remove a leading `./` and clean internal dot
segments. Reject absolute/traversing/out-of-root paths. Do not case-fold paths,
resolve a symlink into a second identity or merge equal byte hashes at different
paths. Record a symlink's target bytes without following it. Excluded symlink
contents/submodules/generated outputs remain uncovered with counts. Generated
tracked source can be graded only if both populations and the independent tool
explicitly cover those exact bytes. Macro expansions without written source are
unknown for source extent. Unreadable bytes never get an empty hash or range.

Use a structured truth identity: subject pin, document, producer-native symbol
ID and declaration occurrence. Same native ID at multiple declaration sites is
a group of sites, not last-write-wins. Locals require document scope. Equivalent
aliases/re-exports/partial declarations may join only with producer evidence
recorded before ingest. Constructors/accessors/operators use the producer-specific
normalizations already documented by `SCIPDefs`, preserving the original ID.
Never join on bare name alone. Distinct overloads remain separate even if the
product's existing `core.SymbolKey(service, package, name)` collides.

Answer keys are decoded using [core/keys.go](../../internal/core/keys.go), including
`QualifiedPackage`. Build any expected display-key projection from the independent
name/package policy and pinned source, never a lookup in the tested store. Match
returned definitions by path, kind, qualified identity and declaration location.
If a returned key cannot distinguish multiple truth targets, record an
unaddressable/colliding surface identity. Do not collapse truth to award credit.
A genuinely ambiguous bare-name answer may be correct abstention, but a colliding
qualified answer is not proof of overload coverage. Unknown adapter naming policy
is truth uncertainty, not permission to derive the expectation from fathom.

Canonical positions are zero-based line and UTF-8 byte column, half-open
`[start,end)`, over original source bytes. Preserve producer positions and their
encoding alongside normalized positions. SCIP's three/four-element range arrays
and per-document `PositionEncoding` are defined by
[scip.proto](../../third_party/scip/scip.proto). Convert UTF-16/UTF-32 by decoding
the pinned line, reject split code points/surrogate pairs and out-of-bounds spans.
Unspecified encoding requires a pinned producer guarantee, otherwise columns
are unknown. Split lines on LF, preserve CR in CRLF bytes and do not normalize
Unicode, BOMs or line endings before hashing/conversion. An exclusive end at
column zero belongs to the preceding line for inclusive-end projection.

Keep three independent extent fields, each available or unknown with reason:

1. **Name/declaration site:** the identifier occurrence, not a function body.
2. **Full definition:** the complete defining syntax node, including attached
   documentation/decorators only when the producer includes them. Preserve that
   producer policy. A forward declaration is its own declaration extent, not an
   implementation body. Group associated declaration/implementation sites only
   with native evidence, retaining both ranges.
3. **Body:** the callable body's independent range, absent for declarations with
   no body. A SCIP `enclosing_range` is a nearest nontrivial node and is usable
   as a full definition/body only when its pinned producer semantics establish
   that meaning. Never expand a name token using the tested node's `EndLine`.

The MCP DTO exposes one-based `line` and inclusive `end_line`, without columns.
Grade `line` against the independently identified definition-start line and
`end_line` against the full-definition inclusive end, only where the producer
policies match. No ±1 tolerance is inferred from `langbench.ScoreL1`. Report
name-site match and full-start/end agreement separately. A known full extent
with returned end zero is missing extent, not a correct token-only extent.
A mismatched documentation/decorator policy remains a disclosed incomparable
extent until the independent adapter can give the matching projection. Never
mark incomparable as correct or use containment alone as equality. Byte-column
accuracy is a truth/conversion assertion, not an unexposed MCP capability.

`symbol_context.snippet` is a displayed source window, not any of those extents.
[serveDefinitionSnippet](../../internal/mcp/snippet_fresh.go) reads from `def.Line`
with a 40-line cap, not through `def.EndLine`. Compare every displayed line number
and text with the pinned bytes and check clipping/withholding metadata. Text
beyond the function can be a correct window. It proves no full-body coverage.
Source drift invalidates snippet comparison and is disclosed independently of
index/definition accuracy.

## Deterministic sampler and denominator

The shared indexbench owner consumes only the frozen truth handoff and a
preregistered request: subject pins, truth/adaptation/source digests, language
roster, allowed kinds/scope, per-stratum budgets, query variants and blast bounds.
Require nonnegative sample budgets and explicit valid blast bounds. Missing
request fields refuse acceptance rather than supply a hidden population or floor.
Reuse `sampleDeclaredNames`/L0's sorted stride rule, generalized to structured
truth identities: sort by subject, path, start position, native ID and declaration
site. For population N and requested k, k=0 means all, otherwise select indices
`floor(i*N/k)` for `0 <= i < min(k,N)` when k<N, or all when k>=N. Seed is recorded
as zero: this version uses no RNG and refuses a nonzero seed. No wall clock,
filesystem iteration or tested index affects selection.

| Stratum | Independent population and generated queries |
|---|---|
| Definitions | All in-scope source definitions in the preregistered kind set, keeping overloaded/duplicate sites. Ask exact name and, when independently expressible, qualified `find_symbol`; qualified `symbol_context` for callable definitions. Bare ambiguity is a separate query variant. |
| High fan-in | C6's symbols with ≥5 distinct product callers across ≥3 external caller units. Apply bench-prep D24: import package, directory for Python/TypeScript, Go external test package merged. Apply D26 exclusions for tests/examples/benchmarks/samples/docs/tools and the defining unit. Measure fan-in before sampling. These are stratum membership rules, not #183 acceptance floors. |
| Cross-unit calls | Distinct resolved product caller/callee pairs whose D24 units differ. Sort by callee then caller identity/site, stride sample, ask target context and `blast_radius` on the callee's file. Cross-unit never implies cross-service. |
| Negatives and boundaries | Truth-proven zero-caller definitions; independently absent names only within closed name coverage; ambiguity/collisions; missing extents, incomplete units and cap-boundary cases where present. No negative is inferred from an omitted occurrence. |

C6 filters select queries only. They do not discard test or same-unit answers
from a selected query. Grade all covered in-scope answers, labeling product/test
projections separately. Truth lacking test coverage makes test-answer components
unknown, not false positives. The current related-test classifier is product
behavior, not independent truth about whether a test exercises a symbol.

Record full population (or null with reason), selected, attempted, graded,
unknown, correct-abstention, cap/partial, failed and exact-complete counts per
language, subject, stratum, tool and query variant. Unavailable languages still
have rows. File/unit unknown counts remain even when their definition count is
unknowable. No invented zero population. Publish every exclusion's reason/count.
Overlapping strata retain memberships but identical tool/argument queries execute
once and aggregate once. Publish both membership denominators and unique queries.
Never replace a miss/unknown with the next successful sample. Freeze request and
selected IDs before ingest. Repeated runs preserve question IDs and denominator.

## Surface execution and exact grading

Reuse a scratch local client/store as in `cmdCompare`, then register the real
`mcp.Server` and invoke `tools/list` and `tools/call` via `HandleRequest` through
`Dispatcher`. Verify these three tools are actually registered. No live daemon,
shared fleet store, fake answer backend or internal graph function substitutes
for the surface reading. Truth remains outside ingest. If a reading imports SCIP
as product input, label that configuration and retain independent raw decoding:
it measures the import/surface path conditional on that producer, not independent
agreement of two semantic tools. Never seed expected answers into the store.

Keep raw response bytes, arguments, tool/binary commit, source/truth digests,
profile, graph mode, budgets/deadline and cold/warm condition locally. Decode
JSON-RPC errors and MCP content/structured payloads, requiring consistency when
both copies exist. Invalid JSON, missing/wrong-type fields or contradictory
copies fail response-shape assertions, never become an empty success. The scorer
must iterate the selected query list, including completely empty responses;
`CompareEdges` groups only supplied edges and is insufficient for this alone.

| Tool | Exact answer scope to grade, from current source |
|---|---|
| `find_symbol` | `tools.go`/`findsymbol.go` and query `symbolMatches`: exact/suffix-qualified, then case-insensitive, then substring. `matched_via` must identify the tier actually served. Suggestions are not definitions. Fallbacks cap at 20, suggestions at 5, with name/file/line/kind tie order (substring first orders shortest name). Exact hits have no 20-row handler cap but still pass dispatch budgeting. Grade returned sites and the independently expected candidates for that query tier, not only whether one desired name appeared. |
| `symbol_context` | `nav_tools.go`/`cold_graph.go`: unique definition plus direct callers/callees from the served combined graph, not transitive closure. Compare these relations to independently resolved direct-call truth where covered; separate type references. Ambiguity lists at most 10 candidates. More than 50 callers triggers `scope_required` on the current handler's graph count. Verify flags/count/candidates and absence of a fabricated neighborhood. Truth fan-in and observed cap behavior are separate, since a wrong graph could trigger a correct-looking refusal. Callers/callees each have half the 16000-byte tool budget, references each a quarter, plus the dispatch backstop. Snippet is optional/default on, capped at 40 lines. |
| `blast_radius` | `kind:change`, one callee file as `changed_files`. Independently seed every covered in-scope symbol in that file, reverse-walk calls to the requested depth with cycle/diamond dedup. Explicitly record `include_heuristic`, `max_depth`, `max_symbols`. Defaults are depth 2 and 500 symbols. Selection is shallowest then display key, output symbols sorted by key. Grade symbols, files, depths and total/truncation/depth-cap metadata against that same bounded scope. The current default uses resolved edges when available, otherwise syntax with `edge_filter:all`; never compare a depth-2 listing with unbounded truth. Rename/delete, service-ref hops, IaC facts and unrelated test-coverage claims are outside this initial contract. Their omission from scoring is explicit. |

Normative source: [tools.go](../../internal/mcp/tools.go),
[findsymbol.go](../../internal/mcp/findsymbol.go),
[nav_tools.go](../../internal/mcp/nav_tools.go),
[cold_graph.go](../../internal/mcp/cold_graph.go),
[extra_tools.go](../../internal/mcp/extra_tools.go),
[resolved_nav.go](../../internal/mcp/resolved_nav.go),
[impact/bounded.go](../../internal/analysis/impact/bounded.go),
[budgeted_handler.go](../../internal/mcp/budgeted_handler.go).
These are observed policies, not permission to make a truth graph from them.
Adapter tests must expose policy drift rather than update expectations silently.

For each independently adjudicable component let T be its bounded truth set and
P its returned set after identity normalization. TP=|P∩T|, FN=|T−P|, FP=|P−T|
only in closed negative coverage. An item in an uncovered region is unknown,
neither TP nor FP. Partial positive truth can prove a TP/FN, but cannot certify
complete answer recall or negative absence. Publish TP/FP/FN/unknown counts,
micro precision TP/(TP+FP), recall TP/(TP+FN), query exact-set agreement,
identity/kind/start/end agreement and known-extent coverage with numerators and
denominators. Zero denominators render null/undefined, never a PASS. Publish
per-subject numbers before pooling; do not merge truth tiers/modes silently.

Keep semantic outcome and cause separate:

- **Index miss:** independent expected definition/file/edge absent from the
  scratch index, diagnosed after the scored call by source/symbol/edge reads.
  The diagnostic does not create or remove expected answers. It stays a recall
  miss when truth is known, even if the surface correctly says `not_indexed`.
- **Surface miss/wrong answer:** indexed evidence exists but response omits it,
  chooses another site/overload, emits a false edge or wrong extent/metadata.
  Truth remains the judge even when product graph and handler agree.
- **Surface cap/partial:** truthful `elided`/`elision`, `truncated`, `depth_capped`,
  `scope_required` or cold `partial` explains extent. Grade visible members for
  accuracy, cap policy/ordering/disclosure separately and retain omitted known
  answers in full-answer FN. Also report recall against the permitted displayed
  subset when independently computable. A disclosed cap is not an uncapped
  success. Silent omission or false total is a wrong answer. Respect the generic
  dispatch backstop too, not just the three handlers' local caps.
- **Correct abstention:** independently justified ambiguity, unsupported fact,
  unknown source or withheld snippet is labeled honestly without a fabricated
  claim. It earns abstention correctness only, never definition/edge credit.
  A declared negative is correct only in complete truth coverage. A well-formed
  refusal caused by a product miss still retains that miss.
- **Truth unknown / run failure:** missing pins, unresolved/native-blind calls,
  unavailable extents or incomplete documents stay ungraded with cause and
  denominator. Admission/timeouts/RPC failures retain attempted query rows and
  reduce response availability. They are not unknown truth or evidence of a
  correct empty answer. Cold responses remain scored as cold, with any separately
  preregistered warm attempt retained rather than substituted.

For blast caps, incomplete truth beyond the boundary also makes total/depth-cap
comparison unknown. Display-key collisions make order-dependent cutoff credit
unavailable. A complete independent callable graph does not establish complete
truth for non-callable blast seeds or type-reference reachability. Grade those
components only with their own coverage; otherwise retain them as unknown.

## Serial ownership, prerequisites and required tests

All filenames below are **proposed dispatch grants**, not edits authorized by
this draft. The foreman checks current source and transfers exact paths at each
landed parent commit. No parallel writer owns a shared indexbench file.

| Turn | Owner, prerequisite and bounded implementation |
|---|---|
| S1 truth foundation | Shared indexbench owner, after this contract's critic LAND/approval: new `internal/indexbench/surface_truth.go` and `surface_truth_test.go`. Pure identity/coordinate/coverage handoff plus raw adapter boundary. Use existing raw SCIP/Go foundations. If access requires changing an existing shared helper, request its exact path before editing. No dialect/reading/schema work. |
| S2 first adapters | Same foundation lane after S1 lands, exclusive `internal/indexbench/surface_truth_go.go`, `surface_truth_scip.go` and paired `_test.go` files, plus only explicitly transferred existing helper paths. Prove the Go bridge and first available non-Go raw producer path with independent extents/call ownership, not a promise of ten adapters. Missing producer capabilities stay unknown. |
| S3 sampler and grader | Shared indexbench owner after S2's handoff: `internal/indexbench/surface_acceptance.go` and `surface_acceptance_test.go`, then exclusive `cmd/indexbench/surface_acceptance.go`, `surface_acceptance_test.go` and a separately granted registration edit in `main.go`. Reuse Dispatcher/L0 accounting, one indexbench command path, no new harness package. Sampler owns selection and grading; language authors supply provenance/source facts only. |
| S4 readings | Reading lane after S3 lands, pins/artifacts exist, Q1 is resolved in a preregistration and the foreman assigns exact output paths. First review independent truth coverage, freeze the sample, then execute registered handlers on a scratch store. Publish honest below-bar/unknown results. At least five readings and a plan are parent-issue work, not produced or closed here. |

Minimum implementing-lane failing tests, run before implementation and then with
`-race` in the exact owning package. These test names are prospective assertions,
not claims that tests exist or ran:

| Test, file under `internal/indexbench/` unless qualified | Required assertions |
|---|---|
| `TestSurfaceTruthIndependent`, `surface_truth_test.go` | Mutating/deleting every tested symbol/edge leaves raw and normalized truth/sample IDs unchanged. Changing a known source fact changes truth. Reject absent pin/hash/coverage, never read a tested store in the truth builder. |
| `TestSurfaceTruthCoordinatesAndIdentity`, same file | UTF-8/16/32 with multibyte/surrogate input, CRLF and end-column zero project exactly. Invalid/unknown encoding remains unknown/error. Overloads, local IDs in two documents, partial declarations, symlinks, generated and unreadable files neither merge nor disappear. |
| `TestSurfaceTruthGoBridge`, `surface_truth_go_test.go` | Two packages with the same function name, method, closure, initializer and test-inclusive scope retain independent IDs/sites/full extents while old by-name projections remain unchanged. |
| `TestSurfaceTruthSCIPCoverage`, `surface_truth_scip_test.go` | Raw definition absent from tested nodes remains truth. Name-only/enclosing-expression ranges never become body truth. Reference without proven call role, missing owner, unresolved target, namespace/module and producer-generated site each get the specified disposition/count. No imported mapped-node fallback supplies truth. |
| `TestSurfaceSampler`, `surface_acceptance_test.go` | Permuted inputs yield byte-identical IDs/selection. Stride boundaries, seed refusal, C6 external-unit/test rules and overlap dedup are exact. Missing language/stratum and empty responses retain denominator. No outcome-dependent replacement. |
| `TestSurfaceAnswerGrading`, same file | Known index miss differs from present-index surface miss. Wrong site/extent/edge is wrong. Positive-only truth never labels uncovered predictions FP or absent calls true negatives. Zero denominator is null. Unknowns, overload collision and correct abstention cannot inflate exact-complete counts. |
| `TestSurfaceRegisteredHandlers`, same file | Real scratch ingest plus registered tools/call, not graph-only/fake backend. Test exact/fallback/ambiguous/colliding names, 20/21 fallback candidates, 10/11 ambiguity candidates, 50/51 callers, 40/41 snippet lines, per-field and dispatch-byte cap boundaries, depth/cycle/diamond/500/501 blast membership, truth zero-call negatives, not-indexed, cold partial, malformed/error and disagreeing payload copies. Assert wrong disclosure/total/order is detected. Test source drift independently of graph correctness. Synthetic inputs prove the grader only, never public language results. |
| `TestSurfaceAcceptanceCommand`, `cmd/indexbench/surface_acceptance_test.go` | Command freezes truth/request/sample before ingest, requires real registered tools, contains all artifacts in workspace, rejects incomplete preregistration for acceptance and reports numbers without PASS when no bar exists. Truth/handler failure retains query records and returns a named operational error, never a zero-success reading. |

An unavailable native tool does not block hermetic grader tests or create oracle
evidence. Collection remains an explicitly assigned reading action. No automatic
installation/download, SDK/native execution or paid call belongs to this contract.

## Numeric authority and handback boundary

Apply ADR-032's 0.95 only where its exact surface and population govern, including
bench-prep C4's named `find_callers` and resolved `blast_radius` comparison on its
admitted subjects. Do not transfer a backend, Go, depth-1 or caller bar to all
non-Go `symbol_context`, definition, extent or fallback strata. ADR-048 language
thresholds still compute language levels only. This draft defines and reports
#183 metrics without PASS until preregistered bars and sample floors exist.

**Q1 (unguided, architect):** No #183 numeric floor or surface-wide bar is governed.
Recommend preregister per-stratum bars/floors before readings, with no PASS before
then. Alternative: approve explicit #183-wide numbers now. Existing scoped
ADR-032 bars remain unchanged.

Send Q1 through the wave foreman; continue guided contract work. A truthful
source-check commit proves this draft's anchors and scope, not implementation or
acceptance. The next exact implementing lane is S1, contingent on separate critic
LAND and foreman approval. No plan box or language grade changes here.

## Decision models

None. Truth, sampling, identity/extent projection and grading are deterministic.
A model never creates an oracle fact, adjudicates a result or supplies a bar.
