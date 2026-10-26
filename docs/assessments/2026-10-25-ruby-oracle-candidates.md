# Ruby oracle candidates — 2026-10-25

**Measured:** the [committed reading](../bench/readings/lang/ruby.json) stays
unchanged: L1 excellent, L2 good (.9942 precision/.9944 recall, C), L3 good
(.9767/.6528, B). Released TypeProf and native TracePoint now provide additional tier-A truth
under OL-R10; their full-scope and executed-only limits are measured below.
The existing native require instrument also exposes product misses. This is a
measured oracle/product limitation, not a claim that Ruby lacks native tools.

## Named candidates and concrete outcomes

| Candidate / route | Version / revision | Check and why it does not lift the current target |
|---|---|---|
| scip-ruby on unchanged untyped subjects | 0.5.0 e847fe92, Sorbet 0.6 e8f0eb82 | Existing executed instrument. On sinatra/jekyll it omits occurrences on untyped receivers; diagnostic call P .3869/R .5382 cannot grade the full written call population. No types or signatures may be invented to make the current subjects typed. |
| scip-ruby on typed subjects' own configs | Same indexer; ruby-lsp 0.26.11 and spoom 1.8.10 | Already measured A. P/R .5845/.8355 and .7511/.8566; blind shares 2.9%/1.5% disclosed. Constant references, `new` versus `initialize`, operators and class-body calls expose shared ScoreSCIPWith normalization gaps. L3 recall .0613/.0794 exposes transitive require binding. Changing the oracle alone does not repair these product/scorer gaps. |
| Current Sorbet checker / dump / LSP | 0.6.13594 release metadata; official CLI contract inspected | Standalone third-party checker/LSP/custom exports remain C under P-T1, as OL-R2/OL-R7; a maintained SCIP producer is the distinct A route above. The contract retains T.untyped, and a syntax/CFG dump does not supply missing receiver types in unchanged untyped code. No claim that this release was corpus-executed or has the old release's exact accuracy. A new index with materially broader native occurrences reopens measurement. |
| Ruby LSP | 0.26.11 gem/release README inspected | Editor definition/reference support is a candidate, not proof of complete native Ruby call binding. It is a third-party static analysis route; a custom export does not remove ADR-048's population/tier requirement. Its own typed codebase was already independently measured above. No unmeasured accuracy claim. |
| Prism AST / native parser | 2.0.0 release README | `bin/parse` prints syntax trees, without runtime method dispatch or load-path evaluation. Native parser syntax B can aid L1; it cannot lift L2/L3 to excellent. |
| RBS signatures / prototype rb or runtime | 4.2.0 release README | Describes class/module/method structure. Prototype prints signatures, not call occurrences. Runtime prototype is deprecated and does not enumerate written call sites or their target files; no complete L2 oracle. |
| RubyVM::InstructionSequence dump | Ruby 3.3.8, actual released-runtime fixture executed | Compile/disassembly contains the written `target` send and no evaluated receiver binding; native bytecode syntax B cannot lift L2. ISeq code_location independently supplies runtime caller ranges in the measured TracePoint route. |
| ruby/typeprof | Released 0.33.2, gem SHA256 `aa777c684d578ed27dc3e5d4bc4104c9eb5a76eedf7f5dd3366d571eaa6ca7c3`, MIT | Ruby-org owner analyzer, A by OL-R10. Actual native method/constant answers over both complete original subjects; all unresolved sites count as misses. Full measurement below, not a syntax walk filtered by whether a response exists. |
| Ruby TracePoint / caller_locations / Reflection | Ruby 3.3.8, original declared test tasks | A on executed population only, OL-R10. Full literal-site coverage 2,550/4,254 and 3,366/6,319; native-pair recall .9981 and 1.0000. Unexecuted sites never enter recall; coverage cannot establish a whole-written-population grade. |
| Solargraph | 0.61.0, gem SHA256 `7fcc98d75bf5483b09ea07c86ba695486b9859c557574f7a6d26ec3009c9f138`, MIT | Registry/released interface inspected, not corpus-executed. Independent language server with definition/reference/type inference; custom semantic export C under ADR-048, not Ruby-owner evaluation or a maintained SCIP release. |
| Steep | 2.1.0, gem SHA256 `1daf53573283b0e6d404a7273cca41fe5c15b14ece05167934f5ffc179e171d9`, MIT | Registry/released interface inspected, not corpus-executed. Independent RBS-based checker/LSP; custom export C under ADR-048. No signatures/config may be invented for original subjects; no unsupported accuracy claim. |
| Static ruby-require lookup | Existing committed recipe, measured baseline | Operator-selected load paths are B. They omit real nested gem/test roots; the stronger runtime route below already identifies the missing files. |
| Native ruby-require-trace | Ruby 3.3.8 portable runtime, committed LB-F256 recipe | Existing A trace, 145/147 sinatra and 114/119 jekyll literal in-repo statements attested (98.64%/95.80%, above 90%). Native resolution .8031, edge P .9457/R .4710; sinatra's 51 nested-gem requires and test load paths reveal product source-root gaps. Better truth is already available. |

