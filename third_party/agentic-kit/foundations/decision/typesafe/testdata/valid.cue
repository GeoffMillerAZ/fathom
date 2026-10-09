// valid.cue — concrete values that satisfy the constraints defined in
// schema.cue. Running `cue vet ./testdata/` from the package directory
// validates these against the schema. Keys are placeholders only; NEVER a
// real key, and configuration names a key file or variable, not a key.
package typesafe

// minimal names a key file and takes every default.
minimal: #TypeSafeConfig & {
	APIKeyFile: "~/secrets/<provider>/token"
}

// local points the adapter at a local test server and pins a model, with
// the key in an environment variable.
local: #TypeSafeConfig & {
	APIKeyEnv: "TYPESAFE_API_KEY"
	BaseURL:   "http://127.0.0.1:8080"
	Model:     "model-example"
}

// openRouter sends the same wire through OpenRouter's Decisions router.
openRouter: #TypeSafeConfig & {
	APIKeyFile: "~/secrets/<provider>/token"
	Transport:  "openrouter"
}

// inCode is the shape of a config whose key a caller set in code.
inCode: #TypeSafeConfig & {
	APIKey: "test-placeholder-key-not-real"
}
