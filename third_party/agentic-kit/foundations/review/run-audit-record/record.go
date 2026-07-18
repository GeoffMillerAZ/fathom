package runauditrecord

import (
	"fmt"
	"time"
)

// Record is an append-only audit log for a single run. Entries can never be
// modified or removed; the log is suitable for immutable replay.
type Record struct {
	entries []AuditEntry

	// Now is the clock used to stamp entries whose At is zero.
	// nil means time.Now (documented default); inject a fixed clock for
	// deterministic tests.
	Now func() time.Time
}

// Append adds a new AuditEntry to the end of the record. The At timestamp is
// set to the current UTC time from the record's clock if it is zero. Returns
// an error if Step is empty.
func (r *Record) Append(entry AuditEntry) error {
	if entry.Step == "" {
		return fmt.Errorf("runauditrecord: AuditEntry.Step must not be empty")
	}
	if entry.At.IsZero() {
		now := r.Now
		if now == nil {
			now = time.Now
		}
		entry.At = now().UTC()
	}
	r.entries = append(r.entries, entry)
	return nil
}

// Entries returns a read-only snapshot of the audit entries in insertion order.
// The returned slice must not be modified by the caller.
func (r *Record) Entries() []AuditEntry {
	out := make([]AuditEntry, len(r.entries))
	copy(out, r.entries)
	return out
}
