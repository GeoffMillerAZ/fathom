package testdata
import "github.com/geoffmilleraz/fathom/schema/records"
lane: records.#Lane & {
 kind: "lane"
 id: "demo"
 goal: "Fixture only, no live reservations"
 rows: ["REC-1"]
 class: "L-P"
 base_commit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
 owned_paths: ["ADR.md", "invariants.md", "futures.md", "incidents.md", "observations.md", "docs/plans/demo.md"]
 read_only_paths: []
 contract_files: ["contract.md"]
 proof_command: "test"
 budget: "fixture"
 harness_class: "test"
 id_ranges: [
 {family: "adr", prefix: "ADR-", first: 10000, last: 10001},
 {family: "invariant", prefix: "I", first: 10000, last: 10001},
 {family: "future", prefix: "F", first: 10000, last: 10001},
 {family: "incident", prefix: "INC-", first: 10000, last: 10001},
 {family: "observation", prefix: "O", first: 10000, last: 10001},
 {family: "plan", prefix: "DEMO-", first: 10000, last: 10001},
 ]
}
