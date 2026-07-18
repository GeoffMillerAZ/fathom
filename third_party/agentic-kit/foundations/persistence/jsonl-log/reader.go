package jsonllog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ErrPartialTrailingLine reports that the final line of a JSONL stream was
// not terminated by '\n'. It is the sentinel for ReadAll's documented
// recovery mode: match it with errors.Is, and use errors.As with
// *PartialLineError to inspect the torn bytes.
var ErrPartialTrailingLine = errors.New("partial trailing line")

// PartialLineError is returned by ReadAll when the stream ends in an
// unterminated line — the signature of a torn write (crash between Append's
// Write call starting and completing) or of a log produced without a final
// newline. It wraps ErrPartialTrailingLine, so
// errors.Is(err, ErrPartialTrailingLine) is true.
//
// All complete lines before the partial one have already been delivered to
// the ReadAll callback by the time this error is returned, so callers can
// treat it as a recovery checkpoint: everything delivered is intact, and the
// torn tail is available in Bytes for inspection or salvage.
type PartialLineError struct {
	// Line is the 1-based line number of the partial line.
	Line int
	// Bytes holds the raw bytes of the partial line (no trailing newline).
	Bytes []byte
}

// Error implements the error interface.
func (e *PartialLineError) Error() string {
	return fmt.Sprintf("jsonllog: line %d: %v (%d bytes)", e.Line, ErrPartialTrailingLine, len(e.Bytes))
}

// Unwrap makes errors.Is(err, ErrPartialTrailingLine) work.
func (e *PartialLineError) Unwrap() error { return ErrPartialTrailingLine }

// ReadAll streams a JSONL log from r, invoking fn once per complete line with
// the line's raw JSON payload (newline stripped). Lines are delivered in
// order. There is no length limit on lines beyond available memory.
//
// Errors always carry 1-based line numbers:
//   - A blank line or a line that is not valid JSON stops the read with an
//     error naming the offending line; fn is not invoked for it or for any
//     later line.
//   - An error returned by fn stops the read and is propagated wrapped with
//     the line number (errors.Is/As still see the original).
//   - A read error from r is propagated wrapped with the line number.
//
// Recovery mode — partial trailing line: if the stream ends in a line with no
// '\n' terminator, ReadAll returns a *PartialLineError (wrapping
// ErrPartialTrailingLine) AFTER delivering every complete line to fn. This is
// the expected shape of a log torn by a crash mid-Append. The condition is
// surfaced, never silent: even if the unterminated tail happens to parse as
// valid JSON it is still reported as partial, because without the newline its
// completeness cannot be confirmed (a torn prefix can itself be valid JSON).
// Callers that accept the recovery semantics check
// errors.Is(err, ErrPartialTrailingLine) and keep what was delivered; the
// torn bytes are available via errors.As for inspection.
func ReadAll(r io.Reader, fn func(json.RawMessage) error) error {
	br := bufio.NewReader(r)
	for line := 1; ; line++ {
		raw, err := br.ReadBytes('\n')
		switch {
		case err == io.EOF && len(raw) == 0:
			return nil // clean end: previous line was newline-terminated
		case err == io.EOF:
			return &PartialLineError{Line: line, Bytes: raw}
		case err != nil:
			return fmt.Errorf("jsonllog: line %d: read: %w", line, err)
		}
		payload := raw[:len(raw)-1]
		if len(bytes.TrimSpace(payload)) == 0 {
			return fmt.Errorf("jsonllog: line %d: blank line", line)
		}
		if !json.Valid(payload) {
			return fmt.Errorf("jsonllog: line %d: invalid JSON", line)
		}
		if err := fn(json.RawMessage(payload)); err != nil {
			return fmt.Errorf("jsonllog: line %d: %w", line, err)
		}
	}
}
