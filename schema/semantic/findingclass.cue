// findingclass.cue — the findings-as-evidence typed failure-class
// vocabulary (docs/design/findings-as-evidence.md §3.2, §6 item 2): a
// small, closed, deliberately coarse set every finding-kind claim's
// `class` belongs to. FindingClasses is the single source of truth;
// #FindingClass is derived FROM it via or() so the disjunction can never
// drift from the list a human reads. internal/core.ValidFindingClasses
// mirrors this list in Go (the runtime enforcement point — compile/persist
// time, mirroring O19's rejection-cause taxonomy); internal/semantic's
// TestFindingClassVocabularyMatchesSchema cross-checks the two.
package semantic

FindingClasses: [
	"correctness",
	"contract-violation",
	"resource-leak",
	"error-handling",
	"concurrency",
	"security",
	"config-drift",
	"test-gap",
	"performance",
	"conformance",
]

#FindingClass: or(FindingClasses)
