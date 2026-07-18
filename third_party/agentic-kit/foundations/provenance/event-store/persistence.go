package eventstore

// persistence.go wires the event-store to the persistence/jsonl-log foundation:
// JSONLSink returns a sink function that makes a running Store durable (attach
// as a post-Append callback via Store.AttachSink after open), LoadJSONL reads a
// JSONL stream back into validated entries, and NewStoreFromJSONL rebuilds a
// Store from them.  Together: store → JSONL → store.
//
// Wire format: each line is the encoding/json encoding of a wireEntry — a flat
// struct whose fields match the CUE #Entry shape (RFC 3339 RecordedAt string,
// all other fields directly from Entry).  Binary Payload bytes are NOT stored
// (the schema only records PayloadHash; the payload is opaque to the chain
// representation).  Chain integrity is preserved because Verify re-derives
// PayloadHash from the (absent) raw payload only when the caller re-supplies
// it; for round-trip chain verification with payloads the caller must persist
// them separately and reload via NewStoreFromEntries.
//
// Seam intent: the audit note observes that sqliteledger should become "one
// adapter among several".  JSONLSink + LoadJSONL is the shipped JSONL adapter.
// A caller wiring a different backing store (SQLite, Kafka, S3) implements the
// same three primitives (sink func, load func, rebuild constructor) against its
// own transport; the Store API is unchanged.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	jsonllog "github.com/geoffmilleraz/agentic-kit/foundations/persistence/jsonl-log"
)

// wireEntry is the stable JSON representation of one Entry in the JSONL log.
// It mirrors #Entry in schema.cue: RecordedAt is RFC 3339, Payload is omitted
// (binary opaque), all other fields are the canonical string values.
//
// The struct uses PascalCase JSON keys to match the Go field names and the
// CUE field names (CUE is the source of truth for the shape).
type wireEntry struct {
	Seq         uint64 `json:"Seq"`
	Kind        string `json:"Kind"`
	PayloadHash string `json:"PayloadHash"`
	PrevHash    string `json:"PrevHash"`
	Hash        string `json:"Hash"`
	RecordedAt  string `json:"RecordedAt"` // RFC 3339 UTC
}

// entryToWire converts an Entry to its wire representation.
func entryToWire(e Entry) wireEntry {
	return wireEntry{
		Seq:         e.Seq,
		Kind:        e.Kind,
		PayloadHash: e.PayloadHash,
		PrevHash:    e.PrevHash,
		Hash:        e.Hash,
		RecordedAt:  e.RecordedAt.UTC().Format(time.RFC3339Nano),
	}
}

// wireToEntry converts a wireEntry to an Entry (Payload is always nil —
// the wire format does not store binary payloads).
func wireToEntry(w wireEntry) (Entry, error) {
	if w.Kind == "" {
		return Entry{}, fmt.Errorf("eventstore: wire entry seq=%d: Kind must not be empty", w.Seq)
	}
	if w.PayloadHash == "" {
		return Entry{}, fmt.Errorf("eventstore: wire entry seq=%d: PayloadHash must not be empty", w.Seq)
	}
	if w.Hash == "" {
		return Entry{}, fmt.Errorf("eventstore: wire entry seq=%d: Hash must not be empty", w.Seq)
	}
	if w.PrevHash == "" {
		return Entry{}, fmt.Errorf("eventstore: wire entry seq=%d: PrevHash must not be empty", w.Seq)
	}
	if w.RecordedAt == "" {
		return Entry{}, fmt.Errorf("eventstore: wire entry seq=%d: RecordedAt must not be empty", w.Seq)
	}
	t, err := time.Parse(time.RFC3339Nano, w.RecordedAt)
	if err != nil {
		return Entry{}, fmt.Errorf("eventstore: wire entry seq=%d: RecordedAt parse: %w", w.Seq, err)
	}
	return Entry{
		Seq:         w.Seq,
		Kind:        w.Kind,
		PayloadHash: w.PayloadHash,
		PrevHash:    w.PrevHash,
		Hash:        w.Hash,
		RecordedAt:  t.UTC(),
	}, nil
}

// JSONLSink adapts a jsonllog.Writer into a Store entry sink.  Attach it via
// Store.AttachSink; every entry successfully appended to the store will be
// written to w as one JSON line.  Marshal or write failures propagate back
// through Append per the AttachSink ordering contract.
//
// Typical use:
//
//	f, err := jsonllog.OpenFile(path)
//	…
//	s.AttachSink(eventstore.JSONLSink(f.Writer))
func JSONLSink(w *jsonllog.Writer) func(Entry) error {
	return func(e Entry) error {
		return w.Append(entryToWire(e))
	}
}

// AttachSink registers fn as the post-Append sink on the Store.  Every entry
// appended after this call is delivered to fn synchronously (in a deep copy)
// before it is added to the in-memory chain.  Pass nil to detach.
//
// Ordering contract: fn observes entries in chain order.  If fn returns an
// error the chain is left unchanged (the entry is not stored) and the error
// propagates from Append.  A durable JSONL log may therefore run ahead by at
// most one torn write; LoadJSONL's recovery mode surfaces that.
//
// AttachSink is not safe to call concurrently with Append — call it once at
// construction time, before any Append.
func (s *Store) AttachSink(fn func(Entry) error) {
	s.sink = fn
}

// LoadJSONL reads a JSONL stream previously produced by JSONLSink and returns
// the decoded entries in log order, validated per Entry.Validate.  The result
// is suitable for NewStoreFromEntries to rebuild a Store.
//
// NOTE: because the wire format does not store raw Payload bytes, the Payload
// field of every returned Entry is nil.  Chain verification via Store.Verify
// after rebuild will succeed (all hash fields are preserved), but
// re-verification of the payload hash (PayloadHash) will observe sha256([]) —
// the SHA-256 of an empty input — which matches only if the original payload
// was also empty.  Callers that need payload verification must persist payloads
// separately and reload via NewStoreFromEntries with fully-populated entries.
//
// Recovery mode — torn tail: identical semantics to investigationjournal's
// LoadJSONL: if the stream ends in a partial line, LoadJSONL returns every
// complete entry AND a non-nil error satisfying
// errors.Is(err, jsonllog.ErrPartialTrailingLine).
func LoadJSONL(r io.Reader) ([]Entry, error) {
	var out []Entry
	err := jsonllog.ReadAll(r, func(raw json.RawMessage) error {
		var w wireEntry
		if err := json.Unmarshal(raw, &w); err != nil {
			return fmt.Errorf("decode entry: %w", err)
		}
		e, err := wireToEntry(w)
		if err != nil {
			return err
		}
		if err := e.Validate(); err != nil {
			return err
		}
		out = append(out, e)
		return nil
	})
	if err != nil {
		if errors.Is(err, jsonllog.ErrPartialTrailingLine) {
			return out, err
		}
		return nil, err
	}
	return out, nil
}

// NewStoreFromJSONL is a convenience wrapper: it calls LoadJSONL and, on
// success, NewStoreFromEntries.  On a partial trailing line it returns both
// the (partial) store and the error — callers that accept the recovery
// semantics check errors.Is(err, jsonllog.ErrPartialTrailingLine) and use
// the returned store for the recovered prefix.
func NewStoreFromJSONL(r io.Reader) (*Store, error) {
	entries, err := LoadJSONL(r)
	if err != nil && !errors.Is(err, jsonllog.ErrPartialTrailingLine) {
		return nil, err
	}
	s := NewStoreFromEntries(entries)
	return s, err
}
