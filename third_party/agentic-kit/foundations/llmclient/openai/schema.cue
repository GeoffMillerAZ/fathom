// Package openai defines the CUE schema for the OpenAI Chat Completions
// provider client configuration. These constraints mirror config.go (Config
// and the defaults applied by New) and are evaluated with `cue vet ./...`.
// Only data-shaped fields appear here — Config.HTTPClient, Config.Now, and
// Config.CapabilityCheck are Go-only injection points with no data representation.
package openai

// #Config is the data shape of openai.Config. Defaults match New.
#Config: {
	UseResponses: *false | bool
	ProviderFamily?: string
	if UseResponses { AzureDeployment?: _|_ }
	// BaseURL is the API base URL including path prefix, e.g.
	// "https://api.openai.com/v1" (default) or "http://localhost:11434/v1"
	// (Ollama) or "http://localhost:8000/v1" (vLLM). The vanilla default is
	// applied only when AzureDeployment is absent: in Azure mode there is no
	// sensible default (the resource endpoint is caller-specific), and
	// letting the OpenAI public URL fill in would export a config New's own
	// empty-BaseURL rejection exists to prevent — a silent misroute to
	// api.openai.com/openai/deployments/... instead of a loud error.
	BaseURL: string & =~"^https?://"
	if AzureDeployment == _|_ {
		BaseURL: *"https://api.openai.com/v1" | string
	}

	// APIKey is the OpenAI API key sent as "Authorization: Bearer <key>".
	// Optional for local endpoints (Ollama, vLLM) that do not require auth.
	// Fixtures must NEVER contain a real key.
	APIKey?: string

	// DefaultModel is used when Request.Model is empty. Optional: a client may
	// rely on per-request models, but a call with neither fails before HTTP.
	DefaultModel?: string & !=""

	// DefaultMaxTokens is used when Request.MaxTokens is zero. Must be positive.
	DefaultMaxTokens: *4096 | (int & >0)

	// AzureDeployment, when set, switches this Config into Azure OpenAI
	// mode — see the Go doc comment (doc.go, "Azure OpenAI" section) for
	// the full endpoint/auth shape change. Optional; absent means vanilla
	// OpenAI / OpenAI-compatible mode.
	AzureDeployment?: string & !=""

	// AzureAPIVersion is the api-version query parameter sent on every
	// Azure request. Only meaningful when AzureDeployment is set; absent
	// defaults to the package's pinned GA version (DefaultAzureAPIVersion).
	AzureAPIVersion?: string & !=""

	// AzureAuthScheme selects the Azure credential header: "api-key" (the
	// default) or "bearer" for Entra-ID token passthrough. Ignored outside
	// Azure mode.
	AzureAuthScheme?: *"api-key" | "bearer"
}
