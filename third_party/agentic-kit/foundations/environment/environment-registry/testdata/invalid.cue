// invalid.cue — NEGATIVE fixtures, documented only (not live-vetted).
//
// These shapes intentionally violate schema.cue. They are kept commented so
// `cue vet ./...` stays green. Their rejection is asserted by the Go tests in
// environmentregistry_test.go (Validate() must reject each).
//
// To verify a case at the CUE layer manually, uncomment it and run:
//   cue vet schema.cue testdata/invalid.cue
//
// 1. Environment missing required Name:
//      badEnv1: #Environment & {Kind: "dev"}
//      -> Name: field is required but not present
//
// 2. Environment with Kind outside the enum:
//      badEnv2: #Environment & {Name: "x", Kind: "unknown"}
//      -> Kind: conflicting values "unknown" and "dev" | "staging" | "prod"
//
// 3. Environment with empty Name:
//      badEnv3: #Environment & {Name: "", Kind: "prod"}
//      -> Name: invalid value "" (does not satisfy !=  "")
