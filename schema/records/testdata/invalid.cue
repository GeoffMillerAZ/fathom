// Negative cases are exercised by Go tests; this file must stay commented.
// shape: lane: {kind: "other"}
// derived-phase: lane: {phase: "active"}
// inverted-range: id_ranges: [{first: 10001, last: 10000}]
// overlap-range: two intervals with prefix ADR- and intersecting endpoints
// duplicate-key: ADR-10000 defined in a.md and b.md
// outside-path: target outside the trusted lane's exact owned_paths
// exhausted-range: both ADR-10000 and ADR-10001 already defined
