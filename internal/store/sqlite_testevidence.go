package store

// sqlite_testevidence.go — test_suites, test_runs, test_cases and
// certify_declarations (vision 33 / ADR-043 stage 0, TC-2/TC-5). Kept in
// its own file, added by its own exec right after the main schema const
// (sqlite.go's createSchema), per the additive-table convention documented
// there (sqlite_reviewruns.go's precedent) — the main ddl const is never
// edited for a new table.
//
// These tables are ADDITIVE and OPTIONAL: not part of store-format-v0's
// required L0/Writer schema (conformance.go's requiredSchemaV0). A store
// with no test evidence ever ingested simply has empty tables.

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const testEvidenceDDL = `
CREATE TABLE IF NOT EXISTS test_suites (
    service_id         TEXT NOT NULL,
    component          TEXT NOT NULL DEFAULT '',
    suite              TEXT NOT NULL DEFAULT '',
    class              TEXT NOT NULL DEFAULT '',
    tier               TEXT NOT NULL DEFAULT '',
    port               TEXT NOT NULL DEFAULT '',
    declared_certifies TEXT NOT NULL DEFAULT '[]',
    watermark          TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (service_id, component, suite)
);

CREATE TABLE IF NOT EXISTS test_runs (
    service_id   TEXT NOT NULL,
    run_digest   TEXT NOT NULL DEFAULT '',
    component    TEXT NOT NULL DEFAULT '',
    suite        TEXT NOT NULL DEFAULT '',
    started_at   TEXT NOT NULL DEFAULT '',
    verdict      TEXT NOT NULL DEFAULT '',
    totals       TEXT NOT NULL DEFAULT '{}',
    coverage     TEXT NOT NULL DEFAULT '',
    signer_tier  TEXT NOT NULL DEFAULT '',
    watermark    TEXT NOT NULL DEFAULT '',
    ingested_at  BIGINT NOT NULL DEFAULT 0,
    commit_sha   TEXT NOT NULL DEFAULT '',
    dirty        INTEGER NOT NULL DEFAULT 0,
    race         INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (service_id, run_digest, suite)
);

CREATE TABLE IF NOT EXISTS test_cases (
    service_id      TEXT NOT NULL,
    run_digest      TEXT NOT NULL DEFAULT '',
    suite           TEXT NOT NULL DEFAULT '',
    case_id         TEXT NOT NULL DEFAULT '',
    name            TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT '',
    attempts        INTEGER NOT NULL DEFAULT 1,
    duration_ms     BIGINT NOT NULL DEFAULT 0,
    failure         TEXT NOT NULL DEFAULT '',
    test_symbol_key TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (service_id, run_digest, suite, case_id)
);

CREATE INDEX IF NOT EXISTS idx_test_runs_service_suite
    ON test_runs (service_id, suite);
CREATE INDEX IF NOT EXISTS idx_test_cases_service_run
    ON test_cases (service_id, run_digest, suite);

CREATE TABLE IF NOT EXISTS certify_declarations (
    service_id      TEXT NOT NULL,
    test_symbol_key TEXT NOT NULL DEFAULT '',
    kind            TEXT NOT NULL DEFAULT '',
    target_id       TEXT NOT NULL DEFAULT '',
    strength        TEXT NOT NULL DEFAULT '',
    file            TEXT NOT NULL DEFAULT '',
    line            INTEGER NOT NULL DEFAULT 0,
    watermark       TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_certify_declarations_service_id
    ON certify_declarations (service_id);
`

// createTestEvidenceSchema creates the test-evidence + certify-declaration
// tables. Called once from createSchema, right after review_runs.
func (s *SQLiteStore) createTestEvidenceSchema() error {
	_, err := s.db.Exec(testEvidenceDDL)
	return err
}

// PutTestSuites replaces serviceID's entire declared-suite set — see the
// Store interface doc comment for the wholesale-replace contract.
func (s *SQLiteStore) PutTestSuites(ctx context.Context, serviceID string, suites []TestSuite) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("PutTestSuites begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM test_suites WHERE service_id = ?`, serviceID); err != nil {
		return fmt.Errorf("PutTestSuites delete: %w", err)
	}
	if len(suites) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
