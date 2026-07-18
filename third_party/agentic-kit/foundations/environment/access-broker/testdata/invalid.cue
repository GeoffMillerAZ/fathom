// invalid.cue — NEGATIVE fixtures, documented only (not live-vetted).
//
// These shapes intentionally violate schema.cue. They are kept commented so
// `cue vet ./...` stays green. Their rejection is asserted by the Go tests in
// accessbroker_test.go (CredentialScope.Validate must reject each).
//
// To verify a case at the CUE layer manually, uncomment it and run:
//   cue vet schema.cue testdata/invalid.cue
//
// 1. CredentialScope missing required Resource:
//      bad1: #CredentialScope & {Level: "read"}
//      -> Resource: field is required but not present
//
// 2. CredentialScope with empty Resource:
//      bad2: #CredentialScope & {Resource: "", Level: "read"}
//      -> Resource: invalid value "" (does not satisfy !="")
//
// 3. CredentialScope with Level outside the enum:
//      bad3: #CredentialScope & {Resource: "db/x", Level: "superuser"}
//      -> Level: conflicting values "superuser" and "read" | "write" | "admin"
//
// 4. CredentialScope missing required Level:
//      bad4: #CredentialScope & {Resource: "db/x"}
//      -> Level: field is required but not present
