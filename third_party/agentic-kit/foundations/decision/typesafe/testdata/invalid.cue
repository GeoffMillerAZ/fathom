// Package typesafe — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid CUE,
// entirely commented out). It documents the constraints enforced by
// schema.cue; NewTypeSafe's own rejection of an empty key is Go-level, in
// typesafe_test.go (TestNewTypeSafe_EmptyKeyErrors).
//
// To verify the schema rejects a binding, temporarily uncomment it and run:
//
//	cue vet ./testdata/   (expect errors for that binding)
//
// The following errors would be produced by `cue vet ./testdata/`:
//
//	_badEmptyKey: "" violates string & !="" (APIKey must be non-empty).
//	_badBaseURL: "api.example" does not match =~"^https?://".
//	_badEmptyModel: "" violates string & !="" (Model must be non-empty).
//	_badTransport: "carrier-pigeon" is not "typesafe" or "openrouter".
//	_badPath: "api/alpha/decisions" does not match =~"^/".
//	_badKeyEnv: "1BAD NAME" is not an environment variable name.
package typesafe

// // _badEmptyKey: APIKey is required and non-empty.
// _badEmptyKey: #TypeSafeConfig & {
// 	APIKey: ""
// }

// // _badBaseURL: BaseURL must carry an http or https scheme.
// _badBaseURL: #TypeSafeConfig & {
// 	APIKey:  "test-placeholder-key-not-real"
// 	BaseURL: "api.example"
// }

// // _badEmptyModel: Model must not be empty.
// _badEmptyModel: #TypeSafeConfig & {
// 	APIKey: "test-placeholder-key-not-real"
// 	Model:  ""
// }

// // _badTransport: Transport is one of the two transports.
// _badTransport: #TypeSafeConfig & {
// 	APIKeyEnv: "TYPESAFE_API_KEY"
// 	Transport: "carrier-pigeon"
// }

// // _badPath: Path begins with "/".
// _badPath: #TypeSafeConfig & {
// 	APIKeyEnv: "TYPESAFE_API_KEY"
// 	Path:      "api/alpha/decisions"
// }

// // _badKeyEnv: APIKeyEnv is an environment variable name.
// _badKeyEnv: #TypeSafeConfig & {
// 	APIKeyEnv: "1BAD NAME"
// }
