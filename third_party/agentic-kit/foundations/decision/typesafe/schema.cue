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
// match NewTypeSafe.
#TypeSafeConfig: {
	// APIKey is the System One API key, sent as a bearer token. Required:
	// NewTypeSafe errors when it is empty. Fixtures must NEVER contain a
	// real key.
	APIKey: string & !=""

	// BaseURL is the API origin; requests go to BaseURL+"/v1/systemone"
	// (a trailing slash is trimmed).
	BaseURL: *"https://api.typesafe.ai" | (string & =~"^https?://")

	// Model is the System One model; the default is the vendor's rolling
	// alias for its latest model, and each Response names the version that
	// ran.
	Model: *"jev-latest" | (string & !="")
}
