// invalid.cue — NEGATIVE fixtures, documented only (not live-vetted).
//
// These shapes intentionally violate schema.cue. They are kept commented so
// `cue vet ./...` stays green. Their rejection is asserted by the Go tests in
// platformconfig_test.go (Load / Validate must reject each).
//
// To verify a case at the CUE layer manually, uncomment it and run:
//   cue vet schema.cue testdata/invalid.cue
//
// 1. Config missing required Environment:
//      badCfg1: #Config & {Settings: {}}
//      -> Environment: field is required but not present
//
// 2. Config with empty Environment:
//      badCfg2: #Config & {Environment: "", Settings: {}}
//      -> Environment: invalid value "" (does not satisfy !="")
//
// 3. Config missing required Settings:
//      badCfg3: #Config & {Environment: "prod"}
//      -> Settings: field is required but not present
