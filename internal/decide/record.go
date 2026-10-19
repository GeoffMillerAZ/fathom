package decide

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"sync"
	"time"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// decisionRecordSchemaVersion identifies this JSONL row shape's field
// semantics (mirrors internal/mcp.callLogSchemaVersion's convention: bump on
// any semantic change to an existing field, never for an additive one).
//
// 2 (JF-24, agentic-kit DP-12): "kind" names the yes/no kind "predicate",
// the kit's generic name; a version 1 row says "noul" for the same kind, and
// every reader folds it through decision.ParseKind.
const decisionRecordSchemaVersion = 2

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
	// RecalibrationDue (JF-10, R8; additive, so no schema_version bump) is
	// true on the first row a site logs under a resolved model (or
	// provider) other than the one it last logged: a calibration reading
	// taken under the old model no longer speaks for the site. Omitted
	// otherwise, and on every RecordError row (no model resolved).
	RecalibrationDue bool `json:"recalibration_due,omitempty"`
	// KillSource (KS-6, additive, so no schema_version bump) names the
	// source that closed an open port before this ask (a RecordError row
	// whose error is a *KilledError): "file" or "enabled", or a site's own
	// "misses" (KS-12), "capped" or "deadline" (KS-10). Omitted otherwise.
	KillSource string `json:"kill_source,omitempty"`
	// ConsentLayer (KS-6, additive) is the layer whose consent opened the
	// port the row's ask went through: "user", or "project" when the
	// user's allow_repo_provider allowed a repository's provider. Omitted
	// for a port built in code.
	ConsentLayer string `json:"consent_layer,omitempty"`
	// Redactions (I61, additive, so no schema_version bump) counts the
	// secret-shaped strings the request plan redacted from the request
	// this row's question went out in. Omitted when none.
	Redactions int `json:"redactions,omitempty"`
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
	// last is each site's last logged (provider, resolved model), as the
	// kit's calibration ReadingKey (agentic-kit DP-23): in memory only,
	// seeded from the log's tail (Seed), never a table or memo (R5).
	last map[string]decision.ReadingKey
	// consentLayer is the open port's Gate.ConsentLayer (Open sets it),
	// written on every row as consent_layer.
	consentLayer string
	// windows is each site's last asks (KS-12, sitekill.go), seeded from
	// the log's tail and moved by every row written; killed is each site
	// a verdict turned off, for the Recorder's lifetime. In memory only.
	windows map[string]*missWindow
	killed  map[string]Gate
	// redacted is each question's redaction count from an answered ask
	// (I61, noteRedactions), consumed by that question's next row. In
	// memory only, and only for asks the plan redacted.
	redacted map[string]int
}

// setConsentLayer records the layer whose consent opened the port this
// Recorder logs for. A nil Recorder is a no-op.
func (r *Recorder) setConsentLayer(layer string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.consentLayer = layer
	r.mu.Unlock()
}

// NewRecorder builds a Recorder writing to w, using now for each row's ts
// (tests pin it for determinism) and fathomVersion for each row's
// fathom_version field.
func NewRecorder(w io.Writer, now func() time.Time, fathomVersion string) *Recorder {
	return &Recorder{w: w, now: now, fathomVersion: fathomVersion, last: map[string]decision.ReadingKey{}}
}

// SeedTailBytes bounds how much of an existing log a caller hands Seed
// (openDecisionRecorder reads at most this much of decisions.jsonl's end).
const SeedTailBytes = 1 << 20

// Seed reads decisions.jsonl rows (typically the existing log's tail) and
// remembers each site's last logged provider and resolved model, so the
// first row after a restart is compared with what the log last said
// (JF-10). Unparseable lines, failed-call rows and rows with no model are
// skipped there; every parsed row also moves its site's miss window
// (KS-12). A nil Recorder is a no-op.
func (r *Recorder) Seed(rd io.Reader) {
	if r == nil || rd == nil {
		return
	}
	sc := bufio.NewScanner(rd)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.last == nil {
		r.last = map[string]decision.ReadingKey{}
	}
	for sc.Scan() {
		var row struct {
			Site       string `json:"site"`
			Provider   string `json:"provider"`
			Model      string `json:"model"`
			Error      string `json:"error"`
			KillSource string `json:"kill_source"`
		}
		if json.Unmarshal(sc.Bytes(), &row) != nil {
			continue
		}
		r.observeLocked(row.Site, row.Error, row.KillSource)
		if row.Site == "" || row.Model == "" || row.Error != "" {
			continue
		}
		r.last[row.Site] = decision.ReadingKey{Site: row.Site, Provider: row.Provider, ResolvedModel: row.Model}
	}
}

// recalibrationDue compares rec with its site's last logged model and
// remembers rec's. Callers must hold r.mu. A row with no model (a provider
// that resolves none) neither marks nor moves anything; a site's first
// logged model is not a move.
func (r *Recorder) recalibrationDue(rec decision.Record) bool {
	if rec.Model == "" {
		return false
	}
	if r.last == nil {
		r.last = map[string]decision.ReadingKey{}
	}
	cur := decision.ReadingKey{Site: rec.Site, Provider: rec.Provider, ResolvedModel: rec.Model}
	prev, seen := r.last[rec.Site]
	r.last[rec.Site] = cur
	return seen && decision.Reading{Key: prev}.RecalibrationDue(cur)
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
	due := r.recalibrationDue(rec)
	r.observeLocked(rec.Site, "", "")
	r.writeJSONLine(decisionRow{
		SchemaVersion: decisionRecordSchemaVersion,
		FathomVersion: r.fathomVersion,
		TS:            ts,
		Site:          rec.Site,
		Provider:      rec.Provider,
		Model:         rec.Model,
		QuestionID:    rec.QuestionID,
		Kind:          string(decision.CanonicalKind(rec.Kind)),
		Choice:        rec.Choice,
		Confidence:    rec.Confidence,
		Score:         rec.Score,
		Noul:          rec.Noul,
		LatencyMS:     rec.Latency.Milliseconds(),
		InputTokens:   rec.InputTokens,
		OutputTokens:  rec.OutputTokens,
		Applied:       applied,

		RecalibrationDue: due,
		ConsentLayer:     r.consentLayer,
		Redactions:       r.takeRedactionsLocked(rec.QuestionID),
	})
}

// maxPendingRedactions bounds Recorder.redacted: an answer a site never
// records leaves its count behind, and a full map starts over.
const maxPendingRedactions = 4096

// noteRedactions records that the request plan redacted n strings from
// the request qs went out in (I61), for each question's next row. A nil
// Recorder and n of 0 are no-ops.
func (r *Recorder) noteRedactions(qs []decision.Question, n int) {
	if r == nil || n <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.redacted == nil || len(r.redacted) >= maxPendingRedactions {
		r.redacted = map[string]int{}
	}
	for _, q := range qs {
		r.redacted[q.ID] = n
	}
}

// takeRedactionsLocked is id's pending redaction count, cleared. Callers
// hold r.mu.
func (r *Recorder) takeRedactionsLocked(id string) int {
	n := r.redacted[id]
	if n > 0 {
		delete(r.redacted, id)
	}
	return n
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

	r.observeLocked(site, err.Error(), killSourceOf(err))
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
		KillSource:    killSourceOf(err),
		ConsentLayer:  r.consentLayer,
	})
}

// killSourceOf names the source of a live kill err carries, else "".
func killSourceOf(err error) string {
	var ke *KilledError
	if errors.As(err, &ke) {
		return ke.Gate.Source
	}
	return ""
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
