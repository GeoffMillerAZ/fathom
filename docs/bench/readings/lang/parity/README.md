# lang-parity readings

`indexbench lang-parity` (LB-F18, LB-F118) reports for a port: its dialect against the incumbent extractor it
retires, measured on the same subjects against the same oracles, per-metric new minus old, the SourceKind diff and
the dispatch-at-parity verdict (ADR-048 amendment 2026-10-01, D1): MERGE when every metric of the levels up to the
dialect's demonstrated level_met, measured on both sides, is at least the incumbent's and no SourceKind is lost
without a reason. A MERGE switches fusion's dispatch (internal/fusion/register.go) and bumps ExtractorEpoch.

- `c.json` (2026-10-01): the C tags dialect against the CGO `cextract.go` on libuv v1.53.0 and curl 8.22.0
  (scip-clang 0.4.0 definitions as the L1 oracle, `-scip-defs`, and the build's preprocessor): MERGE, dispatched.
- `cpp.json` (2026-10-09, wave lf1): the C++ tags dialect against the CGO `cppextract.go` on fmtlib/fmt 12.2.0,
  nlohmann/json v3.12.0 and gabime/spdlog v1.17.0 (the cpp.json reading's scope, oracles and clang-calls): MERGE at L4
  (level_met none to L4), every gated metric ahead: sym_recall 0.5359 to 0.9689, sym_precision 0.9057 to 0.9735,
  range_agreement 0.9926 to 0.9966, call_recall 0.8292 to 0.9292, call_precision 0.5267 to 0.9565, file_edge_recall 0.2315
  to 0.4355, def_mapping 0.4278 to 0.8989, typed_recall 0.6418 to 0.9656; CppEnumerator, CppField, CppMacro, CppTypedef,
  CppUnion and CppVariable gained, none lost: dispatched (LB-F117). The 2026-10-01 run read REFUSED on range_agreement
  alone (0.9921 < 0.9923) until the cpp-macros pre-pass read decorator and try/catch macros (O455).
- `c-headers.json` and `cpp-headers.json` (2026-10-10, wave lf3): `.h` through the CT-3 router over the c and cpp
  dialects against the CGO router over `cextract.go` and `cppextract.go` (the retired `NewCHeaderExtractor`), each
  dialect on the headers the sniff gives it (`-only '**/*.h'`): the C side on libuv v1.53.0 and curl 8.22.0 (295
  headers; scip-clang 0.4.0 definitions, `-scip-defs`, and the build's preprocessor), the C++ side on fmt, json and
  spdlog (the cpp.json reading's oracles). C: MERGE at L1 (sym_recall 0.1917 to 0.9364, sym_precision 0.8462 to 0.9637,
  kind_agreement 0.9979 to 0.9996, signature_coverage 0 to 1); beyond L1, ungated, call_recall 0.1008 to 0.0255 (each
  side's call truth is the calls inside the definitions it names, and 404 of curl's 409 truth pairs are a function-like
  macro, `CURLOPT(...)`, `CURL_DEPRECATED(...)`, `BIT(...)`, written in an enumerator or field the dialect names and the
  walker does not, which it records as no call), file_edge_precision 1 to 0.972, file_edge_recall 0.1481 to 0.1775, def_mapping 0.0845 to 0.9705, typed_recall
  0.6357 to 0.9451. C++: MERGE at L2 (sym_recall 0.5079 to 0.8929, sym_precision 0.8973 to 0.9771, range_agreement
  0.9918 to 0.9954, call_recall 0.7917 to 0.907, call_precision 0.5439 to 0.9464); beyond L2, ungated,
  file_edge_precision 0.9409 to 0.8474, file_edge_recall 0.3959 to 0.5905, def_mapping 0.444 to 0.8558, typed_recall
  0.583 to 0.9103. No kind lost on either side. `.h` has dispatched the router over the dialects since LB-F119 and
  LB-F117; these readings retire the CGO router (LB-F392).
- `php.json` (2026-10-02, wave ld): the PHP tags dialect against the CGO `phpextract.go` on guzzle/guzzle 7.9.3 and
  Seldaek/monolog 3.9.0 (the php.json reading's oracles: the nikic/php-parser 5.7.0 definitions dump for L1, a scip-php
  71a5b117 index for the rest): MERGE at L1 (sym_recall 0.8901 to 1, signature_coverage 0 to 1; sym_precision, kind and
  range agreement 1 on both), and level or ahead on every metric measured beyond it (call_precision 0.6636 to 0.6924,
  file_edge_recall 0.4948 to 0.9193, file_edge_precision 0.9979 to 0.9989, def_mapping 0.9826 to 0.9849, typed_recall
  0.9992 to 0.9997; call_recall and import_resolution 1 on both); PhpNamespace gained, no kind lost: dispatched (LB-F183).
- `rust.json` (2026-10-10, wave lf3): the Rust tags dialect against the CGO `rustextract.go` on BurntSushi/ripgrep 14.1.1
  and clap-rs/clap v4.6.1 (the rust.json reading's oracle: rust-analyzer 0.0.0 (03fcb77246 2026-09-27) SCIP indexes,
  cargo.features all, `-scip-defs`): MERGE at L4 (level_met none to L4). Wave lx's refusal (file_edge_precision 0.9797 <
  0.9803, one edge) was the harness taking the first of two documents rust-analyzer defines one symbol in (O539); K25
  ruling 3 resolves such a symbol to every defining document (LB-F387). sym_recall 0.8966 to 0.9984, sym_precision 0.9754
  to 0.9761, signature_coverage 0 to 1, call_recall 0.8125 to 0.9479, call_precision 0.9332 to 0.9846, file_edge_precision
  0.9971 to 0.9972, file_edge_recall 0.6192 to 0.6206, def_mapping 0.8938 to 0.9984, typed_recall 0.9092 to 0.9966;
  kind and range agreement 1 and import_resolution 0.9771 on both. RustConst, RustStatic and RustTypeAlias gained, none
  lost. Dispatched (LB-F388); wave ld's refusal (sym_precision) was O456's duplicate consts. The walker is retired
  (LB-F390), so this reading is the last that can be taken against it.
- `csharp.json` (2026-10-02, wave lh): the C# tags dialect against the CGO `csharpextract.go` on App-vNext/Polly 9ad5ae9b
  and JamesNK/Newtonsoft.Json 4e13299d (the csharp.json reading's oracles: its Roslyn 5.9.0 definitions dump, now dated by
  the definition's name (O457), and scip-dotnet 0.2.14 indexes of 2026-09-28, reused at the pinned checkouts): MERGE at L1
  (sym_recall 0.8283 to 0.9996, sym_precision 0.9823 to 0.9958, range_agreement 0.9519 to 0.9601, signature_coverage 0 to
  1, kind_agreement 0.9997 both). Beyond L1, disclosed: call_recall 0.7777 to 0.9448, call_precision 0.7925 to 0.794 (not
  like for like: the oracle-blind cap is decided per side, so the incumbent's 4.3% blind pairs leave its counts and the
  dialect's 17.7% do not; with the rule the dialect reads 0.9647, O512), import_resolution 0 to 1, file_edge_precision
  0.913 to 0.9049 (-0.0081), file_edge_recall 0.0023 to 0.4346, def_mapping 0.6628 to 0.8222, typed_recall 0.5855 to
  0.746. CSharpProperty gained, none lost. Dispatched (LB-F232); wave ld's refusal (sym_precision 0.985 < 0.9952) was
  the dump dating a declaration from its first attribute list.
- `java.json` (2026-10-03, wave lf1): the Java tags dialect against the CGO `javaextract.go` on google/guava v33.7.1 (its `guava/` module
  directory) and square/retrofit 3.0.0 (the java.json reading's oracles: the JavaParser 3.28.2 definitions dump dated by the definition's
  name (O457), calls emptied and imports removed, and scip-java 0.12.3 indexes): MERGE at L4 (level_met none to L4). Wave lh's refusal
  (file_edge_precision 0.9978 < 1) was a resolver rule: a single-type import, even of a JDK type, and a type the file itself declares
  shadow a same-package class of that name (JLS 6.4.1, internal/fusion/javashadow.go, O513), so file_edge_precision reads 1 on both sides.
  sym_recall 0.9004 to 1, sym_precision 0.9787 to 1, signature_coverage 0 to 1, call_recall 0.8408 to 0.9529, call_precision 0.9801 to
  0.9804, import_resolution 0.9942 to 1, file_edge_recall 0.2922 to 0.829, def_mapping 0.918 to 0.9701, typed_recall 0.462 to 0.9515
  (both sides rise with S1's parameter-target rule, LB-F239); kind and range agreement 1 on both. JavaAnnotation gained, none lost.
  Dispatched (LB-F302), ingest cost O610, ExtractorEpoch need O611.
- `bash.json` (2026-10-10, wave lf3): the bash tags dialect against the CGO `bashextract.go` on bats-core/bats-core 713504bc
  and nvm-sh/nvm 1889911f (the bash.json reading's oracles: universal-ctags 6.2.1 for L1, tier B, and the tier-C tags oracle,
  calibration/oracles/ts-tags with py-tree-sitter 0.25.2 and tree-sitter-bash 0.25.1, for L2 calls, regenerated byte-identical
  to the reading's): MERGE at L2 (level_met none to L2). sym_recall 0.9772 to 0.9805, sym_precision 0.9836 to 0.9837,
  signature_coverage 0 to 1, call_recall 0 to 0.9798 and call_precision none to 1 (the walker records no call), kind_agreement 1
  on both. BashVariable gained, none lost. The dialect's own CPU throughput on the two subjects reads 3.51 MB/s (LB-M5's floor
  is 1). Dispatched (LB-F391).
- `scala.json` (2026-10-10, wave lf3): the Scala tags dialect against the CGO `scalaextract.go` on typelevel/cats v2.13.0 and
  http4s/http4s v0.23.38 (the scala.json reading's oracle: the scalameta 4.17.4 dump, tier B, for definitions and calls): MERGE at L2
  (level_met none to L2). sym_recall 0.5859 to 1, sym_precision 0.9868 to 1, kind_agreement 0.9993 to 1, range_agreement 0.7183 to
  0.8042 (both sides read the grammar's outdent end, O761), signature_coverage 0 to 1, call_recall 0.808 to 0.9999, call_precision
  0.8233 to 0.9999. ScalaConstructor, ScalaType, ScalaVal and ScalaVar gained, none lost. Dispatched and the walker retired (LB-F399,
  LB-F400); ExtractorEpoch need in LB-Q-scala-rest's wave notes (F.54).
- `graphql.json` (2026-10-19, wave lang1): the GraphQL tags dialect against `graphqlextract.go` (gqlparser) on octokit/graphql-schema v15.26.1 and
  99designs/gqlgen v0.17.95 (the graphql.json reading's oracle: graphql-js 16.14.2, tier A): MERGE at L2 (level_met none to L2). sym_recall 0.8725 to 1
  (the incumbent reads a schema only: gqlparser's schema parser rejects a file that holds an operation, a fragment or an extension, so the
  whole file, its types and fields included, returns nothing; a directive definition and an enum value are not symbols), sym_precision 1 on both, ref_recall 0 to 1, ref_precision 1. GraphQLDirective, GraphQLEnumValue, GraphQLExtension,
  GraphQLFragment and GraphQLOperation gained, none lost. Not like for like by design: a type's Signature is its header as written, not the lowercased
  kind word. Dispatched and the incumbent deleted; ExtractorEpoch need in O1284.