INSERT INTO test_suites (service_id, component, suite, class, tier, port, declared_certifies, watermark)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`)
		if err != nil {
			return fmt.Errorf("PutTestSuites prepare: %w", err)
		}
		defer stmt.Close()
		for _, su := range suites {
			certifies := su.DeclaredCertifiesJSON
			if certifies == "" {
				certifies = "[]"
			}
			if _, err := stmt.ExecContext(ctx, serviceID, su.Component, su.Suite, su.Class, su.Tier, su.Port, certifies, su.Watermark); err != nil {
				return fmt.Errorf("PutTestSuites exec: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("PutTestSuites commit: %w", err)
	}
	return nil
}

// PutTestRuns appends runs (with their embedded cases) — see the Store
// interface doc comment for the append/idempotent-by-digest contract.
func (s *SQLiteStore) PutTestRuns(ctx context.Context, serviceID string, runs []TestRun) error {
	if len(runs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("PutTestRuns begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	runStmt, err := tx.PrepareContext(ctx, `
INSERT OR IGNORE INTO test_runs
    (service_id, run_digest, component, suite, started_at, verdict, totals, coverage, signer_tier, watermark, ingested_at, commit_sha, dirty, race)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`)
	if err != nil {
		return fmt.Errorf("PutTestRuns prepare run: %w", err)
	}
	defer runStmt.Close()

	caseStmt, err := tx.PrepareContext(ctx, `
INSERT OR IGNORE INTO test_cases
    (service_id, run_digest, suite, case_id, name, status, attempts, duration_ms, failure, test_symbol_key)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`)
	if err != nil {
		return fmt.Errorf("PutTestRuns prepare case: %w", err)
	}
	defer caseStmt.Close()

	now := time.Now().UnixNano()
	for _, r := range runs {
		if _, err := runStmt.ExecContext(ctx, serviceID, r.RunDigest, r.Component, r.Suite, r.StartedAt, r.Verdict, r.TotalsJSON, r.CoverageJSON, r.SignerTier, r.Watermark, now, r.Commit, r.Dirty, r.Race); err != nil {
			return fmt.Errorf("PutTestRuns exec run: %w", err)
		}
		for _, c := range r.Cases {
			if _, err := caseStmt.ExecContext(ctx, serviceID, r.RunDigest, r.Suite, c.CaseID, c.Name, c.Status, c.Attempts, c.DurationMS, c.FailureJSON, c.TestSymbolKey); err != nil {
				return fmt.Errorf("PutTestRuns exec case: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("PutTestRuns commit: %w", err)
	}
	return nil
}

// TestSuiteStatuses returns every declared suite for serviceID with its
// most-recently-ingested run and that run's join stats — see the Store
// interface doc comment.
func (s *SQLiteStore) TestSuiteStatuses(ctx context.Context, serviceID string) ([]TestSuiteStatus, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT service_id, component, suite, class, tier, port, declared_certifies, watermark
FROM test_suites
WHERE service_id = ?
ORDER BY component, suite
`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("TestSuiteStatuses query suites: %w", err)
	}
	var out []TestSuiteStatus
	var suites []TestSuite
	for rows.Next() {
		var su TestSuite
		su.ServiceID = serviceID
		if err := rows.Scan(&su.ServiceID, &su.Component, &su.Suite, &su.Class, &su.Tier, &su.Port, &su.DeclaredCertifiesJSON, &su.Watermark); err != nil {
			rows.Close()
			return nil, fmt.Errorf("TestSuiteStatuses scan suite: %w", err)
		}
		suites = append(suites, su)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("TestSuiteStatuses rows: %w", err)
	}
	rows.Close()

	for _, su := range suites {
		status := TestSuiteStatus{TestSuite: su}

		var (
			runDigest, component, suiteName, startedAt, verdict, totalsJSON, coverageJSON, signerTier, watermark string
			ingestedAt                                                                                           int64
		)
		err := s.db.QueryRowContext(ctx, `
SELECT run_digest, component, suite, started_at, verdict, totals, coverage, signer_tier, watermark, ingested_at
FROM test_runs
WHERE service_id = ? AND suite = ?
ORDER BY started_at DESC, ingested_at DESC
LIMIT 1
`, serviceID, su.Suite).Scan(&runDigest, &component, &suiteName, &startedAt, &verdict, &totalsJSON, &coverageJSON, &signerTier, &watermark, &ingestedAt)
		switch {
		case err == sql.ErrNoRows:
			// No run ever ingested for this suite — HasRun stays false.
		case err != nil:
			return nil, fmt.Errorf("TestSuiteStatuses query last run: %w", err)
		default:
			status.HasRun = true
			status.LastRun = TestRun{
				ServiceID: serviceID, RunDigest: runDigest, Component: component, Suite: suiteName,
				StartedAt: startedAt, Verdict: verdict, TotalsJSON: totalsJSON, CoverageJSON: coverageJSON,
				SignerTier: signerTier, Watermark: watermark,
			}
			var total, joined int
			cerr := s.db.QueryRowContext(ctx, `
SELECT COUNT(*), SUM(CASE WHEN test_symbol_key != '' THEN 1 ELSE 0 END)
FROM test_cases
WHERE service_id = ? AND run_digest = ? AND suite = ?
`, serviceID, runDigest, suiteName).Scan(&total, &joined)
			if cerr != nil && cerr != sql.ErrNoRows {
				return nil, fmt.Errorf("TestSuiteStatuses query case counts: %w", cerr)
			}
			status.CasesTotal = total
			status.CasesJoined = joined
		}
		out = append(out, status)
	}
	return out, nil
}

