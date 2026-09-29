// invalid.cue — negative fixtures, documented commented-out so live
// `cue vet` stays green; the tests in structural-refusal_test.go assert the
// same rules in Go for judgments that never met the schema.
package structuralrefusal

// 1. an affirmative verdict with no evidence — the structural rule. The
//    list is absent rather than empty, so plain `cue vet` reports the
//    instance incomplete and `cue vet -c` (or any export/decode) rejects it:
// bad1: #Judgment & {subject: "x", verdict: "meets"}

// 2. an affirmative verdict with an empty evidence list — list length is
//    not evidence:
// bad2: #Judgment & {subject: "x", verdict: "meets", evidence: []}

// 3. a citation with no id:
// bad3: #Judgment & {subject: "x", verdict: "meets", evidence: [{kind: "record", id: ""}]}

// 4. a verdict outside the vocabulary:
// bad4: #Judgment & {subject: "x", verdict: "MEETS"}

// 5. a standing verdict carrying a refusal note:
// bad5: #Judgment & {subject: "x", verdict: "does-not-meet", refused: "stale"}

// 6. an empty subject:
// bad6: #Judgment & {subject: "", verdict: "unknown"}

// 7. more unknown than total:
// bad7: #Rate & {unknown: 5, total: 4}
