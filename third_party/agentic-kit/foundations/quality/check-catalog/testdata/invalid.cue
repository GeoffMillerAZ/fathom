// invalid.cue — negative fixtures, documented commented-out so live
// `cue vet` stays green; TestRowValidate in check-catalog_test.go asserts each
// is rejected in Go.
package checkcatalog

// 1. empty id:
// bad1: #Check & {id: "", name: "x", severity: "data", means: "deterministic", rejects: "y"}

// 2. empty name:
// bad2: #Check & {id: "X-01", name: "", severity: "data", means: "deterministic", rejects: "y"}

// 3. severity outside the vocabulary:
// bad3: #Check & {id: "X-01", name: "x", severity: "critical", means: "deterministic", rejects: "y"}

// 4. means outside the vocabulary:
// bad4: #Check & {id: "X-01", name: "x", severity: "data", means: "heuristic", rejects: "y"}

// 5. empty rejects:
// bad5: #Check & {id: "X-01", name: "x", severity: "data", means: "deterministic", rejects: ""}

// 6. a fatal check resting on a model — the schema forces means to
//    "deterministic" and the conflicting value fails to unify:
// bad6: #Check & {id: "X-01", name: "x", severity: "fatal", means: "model", rejects: "y"}

// 7. an empty source (omit it instead):
// bad7: #Check & {id: "X-01", name: "x", severity: "data", means: "deterministic", rejects: "y", source: ""}
