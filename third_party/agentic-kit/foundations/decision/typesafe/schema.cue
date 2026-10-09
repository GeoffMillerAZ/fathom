// Package typesafe defines the CUE schema for the TypeSafe decision
// adapter's configuration. These constraints mirror typesafe.go
// (TypeSafeConfig and the defaults NewTypeSafe applies) and are evaluated
// with `cue vet ./...`. Only data-shaped fields appear here:
// TypeSafeConfig.HTTPClient and TypeSafeConfig.Now are Go-only injection
// points with no data representation. The Request and Response the adapter
// speaks are the parent foundations/decision schema's; its wire body is
// pinned by testdata/typesafe_request.golden.
package typesafe

// #TypeSafeConfig is the data shape of typesafe.TypeSafeConfig. Defaults
// follow the transport and are applied by NewTypeSafe: "typesafe" uses
// https://api.typesafe.ai, /v1/systemone and jev-latest; "openrouter" uses
// https://openrouter.ai, /api/alpha/decisions and typesafe/jev-1.13.
#TypeSafeConfig: {
	// APIKey is the key itself, for a caller that sets it in code.
	// Configuration names APIKeyFile or APIKeyEnv instead, never the key;
	// NewTypeSafe errors when none of the three is set (a rule Go
	// enforces). Fixtures must NEVER contain a real key.
	APIKey?: string & !=""

	// APIKeyFile is the path of a file holding the key ("~" expands to the
	// home directory); the file must be chmod 600. It wins over APIKeyEnv.
	APIKeyFile?: string & !=""

	// APIKeyEnv is the name of an environment variable holding the key.
	APIKeyEnv?: string & =~"^[A-Za-z_][A-Za-z0-9_]*$"

	// Transport selects where the System One wire is sent.
	Transport: *"typesafe" | "openrouter"

	// BaseURL overrides the transport's origin; requests go to BaseURL+Path
	// (a trailing slash is trimmed).
	BaseURL?: string & =~"^https?://"

	// Path overrides the transport's request path.
	Path?: string & =~"^/"

	// Model overrides the transport's model; each Response names the
	// version that ran.
	Model?: string & !=""
}
