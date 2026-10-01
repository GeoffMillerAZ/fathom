package store

// sqlite_compileruns.go — SQLiteStore's compile-run cost/audit ledger and its
// JSON column helpers (rejection reasons, uncompiled scopes), whose only
// consumers are RecordCompileRun and CompileRuns. Split out of sqlite.go
// (wave g1).

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// marshalRejectionReasons (Q-3, docs/plans/expert-suite-q.md, O19's fix) is
// RecordCompileRun's JSON-encode step, factored into its own free function —
// a name unique to this file, unlike the RecordCompileRun method name
// (both SQLiteStore and MemoryStore implement it), so a symbol-based tool
// can anchor to a single, unambiguous definition. RejectionReasons is nil
// on the overwhelming majority of runs (a clean compile with zero
// rejections, or a dry run) — marshal nil as "[]" explicitly rather than
// writing a NULL, so unmarshalRejectionReasons never has to special-case
// NULL vs "[]" vs a real array.
func marshalRejectionReasons(rr []core.RejectionBreakdown) (string, error) {
	if len(rr) == 0 {
		// json.Marshal(nil []T) encodes "null", not "[]" — normalize
		// explicitly so the column always holds a valid JSON array.
		return "[]", nil
	}
	data, err := json.Marshal(rr)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// unmarshalRejectionReasons is CompileRuns' JSON-decode step, the sibling
// of marshalRejectionReasons: a legacy row (empty string — the migration
// default, or a row written before this column existed at all) degrades to
// nil, never an error; a malformed value degrades the same lenient way
// every other JSON column in this file already does, rather than aborting
// the whole CompileRuns call.
func unmarshalRejectionReasons(s string) []core.RejectionBreakdown {
	if s == "" || s == "[]" {
		return nil
	}
	var rr []core.RejectionBreakdown
	_ = json.Unmarshal([]byte(s), &rr)
	return rr
}

// marshalStringSlice/unmarshalStringSlice (O54) are marshalRejectionReasons/
// unmarshalRejectionReasons' sibling for CompileRun.UncompiledScopes — same
// "nil marshals to the literal '[]', a legacy/malformed value degrades to
// nil rather than erroring" contract, so a pre-existing store's freshly
// migrated default ('[]') and a genuinely empty slice read back identically.
func marshalStringSlice(ss []string) (string, error) {
	if len(ss) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(ss)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalStringSlice(s string) []string {
	if s == "" || s == "[]" {
		return nil
	}
	var ss []string
	_ = json.Unmarshal([]byte(s), &ss)
	return ss
}

// RecordCompileRun appends a cost/audit ledger row for one compilation pass.
func (s *SQLiteStore) RecordCompileRun(ctx context.Context, run core.CompileRun) error {
	rejectionJSON, err := marshalRejectionReasons(run.RejectionReasons)
	if err != nil {
		return fmt.Errorf("RecordCompileRun: marshal rejection_reasons_json: %w", err)
	}
	uncompiledJSON, err := marshalStringSlice(run.UncompiledScopes)
	if err != nil {
		return fmt.Errorf("RecordCompileRun: marshal uncompiled_scopes_json: %w", err)
	}
	_, err = s.db.ExecContext(
		ctx, `
INSERT INTO compile_runs
    (run_id, service_id, layer, corpus_watermark, model, prompt_version,
     dry_run, tokens_in, tokens_out, cost_usd, duration_ms,
     sections_total, sections_compiled, sections_failed, claims_emitted, claims_rejected, ran_at,
     refused, refusal_reason, budget_stopped, escape_note, rejection_reasons_json,
     resumed_from, uncompiled_scopes_json, sections_prefiltered)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		run.RunID, run.ServiceID, run.Layer, run.CorpusWatermark,
		run.Model, run.PromptVersion, run.DryRun,
		run.TokensIn, run.TokensOut, run.CostUSD, run.DurationMS,
		run.SectionsTotal, run.SectionsCompiled, run.SectionsFailed,
		run.ClaimsEmitted, run.ClaimsRejected,
		run.RanAt.UnixNano(),
		run.Refused, run.RefusalReason, run.BudgetStopped, run.Escape,
		rejectionJSON, run.ResumedFrom, uncompiledJSON, run.SectionsPrefiltered,
	)
	if err != nil {
		return fmt.Errorf("RecordCompileRun: %w", err)
	}
	return nil
}

// CompileRunRefsAfter returns every compile run recorded after the one
// whose Seq is after (0: all), in record order (the table's rowid, an
// index range, never a scan) — the console's event cursor (RT-119) reads
// what is new, never every run of every service on every poll, and never
// misses a run recorded after a newer-started one.
func (s *SQLiteStore) CompileRunRefsAfter(ctx context.Context, after int64) ([]CompileRunRef, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT rowid, run_id, service_id, layer, ran_at
FROM compile_runs
WHERE rowid > ?
ORDER BY rowid
`, after)
	if err != nil {
		return nil, fmt.Errorf("CompileRunRefsAfter query: %w", err)
	}
	defer rows.Close()
	var out []CompileRunRef
	for rows.Next() {
		var r CompileRunRef
		var ranAt int64
		if err := rows.Scan(&r.Seq, &r.RunID, &r.ServiceID, &r.Layer, &ranAt); err != nil {
			return nil, fmt.Errorf("CompileRunRefsAfter scan: %w", err)
		}
		r.RanAt = time.Unix(0, ranAt).UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// CompileRuns returns compile runs for serviceID, optionally filtered by layer
// (layer "" returns all), newest-first by RanAt then run_id.
func (s *SQLiteStore) CompileRuns(ctx context.Context, serviceID, layer string) ([]core.CompileRun, error) {
	const cols = `run_id, service_id, layer, corpus_watermark, model, prompt_version,
	dry_run, tokens_in, tokens_out, cost_usd, duration_ms,
	sections_total, sections_compiled, sections_failed, claims_emitted, claims_rejected, ran_at,
	refused, refusal_reason, budget_stopped, escape_note, rejection_reasons_json,
	resumed_from, uncompiled_scopes_json, sections_prefiltered`
	var (
		rows *sql.Rows
		err  error
	)
	if layer == "" {
		rows, err = s.db.QueryContext(ctx, `
SELECT `+cols+`
FROM compile_runs
WHERE service_id = ?
ORDER BY ran_at DESC, run_id
`, serviceID)
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT `+cols+`
FROM compile_runs
WHERE service_id = ? AND layer = ?
ORDER BY ran_at DESC, run_id
`, serviceID, layer)
	}
	if err != nil {
		return nil, fmt.Errorf("CompileRuns query: %w", err)
	}
	defer rows.Close()
	var result []core.CompileRun
	for rows.Next() {
		var (
			r              core.CompileRun
			ranAt          int64
			rejectionJSON  string
			uncompiledJSON string
		)
		if err := rows.Scan(
			&r.RunID, &r.ServiceID, &r.Layer, &r.CorpusWatermark,
			&r.Model, &r.PromptVersion, &r.DryRun,
			&r.TokensIn, &r.TokensOut, &r.CostUSD, &r.DurationMS,
			&r.SectionsTotal, &r.SectionsCompiled, &r.SectionsFailed,
			&r.ClaimsEmitted, &r.ClaimsRejected, &ranAt,
			&r.Refused, &r.RefusalReason, &r.BudgetStopped, &r.Escape,
			&rejectionJSON, &r.ResumedFrom, &uncompiledJSON, &r.SectionsPrefiltered,
		); err != nil {
			return nil, fmt.Errorf("CompileRuns scan: %w", err)
		}
		r.UncompiledScopes = unmarshalStringSlice(uncompiledJSON)
		r.RanAt = time.Unix(0, ranAt).UTC()
		// Q-3: a legacy row (empty string — the pre-existing-store
		// migration default, or a row written before this column existed
		// at all) degrades to nil, never an error; a malformed value
		// (should not happen, but this is a lenient read like every other
		// JSON column in this file) degrades the same way rather than
		// aborting the whole CompileRuns call.
		r.RejectionReasons = unmarshalRejectionReasons(rejectionJSON)
		result = append(result, r)
	}
	return result, rows.Err()
}
