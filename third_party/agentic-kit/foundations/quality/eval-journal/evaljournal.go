package evaljournal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	jsonllog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/persistence/jsonl-log"
)

// knownVerdicts holds every permissible Verdict value for fast validation.
var knownVerdicts = map[Verdict]struct{}{
	VerdictPass:         {},
	VerdictFail:         {},
	VerdictAbsentProven: {},
	VerdictAbstain:      {},
	VerdictError:        {},
}

// Validate checks all invariants on r. It is called by Append before writing
// and by Load after decoding each record.
//
// Rules:
//   - RunID must not be empty.
//   - ScenarioID must not be empty.
//   - Producer.Model must not be empty.
//   - Judge.Model and Judge.Family must not be empty.
//   - Verdict must be one of the five known values.
//   - An empty Rationale requires Decisive=false.
//   - Score, when set, must be in [0, 1].
//   - OccurredAt must not be zero.
func (r Record) Validate() error {
	if r.RunID == "" {
		return fmt.Errorf("evaljournal: RunID must not be empty")
	}
	if r.ScenarioID == "" {
		return fmt.Errorf("evaljournal: ScenarioID must not be empty")
	}
	if r.Producer.Model == "" {
		return fmt.Errorf("evaljournal: Producer.Model must not be empty")
	}
	if r.Judge.Model == "" {
		return fmt.Errorf("evaljournal: Judge.Model must not be empty")
	}
	if r.Judge.Family == "" {
		return fmt.Errorf("evaljournal: Judge.Family must not be empty")
	}
	if _, ok := knownVerdicts[r.Verdict]; !ok {
		return fmt.Errorf("evaljournal: Verdict %q is not one of pass|fail|absent_proven|abstain|error", r.Verdict)
	}
	if r.Rationale == "" && r.Decisive {
		return fmt.Errorf("evaljournal: Decisive must be false when Rationale is empty")
	}
	if r.Score != nil && (*r.Score < 0 || *r.Score > 1) {
		return fmt.Errorf("evaljournal: Score %g is outside [0, 1]", *r.Score)
	}
	if r.OccurredAt.IsZero() {
		return fmt.Errorf("evaljournal: OccurredAt must not be zero")
	}
	return nil
}

// Append validates rec and writes it to w as a single JSONL line. The record
// is marshalled with encoding/json (struct field order = canonical wire order).
//
// Errors: validation failure returns before any write; write failure is
// wrapped with "evaljournal: append: ...".
func Append(w *jsonllog.Writer, rec Record) error {
	if err := rec.Validate(); err != nil {
		return err
	}
	if err := w.Append(rec); err != nil {
		return fmt.Errorf("evaljournal: append: %w", err)
	}
	return nil
}

// Load reads a JSONL stream produced by Append and returns the decoded records
// in log order. Every record is validated via Validate — the same invariants
// Append enforces.
//
// Recovery mode — torn tail: if the stream ends in a partial line (the
// signature of a crash mid-append), Load returns every complete record AND a
// non-nil error satisfying errors.Is(err, jsonllog.ErrPartialTrailingLine).
// Callers that accept recovery semantics check for that sentinel and proceed
// with the returned records; strict callers treat any non-nil error as fatal.
//
// Errors on complete lines (malformed JSON, validation failures) stop the read
// and return (nil, err) — no partial results are returned for non-recovery
// errors.
func Load(r io.Reader) ([]Record, error) {
	var out []Record
	err := jsonllog.ReadAll(r, func(raw json.RawMessage) error {
		var rec Record
		if err := json.Unmarshal(raw, &rec); err != nil {
			return fmt.Errorf("decode record: %w", err)
		}
		if err := rec.Validate(); err != nil {
			return err
		}
		out = append(out, rec)
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

// NewWithClock is a convenience that builds a Record with OccurredAt set from
// the provided clock function. Callers should use this instead of setting
// OccurredAt directly to keep the injected-clock seam consistent.
//
// The returned record is not yet validated; call Validate or Append to enforce
// invariants.
func NewWithClock(now func() time.Time, r Record) Record {
	r.OccurredAt = now()
	return r
}
