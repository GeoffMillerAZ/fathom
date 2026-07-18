// Package untrustedcontentwrapper defines CUE contracts for untrusted-content
// fencing configuration and the per-wrap artifact. Fences embed a per-wrap
// random nonce, so the config carries fence *tags*, not full delimiter
// strings; the rendered fence is <<<Tag:nonce>>>.
package untrustedcontentwrapper

// #WrapConfig defines how untrusted content is fenced.
#WrapConfig: {
	// OpenTag names the opening fence, rendered as <<<OpenTag:nonce>>>.
	// Must not contain '<', '>', ':', or line breaks.
	OpenTag: string & !="" & !~"[<>:\n\r]"

	// CloseTag names the closing fence, rendered as <<<CloseTag:nonce>>>.
	CloseTag: string & !="" & !~"[<>:\n\r]"

	// Label is an optional tag for the kind of untrusted source.
	// May be empty; must not contain line breaks.
	Label: string & !~"[\n\r]"

	// NonceBytes is the number of random bytes drawn per wrap
	// (hex-encoded to 2*NonceBytes characters). 0 selects the Go-side
	// default of 16; otherwise must be in [8, 64].
	NonceBytes: int & (0 | >=8 & <=64) | *16
}

// #Wrapped is the artifact produced by a single Wrap call. The fences are
// per-wrap values carrying the nonce; unwrapping requires this artifact.
#Wrapped: {
	// Text is the full fenced text presented to the model.
	Text: string & !=""

	// Nonce is the lowercase hex nonce embedded in both fences
	// (16..128 hex characters, i.e. 8..64 bytes).
	Nonce: string & =~"^[0-9a-f]{16,128}$"

	// OpenFence is the exact opening fence line, including the nonce.
	OpenFence: string & !=""

	// CloseFence is the exact closing fence line, including the nonce.
	CloseFence: string & !=""

	// Label is the label rendered after the opening fence, if any.
	Label: string & !~"[\n\r]"
}
