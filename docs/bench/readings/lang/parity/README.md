# lang-parity readings

`indexbench lang-parity` (LB-F18, LB-F118) reports for a port: its dialect against the incumbent extractor it
retires, measured on the same subjects against the same oracles, per-metric new minus old, the SourceKind diff and
the dispatch-at-parity verdict (ADR-048 amendment 2026-10-01, D1): MERGE when every metric of the levels up to the
dialect's demonstrated level_met, measured on both sides, is at least the incumbent's and no SourceKind is lost
without a reason. A MERGE switches fusion's dispatch (internal/fusion/register.go) and bumps ExtractorEpoch.

- `c.json` (2026-10-01): the C tags dialect against the CGO `cextract.go` on libuv v1.53.0 and curl 8.22.0
  (scip-clang 0.4.0 definitions as the L1 oracle, `-scip-defs`, and the build's preprocessor): MERGE, dispatched.
- `cpp.json` (2026-10-01): the C++ tags dialect against the CGO `cppextract.go` on fmtlib/fmt 12.2.0,
  nlohmann/json v3.12.0 and gabime/spdlog v1.17.0 (the cpp.json reading's scope, oracles and clang-calls):
  REFUSED on range_agreement alone (0.9921 < 0.9923), not dispatched. Re-checked 2026-10-02 (wave ld): neither
  `dialects/cpp/tags.scm` nor `cppextract.go`'s symbol path changed since (only the EW-53 file specs, which no gated
  metric reads), so it was not re-run and stands; the lever is a lexical macro pre-pass in the tags engine (O455).
- `php.json` (2026-10-02, wave ld): the PHP tags dialect against the CGO `phpextract.go` on guzzle/guzzle 7.9.3 and
  Seldaek/monolog 3.9.0 (the php.json reading's oracles: the nikic/php-parser 5.7.0 definitions dump for L1, a scip-php
  71a5b117 index for the rest): MERGE at L1 (sym_recall 0.8901 to 1, signature_coverage 0 to 1; sym_precision, kind and
  range agreement 1 on both), and level or ahead on every metric measured beyond it (call_precision 0.6636 to 0.6924,
  file_edge_recall 0.4948 to 0.9193, file_edge_precision 0.9979 to 0.9989, def_mapping 0.9826 to 0.9849, typed_recall
  0.9992 to 0.9997; call_recall and import_resolution 1 on both); PhpNamespace gained, no kind lost: dispatched (LB-F183).
- `rust.json` (2026-10-02, wave ld): the Rust tags dialect against the CGO `rustextract.go` on BurntSushi/ripgrep 14.1.1
  and clap-rs/clap v4.6.1 (the rust.json reading's oracle: rust-analyzer 0.0.0 (03fcb77246 2026-09-27) SCIP indexes,
  cargo.features all, `-scip-defs`): REFUSED on sym_precision alone (0.9746 < 0.9756, -0.0010); every other L1 and L2
  metric is level or ahead (sym_recall 0.8909 to 0.9984, signature_coverage 0 to 1, call_recall 0.8125 to 0.9479,
  call_precision 0.9332 to 0.9846; kind and range agreement 1 on both), and beyond L2 import_resolution reads 0.4732 on
  both, file_edge_precision 0.9139 to 0.915, file_edge_recall 0.0597 to 0.0606, def_mapping 0.8001 to 0.8988,
  typed_recall 0.7476 to 0.8212. RustConst, RustMacro, RustStatic and RustTypeAlias gained, none lost. The dialect's
  38 more unpaired definitions: 16 duplicates (a const or type alias inside an inline `mod { }` emitted twice, as a
  module item and as an associated item), 21 real items under a cfg rust-analyzer does not analyze, 1 raw identifier
  (O456). Not dispatched.
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
- `java.json` (2026-10-02, wave lh): the Java tags dialect against the CGO `javaextract.go` on google/guava v33.7.1
  (its `guava/` module directory) and square/retrofit 3.0.0 (the java.json reading's oracles: the JavaParser 3.28.2
  definitions dump dated by the definition's name (O457), calls emptied and imports removed, and scip-java 0.12.3
  indexes rebuilt by calibration/oracles/scip-java's recipe): REFUSED on file_edge_precision alone (0.9978 < 1). Every
  other gated metric (L1 to L3) is level or ahead: sym_recall 0.9004 to 1, sym_precision 0.9787 to 1,
  signature_coverage 0 to 1, call_recall 0.8408 to 0.9529 and call_precision 0.9801 to 0.9804 (the dialect records a
  constructor call, O268, and the call truth gives an anonymous class's method its own calls, O458),
  import_resolution 0.9942 to 1, file_edge_recall 0.2922 to 0.829, kind and range agreement 1 on both; beyond L3,
  disclosed: def_mapping 0.918 to 0.9701, typed_recall 0.3634 to 0.669. JavaAnnotation gained, none lost. The 9
  fathom-only file pairs are a same-package class shadowed by a single-type import of a JDK type (8) or by the file's
  own nested type (1), which the file resolver binds anyway: a fileresolve.go rule, not a query's (O513). Not dispatched.
