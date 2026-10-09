// Package openai defines the CUE schema for the OpenAI Decisions API
// adapter's configuration. These constraints mirror openai.go (OpenAIConfig
// and the defaults NewOpenAI applies) and are evaluated with
// `cue vet ./...`. Only data-shaped fields appear here: OpenAIConfig's
// HTTPClient and Now are Go-only injection points with no data
// representation. The Request and Response the adapter speaks are the
// parent foundations/decision schema's; its wire body is pinned by
// testdata/openai_request.golden and testdata/openai_request_images.golden.
package openai

// #OpenAIConfig is the data shape of openai.OpenAIConfig. Defaults match
// NewOpenAI.
#OpenAIConfig: {
	// APIKey is the key itself, for a caller that sets it in code.
	// Configuration names APIKeyFile or APIKeyEnv instead, never the key;
	// NewOpenAI errors when none of the three is set (a rule Go enforces).
	// Fixtures must NEVER contain a real key.
	APIKey?: string & !=""

	// APIKeyFile is the path of a file holding the key ("~" expands to the
	// home directory); the file must be chmod 600. It wins over APIKeyEnv.
	APIKeyFile?: string & !=""

	// APIKeyEnv is the name of an environment variable holding the key.
	APIKeyEnv?: string & =~"^[A-Za-z_][A-Za-z0-9_]*$"

	// BaseURL is the API base, version included; requests go to
	// BaseURL+"/decisions" (a trailing slash is trimmed).
	BaseURL: *"https://api.openai.com/v1" | (string & =~"^https?://")

	// Model is the Decisions model; the API supports one.
	Model: *"gpt-6-luna" | (string & !="")
}
