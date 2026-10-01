// valid.cue — concrete values that satisfy the constraints defined in
// schema.cue. Running `cue vet ./testdata/` from the package directory
// validates these against the schema. Keys are placeholders only; NEVER a
// real key.
package typesafe

// _minimal sets only the required key and takes every default.
_minimal: #TypeSafeConfig & {
	APIKey: "test-placeholder-key-not-real"
}

// _local points the adapter at a local test server and pins a model.
_local: #TypeSafeConfig & {
	APIKey:  "test-placeholder-key-not-real"
	BaseURL: "http://127.0.0.1:8080"
	Model:   "model-example"
}
