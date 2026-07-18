// Package evidencecompactor defines the CUE-first contract for the journal
// record written before every compaction summary call. Go types in types.go
// mirror these definitions; any drift is a bug. Behaviour (journalling,
// LLM call, marker embedding) lives entirely in Go — CUE owns only the
// shape of the persisted record.
package evidencecompactor

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #JournalRecord is the record written to the caller-supplied io.Writer
// (via jsonl-log) before any LLM summarisation call. Its presence in the
// journal guarantees that dropped ground truth survives a crash between
// journalling and the LLM call.
//
// Go mirror: JournalRecord in types.go.
#JournalRecord: {
	// kind is always "compaction-dropped" — discriminates this record in
	// a mixed journal.
	kind: "compaction-dropped"

	// at is the RFC 3339 UTC timestamp when the record was written.
	at: _nonEmpty

	// dropped_count is the number of messages journalled in this record.
	dropped_count: int & >0

	// prompt_hash is the lowercase hex SHA-256 of the rendered summary
	// prompt text, set AFTER the LLM call completes; absent in records
	// written before the call (crash-safe: journal first, then call).
	prompt_hash?: string

	// template_version is the semver version of the summary prompt template.
	template_version: _nonEmpty

	// params_hash is the lowercase hex SHA-256 of the canonical JSON
	// encoding of the template parameters.
	params_hash: _nonEmpty

	// messages are the raw dropped llmclient.Message values, oldest first.
	// Each message carries role and content blocks verbatim so the journal
	// is self-contained ground truth.
	messages: [...#JournalMessage]
}

// #JournalMessage is the portable representation of one llmclient.Message
// as stored in the journal. Mirrors llmclient.Message with stable JSON
// field names.
#JournalMessage: {
	role:    "user" | "assistant"
	content: [...#JournalBlock]
}

// #JournalBlock is a single content block as stored in the journal.
// Fields are all optional (tagged omitempty in Go) since each block type
// populates a different subset.
#JournalBlock: {
	type:        _nonEmpty
	text?:       string
	id?:         string
	name?:       string
	tool_use_id?: string
	content?:    string
	is_error?:   bool
}
