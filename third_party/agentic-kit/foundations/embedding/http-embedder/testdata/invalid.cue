// Package httpembedder — invalid examples.
// ALL blocks below are commented out so that `cue vet ./...` passes.
// Uncomment individual blocks to observe the validation error described.
// The Go constructor (New) rejects the same shapes at runtime; see
// TestNew_Validation in httpembedder_test.go.
package httpembedder

// import "github.com/geoffmilleraz/agentic-kit/foundations/embedding/http-embedder:httpembedder"

// -- Example 1: missing scheme on BaseURL ------------------------------------
// badScheme would fail because BaseURL must match ^https?://.
//
// badScheme: httpembedder.#Config & {
// 	BaseURL: "api.openai.com/v1" // ERROR: violates =~"^https?://"
// 	Model:   "text-embedding-3-small"
// }

// -- Example 2: empty Model ---------------------------------------------------
// emptyModel would fail because Model must be non-empty.
//
// emptyModel: httpembedder.#Config & {
// 	BaseURL: "https://api.openai.com/v1"
// 	Model:   "" // ERROR: violates string & !=""
// }

// -- Example 3: negative Timeout ----------------------------------------------
// negativeTimeout would fail because Timeout must be >= 0.
//
// negativeTimeout: httpembedder.#Config & {
// 	BaseURL: "https://api.openai.com/v1"
// 	Model:   "text-embedding-3-small"
// 	Timeout: -1 // ERROR: violates int & >=0
// }

// -- Example 4: negative MaxBatch ----------------------------------------------
// negativeMaxBatch would fail because MaxBatch must be >= 0.
//
// negativeMaxBatch: httpembedder.#Config & {
// 	BaseURL:  "https://api.openai.com/v1"
// 	Model:    "text-embedding-3-small"
// 	MaxBatch: -5 // ERROR: violates int & >=0
// }
