// Package anthropic — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (the examples are intentionally
// invalid and therefore commented out). It documents the constraints
// enforced by schema.cue; the matching Go rejection tests live in
// config (New) tests.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidConfig_EmptyAPIKeyEnv:
//     APIKeyEnv: "" violates *"ANTHROPIC_API_KEY" | (string & !="")
//
//   invalidConfig_BadBaseURL:
//     BaseURL: "api.anthropic.com" violates =~"^https?://" — must carry a scheme
//
//   invalidConfig_ZeroMaxTokens:
//     DefaultMaxTokens: 0 violates *4096 | (int & >0) — max_tokens is required
//     by the Messages API and must be positive (use the default instead)
//
//   invalidConfig_NegativeMaxTokens:
//     DefaultMaxTokens: -1 violates int & >0
//
//   invalidConfig_EmptyDefaultModel:
//     DefaultModel: "" violates string & !="" — omit the field instead
//
//   invalidConfig_EmptyVersion:
//     Version: "" violates *"2023-06-01" | (string & !="")
//
//   invalidConfig_UnknownAuthScheme:
//     AuthScheme: "basic" violates *"x-api-key" | "bearer" — the scheme set
//     is closed; New rejects anything else too
//
//   invalidConfig_EmptyBetaHeader:
//     BetaHeaders: [""] violates [...string & !=""] — every anthropic-beta
//     value must be non-empty (omit the field instead)
//
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package anthropic

// // invalidConfig_EmptyAPIKeyEnv — APIKeyEnv must not be empty
// invalidConfig_EmptyAPIKeyEnv: #Config & {
// 	APIKeyEnv: ""
// }

// // invalidConfig_BadBaseURL — BaseURL must carry an http(s) scheme
// invalidConfig_BadBaseURL: #Config & {
// 	BaseURL: "api.anthropic.com"
// }

// // invalidConfig_ZeroMaxTokens — DefaultMaxTokens must be positive
// invalidConfig_ZeroMaxTokens: #Config & {
// 	DefaultMaxTokens: 0
// }

// // invalidConfig_NegativeMaxTokens — DefaultMaxTokens must be positive
// invalidConfig_NegativeMaxTokens: #Config & {
// 	DefaultMaxTokens: -1
// }

// // invalidConfig_EmptyDefaultModel — omit DefaultModel rather than ""
// invalidConfig_EmptyDefaultModel: #Config & {
// 	DefaultModel: ""
// }

// // invalidConfig_EmptyVersion — Version must not be empty
// invalidConfig_EmptyVersion: #Config & {
// 	Version: ""
// }

// // invalidConfig_UnknownAuthScheme — only "x-api-key" and "bearer" exist
// invalidConfig_UnknownAuthScheme: #Config & {
// 	AuthScheme: "basic"
// }

// // invalidConfig_EmptyBetaHeader — beta values must be non-empty
// invalidConfig_EmptyBetaHeader: #Config & {
// 	BetaHeaders: [""]
// }
