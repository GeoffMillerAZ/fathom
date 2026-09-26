package assess

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// RowSchemaVersion identifies the assess ledger's row shape (bump on any
// semantic change to an existing field, never for an additive one).
const RowSchemaVersion = 1

// Row is one assess run on the assess ledger — the spend meter IR-4 keeps
// apart from the session ledger. Every "if known" figure is a pointer with
// a sibling note, so a reader never has to guess whether a zero is real.
type Row struct {
	SchemaVersion int    `json:"schema_version"`
	FathomVersion string `json:"fathom_version"`
	At            string `json:"at"` // RFC3339
	RunID         string `json:"run_id"`

	SpendLabel string `json:"spend_label"` // assess | assess-interventions
	Lane       string `json:"lane"`        // unit | interventions
	Session    string `json:"session"`
	RepoRoot   string `json:"repo_root"`

	QuestionSet string `json:"question_set"`
	Provider    string `json:"provider"`
	Model       string `json:"model,omitempty"`
	Questions   int    `json:"questions"`

	Proceed int `json:"proceed"`
	Confirm int `json:"confirm"`
	Unsure  int `json:"unsure"`
	Invalid int `json:"invalid"`

	Verdicts []Verdict `json:"verdicts,omitempty"`

	TokensIn   *int   `json:"tokens_in,omitempty"`
	TokensOut  *int   `json:"tokens_out,omitempty"`
	TokensNote string `json:"tokens_note,omitempty"`

	CostUSD  *float64 `json:"cost_usd,omitempty"`
	CostNote string   `json:"cost_note,omitempty"`

	LatencyMS int64 `json:"latency_ms"`

	// Partial marks a run whose state was cut at a cap: never a
	// calibration sample. Calibrated is false until the question set passes
	// the adjudication bar (docs/design/assess-lanes.md §4, M-A1); v1 has
	// no calibrated set, so every row says false.
	Partial    bool `json:"partial,omitempty"`
	Calibrated bool `json:"calibrated"`

	// Isolation names the spend-isolation registration this run was
	// metered under (IsolationRegistration).
	Isolation string `json:"isolation"`

	// Error is set when the provider call failed: no verdict exists.
	Error string `json:"error,omitempty"`
}

// AppendRow appends r as one JSON line, creating parent directories.
// SchemaVersion is stamped here so no caller can forget it.
func AppendRow(path string, r Row) error {
	if path == "" {
		return errors.New("assess: no ledger path")
	}
	r.SchemaVersion = RowSchemaVersion
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

// ReadRows reads the ledger. An absent ledger is empty, not an error; a
// malformed line is skipped rather than hiding every good row.
func ReadRows(path string) ([]Row, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Row
	for _, ln := range strings.Split(string(data), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var r Row
		if json.Unmarshal([]byte(ln), &r) == nil && r.RunID != "" {
			out = append(out, r)
		}
	}
	return out, nil
}

// Cost classes: what a ledger row's spend figures are known to be. Only
// CostPriced has a cost; CostUnpriced has tokens and no price; CostNoUsage
// and CostFailed have neither — and a failed call may still have been
// billed, so its unknown is never a zero.
const (
	CostPriced   = "priced"
	CostUnpriced = "unpriced" // tokens known, no price for the model in this binary's meter
	CostNoUsage  = "no_usage" // the provider reported no token usage
	CostFailed   = "failed"   // the call failed: tokens and cost unknown
)

// CostClass classifies one row's spend figures.
func CostClass(r Row) string {
	switch {
	case r.Error != "":
		return CostFailed
	case r.CostUSD != nil:
		return CostPriced
	case r.TokensIn != nil || r.TokensOut != nil:
		return CostUnpriced
	}
	return CostNoUsage
}

// SpendRuns maps ledger rows onto the compile spend meter's row type
// (core.CompileRun, the input of metrics.AggregateSpend), with the spend
// label as the group key, so `fathom assess spend` totals through the same
// aggregation `fathom metrics spend` uses. classes[i] is runs[i]'s
// CostClass: a run that is not CostPriced contributes NO cost (and, unless
// it is CostUnpriced, no tokens) — the caller aggregates those runs again
// to say, per group, that a figure is a lower bound or unknown, never zero.
func SpendRuns(rows []Row) (runs []core.CompileRun, classes []string) {
	for _, r := range rows {
		run := core.CompileRun{RunID: r.RunID, Layer: r.SpendLabel, Model: r.Model}
		if t, err := time.Parse(time.RFC3339, r.At); err == nil {
			run.RanAt = t
		}
		if r.TokensIn != nil {
			run.TokensIn = *r.TokensIn
		}
		if r.TokensOut != nil {
			run.TokensOut = *r.TokensOut
		}
		class := CostClass(r)
		if class == CostPriced {
			run.CostUSD = *r.CostUSD
		}
		runs = append(runs, run)
		classes = append(classes, class)
	}
	return runs, classes
}