// PutCertifyDeclarations replaces serviceID's entire certify_declarations
// set — see the Store interface doc comment for the wholesale-replace
// contract.
func (s *SQLiteStore) PutCertifyDeclarations(ctx context.Context, serviceID string, decls []CertifyDeclaration) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("PutCertifyDeclarations begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM certify_declarations WHERE service_id = ?`, serviceID); err != nil {
		return fmt.Errorf("PutCertifyDeclarations delete: %w", err)
	}
	if len(decls) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
INSERT INTO certify_declarations (service_id, test_symbol_key, kind, target_id, strength, file, line, watermark)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`)
		if err != nil {
			return fmt.Errorf("PutCertifyDeclarations prepare: %w", err)
		}
		defer stmt.Close()
		for _, d := range decls {
			if _, err := stmt.ExecContext(ctx, serviceID, d.TestSymbolKey, d.Kind, d.TargetID, d.Strength, d.File, d.Line, d.Watermark); err != nil {
				return fmt.Errorf("PutCertifyDeclarations exec: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("PutCertifyDeclarations commit: %w", err)
	}
	return nil
}

// TestCaseRunsForSymbol returns testSymbolKey's case observations across
// every attested run for serviceID, most-recent-run-first — see the Store
// interface doc comment (vision 33 stage 1, TC-6/TC-8).
func (s *SQLiteStore) TestCaseRunsForSymbol(ctx context.Context, serviceID, testSymbolKey string, limit int) ([]TestCaseRun, error) {
	query := `
SELECT tc.run_digest, tc.suite, tr.component, tr.started_at, tc.status, tc.attempts, tr.ingested_at, tr.commit_sha, tr.dirty, tr.race
FROM test_cases tc
JOIN test_runs tr ON tr.service_id = tc.service_id AND tr.run_digest = tc.run_digest AND tr.suite = tc.suite
WHERE tc.service_id = ? AND tc.test_symbol_key = ?
ORDER BY tr.started_at DESC, tr.ingested_at DESC
`
	args := []any{serviceID, testSymbolKey}
	if limit > 0 {
		query += `LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("TestCaseRunsForSymbol query: %w", err)
	}
	defer rows.Close()
	var out []TestCaseRun
	for rows.Next() {
		var r TestCaseRun
		if err := rows.Scan(&r.RunDigest, &r.Suite, &r.Component, &r.StartedAt, &r.Status, &r.Attempts, &r.IngestedAt, &r.Commit, &r.Dirty, &r.Race); err != nil {
			return nil, fmt.Errorf("TestCaseRunsForSymbol scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("TestCaseRunsForSymbol rows: %w", err)
	}
	return out, nil
}

// certifyDeclarationsForServiceRaw returns every declared certification fact
// for serviceID, ordered by (file, line).
func (s *SQLiteStore) certifyDeclarationsForServiceRaw(ctx context.Context, serviceID string) ([]CertifyDeclaration, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT service_id, test_symbol_key, kind, target_id, strength, file, line, watermark
FROM certify_declarations
WHERE service_id = ?
ORDER BY file, line
`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("CertifyDeclarationsForService query: %w", err)
	}
	defer rows.Close()
	var out []CertifyDeclaration
	for rows.Next() {
		var d CertifyDeclaration
		if err := rows.Scan(&d.ServiceID, &d.TestSymbolKey, &d.Kind, &d.TargetID, &d.Strength, &d.File, &d.Line, &d.Watermark); err != nil {
			return nil, fmt.Errorf("CertifyDeclarationsForService scan: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("CertifyDeclarationsForService rows: %w", err)
	}
	return out, nil
}
