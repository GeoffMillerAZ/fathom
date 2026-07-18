// Package mcpserver — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidConfig_EmptyName:
//     Name: "" violates string & !="" — the initialize serverInfo needs a name
//
//   invalidConfig_EmptyVersion:
//     Version: "" violates string & !="" — the initialize serverInfo needs a
//     version
//
//   invalidConfig_ZeroMaxMessageBytes:
//     MaxMessageBytes: 0 violates int & >=1 — omit the field to take the Go
//     default of 4 MiB; an explicit 0 is a misconfiguration
//
//   invalidConfig_NegativeMaxMessageBytes:
//     MaxMessageBytes: -1 violates int & >=1 — a negative bound is meaningless
//
//   invalidConfig_UnknownField:
//     CallLog: "audit.jsonl" — #Config is closed; CallLog is a Go-side
//     io.Writer seam, never a data field
//
//   invalidHTTPOptions_ZeroMaxBodyBytes:
//     MaxBodyBytes: 0 violates int & >=1 — omit the field to inherit the
//     server's MaxMessageBytes; an explicit 0 is a misconfiguration
//
//   invalidHTTPOptions_EmptyOrigin:
//     AllowedOrigins: [""] violates [...string & !=""] — an empty origin
//     entry can never match a real Origin header; omit the list (fail-closed
//     default) instead of padding it
//
//   invalidHTTPOptions_UnknownField:
//     RandHex: "seed" — #HTTPOptions is closed; RandHex is a Go-side func
//     seam (the CallLog precedent), never a data field
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment the import and
// each block, then run (note: `cue vet ./...` skips testdata dirs):
//   cue vet ./testdata/   (expect errors)

package mcpserver

// import "github.com/geoffmilleraz/agentic-kit/kits/mcp-server:mcpserver"

// // invalidConfig_EmptyName — Name must not be empty
// invalidConfig_EmptyName: mcpserver.#Config & {
// 	Name:    ""
// 	Version: "1.0.0"
// }

// // invalidConfig_EmptyVersion — Version must not be empty
// invalidConfig_EmptyVersion: mcpserver.#Config & {
// 	Name:    "demo"
// 	Version: ""
// }

// // invalidConfig_ZeroMaxMessageBytes — omit the field instead of setting 0
// invalidConfig_ZeroMaxMessageBytes: mcpserver.#Config & {
// 	Name:            "demo"
// 	Version:         "1.0.0"
// 	MaxMessageBytes: 0
// }

// // invalidConfig_NegativeMaxMessageBytes — must be >= 1 when present
// invalidConfig_NegativeMaxMessageBytes: mcpserver.#Config & {
// 	Name:            "demo"
// 	Version:         "1.0.0"
// 	MaxMessageBytes: -1
// }

// // invalidConfig_UnknownField — #Config is closed; CallLog is not data
// invalidConfig_UnknownField: mcpserver.#Config & {
// 	Name:    "demo"
// 	Version: "1.0.0"
// 	CallLog: "audit.jsonl"
// }

// // invalidHTTPOptions_ZeroMaxBodyBytes — omit the field instead of 0
// invalidHTTPOptions_ZeroMaxBodyBytes: mcpserver.#HTTPOptions & {
// 	MaxBodyBytes: 0
// }

// // invalidHTTPOptions_EmptyOrigin — origin entries must be non-empty
// invalidHTTPOptions_EmptyOrigin: mcpserver.#HTTPOptions & {
// 	AllowedOrigins: [""]
// }

// // invalidHTTPOptions_UnknownField — #HTTPOptions is closed; RandHex is
// // a Go-side func seam, not data
// invalidHTTPOptions_UnknownField: mcpserver.#HTTPOptions & {
// 	RandHex: "seed"
// }
