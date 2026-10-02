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
