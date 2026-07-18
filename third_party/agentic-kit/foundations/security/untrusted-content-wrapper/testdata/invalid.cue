// invalid.cue — examples that do NOT satisfy #WrapConfig or #Wrapped.
// All examples are commented out. Uncomment any block to observe the cue vet error.
package untrustedcontentwrapper

// EmptyOpenTag: a config with an empty OpenTag.
// cue vet error: OpenTag: invalid value "" (does not satisfy != "")
//
// EmptyOpen: {
// 	OpenTag:    ""  // ERROR: must be non-empty
// 	CloseTag:   "UNTRUSTED_CONTENT_END"
// 	Label:      ""
// 	NonceBytes: 16
// }

// ColonInTag: tags must not contain ':' — it is the fence's nonce separator.
// cue vet error: OpenTag: invalid value "BEGIN:HERE" (does not match !~"[<>:\n\r]")
//
// ColonTag: {
// 	OpenTag:    "BEGIN:HERE"  // ERROR: ':' is reserved for the nonce separator
// 	CloseTag:   "UNTRUSTED_CONTENT_END"
// 	Label:      ""
// 	NonceBytes: 16
// }

// AngleBracketTag: tags must not contain '<' or '>'.
// cue vet error: CloseTag: invalid value "<END>" (does not match !~"[<>:\n\r]")
//
// AngleTag: {
// 	OpenTag:    "UNTRUSTED_CONTENT_BEGIN"
// 	CloseTag:   "<END>"  // ERROR: angle brackets are fence syntax
// 	Label:      ""
// 	NonceBytes: 16
// }

// TinyNonce: NonceBytes below the minimum of 8.
// cue vet error: NonceBytes: invalid value 4 (out of bound)
//
// Tiny: {
// 	OpenTag:    "UNTRUSTED_CONTENT_BEGIN"
// 	CloseTag:   "UNTRUSTED_CONTENT_END"
// 	Label:      ""
// 	NonceBytes: 4  // ERROR: must be 0 or in [8, 64]
// }

// UppercaseNonce: #Wrapped nonces must be lowercase hex.
// cue vet error: Nonce: invalid value "9F86D081884C7D65" (does not match =~"^[0-9a-f]{16,128}$")
//
// BadWrapped: {
// 	Text:       "<<<A:9f>>>\nx\n<<<B:9f>>>"
// 	Nonce:      "9F86D081884C7D65"  // ERROR: must be lowercase hex, >= 16 chars
// 	OpenFence:  "<<<A:9f>>>"
// 	CloseFence: "<<<B:9f>>>"
// 	Label:      ""
// }
