// Package jsonllog provides append-only JSONL (JSON Lines) persistence: a
// Writer that appends one canonical JSON value per line to any io.Writer, a
// File convenience for durable append-only logs on disk, and a streaming
// reader (ReadAll) that replays a log with line-accurate errors and an
// explicit, surfaced recovery mode for torn trailing writes.
//
// Composition: this package is the kit's persistence seam. It is behavioral
// only — it has no data contract of its own (hence no schema.cue); the shape
// of what gets persisted belongs to the producing package's CUE schema.
// Higher layers (e.g. foundations/investigation/investigation-journal) attach
// a *Writer as an entry sink to make their in-memory state durable, and
// rebuild from the log via ReadAll on restart. For tamper-evident,
// hash-chained persistence use foundations/provenance/event-store instead —
// this package deliberately offers plain, greppable JSONL with no integrity
// chain.
//
// Boundary with foundations/telemetry/sink: that package's JSONLSink also
// writes one JSON object per line, but as fire-and-forget telemetry emission
// with no read-back contract. This package is the durable append-log seam —
// writer plus reader with torn-tail recovery — for state you rebuild from;
// telemetry events you only ever emit belong in telemetry/sink.
//
// Concurrency contract: Writer (and therefore File) is safe for concurrent
// use via sync.Mutex — each Append writes exactly one whole line while
// holding the lock. ReadAll holds no state of its own; it is safe for
// concurrent use only insofar as the supplied io.Reader is.
package jsonllog

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Writer appends values to an underlying io.Writer in JSONL form. Each
// successful Append writes the canonical encoding of the value followed by a
// single '\n', as one Write call on the underlying writer.
//
// Canonical form: the compact encoding produced by encoding/json.Marshal —
// no insignificant whitespace, map keys sorted, strings HTML-escaped per
// json.Marshal's defaults. Because json.Marshal never emits raw newlines
// (newlines inside strings are escaped as \n), every appended value occupies
// exactly one line. Values are typically JSON objects, but any
// json-marshalable value is accepted.
//
// Flush semantics: Writer itself does not buffer. Each Append hands the
// complete line (payload + '\n') to the underlying io.Writer in a single
// Write call and returns that call's error. Durability is therefore the
// underlying writer's business: a bytes.Buffer holds the line immediately, a
// bufio.Writer keeps it buffered until the caller flushes, and an *os.File
// hands it to the OS immediately but it reaches stable storage only on sync
// (see File.Close). On a Write error the line may have been partially
// written; ReadAll's partial-line recovery exists for exactly that torn-tail
// case.
//
// A Writer must not be copied after first use.
type Writer struct {
	mu sync.Mutex
	w  io.Writer
}

// NewWriter returns a Writer that appends JSONL lines to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// Append marshals v with encoding/json and writes the resulting single line
// (payload + '\n') to the underlying writer in one Write call.
//
// Errors: a marshal failure returns before anything is written; a write
// failure returns the underlying writer's error (wrapped). Append is
// mutex-guarded — concurrent Appends never interleave bytes within a line.
// The order of lines from concurrent Appends is the order in which they
// acquire the lock, which is unspecified relative to caller scheduling.
func (w *Writer) Append(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("jsonllog: marshal: %w", err)
	}
	line := make([]byte, 0, len(data)+1)
	line = append(line, data...)
	line = append(line, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.w.Write(line); err != nil {
		return fmt.Errorf("jsonllog: write: %w", err)
	}
	return nil
}
