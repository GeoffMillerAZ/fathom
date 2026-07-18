// Package runauditrecord provides an append-only audit record for a run,
// capturing each step with detail and timestamp for immutable replay.
//
// Concurrency: Record is not safe for concurrent use; guard with your own synchronisation.
package runauditrecord

import "time"

// AuditEntry is a single immutable record in the run audit.
type AuditEntry struct {
	// Step is the name or identifier of the step being recorded.
	Step string

	// Detail contains a human-readable description of what occurred.
	Detail string

	// At is the wall-clock time the step was recorded.
	At time.Time
}
