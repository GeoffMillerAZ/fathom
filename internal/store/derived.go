package store

// derived.go — ingest-time derived facts (docs/design/responsible-tenant.md
// §6, RT-76, RT-77, RT-160, invariant I48). Whole-repository facts come
// from ingest, not from request-time walks or whole-table loads:
//
//   - file_stats: one row per discovered file (size, mtime, inode, content
//     hash, kind, exclusion), replaced at each ingest's final commit.
//   - service_summary: the service's symbol count, source kinds,
//     unsupported languages, file count, bytes and exclusion counts,
//     written in the same transaction as service_meta, so its watermark is
//     the watermark readers key on.
//   - observation_calls: observations' calls, normalized one row per
//     (caller, callee key) with an index on the callee key and on the
//     callee's final name segment — a reverse lookup instead of a scan of
//     every service's observations. Kept by triggers on observations, so
//     every insert, delete (PurgeSource, purge, RemoveService) and rename
//     keeps it in step without each writer remembering to.
//
// service_summary and observation_calls are derivable from rows the store
// already holds, so a store written before them is backfilled by SQL at
// first use (BackfillDerivedTables: GROUP BY and json_each, one service
// per transaction, no content reads) rather than by re-ingesting the
// fleet. Only file_stats needs a walk and waits for the service's next
// ingest.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// FileStat is one file_stats row.
type FileStat struct {
	Path        string
	Size        int64
	MtimeNS     int64
	Ino         uint64
	ContentHash string
	// Lang is the file's source kind, or UnsupportedLangPrefix + the
	// language name for a known language with no extractor.
	Lang string
	// Excluded names why ingest did not extract it: "", "oversize",
	// "unreadable".
	Excluded string
}

// UnsupportedLangPrefix marks a FileStat.Lang naming a known programming
// language fathom has no extractor for (O69).
const UnsupportedLangPrefix = "unsupported:"

// Summary sources.
const (
	SummaryFromIngest   = "ingest"
	SummaryFromBackfill = "backfill"
)

// ServiceSummary is one service_summary row: what list_services and the
// console report about a service, derived once per ingest (or backfill).
type ServiceSummary struct {
	ServiceID string
	// Watermark is WatermarkOf(service_meta) when the row was written; a
	// row whose watermark is not the service's current one is stale.
	Watermark        string
	SymbolCount      int
	SourceKinds      []core.SourceKind
	UnsupportedLangs map[string]int
	FileCount        int
	// Bytes is the discovered files' total size; -1 when unknown (a
	// backfilled row: only an ingest stats the files).
	Bytes          int64
	ExcludedCounts map[string]int
	Source         string
	// HasFileStats reports whether an ingest has written file_stats rows
	// for the service (a backfill cannot).
	HasFileStats bool
	WrittenAt    int64
	// Filtered is set when a suppression applies to the service and the
	// counts were recomputed at read time without the hidden files.
	Filtered bool
}

// CallerRow is one CallersOfKeys hit: the calling observation and the
// callee key its calls list carries.
type CallerRow struct {
	ServiceID string
	Symbol    string
	Package   string
	SourceRef string
	Line      int
	CalleeKey string
}

// DerivedState is what the derived tables hold for a service.
type DerivedState struct {
	HasSummary       bool
	SummaryWatermark string
	// SummaryCurrent is true when the summary's watermark is the
	// service's current one.
	SummaryCurrent bool
	CallsIndexed   bool
	HasFileStats   bool
}

// BackfillOptions select what BackfillDerivedTables derives. Classify
// names a path's unsupported language by its NAME (connector.
// UnsupportedLanguageForPath) — the store reads no file.
type BackfillOptions struct {
	Summary  bool
	Calls    bool
	Classify func(path string) (lang string, ok bool)
	// NoWrite derives the summary into BackfillReport.Derived and writes
	// nothing (a read-only mount); Calls is ignored.
	NoWrite bool
}

// BackfillReport is what one backfill wrote.
type BackfillReport struct {
	Summary  bool // a summary row was (re)written
	CallRows int  // observation_calls rows written (0 when already indexed)
	// Derived is the summary derived (written or, under NoWrite, not);
	// nil when the row was current or none was derived.
	Derived *ServiceSummary
	// IngestInProgress is set when the service's ingest_state is
	// in_progress: its rows may be between an ingest's commits (in any
	// process), so no summary is derived for it.
	IngestInProgress bool
}

