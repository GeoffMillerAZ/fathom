// Package schemamigrationledger defines CUE contracts for the schema migration ledger.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package schemamigrationledger

// #MigrationRecord represents a single schema migration that has been applied
// to the data layer.
#MigrationRecord: {
	// ID is a non-empty, unique migration identifier (e.g. "0001_add_users").
	ID: string & !=""

	// From is the schema version before this migration was applied.
	// May be empty for the very first migration in a ledger.
	From: string

	// To is the schema version after this migration was applied.
	// Must be non-empty.
	To: string & !=""

	// AppliedAt is the Unix timestamp (seconds) when the migration was applied.
	// Must be a positive integer.
	AppliedAt: int & >0
}
