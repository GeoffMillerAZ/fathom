package llmclient

// #ProviderCapabilities records guaranteed adapter mappings. False includes
// unknown; it is not a model availability or host policy claim.
#ProviderCapabilities: {
	Tools: *false | bool
	StructuredOutput: *false | bool
	Streaming: *false | bool
	Usage: *false | bool
	Refusal: *false | bool
	Truncation: *false | bool
	Cancellation: *false | bool
}

// #ProviderInfo mirrors optional request-scoped provider metadata. Empty means
// unknown and cannot establish independent evaluation.
#ProviderInfo: {
	Provider: *"" | string
	Family: *"" | string
	Model: *"" | string
	Capabilities: #ProviderCapabilities
}
