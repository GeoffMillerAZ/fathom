// invalid.cue — negative fixtures for the plan schema, documented
// commented-out so live `cue vet` stays green (repo convention). Each case is
// asserted to actually fail by internal/verify/schema_test.go
// (TestPlanSchema_Negatives).
package plan

// 1. state outside the grammar-v2 vocabulary:
// bad1: #Task & {id: "T-1", state: "half-done"}

// 2. disposition outside refused|deferred:
// bad2: #Task & {id: "T-2", state: "skipped", disposition: "sideways"}

// 3. min_tier outside the evidence-tier vocabulary:
// bad3: #Task & {id: "T-3", expect: {symbol: "X", min_tier: "vibes"}}

// 4. a plan with a non-string service (missing fields are incomplete, not
//    invalid, under non-concrete validation — so the negative is a conflict):
// bad4: #Plan & {service: 42, tasks: []}

// 5. oracle outside the ADR-030 vocabulary (only "lexical" exists):
// bad5: #Task & {id: "T-5", expect: {oracle: "vibes", path: "docs/x.md", pattern: "y"}}

// 6. weight below 1 (ADR-049: a positive integer, default 1):
// bad6: #Task & {id: "T-6", weight: 0}

// 7. library is a boolean waiver (ADR-018 amendment 2026-09-28), not a reason:
// bad7: #Task & {id: "T-7", expect: {symbol: "X", library: "the API is the deliverable"}}
