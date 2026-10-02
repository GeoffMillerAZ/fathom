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
  REFUSED on range_agreement alone (0.9921 < 0.9923), not dispatched.
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
