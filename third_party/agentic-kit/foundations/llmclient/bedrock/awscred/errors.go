package awscred

import "fmt"

// ExpiredSSOTokenError reports that the cached SSO access token for a
// profile is absent or expired. Error names the exact fix so a caller
// (or a human reading a surfaced log line) knows what to run, rather than
// having to reverse-engineer "not found" into "log in again."
type ExpiredSSOTokenError struct {
	// Profile is the shared-config profile name whose SSO session needs
	// re-authenticating.
	Profile string
}

// Error implements error. The message always names the fix:
// `aws sso login --profile <name>`.
func (e *ExpiredSSOTokenError) Error() string {
	return fmt.Sprintf("awscred: SSO token for profile %q is missing or expired: run `aws sso login --profile %s`", e.Profile, e.Profile)
}

// CredentialProcessError reports that a profile's credential_process command
// failed to run or produced output that does not satisfy the
// credential_process JSON contract. Stderr, when the process wrote any, is
// carried so the underlying tool's own diagnostic reaches the operator
// instead of being swallowed — but bounded by runCredentialProcess at
// maxCredentialProcessStderr bytes: anything past the cap is dropped at
// capture time (never buffered) and the message notes the truncation, so a
// misbehaving tool that echoes its own credentials deep into a stderr trace
// (set -x) cannot leak them into error messages and thence logs.
type CredentialProcessError struct {
	// Command is the raw credential_process command line that failed.
	Command string

	// Err is the underlying cause (a *exec.ExitError, a JSON parse error, or
	// a contract-violation error).
	Err error

	// Stderr is the process's captured standard error, trimmed and capped at
	// maxCredentialProcessStderr bytes (a "[stderr truncated ...]" marker is
	// appended when the cap was hit). Empty when the process wrote nothing
	// to stderr.
	Stderr string
}

// Error implements error.
func (e *CredentialProcessError) Error() string {
	msg := fmt.Sprintf("awscred: credential_process %q failed: %v", e.Command, e.Err)
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

// Unwrap returns the underlying cause so errors.Is/errors.As see through
// CredentialProcessError to e.Err.
func (e *CredentialProcessError) Unwrap() error {
	return e.Err
}

// PartialCredentialsError reports that exactly one half of an access-key
// pair was supplied on a leg that requires both (explicit Config fields, or
// the AWS_* environment variables). A partial pair is a configuration error,
// never a fall-through: silently completing it from — or abandoning it for —
// ambient state (the other leg, the shared files, IMDS) would sign requests
// as a different principal than the caller named, which is exactly the
// misattribution the AWS SDKs also hard-error on.
type PartialCredentialsError struct {
	// Source is the leg the partial pair was found on: SourceExplicit or
	// SourceEnv.
	Source Source

	// Present names the half that was supplied, e.g.
	// "access key (Config.AccessKeyID)".
	Present string

	// Missing names the half that was not, e.g.
	// "secret access key (AWS_SECRET_ACCESS_KEY)".
	Missing string
}

// Error implements error.
func (e *PartialCredentialsError) Error() string {
	return fmt.Sprintf("awscred: partial %s credentials: %s is set but %s is missing — supply both halves; a partial pair never falls through to ambient credentials", e.Source, e.Present, e.Missing)
}

// SSOPortalError reports that the SSO portal's GetRoleCredentials call
// failed in a way that is NOT the expired/absent-token case (that case is
// *ExpiredSSOTokenError, with its `aws sso login` guidance): a non-2xx
// status other than 401/403, or a 200 whose roleCredentials payload carries
// no usable credential.
type SSOPortalError struct {
	// Profile is the shared-config profile name being resolved.
	Profile string

	// Reason describes the failure (an HTTP status + body snippet, or the
	// payload-shape problem).
	Reason string
}

// Error implements error.
func (e *SSOPortalError) Error() string {
	return fmt.Sprintf("awscred: SSO GetRoleCredentials for profile %q: %s", e.Profile, e.Reason)
}
