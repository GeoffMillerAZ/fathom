// Package openai — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid CUE,
// entirely commented out). It documents the constraints enforced by
// schema.cue; NewOpenAI's own rejection of a config with no key source is
// Go-level, in openai_test.go (TestNewOpenAIRequiresAKeySource).
//
// To verify the schema rejects a binding, temporarily uncomment it and run:
//
//	cue vet ./testdata/   (expect errors for that binding)
//
// The following errors would be produced by `cue vet ./testdata/`:
//
//	_badBaseURL: "api.openai.com/v1" does not match =~"^https?://".
//	_badEmptyModel: "" violates string & !="".
//	_badKeyEnv: "OPENAI API KEY" is not an environment variable name.
package openai

// // _badBaseURL: BaseURL must carry an http or https scheme.
// _badBaseURL: #OpenAIConfig & {
// 	APIKeyEnv: "OPENAI_API_KEY"
// 	BaseURL:   "api.openai.com/v1"
// }

// // _badEmptyModel: Model must not be empty.
// _badEmptyModel: #OpenAIConfig & {
// 	APIKeyEnv: "OPENAI_API_KEY"
// 	Model:     ""
// }

// // _badKeyEnv: APIKeyEnv is an environment variable name.
// _badKeyEnv: #OpenAIConfig & {
// 	APIKeyEnv: "OPENAI API KEY"
// }