// WatermarkOf is the store watermark of a service's meta row: the token
// readers key caches on (pkg/client StoreWatermark) and the summary row
// records.
func WatermarkOf(meta ServiceMeta) string {
	return fmt.Sprintf("%s@%d", meta.IndexedCommit, meta.IndexedAt)
}

// calleeNameSQL is the final name segment of a callee key, in SQL: the
// tail after the last ':' ("svc::pkg.Name" → "pkg.Name"), then after the
// last '.' ("pkg.Name" → "Name"). rtrim(x, <x without the separator>)
// leaves x's prefix through its last separator.
func calleeNameSQL(x string) string {
	t1 := fmt.Sprintf("substr(%[1]s, length(rtrim(%[1]s, replace(%[1]s, ':', ''))) + 1)", x)
	return fmt.Sprintf("substr(%[1]s, length(rtrim(%[1]s, replace(%[1]s, '.', ''))) + 1)", t1)
}

// CalleeName is calleeNameSQL in Go (the memory backend and tests).
func CalleeName(key string) string {
	if i := strings.LastIndex(key, ":"); i >= 0 {
		key = key[i+1:]
	}
	if i := strings.LastIndex(key, "."); i >= 0 {
		key = key[i+1:]
	}
	return key
}

var derivedDDL = `
CREATE TABLE IF NOT EXISTS file_stats (
    service_id   TEXT    NOT NULL,
    path         TEXT    NOT NULL,
    size         INTEGER NOT NULL DEFAULT 0,
    mtime_ns     INTEGER NOT NULL DEFAULT 0,
    ino          INTEGER NOT NULL DEFAULT 0,
    content_hash TEXT    NOT NULL DEFAULT '',
    lang         TEXT    NOT NULL DEFAULT '',
    excluded     TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (service_id, path)
);

CREATE TABLE IF NOT EXISTS service_summary (
    service_id             TEXT    NOT NULL PRIMARY KEY,
    watermark              TEXT    NOT NULL DEFAULT '',
    symbol_count           INTEGER NOT NULL DEFAULT 0,
    source_kinds_json      TEXT    NOT NULL DEFAULT '[]',
    unsupported_langs_json TEXT    NOT NULL DEFAULT '{}',
    file_count             INTEGER NOT NULL DEFAULT 0,
    bytes                  INTEGER NOT NULL DEFAULT -1,
    excluded_counts_json   TEXT    NOT NULL DEFAULT '{}',
    source                 TEXT    NOT NULL DEFAULT '',
    written_at             INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS observation_calls (
    service_id   TEXT    NOT NULL,
    caller_rowid INTEGER NOT NULL,
    callee_key   TEXT    NOT NULL,
    callee_name  TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_observation_calls_key    ON observation_calls (callee_key);
CREATE INDEX IF NOT EXISTS idx_observation_calls_name   ON observation_calls (callee_name);
CREATE INDEX IF NOT EXISTS idx_observation_calls_caller ON observation_calls (caller_rowid);

-- A service's observation_calls rows are complete only once the service
-- has been indexed: a backfill, or an ingest into a store whose insert
-- trigger has existed since the store held no observation
-- (derived_meta calls_from_birth).
CREATE TABLE IF NOT EXISTS observation_calls_state (
    service_id TEXT    NOT NULL PRIMARY KEY,
    indexed_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS derived_meta (
    key   TEXT NOT NULL PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);

CREATE TRIGGER IF NOT EXISTS trg_observation_calls_ai AFTER INSERT ON observations
WHEN json_array_length(NEW.calls) > 0
BEGIN
    INSERT INTO observation_calls (service_id, caller_rowid, callee_key, callee_name)
    SELECT NEW.service_id, NEW.rowid, j.value, ` + calleeNameSQL("j.value") + `
    FROM json_each(NEW.calls) j WHERE j.type = 'text';
END;

CREATE TRIGGER IF NOT EXISTS trg_observation_calls_ad AFTER DELETE ON observations
WHEN json_array_length(OLD.calls) > 0
BEGIN
    DELETE FROM observation_calls WHERE caller_rowid = OLD.rowid;
END;

CREATE TRIGGER IF NOT EXISTS trg_observation_calls_au AFTER UPDATE OF service_id ON observations
BEGIN
    UPDATE observation_calls SET service_id = NEW.service_id WHERE caller_rowid = NEW.rowid;
END;
`