**Source inspection:** [scip-ruby recipe and measured misses](../../calibration/oracles/scip-ruby/README.md),
[native trace recipe](../../calibration/oracles/ruby-require-trace/README.md),
[Sorbet CLI](https://sorbet.org/docs/cli-ref),
[Ruby LSP 0.26.11](https://github.com/Shopify/ruby-lsp/blob/v0.26.11/README.md),
[Prism 2.0.0](https://github.com/ruby/prism/blob/v2.0.0/README.md) and
[RBS 4.2.0](https://github.com/ruby/rbs/blob/v4.2.0/README.md).
The old native measurements above are explicitly existing evidence, not fresh
runs of newly published versions.

## Full original-population owner measurement

The [committed recipe](../../calibration/oracles/ruby-native/README.md) pins
Ruby 3.3.8, TypeProf 0.33.2, Prism 2.0.0 and native RBS 4.2.0. Both original
Git pins declare no tracked `.rbs` files or rbs_collection configuration; core
RBS supplies built-in signatures. All eligible tracked Ruby files are loaded.
TypeProf MethodCallBox resolution supplies actual inherited/alias targets and
its own Class#new initialization handling; ConstantRead supplies native constant
bindings. Caller ranges come from native MethodDefBox data. Fathom output is
never an oracle input. No inferred reference is synthesized from a syntax pair.

| Original subject / revision | Current full eligible scope | Native unresolved sites, counted as misses | Written-pair attestation audit | Resolved-native-pair diagnostic P / R |
|---|---:|---:|---:|---:|
| Sinatra 4.2.1 `599a007a779d` | 151 files / 675,263 bytes; 151 loaded, 0 rejected | 1,958/5,183 (37.78%) | 1,999/3,326 matched; 1,327 unattested (39.90%) | 1,988/3,327 (.5975) / 1,988/2,570 (.7735) |
| Jekyll 4.4.1 `1f319fb273b6` | 183 files / 709,051 bytes; 183 loaded, 0 rejected | 1,997/7,240 (27.58%) | 3,659/5,188 matched; 1,529 unattested (29.47%) | 3,653/5,138 (.7110) / 3,653/4,365 (.8369) |

The written-pair audit uses upstream C-query pairs only as a coverage denominator,
never as native truth. Written-name disagreement includes valid native aliases
or initialization targets, so its unattested count is conservative; it is not
an assertion that every such pair is a failed native binding. The separate
native unresolved-site counts remain misses in their entirety. Resolved-only
recall cannot conceal those misses. These measurements do not meet the excellent
bars and cannot relabel the existing C reading as A.

Jekyll's historical canonical reading has 165 files / 694,927 bytes and 5,137
written pairs. Current discovery retains **18 additional original extensionless
Ruby-shebang scripts**, 14,124 bytes and 51 written pairs: same Git pin, expanded
eligible set, no exclusion or new subject. The canonical reading is unchanged;
this supplementary comparison is not an equal-set grade improvement.
The [static receipt](data/ruby-oracle-2026-10-26/typeprof.json) records all
per-subject counts, manifests, tool/recipe hashes and the scoring revision.

## Native executed-call measurement

TracePoint `:call` / `:c_call` records actual method identity and native
caller_locations. Reflection/ISeq source ranges independently validate the
caller. The C query supplies literal-site scope only. Alias mismatches,
reflective same-line dispatch, duplicates, implicit callbacks and missing native
ranges stay unattested; no syntax caller is promoted into truth.

| Original subject | Attested original literal sites | Executed native-pair recall | Original declared drivers / result |
|---|---:|---:|---|
| Sinatra | 2,550/4,254 (59.94%) | 2,083/2,087 (.9981) | root `rake test`: 1,114 runs, 2 failures, 6 errors, 3 skips; contrib `rake spec`: 1,056 examples, 3 failures; protection `rake spec`: 294 examples, 0 failures |
| Jekyll | 3,366/6,319 (53.27%) | 2,937/2,937 (1.0000) | `rake test`: 957 tests, 4 failures, 0 errors, 0 skips |

All declared bundles installed without changing Gemfiles. Matching private
OpenSSL 3.4.1 headers permit the declared OpenSSL gem dependency to build inside
the workspace. The [runtime receipt](data/ruby-oracle-2026-10-26/runtime.json)
and [dependency provenance](data/ruby-oracle-2026-10-26/bundles.json) retain
sanitized package versions and lock hashes before cleanup. Completed suite
failures are disclosed; the result is neither a passing-suite claim nor a
ceiling on what a repaired environment or broader driver could execute.
The current suite failures concern template/dependency expectations and local
server behavior; no instrumentation traceback occurs in the published run.

Two instrumentation failures were reproduced and fixed before the published
run: preloading JSON before Bundler selected its declared version, and subject
stubs intercepting File.expand_path. The hook now loads no gems before Bundler
and captures Ruby's original method. `prove_hook.py` reproduces the stub and
passes; the final run has no timeout/signal and preserves both tracked digests.
The earlier crashed runs are not evidence for a ceiling. The saved final native
events were re-decoded under the current fail-closed rules and every driver log
checked for instrumentation traceback/timeout/signal by `audit_runtime.py`;
all pass with identical native truth, with code hashes recorded in the receipt.

Unexecuted sites stay in **coverage** counts, never runtime recall. Precision
is unmeasured: diagnostic Fathom predictions include unexecuted calls. Neither
90% coverage nor a union with static answers would admit whole-written-population
L2 recall/precision under P-T1/OL-R10. No runtime promotion trigger is asserted.

## Scope, independence, mutation and caps

The exact design section 13.3.1 manifest body runs before/after both full
measurements. Sinatra: 292 regular tracked files / 933,350 bytes, digest
`92352934f2ec5385f6417ac3b17ae44a69e2a50cd2ce116345c9183f0511c0ac`.
Jekyll: 803 regular files / 3,251,760 bytes plus nine symlinks, 812 entries,
digest `19fd688eef75327c4d4aab6da126fcb533b0d2158aba89b85fba2c6b8aec5110`.
Before equals after; non-Ruby tracked files reconcile the larger totals.
Symlinks are hashed, never followed; neither subject has a gitlink. Submodule
contents are explicitly uncovered. Static remeasurement has zero untracked or
ignored paths. Runtime generated 4/112 data/coverage/lock paths, inspected with
**zero eligible Ruby inputs**, then removed before static remeasurement; the
[scope audit](data/ruby-oracle-2026-10-26/generated-scope.json) holds counts only.
All full original source remains in scope. No holdout enters a canonical shard.

Independence: `prove_native.py` places real extractor output inside both the
native input directory and subject root, then changes it to fabricated calls.
Both raw native truth digests remain unchanged. A known native `length` to
`bytesize` edit changes exactly one TypeProf and one TracePoint truth pair.
The fixture also proves an inherited alias resolves to the native original,
and both unresolved native references remain counted. The [proof receipt](data/ruby-oracle-2026-10-26/proof.json)
contains digests and the actual Ruby 3.3.8 ISeq contract check.

Mutation: `python3.13 -B calibration/oracles/ruby-native/prove_mutations.py --tools workspace/tools/ruby-native --indexbench workspace/tools/indexbench --out workspace/artifacts/lang-ruby/owner-mutations`
kills five decoder mutants (syntax caller substitution, callback/reflective/alias
guard removal and row-cap bypass), plus four owner-export mutants (A to C,
fabricated target, dropped unresolved count and truncated original file scope).
Every focused assertion reaches `FAIL:`; source is restored in `finally` and
reruns GREEN. The pure decoder/driver suite passes 19 tests; [mutation summaries](data/ruby-oracle-2026-10-26/mutations.json) retain the nine FAIL assertions and restored results.

Caps: native proofs exercise TypeProf file 2/1, total-source bytes N/(N-1),
answers 4/3; syntax file 2/1, bytes N/(N-1), sites 8/7; and hook unique rows
and native ranges 2/1. All refuse the whole unit. Pure cap+1 regressions cover
raw file count 2/1, total/per-file bytes N/(N-1), rows 2/1 and ranges 2/1,
plus truncated JSON, native refusal and wrong runtime. Hard limits: 2,048
source files, 64 MiB source, 1,000,000 static answers/sites; 64 raw files,
256 MiB total, 128 MiB per file, 1,000,000 rows and 100,000 native ranges per
file. Timeout/signal refuses the runtime unit; no partial oracle is published.

Runnable commands and prerequisites are in the committed recipe. Raw native
answers, source/file lists and subject identity metadata remain private. Public
evidence contains counts/hashes and sanitized tool/dependency versions only.

## Search sources (X-R4)

Checked 2026-10-26; bounded searches, including negative results:

| Category | Source actually searched | Result |
|---|---|---|
| Owner toolchain | Ruby 3.3.8 executed ISeq/TracePoint/Reflection, [TypeProf v0.33.2](https://github.com/ruby/typeprof/tree/v0.33.2), [native TracePoint contract](https://ruby-doc.org/3.3/TracePoint.html) | Native static and runtime routes measured above; compiler dump alone has no evaluated target. |
| SCIP / LSIF | [Sourcegraph full supported table](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [writing-indexer table](https://sourcegraph.com/docs/code-navigation/precise-code-navigation/writing_an_indexer), [LSIF.dev full table](https://lsif.dev/), GitHub repository query `scip-ruby` | Query returned three hits: sourcegraph/scip-ruby plus two unrelated spelling variants. Checked lists provide no additional Ruby producer; this is not a universal absence claim. Existing maintained producer measured in LB-F257. |
| Language servers | Released TypeProf definition/reference handlers; [Ruby LSP v0.26.11](https://github.com/Shopify/ruby-lsp/tree/v0.26.11), [Solargraph](https://rubygems.org/api/v1/gems/solargraph.json), [Steep](https://rubygems.org/api/v1/gems/steep.json), Sorbet official CLI/LSP contract | TypeProf is owner A by OL-R10, measured; third-party custom semantic exports remain C, while maintained scip-ruby is a distinct A route. |
| Package registry | RubyGems version/metadata endpoints for typeprof, prism, native-platform rbs, solargraph and steep | Released versions, licence declarations, native-platform artifact hashes and dependency contracts checked. Parser/signature prototypes do not enumerate resolved full-source calls; no accuracy inferred for unexecuted analyzers. |
| Runtime call / require tracing | Native TracePoint actual test tasks; existing committed LB-F256 require trace | Call trace measured with executed-only scoring; require truth already exposes shared resolver gaps. Neither route is dismissed for requiring an adapter. |

## Disposition / retry

Keep the canonical grades unchanged. Excellent whole-population L2 needs either
materially broader independently enumerated owner/maintained-indexer reference
truth on both complete original populations, with unresolved references counted
as misses, or a shared scorer/product repair proven against the existing A
truth. The latter requires controller serialization and regrading every affected
item. L3 already has A require truth and waits for the recorded source-root and
transitive-binding repair, not another oracle search.
No engine/epoch change, generated board edit, canonical reading, corpus addition
or plan checkbox change is made here. The binding TypeProf/TracePoint omissions,
ISeq version mismatch, named Solargraph/Steep candidates and index/spacing issues
are addressed; the previous Sorbet C correction is retained.
