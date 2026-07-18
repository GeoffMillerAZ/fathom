package investigationjournal

// persistence.go wires the journal to the persistence/jsonl-log foundation:
// JSONLSink makes a journal durable (attach via Journal.AttachSink), LoadJSONL
// reads a JSONL stream back into validated entries, and NewFromEntries
// rebuilds a Journal from them. Together: journal → JSONL → journal.
//
// Wire format: each line is the encoding/json encoding of Entry — PascalCase
// keys matching the Go field names, At as an RFC 3339 timestamp string. This
// matches #Entry in schema.cue (CUE owns the shape; At is the string form of
// the timestamp).

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	jsonllog "github.com/geoffmilleraz/agentic-kit/foundations/persistence/jsonl-log"
)

// JSONLSink adapts a jsonllog.Writer into a journal entry sink for
// Journal.AttachSink. Every entry the journal commits is appended to w as one
// JSON line; the Writer's error (marshal or write failure) propagates back
// through Append/AppendTraced per the AttachSink ordering contract.
//
// Typical use:
//
//	f, err := jsonllog.OpenFile(path) // durable, append-only
//	...
//	j.AttachSink(investigationjournal.JSONLSink(f.Writer))
func JSONLSink(w *jsonllog.Writer) func(Entry) error {
	return func(e Entry) error {
		return w.Append(e)
	}
}

// LoadJSONL reads a JSONL stream previously produced by JSONLSink and returns
// the decoded entries in log order, validated per Entry.Validate — the same
// invariants Append enforces. The result is suitable for NewFromEntries to
// rebuild a Journal.
//
// Errors carry the offending 1-based line number (via jsonllog.ReadAll).
// Decode and validation failures return (nil, err).
//
// Recovery mode — torn tail: if the stream ends in a partial line (the
// signature of a crash mid-append), LoadJSONL returns every complete entry
// AND a non-nil error satisfying errors.Is(err, jsonllog.ErrPartialTrailingLine).
// The condition is surfaced, never silent: callers that accept the recovery
// semantics check for that sentinel and proceed with the returned entries;
// strict callers treat any non-nil error as fatal.
func LoadJSONL(r io.Reader) ([]Entry, error) {
	var out []Entry
	err := jsonllog.ReadAll(r, func(raw json.RawMessage) error {
		var e Entry
		if err := json.Unmarshal(raw, &e); err != nil {
			return fmt.Errorf("decode entry: %w", err)
		}
		if err := e.Validate(); err != nil {
			return err
		}
		out = append(out, e)
		return nil
	})
	if err != nil {
		if errors.Is(err, jsonllog.ErrPartialTrailingLine) {
			// Recovery mode: everything before the torn tail is intact.
			return out, err
		}
		return nil, err
	}
	return out, nil
}

// NewFromEntries rebuilds a Journal from previously recorded entries (e.g.
// the output of LoadJSONL), preserving order. Every entry is validated per
// Entry.Validate — the same invariants Append enforces — and stored as a
// defensive copy; an invalid entry fails the whole rebuild with its index.
//
// The rebuilt journal uses wall-clock time for AppendTraced. To resume
// persistence, attach a sink afterwards (typically a JSONLSink over the same
// file reopened in append mode); entries already present are NOT re-delivered
// to the sink — they are assumed to be in the log this journal was rebuilt
// from.
//
// Note: this constructor performs per-entry validation only. It does not (and
// cannot) verify tamper-evidence — for hash-chain verification persist via
// the provenance/event-store foundation instead, as described in the package
// doc.
func NewFromEntries(entries []Entry) (*Journal, error) {
	j := &Journal{}
	for i, e := range entries {
		if err := e.Validate(); err != nil {
			return nil, fmt.Errorf("investigationjournal: entry %d: %w", i, err)
		}
		j.entries = append(j.entries, e.clone())
	}
	return j, nil
}
