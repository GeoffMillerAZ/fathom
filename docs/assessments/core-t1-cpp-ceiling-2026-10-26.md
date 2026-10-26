# cpp named ceiling routes (X-R1)

The committed reading is unchanged: level L4, grades L1 excellent, L2 excellent, L3 good, L4 good.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| Semantic index | scip-clang 0.4.0 | Existing def_mapping .8988 misses special members, aliases and macro-written definitions. typed_recall .9656 is already excellent; changing oracles alone does not emit the missing extractor symbols. |
| Definition oracle | Universal Ctags 6.2.1; Apple clang 21.0.0 (clang-2100.3.34.2) | Existing range agreement .9966 and symbol recall .9689 already provide independent L1 truth. No compiler upgrade is silently substituted. |
| Build call oracle / preprocessing | Apple clang 21.0.0, own compilation databases; cpp-macros at 5b84aa5b4 | Existing call precision .9565/recall .9292. Macro blanking and template-declaration shapes remain concrete extractor/pre-pass work; the pass cost is already near the throughput floor. |
| C-family implementation changes | CUDA issue4 and HLSL issue5 pending integration | The lane brief explicitly defers C++ until both land. Fresh origin/main5b84aa5b4 contains neither latest round. No simultaneous shared C-family edit is made. |

Dependency hold, not an oracle-exhaustion claim. Re-read dependency landing and the existing pre-pass/special-member triggers before touching the shared path.

Governing row: `LB-Q-cpp-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.
