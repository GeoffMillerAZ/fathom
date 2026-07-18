// Package bedrock defines the CUE schema for the AWS Bedrock (Claude /
// Anthropic-on-Bedrock) provider client configuration. These constraints
// mirror config.go (Config and the defaults applied by New) and are evaluated
// with `cue vet ./...`. Only data-shaped fields appear here — Config.HTTPClient
// and Config.Now are Go-only injection points with no data representation.
package bedrock

// #Config is the data shape of bedrock.Config. Defaults match New.
#Config: {
	// Region is the AWS region, e.g. "us-gov-west-1" (GovCloud) or
	// "us-east-1". Required (no default): GovCloud and commercial endpoints
	// are distinct and must be chosen explicitly. When omitted in Go it is
	// read from AWS_REGION, but a fixture must name it.
	Region?: string & !=""

	// AccessKeyID is the AWS access-key id. Optional: when omitted or empty it
	// is resolved at construction from AWS_ACCESS_KEY_ID. Fixtures must NEVER
	// contain a real key.
	AccessKeyID?: string

	// SecretAccessKey is the AWS secret access key, used only to derive the
	// SigV4 signing key (never sent on the wire). Optional: resolved from
	// AWS_SECRET_ACCESS_KEY when empty. Fixtures must NEVER contain a real
	// secret.
	SecretAccessKey?: string

	// SessionToken is the AWS session token for temporary credentials. Empty
	// for long-lived keys; resolved from AWS_SESSION_TOKEN when empty.
	SessionToken?: string

	// BaseURL overrides the endpoint origin (scheme + host, no trailing path).
	// Empty derives "https://bedrock-runtime.{Region}.amazonaws.com". Set for a
	// GovCloud FIPS or VPC endpoint.
	BaseURL?: string & =~"^https?://"

	// DefaultModel is the Bedrock model id used when Request.Model is empty,
	// e.g. "anthropic.claude-3-5-sonnet-20241022-v2:0" or an inference-profile
	// id. Optional: a client may rely on per-request models, but a call with
	// neither fails before HTTP.
	DefaultModel?: string & !=""

	// DefaultMaxTokens is used when Request.MaxTokens is zero. The
	// Anthropic-on-Bedrock body requires max_tokens, so it must be positive.
	DefaultMaxTokens: *4096 | (int & >0)
}
