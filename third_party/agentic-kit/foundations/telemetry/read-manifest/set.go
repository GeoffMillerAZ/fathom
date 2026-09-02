package readmanifest

import "sync"

// Set records every read made while building one report. Obtain one with
// NewSet; the zero value is also ready to use. A Set belongs to one
// invocation — the only scope over which "what did I fail to see" is a
// meaningful question.
type Set struct {
	mu      sync.Mutex
	root    string
	entries map[string]Entry
}
