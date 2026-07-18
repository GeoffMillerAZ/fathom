package llmclient

import (
	"errors"
	"fmt"
	"time"
)

// RateLimitError signals that the API has throttled the request.
// WithRetry treats this as retryable.
type RateLimitError struct {
	// RetryAfter is the server's wait hint, or zero if unknown.
	// When set, WithRetry sleeps exactly this long before the next attempt
	// instead of the computed exponential backoff.
	//
	// Breaking change from v1: this field was previously an int number of
	// seconds; it is now a time.Duration.
	RetryAfter time.Duration

	// Message is the raw message from the API.
	Message string
}

// Error implements the error interface, reporting the API's rate-limit
// message and, when known, the retry-after hint.
func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("rate limit exceeded: %s (retry after %s)", e.Message, e.RetryAfter)
	}
	return fmt.Sprintf("rate limit exceeded: %s", e.Message)
}

// ServerError signals a transient 5xx-class error from the API backend.
// WithRetry treats this as retryable.
type ServerError struct {
	StatusCode int
	Message    string
}

// Error implements the error interface, reporting the backend's HTTP status
// code and message.
func (e *ServerError) Error() string {
	return fmt.Sprintf("server error %d: %s", e.StatusCode, e.Message)
}

// InvalidRequestError signals that the request itself is malformed (e.g. prompt
// too long, unknown model, bad parameters). WithRetry treats this as
// non-retryable — retrying the identical request cannot succeed.
type InvalidRequestError struct {
	Message string
}

// Error implements the error interface, reporting why the request was
// malformed.
func (e *InvalidRequestError) Error() string {
	return fmt.Sprintf("invalid request: %s", e.Message)
}

// AuthError signals an authentication or authorisation failure.
// WithRetry treats this as non-retryable.
type AuthError struct {
	Message string
}

// Error implements the error interface, reporting the authentication or
// authorisation failure message.
func (e *AuthError) Error() string {
	return fmt.Sprintf("auth error: %s", e.Message)
}

// UnavailableError signals that no underlying client is configured: a
// late-bound Swappable has not (yet) been wired with a real provider client,
// typically because credentials arrive at runtime (e.g. a Settings page) and
// the user has not entered them. WithRetry treats this as NON-retryable —
// retrying cannot conjure a credential. Callers should surface a "configure
// your API key" prompt, never a retry loop.
type UnavailableError struct {
	Message string
}

// Error implements the error interface, reporting that no underlying
// provider client is configured.
func (e *UnavailableError) Error() string {
	return fmt.Sprintf("llm client unavailable: %s", e.Message)
}

// isRetryable returns true for errors that are transient and worth retrying.
// Wrapped errors are unwrapped via errors.As.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	var rle *RateLimitError
	var sve *ServerError
	return errors.As(err, &rle) || errors.As(err, &sve)
}

// retryAfterHint extracts the RateLimitError.RetryAfter hint from err, or zero
// when err is not a rate-limit error or carries no hint.
func retryAfterHint(err error) time.Duration {
	var rle *RateLimitError
	if errors.As(err, &rle) && rle.RetryAfter > 0 {
		return rle.RetryAfter
	}
	return 0
}
