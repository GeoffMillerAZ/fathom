package httpembedder

import "errors"

// Typed error mapping for the OpenAI-compatible wire protocol. Every error
// returned from an HTTP exchange wraps exactly one of these sentinels, so
// callers classify failures with errors.Is and decide retry policy without
// parsing strings:
//
//	HTTP 429            → ErrRateLimited       (retryable, back off)
//	HTTP 401, 403       → ErrAuth              (not retryable)
//	HTTP 5xx            → ErrServer            (retryable)
//	other HTTP 4xx      → ErrInvalidRequest    (not retryable)
//	undecodable or      → ErrMalformedResponse (not retryable; the backend
//	inconsistent body                            does not speak the protocol)
//
// Error strings never contain the API key or the Authorization header: the
// key is sent only as a header, and server-echoed message bodies are passed
// through redact before being embedded in an error.
var (
	// ErrRateLimited reports an HTTP 429 from the backend.
	ErrRateLimited = errors.New("httpembedder: rate limited")

	// ErrAuth reports an HTTP 401 or 403 from the backend.
	ErrAuth = errors.New("httpembedder: authentication failed")

	// ErrServer reports an HTTP 5xx from the backend.
	ErrServer = errors.New("httpembedder: server error")

	// ErrInvalidRequest reports any other HTTP 4xx from the backend.
	ErrInvalidRequest = errors.New("httpembedder: invalid request")

	// ErrMalformedResponse reports a 200 response whose body could not be
	// decoded or is inconsistent with the request (wrong count, empty or
	// missing embeddings).
	ErrMalformedResponse = errors.New("httpembedder: malformed response")
)

// classify maps an HTTP status code to its sentinel error.
func classify(status int) error {
	switch {
	case status == 429:
		return ErrRateLimited
	case status == 401 || status == 403:
		return ErrAuth
	case status >= 500:
		return ErrServer
	default:
		return ErrInvalidRequest
	}
}
