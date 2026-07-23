package archconformance

// INVALID examples — kept commented so `cue vet ./...` stays green; the Go
// test (internal/analysis/archconformance/schema_test.go, mirroring
// internal/config/schema_test.go's precedent) asserts each fails when
// uncommented into a unification.
//
// 1. A rule missing a required field (to_pkg):
// bad1: #RuleSet & {
// 	rules: [
// 		{from_pkg: "connector", allow: false},
// 	]
// }
//
// 2. allow is not a bool:
// bad2: #RuleSet & {
// 	rules: [
// 		{from_pkg: "a", to_pkg: "b", allow: "yes"},
// 	]
// }
//
// 3. deny_by_default is not a bool:
// bad3: #RuleSet & {
// 	rules: []
// 	deny_by_default: "true"
// }
//
// 4. an extra unknown field on a rule (#Rule is closed):
// bad4: #RuleSet & {
// 	rules: [
// 		{from_pkg: "a", to_pkg: "b", allow: true, extra: "nope"},
// 	]
// }
//
// 5. rules is not a list:
// bad5: #RuleSet & {
// 	rules: "not-a-list"
// }
