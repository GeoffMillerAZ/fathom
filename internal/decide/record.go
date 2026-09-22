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

// decisionRow is one JSONL row written per answered decision.Question —
// deterministic field order (a struct, never a map).
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
}

// Recorder appends one JSONL row per answered decision.Question to an
// append-only log — the call-log-shaped instrument truth I42 requires: a
// decision in the unsure band is recorded as NOT applied regardless of what
// the call site itself did with the answer.
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
		row := decisionRow{
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
			Applied:       app,
		}
		b, err := json.Marshal(row)
		if err != nil {
			continue // decisionRow is all primitives; unreachable in practice
		}
		b = append(b, '\n')
		_, _ = r.w.Write(b) // best-effort, mirrors internal/mcp.CallLogger's own logging discipline
	}
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
