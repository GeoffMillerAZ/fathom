// invalid.cue — negative fixtures, documented commented-out so live
// `cue vet` stays green; TestConfigValidate in capped-confidence_test.go
// asserts each is rejected in Go.
package cappedconfidence

// 1. a non-positive scale:
// bad1: #Table & {max: 0, ceilings: [{class: "a", ceiling: 0, why: "w"}]}

// 2. no rows — a table with no rows makes every class unscored:
// bad2: #Table & {max: 100, ceilings: []}

// 3. an empty class:
// bad3: #Table & {max: 100, ceilings: [{class: "", ceiling: 50, why: "w"}]}

// 4. a ceiling above the scale:
// bad4: #Table & {max: 100, ceilings: [{class: "a", ceiling: 101, why: "w"}]}

// 5. a negative ceiling:
// bad5: #Table & {max: 100, ceilings: [{class: "a", ceiling: -1, why: "w"}]}

// 6. no rationale — a ceiling nobody can re-derive is one nobody can revise:
// bad6: #Table & {max: 100, ceilings: [{class: "a", ceiling: 50, why: ""}]}

// 7. a clamped score whose ceiling is not below its raw:
// bad7: #Score & {raw: 40, ceiling: 50, class: "a", clamped: true}

// 8. a clamped score with no class to blame. The class is absent rather
//    than wrong, so plain `cue vet` reports the instance incomplete and
//    `cue vet -c` (or any export/decode) rejects it:
// bad8: #Score & {raw: 92, ceiling: 50, clamped: true}
