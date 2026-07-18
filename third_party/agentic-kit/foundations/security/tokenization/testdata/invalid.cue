// invalid.cue — demonstrates values that VIOLATE the tokenization schema.
// All examples are commented out; uncomment any block to observe the cue vet
// error it produces. This file is documentation only — it is not vetted in CI.
package tokenization

// BadTokenClass: demonstrates an unrecognised TokenClass value.
// cue vet error: Class: 5 errors in empty disjunction:
//   Class: conflicting values "top_secret" and "pii" (mismatched types)
//   ... (not a valid #TokenClass value)
//
// BadTokenClass: {
// 	Value:     "tok_top_secret_abc123"
// 	Class:     "top_secret"
// 	Namespace: "ns"
// }

// EmptyTokenValue: demonstrates a Token with an empty Value.
// cue vet error: Value: invalid value "" (does not satisfy !="")
//
// EmptyTokenValue: {
// 	Value:     ""
// 	Class:     "pii"
// 	Namespace: "ns"
// }

// EmptyNamespace: demonstrates a Token with an empty Namespace.
// cue vet error: Namespace: invalid value "" (does not satisfy !="")
//
// EmptyNamespace: {
// 	Value:     "tok_pii_abc123"
// 	Class:     "pii"
// 	Namespace: ""
// }

// BadAuditAction: demonstrates an AuditEvent with an invalid Action.
// cue vet error: Action: 2 errors in empty disjunction:
//   Action: conflicting values "read" and "tokenize" (mismatched types)
//   ...
//
// BadAuditAction: {
// 	At:          "2024-01-01T00:00:00Z"
// 	Action:      "read"
// 	TokenValue:  "tok_pii_abc123"
// 	Class:       "pii"
// 	PrincipalID: "user@example.com"
// 	Purpose:     ""
// 	Allowed:     true
// }

// EmptyAuditAt: demonstrates an AuditEvent with an empty At timestamp.
// cue vet error: At: invalid value "" (does not satisfy !="")
//
// EmptyAuditAt: {
// 	At:          ""
// 	Action:      "tokenize"
// 	TokenValue:  "tok_pii_abc123"
// 	Class:       "pii"
// 	PrincipalID: "user@example.com"
// 	Purpose:     ""
// 	Allowed:     true
// }

// EmptyPrincipalID: demonstrates an AuditEvent with an empty PrincipalID.
// cue vet error: PrincipalID: invalid value "" (does not satisfy !="")
//
// EmptyPrincipalID: {
// 	At:          "2024-01-01T00:00:00Z"
// 	Action:      "tokenize"
// 	TokenValue:  "tok_pii_abc123"
// 	Class:       "pii"
// 	PrincipalID: ""
// 	Purpose:     ""
// 	Allowed:     true
// }