// callsFromBirthKey marks a store whose call-index insert trigger has
// existed since it held no observation: every calls row was captured, so
// an ingest can mark its service call-indexed without a backfill.
const callsFromBirthKey = "calls_from_birth"

func (s *SQLiteStore) createDerivedSchema() error {
	var had int
	if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'trigger' AND name = 'trg_observation_calls_ai'`).Scan(&had); err != nil {
		return err
	}
	if _, err := s.db.Exec(derivedDDL); err != nil {
		return err
	}
	if _, err := s.db.Exec(serviceRefsDDL); err != nil { // RT-89
		return err
	}
	if had == 0 {
		var any int
		if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM observations)`).Scan(&any); err != nil {
			return err
		}
		if any == 0 {
			if _, err := s.db.Exec(`INSERT OR REPLACE INTO derived_meta (key, value) VALUES (?, '1')`, callsFromBirthKey); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteServiceSummary is an ingest's final transaction (RT-76): it records
// meta (the watermark), replaces the service's file_stats with files, and
// writes the service_summary row — its symbol count and source kinds
// counted in the same transaction from the observations the ingest just
// wrote, its watermark WatermarkOf(meta). A service with no observations
// yet (the first ingest into it) is also marked call-indexed: the insert
// trigger wrote every calls row it has.
func (s *SQLiteStore) WriteServiceSummary(ctx context.Context, meta ServiceMeta, sum ServiceSummary, files []FileStat) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("WriteServiceSummary begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
INSERT INTO service_meta (service_id, path, indexed_commit, indexed_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(service_id) DO UPDATE SET
    path = excluded.path, indexed_commit = excluded.indexed_commit, indexed_at = excluded.indexed_at
`, meta.ServiceID, meta.Path, meta.IndexedCommit, meta.IndexedAt); err != nil {
		return fmt.Errorf("WriteServiceSummary meta: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM file_stats WHERE service_id = ?`, meta.ServiceID); err != nil {
		return fmt.Errorf("WriteServiceSummary clear file_stats: %w", err)
	}
	if len(files) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
INSERT OR REPLACE INTO file_stats (service_id, path, size, mtime_ns, ino, content_hash, lang, excluded)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("WriteServiceSummary prepare: %w", err)
		}
		for _, f := range files {
			if _, err := stmt.ExecContext(ctx, meta.ServiceID, f.Path, f.Size, f.MtimeNS, int64(f.Ino), f.ContentHash, f.Lang, f.Excluded); err != nil {
				_ = stmt.Close()
				return fmt.Errorf("WriteServiceSummary file_stats: %w", err)
			}
		}
		_ = stmt.Close()
	}
	sum.ServiceID = meta.ServiceID
	sum.Watermark = WatermarkOf(meta)
	sum.Source = SummaryFromIngest
	if sum.WrittenAt == 0 {
		sum.WrittenAt = time.Now().UnixNano()
	}
	count, kinds, err := symbolAggregateTx(ctx, tx, meta.ServiceID)
	if err != nil {
		return err
	}
	sum.SymbolCount, sum.SourceKinds = count, kinds
	if err := upsertSummaryTx(ctx, tx, sum); err != nil {
		return err
	}
	// The call index: complete from birth (the trigger saw every insert),
	// or derived now for a service a pre-trigger store — or a purge's
	// VACUUM — left unindexed, so a service is never silently missing from
	// CallersOfKeys until the next daemon start (I38).
	var indexed, birth int
	if err := tx.QueryRowContext(ctx, `
SELECT EXISTS(SELECT 1 FROM observation_calls_state WHERE service_id = ?),
       EXISTS(SELECT 1 FROM derived_meta WHERE key = ? AND value = '1')`,
		meta.ServiceID, callsFromBirthKey).Scan(&indexed, &birth); err != nil {
		return fmt.Errorf("WriteServiceSummary calls state: %w", err)
	}
	if indexed == 0 {
		if birth == 0 {
			if _, err := indexServiceCallsTx(ctx, tx, meta.ServiceID); err != nil {
				return fmt.Errorf("WriteServiceSummary: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO observation_calls_state (service_id, indexed_at) VALUES (?, ?)`, meta.ServiceID, sum.WrittenAt); err != nil {
			return fmt.Errorf("WriteServiceSummary calls mark: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("WriteServiceSummary commit: %w", err)
	}
	return nil
}

// RecountServiceSummary recounts serviceID's summary symbol count and
// source kinds in one transaction (a single-file refresh: same watermark,
// new rows). A no-op when no row exists.
func (s *SQLiteStore) RecountServiceSummary(ctx context.Context, serviceID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("RecountServiceSummary begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	count, kinds, err := symbolAggregateTx(ctx, tx, serviceID)
	if err != nil {
		return err
	}
	if kinds == nil {
		kinds = []core.SourceKind{}
	}
	kj, _ := json.Marshal(kinds)
	if _, err := tx.ExecContext(ctx, `UPDATE service_summary SET symbol_count = ?, source_kinds_json = ? WHERE service_id = ?`, count, string(kj), serviceID); err != nil {
		if isNoSuchTable(err) {
			return nil
		}
		return fmt.Errorf("RecountServiceSummary: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("RecountServiceSummary commit: %w", err)
	}
	return nil
}

// symbolAggregateTx counts the service's observations and their distinct
// source kinds (sorted) — the RT-144 aggregate, unfiltered.
func symbolAggregateTx(ctx context.Context, q queryer, serviceID string) (int, []core.SourceKind, error) {
	rows, err := q.QueryContext(ctx, `
SELECT source_kind, count(*) FROM observations WHERE service_id = ? GROUP BY source_kind`, serviceID)
	if err != nil {
		return 0, nil, fmt.Errorf("summary aggregate: %w", err)
	}
	defer rows.Close()
	var (
		total int
		kinds []core.SourceKind
	)
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return 0, nil, fmt.Errorf("summary aggregate scan: %w", err)
		}
		total += n
		kinds = append(kinds, core.SourceKind(k))
	}
	if err := rows.Err(); err != nil {
		return 0, nil, err
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
	return total, kinds, nil
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func upsertSummaryTx(ctx context.Context, tx *sql.Tx, sum ServiceSummary) error {
	kinds := sum.SourceKinds
	if kinds == nil {
		kinds = []core.SourceKind{}
	}
	kj, _ := json.Marshal(kinds)
	uj, _ := json.Marshal(nonNilCounts(sum.UnsupportedLangs))
	ej, _ := json.Marshal(nonNilCounts(sum.ExcludedCounts))
	_, err := tx.ExecContext(ctx, `
INSERT OR REPLACE INTO service_summary
    (service_id, watermark, symbol_count, source_kinds_json, unsupported_langs_json,
     file_count, bytes, excluded_counts_json, source, written_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sum.ServiceID, sum.Watermark, sum.SymbolCount, string(kj), string(uj),
		sum.FileCount, sum.Bytes, string(ej), sum.Source, sum.WrittenAt)
	if err != nil {
		return fmt.Errorf("service_summary upsert: %w", err)
	}
	return nil
}

func nonNilCounts(m map[string]int) map[string]int {
	if m == nil {
		return map[string]int{}
	}
	return m
}

// serviceSummaryRaw reads the service_summary row as written.
func (s *SQLiteStore) serviceSummaryRaw(ctx context.Context, serviceID string) (ServiceSummary, bool, error) {
	var (
		sum            ServiceSummary
		kj, uj, ej     string
		hasFS          int
		hasSummaryRows int
	)
	err := s.db.QueryRowContext(ctx, `
SELECT service_id, watermark, symbol_count, source_kinds_json, unsupported_langs_json,
       file_count, bytes, excluded_counts_json, source, written_at,
       EXISTS(SELECT 1 FROM file_stats WHERE service_id = ?), 1
FROM service_summary WHERE service_id = ?`, serviceID, serviceID).Scan(
		&sum.ServiceID, &sum.Watermark, &sum.SymbolCount, &kj, &uj,
		&sum.FileCount, &sum.Bytes, &ej, &sum.Source, &sum.WrittenAt, &hasFS, &hasSummaryRows)
	if err == sql.ErrNoRows || isNoSuchTable(err) {
		return ServiceSummary{}, false, nil
	}
	if err != nil {
		return ServiceSummary{}, false, fmt.Errorf("ServiceSummary: %w", err)
	}
	_ = json.Unmarshal([]byte(kj), &sum.SourceKinds)
	_ = json.Unmarshal([]byte(uj), &sum.UnsupportedLangs)
	_ = json.Unmarshal([]byte(ej), &sum.ExcludedCounts)
	if len(sum.UnsupportedLangs) == 0 {
		sum.UnsupportedLangs = nil
	}
	if len(sum.ExcludedCounts) == 0 {
		sum.ExcludedCounts = nil
	}
	sum.HasFileStats = hasFS == 1
	return sum, true, nil
}

// fileStatsRaw reads the service's file_stats rows, sorted by path.
func (s *SQLiteStore) fileStatsRaw(ctx context.Context, serviceID string) ([]FileStat, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT path, size, mtime_ns, ino, content_hash, lang, excluded
FROM file_stats WHERE service_id = ? ORDER BY path`, serviceID)
	if isNoSuchTable(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("FileStats: %w", err)
	}
	defer rows.Close()
	var out []FileStat
	for rows.Next() {
		var f FileStat
		var ino int64
		if err := rows.Scan(&f.Path, &f.Size, &f.MtimeNS, &ino, &f.ContentHash, &f.Lang, &f.Excluded); err != nil {
			return nil, fmt.Errorf("FileStats scan: %w", err)
		}
		f.Ino = uint64(ino)
		out = append(out, f)
	}
	return out, rows.Err()
}

// callersOfKeysChunk bounds one IN list.
const callersOfKeysChunk = 400

// callersOfKeysRaw is RT-77's indexed reverse lookup over call-indexed
// services: every observation whose calls carry a key whose full text, or
// whose final name segment, is one of keys — a candidate superset the
// caller narrows with its own match (matchesSymbol). The EXISTS re-checks
// the caller row still lists the key, so a row left behind by a VACUUM
// that renumbered rowids can never become a false hit.
func (s *SQLiteStore) callersOfKeysRaw(ctx context.Context, keys []string) ([]CallerRow, error) {
	keys = dedupeSorted(keys)
	var out []CallerRow
	for start := 0; start < len(keys); start += callersOfKeysChunk {
		chunk := keys[start:min(start+callersOfKeysChunk, len(keys))]
		ph := placeholders(len(chunk))
		args := make([]any, 0, 2*len(chunk))
		for _, k := range chunk {
			args = append(args, k)
		}
		for _, k := range chunk {
			args = append(args, k)
		}
		rows, err := s.db.QueryContext(ctx, `
SELECT o.service_id, o.symbol, o.package, o.source_ref, o.line, oc.callee_key
FROM observation_calls oc
JOIN observation_calls_state st ON st.service_id = oc.service_id
JOIN observations o ON o.rowid = oc.caller_rowid AND o.service_id = oc.service_id
WHERE (oc.callee_key IN (`+ph+`) OR oc.callee_name IN (`+ph+`))
  AND EXISTS (SELECT 1 FROM json_each(o.calls) j WHERE j.value = oc.callee_key)`, args...)
		if isNoSuchTable(err) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("CallersOfKeys: %w", err)
		}
		for rows.Next() {
			var r CallerRow
			if err := rows.Scan(&r.ServiceID, &r.Symbol, &r.Package, &r.SourceRef, &r.Line, &r.CalleeKey); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("CallersOfKeys scan: %w", err)
			}
			out = append(out, r)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	sortCallerRows(out)
	return dedupeCallerRows(out), nil
}

func sortCallerRows(rows []CallerRow) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.ServiceID != b.ServiceID {
			return a.ServiceID < b.ServiceID
		}
		if a.Symbol != b.Symbol {
			return a.Symbol < b.Symbol
		}
		if a.SourceRef != b.SourceRef {
			return a.SourceRef < b.SourceRef
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		return a.CalleeKey < b.CalleeKey
	})
}

func dedupeCallerRows(rows []CallerRow) []CallerRow {
	out := rows[:0]
	for i, r := range rows {
		if i > 0 && r == rows[i-1] {
			continue
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func dedupeSorted(keys []string) []string {
	out := append([]string(nil), keys...)
	sort.Strings(out)
	w := 0
	for i, k := range out {
		if k == "" || (i > 0 && k == out[i-1]) {
			continue
		}
		out[w] = k
		w++
	}
	return out[:w]
}

// DerivedState reports what the derived tables hold for serviceID.
func (s *SQLiteStore) DerivedState(ctx context.Context, serviceID string) (DerivedState, error) {
	var st DerivedState
	var calls, fs int
	var wm sql.NullString
	var cur sql.NullString
	err := s.db.QueryRowContext(ctx, `
SELECT (SELECT watermark FROM service_summary WHERE service_id = ?),
       EXISTS(SELECT 1 FROM observation_calls_state WHERE service_id = ?),
       EXISTS(SELECT 1 FROM file_stats WHERE service_id = ?),
       (SELECT indexed_commit || '@' || indexed_at FROM service_meta WHERE service_id = ?)`,
		serviceID, serviceID, serviceID, serviceID).Scan(&wm, &calls, &fs, &cur)
	if isNoSuchTable(err) {
		return st, nil
	}
	if err != nil {
		return st, fmt.Errorf("DerivedState: %w", err)
	}
	st.HasSummary = wm.Valid
	st.SummaryWatermark = wm.String
	st.SummaryCurrent = wm.Valid && cur.Valid && wm.String == cur.String
	st.CallsIndexed = calls == 1
	st.HasFileStats = fs == 1
	return st, nil
}

// BackfillDerivedTables derives serviceID's summary and call index from
// rows the store already holds (RT-160): the summary by GROUP BY over its
// observations and a name-only classification of its fingerprinted paths,
// the call index by json_each over its observations' calls — one
// transaction, no file read. A current summary and an indexed service are
// left alone. Callers hold the ingest slot (one backfill or ingest at a
// time in this process); a service whose ingest_state is in_progress —
// an ingest between its commits in any process, or one that was killed —
// gets no summary (IngestInProgress), never a count over half its rows.
func (s *SQLiteStore) BackfillDerivedTables(ctx context.Context, serviceID string, opts BackfillOptions) (BackfillReport, error) {
	var rep BackfillReport
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: opts.NoWrite})
	if err != nil {
		return rep, fmt.Errorf("BackfillDerivedTables begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if opts.Summary {
		var state sql.NullString
		err := tx.QueryRowContext(ctx, `SELECT state FROM ingest_state WHERE service_id = ?`, serviceID).Scan(&state)
		if err != nil && !errors.Is(err, sql.ErrNoRows) && !isNoSuchTable(err) {
			return rep, fmt.Errorf("BackfillDerivedTables ingest state: %w", err)
		}
		if state.String == IngestInProgress {
			rep.IngestInProgress = true
			opts.Summary = false
		}
	}
	if opts.NoWrite {
		opts.Calls = false
	}

	if opts.Summary {
		var (
			wm, cur sql.NullString
			path    sql.NullString
		)
		if err := tx.QueryRowContext(ctx, `
SELECT (SELECT watermark FROM service_summary WHERE service_id = ?),
       (SELECT indexed_commit || '@' || indexed_at FROM service_meta WHERE service_id = ?),
       (SELECT path FROM service_meta WHERE service_id = ?)`, serviceID, serviceID, serviceID).Scan(&wm, &cur, &path); err != nil {
			return rep, fmt.Errorf("BackfillDerivedTables state: %w", err)
		}
		if cur.Valid && (!wm.Valid || wm.String != cur.String) {
			sum := ServiceSummary{ServiceID: serviceID, Watermark: cur.String, Source: SummaryFromBackfill, Bytes: -1, WrittenAt: time.Now().UnixNano()}
			sum.SymbolCount, sum.SourceKinds, err = symbolAggregateTx(ctx, tx, serviceID)
			if err != nil {
				return rep, err
			}
			rows, err := tx.QueryContext(ctx, `SELECT source_ref FROM fingerprints WHERE service_id = ?`, serviceID)
			if err != nil {
				return rep, fmt.Errorf("BackfillDerivedTables refs: %w", err)
			}
			for rows.Next() {
				var ref string
				if err := rows.Scan(&ref); err != nil {
					_ = rows.Close()
					return rep, err
				}
				sum.FileCount++
				if opts.Classify != nil {
					if lang, ok := opts.Classify(ref); ok {
						if sum.UnsupportedLangs == nil {
							sum.UnsupportedLangs = map[string]int{}
						}
						sum.UnsupportedLangs[lang]++
					}
				}
			}
			err = rows.Err()
			_ = rows.Close()
			if err != nil {
				return rep, err
			}
			rep.Derived = &sum
			if opts.NoWrite {
				return rep, nil
			}
			if err := upsertSummaryTx(ctx, tx, sum); err != nil {
				return rep, err
			}
			rep.Summary = true
		}
	}
	if opts.Calls {
		var indexed int
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM observation_calls_state WHERE service_id = ?)`, serviceID).Scan(&indexed); err != nil {
			return rep, fmt.Errorf("BackfillDerivedTables calls state: %w", err)
		}
		if indexed == 0 {
			n, err := indexServiceCallsTx(ctx, tx, serviceID)
			if err != nil {
				return rep, fmt.Errorf("BackfillDerivedTables: %w", err)
			}
			rep.CallRows = n
			if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO observation_calls_state (service_id, indexed_at) VALUES (?, ?)`, serviceID, time.Now().UnixNano()); err != nil {
				return rep, fmt.Errorf("BackfillDerivedTables calls mark: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return rep, fmt.Errorf("BackfillDerivedTables commit: %w", err)
	}
	return rep, nil
}

// indexServiceCallsTx replaces serviceID's observation_calls rows with
// ones derived by json_each over its observations' calls (no file read).
// The caller marks the service indexed.
func indexServiceCallsTx(ctx context.Context, tx *sql.Tx, serviceID string) (int, error) {
	if _, err := tx.ExecContext(ctx, `DELETE FROM observation_calls WHERE service_id = ?`, serviceID); err != nil {
		return 0, fmt.Errorf("clear calls: %w", err)
	}
	res, err := tx.ExecContext(ctx, `
INSERT INTO observation_calls (service_id, caller_rowid, callee_key, callee_name)
SELECT o.service_id, o.rowid, j.value, `+calleeNameSQL("j.value")+`
FROM observations o, json_each(o.calls) j
WHERE o.service_id = ? AND json_array_length(o.calls) > 0 AND j.type = 'text'`, serviceID)
	if err != nil {
		return 0, fmt.Errorf("index calls: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// rebuildCallIndex re-derives every service's call index in place after a
// VACUUM, which may renumber observations' rowids (the index keys on
// them). It used to forget the index, and only a daemon restart's
// backfill rebuilt it — until then CallersOfKeys skipped every service,
// silently (I38). After the rebuild every stored observation's calls are
// indexed and the insert trigger captures every later one, so the store
// counts as indexed from birth. A crash midway leaves the services
// unindexed (the state rows go first), never wrongly indexed.
func rebuildCallIndex(ctx context.Context, conn *sql.Conn) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UnixNano()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM observation_calls_state`, nil},
		{`DELETE FROM derived_meta WHERE key = ?`, []any{callsFromBirthKey}},
		{`DELETE FROM observation_calls`, nil},
		{`INSERT INTO observation_calls (service_id, caller_rowid, callee_key, callee_name)
SELECT o.service_id, o.rowid, j.value, ` + calleeNameSQL("j.value") + `
FROM observations o, json_each(o.calls) j
WHERE json_array_length(o.calls) > 0 AND j.type = 'text'`, nil},
		{`INSERT OR REPLACE INTO observation_calls_state (service_id, indexed_at) SELECT DISTINCT service_id, ? FROM observations`, []any{now}},
		{`INSERT OR REPLACE INTO derived_meta (key, value) VALUES (?, '1')`, []any{callsFromBirthKey}},
	} {
		if _, err := tx.ExecContext(ctx, q.sql, q.args...); err != nil {
			if isNoSuchTable(err) {
				return nil
			}
			return err
		}
	}
	return tx.Commit() // one transaction: a reader sees the old state or the rebuilt index
}

// markCallIndexStale makes every service read as call-unindexed (their
// state rows and the from-birth mark go) ahead of a VACUUM that may
// renumber rowids; rebuildCallIndex restores both.
func markCallIndexStale(ctx context.Context, tx *sql.Tx) error {
	for _, q := range []string{`DELETE FROM observation_calls_state`, `DELETE FROM derived_meta WHERE key = ?`} {
		var args []any
		if strings.Contains(q, "?") {
			args = []any{callsFromBirthKey}
		}
		if _, err := tx.ExecContext(ctx, q, args...); err != nil && !isNoSuchTable(err) {
			return err
		}
	}
	return nil
}

// isNoSuchTable reports a read against a store that predates a derived
// table and cannot create it (a read-only mount).
func isNoSuchTable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such table")
}
