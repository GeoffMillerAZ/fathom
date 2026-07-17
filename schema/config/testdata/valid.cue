package config

// A well-formed config carrying one profile of each kind that needs
// non-trivial fields, plus a daemon table — what an operator's
// ~/.fathom/config.toml converts to after TOML→CUE.
example: #Config & {
	default_profile: "work"
	profiles: {
		work: {
			provider: "anthropic"
			model:    "claude-opus-4-1-20250805"
		}
		subscription: {
			provider: "claude-cli"
			bin:      "/opt/homebrew/bin/claude"
		}
		vault: {
			provider:    "openai"
			api_key_cmd: ["op", "read", "op://vault/openai/credential"]
		}
		"bedrock-govcloud": {
			provider: "bedrock"
			region:   "us-gov-west-1"
			model:    "anthropic.claude-3-5-sonnet-20241022-v2:0"
		}
		"bedrock-fips": {
			provider: "bedrock"
			region:   "us-gov-west-1"
			endpoint: "https://bedrock-runtime-fips.us-gov-west-1.amazonaws.com"
			model:    "anthropic.claude-3-5-sonnet-20241022-v2:0"
		}
		"azure-gov": {
			provider:     "azure-openai"
			endpoint:     "https://example-resource.openai.azure.us"
			deployment:   "gov-deployment"
			api_version:  "2024-06-01"
			api_key_env:  "AZURE_OPENAI_API_KEY"
		}
	}
	daemon: {
		store:           "workspace/db/fleet.duckdb"
		addr:            "127.0.0.1:7575"
		auto_refresh:    true
		default_profile: "work"
	}
}
