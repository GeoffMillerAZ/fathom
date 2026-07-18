package awscred

import "time"

// Environment variable names consulted by the resolution chain. These are
// the conventional AWS CLI/SDK names so this resolver interoperates with any
// existing AWS setup on the host.
const (
	envAccessKeyID           = "AWS_ACCESS_KEY_ID"
	envSecretAccessKey       = "AWS_SECRET_ACCESS_KEY"
	envSessionToken          = "AWS_SESSION_TOKEN"
	envRegion                = "AWS_REGION"
	envDefaultRegion         = "AWS_DEFAULT_REGION"
	envProfile               = "AWS_PROFILE"
	envConfigFile            = "AWS_CONFIG_FILE"
	envSharedCredentialsFile = "AWS_SHARED_CREDENTIALS_FILE"
	envIMDSDisabled          = "AWS_EC2_METADATA_DISABLED"
)

// Config configures Resolve. Every field is optional; the zero Config walks
// the full default chain using the real environment, the real home
// directory, the real AWS endpoints, and time.Now.
type Config struct {
	// AccessKeyID, SecretAccessKey, and SessionToken are explicit credentials
	// supplied directly by the caller. When AccessKeyID and SecretAccessKey
	// are both non-empty they win over every other leg (highest priority),
	// exactly like the AWS default chain's first step.
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string

	// Region is an explicit region override. It wins over every other region
	// source (AWS_REGION, AWS_DEFAULT_REGION, the shared profile's "region"
	// key) — see resolveRegion.
	Region string

	// Now overrides the clock used to judge SSO cached-token and role-
	// credential expiry. Nil uses time.Now. Tests pin this to make expiry
	// decisions deterministic.
	Now func() time.Time

	// SSOPortalBaseURL overrides the SSO portal origin normally derived as
	// "https://portal.sso.<region>.amazonaws.com". Set this to an
	// httptest.Server URL in tests; production callers leave it empty.
	SSOPortalBaseURL string

	// IMDSBaseURL overrides the IMDSv2 origin normally
	// "http://169.254.169.254". Set this to an httptest.Server URL in tests;
	// production callers leave it empty.
	IMDSBaseURL string
}

// clock returns cfg.Now, or time.Now when it is nil.
func (cfg Config) clock() func() time.Time {
	if cfg.Now != nil {
		return cfg.Now
	}
	return time.Now
}

// firstNonEmpty returns the first non-empty argument, or "".
func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}
