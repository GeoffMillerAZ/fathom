// valid.cue — concrete values that satisfy the constraints defined in
// schema.cue. Running `cue vet ./testdata/` from the package directory
// validates these against the schema. Configuration names a key file or a
// variable; it never holds a key.
package openai

// fromEnv reads the key from a variable and takes every default.
fromEnv: #OpenAIConfig & {
	APIKeyEnv: "OPENAI_API_KEY"
}

// fromFile reads the key from a chmod 600 file and points at a proxy.
fromFile: #OpenAIConfig & {
	APIKeyFile: "~/secrets/<provider>/token"
	BaseURL:    "https://proxy.example/v1"
}
