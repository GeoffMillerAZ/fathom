// valid.cue — example values satisfying #WrapConfig and #Wrapped.
package untrustedcontentwrapper

import "github.com/geoffmilleraz/agentic-kit/foundations/security/untrusted-content-wrapper:untrustedcontentwrapper"

DefaultConfig: untrustedcontentwrapper.#WrapConfig & {
	OpenTag:    "UNTRUSTED_CONTENT_BEGIN"
	CloseTag:   "UNTRUSTED_CONTENT_END"
	Label:      ""
	NonceBytes: 16
}

LabelledConfig: untrustedcontentwrapper.#WrapConfig & {
	OpenTag:    "UNTRUSTED_CONTENT_BEGIN"
	CloseTag:   "UNTRUSTED_CONTENT_END"
	Label:      "USER_DOCUMENT"
	NonceBytes: 16
}

CustomConfig: untrustedcontentwrapper.#WrapConfig & {
	OpenTag:    "EXTERNAL_DATA_BEGIN"
	CloseTag:   "EXTERNAL_DATA_END"
	Label:      "TOOL_OUTPUT"
	NonceBytes: 32
}

// DefaultedNonce relies on the schema default of 16.
DefaultedNonce: untrustedcontentwrapper.#WrapConfig & {
	OpenTag:  "UNTRUSTED_CONTENT_BEGIN"
	CloseTag: "UNTRUSTED_CONTENT_END"
	Label:    ""
}

// ExampleWrapped mirrors the artifact a Go-side Wrap call produces.
ExampleWrapped: untrustedcontentwrapper.#Wrapped & {
	Text: """
		<<<UNTRUSTED_CONTENT_BEGIN:9f86d081884c7d659a2feaa0c55ad015>>>
		[USER_DOCUMENT]
		external document text
		<<<UNTRUSTED_CONTENT_END:9f86d081884c7d659a2feaa0c55ad015>>>
		"""
	Nonce:      "9f86d081884c7d659a2feaa0c55ad015"
	OpenFence:  "<<<UNTRUSTED_CONTENT_BEGIN:9f86d081884c7d659a2feaa0c55ad015>>>"
	CloseFence: "<<<UNTRUSTED_CONTENT_END:9f86d081884c7d659a2feaa0c55ad015>>>"
	Label:      "USER_DOCUMENT"
}
