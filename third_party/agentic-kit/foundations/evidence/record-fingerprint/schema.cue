// Package recordfingerprint defines the CUE-first contract for content-based
// record deduplication. Go types in types.go mirror these definitions; any
// drift is a bug.
package recordfingerprint

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// _rfc3339 constrains a string to the RFC3339 timestamp format used by
// time.Time marshalling in Go (e.g. "2024-06-01T14:02:00Z"). The regex
// matches the minimal structure: date T time optional-fractional-seconds
// timezone offset (Z or +hh:mm / -hh:mm). A bare local timestamp without
// a timezone is rejected — consistent with evidence-span/schema.cue.
_rfc3339: string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$"

// #Fingerprint captures the last-seen content hash for a record ID. It is the
// primitive that lets re-ingestion detect new, unchanged, and changed records
// without relying on record IDs alone.
#Fingerprint: {
	// RecordID is the stable identifier of the record (e.g. a document ID).
	RecordID: _nonEmpty

	// ContentHash is the sha256 hex digest of the record's content bytes.
	// Must be exactly 64 lowercase hexadecimal characters.
	ContentHash: _nonEmpty & =~"^[0-9a-f]{64}$"

	// SeenAt is the RFC3339 timestamp when this fingerprint was first or last
	// stored. Required — a fingerprint without a timestamp is provenance-incomplete.
	// Must include a timezone offset (Z or +hh:mm); bare local timestamps are rejected.
	SeenAt: _rfc3339
}

// #ChangeKind classifies the outcome of an Observe call.
#ChangeKind: "new" | "unchanged" | "changed"
