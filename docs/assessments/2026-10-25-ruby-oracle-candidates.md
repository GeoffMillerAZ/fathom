# Ruby oracle candidates — 2026-10-25

**Measured:** the [committed reading](../bench/readings/lang/ruby.json) stays
unchanged: L1 excellent, L2 good (.9942 precision/.9944 recall, C), L3 good
(.9767/.6528, B). A stronger native require instrument already exists and exposes
product misses. This recheck retains the shared scoring/resolver trigger, not a
claim that Ruby lacks native tools.

## Named candidates and concrete outcomes

| Candidate / route | Version / revision | Check and why it does not lift the current target |
|---|---|---|
| scip-ruby on unchanged untyped subjects | 0.5.0 e847fe92, Sorbet 0.6 e8f0eb82 | Existing executed instrument. On sinatra/jekyll it omits occurrences on untyped receivers; diagnostic call P .3869/R .5382 cannot grade the full written call population. No types or signatures may be invented to make the current subjects typed. |
| scip-ruby on typed subjects' own configs | Same indexer; ruby-lsp 0.26.11 and spoom 1.8.10 | Already measured A. P/R .5845/.8355 and .7511/.8566; blind shares 2.9%/1.5% disclosed. Constant references, `new` versus `initialize`, operators and class-body calls expose shared ScoreSCIPWith normalization gaps. L3 recall .0613/.0794 exposes transitive require binding. Changing the oracle alone does not repair these product/scorer gaps. |
| Current Sorbet checker / dump / LSP | 0.6.13594 release metadata; official CLI contract inspected | Standalone third-party checker/LSP/custom exports remain C under P-T1, as OL-R2/OL-R7; a maintained SCIP producer is the distinct A route above. The contract retains T.untyped, and a syntax/CFG dump does not supply missing receiver types in unchanged untyped code. No claim that this release was corpus-executed or has the old release's exact accuracy. A new index with materially broader native occurrences reopens measurement. |
| Ruby LSP | 0.26.11 gem/release README inspected | Editor definition/reference support is a candidate, not proof of complete native Ruby call binding. It is a third-party static analysis route; a custom export does not remove ADR-048's population/tier requirement. Its own typed codebase was already independently measured above. No unmeasured accuracy claim. |
| Prism AST / native parser | 2.0.0 release README | `bin/parse` prints syntax trees, without runtime method dispatch or load-path evaluation. Native parser syntax B can aid L1; it cannot lift L2/L3 to excellent. |
| RBS signatures / prototype rb or runtime | 4.2.0 release README | Describes class/module/method structure. Prototype prints signatures, not call occurrences. Runtime prototype is deprecated and does not enumerate written call sites or their target files; no complete L2 oracle. |
| RubyVM::InstructionSequence dump | Available Ruby 2.6.10 API contract | Compile/disassembly supplies VM instructions and written send names, not evaluated dynamic receiver targets. At most a native syntax/bytecode B route for calls. Not executed on the corpus in this recheck. |
| Static ruby-require lookup | Existing committed recipe, measured baseline | Operator-selected load paths are B. They omit real nested gem/test roots; the stronger runtime route below already identifies the missing files. |
| Native ruby-require-trace | Ruby 3.3.8 portable runtime, committed LB-F256 recipe | Existing A trace, 145/147 sinatra and 114/119 jekyll literal in-repo statements attested (98.64%/95.80%, above90%). Native resolution .8031, edge P .9457/R .4710; sinatra's 51 nested-gem requires and test load paths reveal product source-root gaps. Better truth is already available. |

**Source inspection:** [scip-ruby recipe and measured misses](../../calibration/oracles/scip-ruby/README.md),
[native trace recipe](../../calibration/oracles/ruby-require-trace/README.md),
[Sorbet CLI](https://sorbet.org/docs/cli-ref),
[Ruby LSP 0.26.11](https://github.com/Shopify/ruby-lsp/blob/v0.26.11/README.md),
[Prism 2.0.0](https://github.com/ruby/prism/blob/v2.0.0/README.md) and
[RBS 4.2.0](https://github.com/ruby/rbs/blob/v4.2.0/README.md).
The old native measurements above are explicitly existing evidence, not fresh
runs of newly published versions.

**Hypothesis / retry:** retain LB-Q-ruby-rest's shared ScoreSCIPWith and fusion
resolver trigger. The native trace is sufficient to prove that source roots and
transitive binding need product work; a new syntax dump cannot fix them. Such a
shared engine change requires every affected language's regrade and controller
serialization. This branch makes no engine/epoch change, adds no canonical
holdout, changes no reading and flips no plan box. The candidate list is bounded;
it does not prove that a future native instrument cannot broaden untyped truth.
