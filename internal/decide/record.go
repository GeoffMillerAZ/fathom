package decide

import (
	"encoding/json"
	"io"
	"path/filepath"
	"sync"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

// decisionRecordSchemaVersion identifies this JSONL row shape's field
// semantics (mirrors internal/mcp.callLogSchemaVersion's convention: bump on
// any semantic change to an existing field, never for an additive one).
const decisionRecordSchemaVersion = 1

// decisionRow is one JSONL row written per answered decision.Question, OR
// (DP-22, O109) per decision-port call that hard-failed and answered
// nothing at all — deterministic field order (a struct, never a map).
type decisionRow struct {
	SchemaVersion int    `json:"schema_version"`
	FathomVersion string `json:"fathom_version"`
	TS            string `json:"ts"`

	Site       string `json:"site"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	QuestionID string `json:"question_id"`
	Kind       string `json:"kind"`

	Choice     string  `json:"choice"`
	Confidence float64 `json:"confidence"`
	Score      float64 `json:"score"`
	Noul       float64 `json:"noul"`

	LatencyMS    int64 `json:"latency_ms"`
	InputTokens  int   `json:"input_tokens"`
	OutputTokens int   `json:"output_tokens"`

	Applied bool `json:"applied"`

	// Questions (DP-22, RecordError only) is how many decision.Question a
	// hard-failed call carried — QuestionID (singular, "one row per
	// answer") has no meaning for a call that answered nothing at all.
	// Omitted (0) on every ordinary Record/Append row.
	Questions int `json:"questions,omitempty"`
	// Error (DP-22, RecordError only) is the failed call's error message,
	// verbatim — never a secret (the kit's own adapters already keep API
	// keys and raw provider bodies out of their returned errors). Absent
	// on every ordinary Record/Append row: a decision that was actually
	// answered has no error to carry, and this field's mere PRESENCE is
	// what lets a reader tell "asked and failed" apart from "never asked
	// at all" — silence in this log used to mean only the latter (O109).
	Error string `json:"error,omitempty"`
}

// Recorder appends one JSONL row per answered decision.Question to an
// append-only log — the call-log-shaped instrument truth I42 requires: a
// decision in the unsure band is recorded as NOT applied regardless of what
// the call site itself did with the answer. RecordError (DP-22, O109) is
// the parallel path for a Decide call that answered NOTHING because it
// hard-failed: one row naming site/provider/questions/latency/error, so
// silence in this log means "never asked", never "asked and failed".
//
// A nil *Recorder is a no-op (every method checks its receiver first), so a
// caller that never wires the decision port (provider = none, or DP-4's
// doctor row off) can pass a nil Recorder through internal/decide.Ports
// unconditionally.
//
// Concurrency: Recorder serialises writes with an internal mutex, so it is
// safe for concurrent use across goroutines sharing one underlying writer.
type Recorder struct {
	mu            sync.Mutex
	w             io.Writer
	now           func() time.Time
	fathomVersion string
}

// NewRecorder builds a Recorder writing to w, using now for each row's ts
// (tests pin it for determinism) and fathomVersion for each row's
// fathom_version field.
func NewRecorder(w io.Writer, now func() time.Time, fathomVersion string) *Recorder {
	return &Recorder{w: w, now: now, fathomVersion: fathomVersion}
}

// Record writes one JSONL row per question resp answered validly (a
// question resp.Invalid names produces no row — nothing was decided),
// sorted by question ID for deterministic output regardless of map
// iteration order. applied classifies whether the caller actually used an
// answer's Choice/Score/Noul to change served content — EXCEPT: per I42, a
// decision in the unsure band (decision.BandOf(answer.Confidence) ==
// decision.BandUnsure) is always recorded as applied:false, whatever
// applied returns, because an unsure decision must never be credited with
// having changed anything.
//
// A nil Recorder, resp with no valid Answers, or a nil applied for a
// Response that has valid Answers (never called — nothing to classify) are
// all safe no-ops/degenerate cases: at most one row is skipped, never
// panics.
func (r *Recorder) Record(site string, resp decision.Response, applied func(decision.Answer) bool) {
	if r == nil || r.w == nil {
		return
	}

	recs := resp.Records(site) // sorted by QuestionID
	if len(recs) == 0 {
		return
	}

	ts := r.now().UTC().Format(time.RFC3339)

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, rec := range recs {
		app := false
		if answer, ok := resp.Answers[rec.QuestionID]; ok {
			if decision.BandOf(answer.Confidence) != decision.BandUnsure && applied != nil {
				app = applied(answer)
			}
		}
		r.writeRow(ts, rec, app)
	}
}

// Append writes one JSONL row per entry in records — the same row shape
// Record produces — for a call site (compile's PrefilterSections,
// obligations' rerankWithinTiers) that already has []decision.Record in
// hand rather than a full decision.Response to classify itself. The kit's
// decision.Record carries no Applied flag, so applied is derived from band
// the same way Record treats the unsure band: per I42, a record whose
// confidence bands as decision.BandUnsure is always applied:false; every
// other record is applied:true, since Append's only callers (the compile
// pre-filter and the obligations re-rank) pass records solely for
// decisions their own logic already acted on — there is no separate
// "applied" callback to consult for a bare Record.
//
// A nil Recorder, a nil writer, or an empty records is a safe no-op,
// matching Record.
func (r *Recorder) Append(records []decision.Record) {
	if r == nil || r.w == nil || len(records) == 0 {
		return
	}

	ts := r.now().UTC().Format(time.RFC3339)

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, rec := range records {
		r.writeRow(ts, rec, decision.BandOf(rec.Confidence) != decision.BandUnsure)
	}
}

// writeRow marshals and writes one JSONL row for rec with the given ts and
// applied classification. Callers must hold r.mu.
func (r *Recorder) writeRow(ts string, rec decision.Record, applied bool) {
	r.writeJSONLine(decisionRow{
		SchemaVersion: decisionRecordSchemaVersion,
		FathomVersion: r.fathomVersion,
		TS:            ts,
		Site:          rec.Site,
		Provider:      rec.Provider,
		Model:         rec.Model,
		QuestionID:    rec.QuestionID,
		Kind:          string(rec.Kind),
		Choice:        rec.Choice,
		Confidence:    rec.Confidence,
		Score:         rec.Score,
		Noul:          rec.Noul,
		LatencyMS:     rec.Latency.Milliseconds(),
		InputTokens:   rec.InputTokens,
		OutputTokens:  rec.OutputTokens,
		Applied:       applied,
	})
}

// writeJSONLine marshals and writes one decisionRow. Callers must hold r.mu.
func (r *Recorder) writeJSONLine(row decisionRow) {
	b, err := json.Marshal(row)
	if err != nil {
		return // decisionRow is all primitives; unreachable in practice
	}
	b = append(b, '\n')
	_, _ = r.w.Write(b) // best-effort, mirrors internal/mcp.CallLogger's own logging discipline
}

// RecordError (DP-22, O109) writes one JSONL row for a decision-port Decide
// call that hard-failed: no decision.Response was ever produced, so
// Record/Append's own Response-shaped row has nothing to write, and before
// this method existed a failed call left NO trace in decisions.jsonl at
// all — the obligations re-rank's own O109 finding was a real, billed
// OpenRouter round trip whose failure was visible only on a transient
// rank_note on whichever wire response happened to hit it, never in the
// append-only log. site identifies the call site ("route",
// "compile_prefilter", "obligations"); provider is the failing Decider's
// own Name(); model is the underlying model identifier when known — empty
// when not (the decision.Decider interface has no Model() accessor, and a
// hard Decide error returns no Response to read one off — every call site
// in this wave passes "" here); questions is how many decision.Question
// the failed call carried; latency is how long the call took before
// failing; err's message is recorded VERBATIM as this row's "error" field
// (never a secret — the kit's own adapters already keep API keys and raw
// provider bodies out of their returned errors). Applied is always false: a
// decision that never happened changed nothing.
//
// A nil Recorder, a nil writer, or a nil err are all safe no-ops — a nil
// err in particular is "nothing failed", not a row with an empty error
// string that would misread as "asked and failed" when nothing was ever
// asked.
func (r *Recorder) RecordError(site, provider, model string, questions int, latency time.Duration, err error) {
	if r == nil || r.w == nil || err == nil {
		return
	}

	ts := r.now().UTC().Format(time.RFC3339)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.writeJSONLine(decisionRow{
		SchemaVersion: decisionRecordSchemaVersion,
		FathomVersion: r.fathomVersion,
		TS:            ts,
		Site:          site,
		Provider:      provider,
		Model:         model,
		Questions:     questions,
		LatencyMS:     latency.Milliseconds(),
		Error:         err.Error(),
		Applied:       false,
	})
}

// DefaultRecorderPath returns "decisions.jsonl" beside callLogPath (the same
// directory the [daemon] call log already lives in) — "" when callLogPath is
// "" (no call log configured, so no default decisions log either).
func DefaultRecorderPath(callLogPath string) string {
	if callLogPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(callLogPath), "decisions.jsonl")
}
