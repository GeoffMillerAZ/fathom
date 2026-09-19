package llmclient

import (
	"reflect"
	"strings"
)

// ProviderCapabilities describes adapter support, not model availability or host
// governance. True is a supported mapping; false means unsupported or unknown.
// Model-specific constraints still belong in the adapter's CapabilityCheck.
type ProviderCapabilities struct {
	// Tools reports support for caller-defined function calls and results.
	Tools bool
	// StructuredOutput reports support for Request.OutputSchema.
	StructuredOutput bool
	// Streaming reports a native provider stream (not simulated text chunks).
	Streaming bool
	// Usage reports provider token accounting on successful completions.
	Usage bool
	// Refusal reports explicit provider refusals as StopRefusal.
	Refusal bool
	// Truncation reports provider output limits as StopMaxTokens.
	Truncation bool
	// Cancellation reports context-aware cancellation of provider work.
	Cancellation bool
}

// ProviderInfo describes the route configured for a request. Empty fields mean
// unknown. Family is the model's provenance family, not its transport vendor;
// different model names or API lanes do not establish independent families.
// Metadata is descriptive evidence supplied by an adapter/configuration, never
// authentication, a promise of semantic independence, or a live capability probe.
type ProviderInfo struct {
	// Provider identifies the configured provider adapter.
	Provider string
	// Family identifies model provenance; empty must not certify independence.
	Family string
	// Model is the requested model after applying the adapter's default.
	Model string
	// Capabilities records supported adapter mappings.
	Capabilities ProviderCapabilities
}

// ProviderDescriber is optional; existing LLMClient implementations need not
// implement it. Implementations must be safe for concurrent use.
type ProviderDescriber interface {
	// DescribeProvider describes the route for req without performing work.
	DescribeProvider(req Request) ProviderInfo
}

// DescribeProvider returns optional adapter metadata, or unknown for a legacy
// client. Mutable/multiple-provider routes should leave identity unknown.
func DescribeProvider(client LLMClient, req Request) ProviderInfo {
	if client == nil {
		return ProviderInfo{}
	}
	v := reflect.ValueOf(client)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Func, reflect.Map, reflect.Slice, reflect.Chan:
		if v.IsNil() {
			return ProviderInfo{}
		}
	}
	if d, ok := client.(ProviderDescriber); ok {
		return d.DescribeProvider(req)
	}
	return ProviderInfo{}
}

// IndependentProviders reports distinct, known provenance families. It rejects
// blank/unknown families and is only a metadata check, not a quality evaluation.
func IndependentProviders(a, b ProviderInfo) bool {
	x, y := strings.ToLower(strings.TrimSpace(a.Family)), strings.ToLower(strings.TrimSpace(b.Family))
	return x != "" && y != "" && x != "unknown" && y != "unknown" && x != y
}

// DescribeProvider preserves the identity of the single retry target.
func (r *retryClient) DescribeProvider(req Request) ProviderInfo {
	return DescribeProvider(r.inner, req)
}
