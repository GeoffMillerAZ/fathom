// invalid.cue — NEGATIVE fixtures, documented only (not live-vetted).
//
// These shapes intentionally violate schema.cue. They are kept commented so
// `cue vet ./...` stays green. Their rejection is asserted by the Go tests in
// toolexecutionpolicy_test.go (ToolPolicy.Validate and NewPolicySet must
// reject each).
//
// To verify a case at the CUE layer manually, uncomment it and run:
//   cue vet schema.cue testdata/invalid.cue
//
// 1. ToolPolicy missing required Tool:
//      bad1: #ToolPolicy & {Allow: true, Environments: ["dev"]}
//      -> Tool: field is required but not present
//
// 2. ToolPolicy with empty Tool:
//      bad2: #ToolPolicy & {Tool: "", Allow: true, Environments: ["dev"]}
//      -> Tool: invalid value "" (does not satisfy !="")
//
// 3. ToolPolicy missing required Allow:
//      bad3: #ToolPolicy & {Tool: "web_search", Environments: ["dev"]}
//      -> Allow: field is required but not present
//
// 4. ToolPolicy with an empty string in Environments:
//      bad4: #ToolPolicy & {Tool: "web_search", Allow: true, Environments: [""]}
//      -> Environments[0]: invalid value "" (does not satisfy !="")
