// Package coverage defines the CUE-first contracts for the completeness ledger.
// Go types in types.go mirror these definitions; any drift is a bug.
package coverage

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #SourceCoverage records how much of one source is known, scanned, failed,
// and pending. Together these values surface "known unknowns" — records the
// system is aware of but has not yet examined — so that no silent truncation
// occurs and every cap or unscanned tail is a queryable fact.
//
// Cross-field invariant (enforced here and in Go Validate):
//   RecordsScanned + RecordsFailed + RecordsPending <= RecordsKnown.
#SourceCoverage: {
	// SourceID uniquely identifies the source being tracked.
	SourceID: _nonEmpty

	// RecordsKnown is the total number of records the source advertises.
	// Must be a non-negative integer (float values are rejected).
	RecordsKnown: int & >=0

	// RecordsScanned is the number of records fully ingested.
	// Must be a non-negative integer (float values are rejected).
	RecordsScanned: int & >=0

	// RecordsFailed is the number of records that were attempted but errored.
	// Must be a non-negative integer (float values are rejected).
	RecordsFailed: int & >=0

	// RecordsPending is the number of known records not yet scanned.
	// Must be a non-negative integer (float values are rejected).
	RecordsPending: int & >=0

	// _sumGuard encodes the cross-field invariant at the schema level:
	// the sum of processed/pending counts must not exceed the known count.
	// Constraining _sumGuard to true causes CUE to reject any concrete value
	// where Scanned + Failed + Pending > Known.
	_sumGuard: RecordsScanned + RecordsFailed + RecordsPending <= RecordsKnown
	_sumGuard: true

	// LastSync is the RFC3339 timestamp of the last synchronisation attempt.
	LastSync?: string

	// Cursor is an opaque resume cursor for incremental ingestion.
	Cursor?: string
}
