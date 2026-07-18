// valid.cue — concrete values that satisfy the #SourceCoverage constraint
// defined in schema.cue. Running `cue vet ./...` from the package root
// validates these against the schema.
package coverage

import "github.com/geoffmilleraz/agentic-kit/foundations/investigation/coverage"

// A source with partial ingestion progress.
_cov1: coverage.#SourceCoverage & {
	SourceID:       "src-emails"
	RecordsKnown:   1000
	RecordsScanned: 600
	RecordsFailed:  20
	RecordsPending: 380
	LastSync:       "2026-06-08T12:00:00Z"
	Cursor:         "offset=600"
}

// A fully scanned source with no failures or pending records.
_cov2: coverage.#SourceCoverage & {
	SourceID:       "src-contracts"
	RecordsKnown:   50
	RecordsScanned: 50
	RecordsFailed:  0
	RecordsPending: 0
}

// A freshly discovered source: known count set, nothing scanned yet.
_cov3: coverage.#SourceCoverage & {
	SourceID:       "src-depositions"
	RecordsKnown:   200
	RecordsScanned: 0
	RecordsFailed:  0
	RecordsPending: 200
}

// A source with zero records known (advertises nothing yet).
_cov4: coverage.#SourceCoverage & {
	SourceID:       "src-new"
	RecordsKnown:   0
	RecordsScanned: 0
	RecordsFailed:  0
	RecordsPending: 0
}

// Boundary: Scanned + Failed + Pending exactly equals Known (10 == 10).
_cov5: coverage.#SourceCoverage & {
	SourceID:       "src-boundary"
	RecordsKnown:   10
	RecordsScanned: 5
	RecordsFailed:  2
	RecordsPending: 3
}
