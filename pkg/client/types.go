// Package client is the public Go embedding API for sextant. It re-exports the
// stable domain types from internal packages so external consumers only need to
// import pkg/client — never an internal package.
package client

import (
	"github.com/geoffmilleraz/sextant/internal/core"
)

// Additional domain types re-exported for external consumers.
// ServiceRef, Query, QueryResult, SymbolNode, and Report are declared in
// client.go alongside the Client type.
type (
	// Observation is the atomic evidence record persisted in the store.
	Observation = core.Observation

	// StalenessReport tells callers whether the underlying data is fresh.
	StalenessReport = core.StalenessReport

	// Contradiction reports two observations asserting incompatible claims.
	Contradiction = core.Contradiction

	// SourceKind classifies what a source/observation represents.
	SourceKind = core.SourceKind
)
