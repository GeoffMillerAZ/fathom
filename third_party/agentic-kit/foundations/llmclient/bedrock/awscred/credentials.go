package awscred

import "time"

// Source labels which leg of the resolution chain produced a Credentials.
type Source string

const (
	// SourceExplicit means both AccessKeyID and SecretAccessKey came from
	// Config fields.
	SourceExplicit Source = "explicit"

	// SourceEnv means the credential came from AWS_ACCESS_KEY_ID /
	// AWS_SECRET_ACCESS_KEY (possibly mixed with an explicit field for the
	// other half — Resolve fills each of AccessKeyID/SecretAccessKey/
	// SessionToken independently from Config-or-env, matching the AWS
	// default chain's own per-field precedence).
	SourceEnv Source = "env"

	// SourceSharedStatic means the credential came from a static
	// aws_access_key_id/aws_secret_access_key pair in the shared config or
	// credentials file.
	SourceSharedStatic Source = "shared-static"

	// SourceCredentialProcess means the credential came from executing the
	// shared profile's credential_process command.
	SourceCredentialProcess Source = "credential-process"

	// SourceSSO means the credential came from exchanging a cached SSO
	// access token for role credentials (GetRoleCredentials).
	SourceSSO Source = "sso"

	// SourceIMDS means the credential came from the EC2 instance metadata
	// service (IMDSv2).
	SourceIMDS Source = "imds"
)

// Credentials is a resolved AWS credential triple plus its provenance,
// resolved region, and expiry.
type Credentials struct {
	// AccessKeyID is the AWS access-key id.
	AccessKeyID string

	// SecretAccessKey is the AWS secret access key.
	SecretAccessKey string

	// SessionToken is the AWS session token for temporary credentials. Empty
	// for long-lived (static) keys.
	SessionToken string

	// Source reports which leg of the chain produced this Credentials.
	Source Source

	// Region is the resolved AWS region (see resolveRegion) — independent of
	// which leg produced the credentials.
	Region string

	// Expiry is when these credentials stop being valid. Zero means static —
	// no known expiry (explicit, env, and shared-static credentials are
	// always static; credential_process, SSO, and IMDS credentials carry a
	// real Expiry).
	Expiry time.Time
}
