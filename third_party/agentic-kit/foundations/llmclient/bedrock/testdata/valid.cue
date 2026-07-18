// Package bedrock — valid test data that must unify with schema.cue.
// Credentials in fixtures are placeholders only; NEVER a real key or secret.
package bedrock

import "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/bedrock"

// validMinimal relies on defaults; region and credentials come from the AWS_*
// environment variables at construction time, the model from per-request.
validMinimal: bedrock.#Config & {}

// validGovCloud pins a GovCloud region and a default model, resolving
// credentials from the environment.
validGovCloud: bedrock.#Config & {
	Region:       "us-gov-west-1"
	DefaultModel: "anthropic.claude-3-5-sonnet-20241022-v2:0"
}

// validExplicit pins every data-shaped field, including temporary credentials
// (session token) and a FIPS GovCloud endpoint.
validExplicit: bedrock.#Config & {
	Region:           "us-gov-west-1"
	AccessKeyID:      "AKIDPLACEHOLDERNOTREAL"
	SecretAccessKey:  "test-placeholder-secret-not-real"
	SessionToken:     "test-placeholder-session-token-not-real"
	BaseURL:          "https://bedrock-runtime-fips.us-gov-west-1.amazonaws.com"
	DefaultModel:     "anthropic.claude-3-5-sonnet-20241022-v2:0"
	DefaultMaxTokens: 8192
}

// validInferenceProfile uses a cross-region inference-profile id as the model.
validInferenceProfile: bedrock.#Config & {
	Region:       "us-gov-west-1"
	DefaultModel: "us-gov.anthropic.claude-3-5-sonnet-20241022-v2:0"
}
