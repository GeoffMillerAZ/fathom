package store

// docindex.go — docs_fts / doc_units / doc_links (KR-1,
// docs/design/knowledge-retrieval.md §4, docs/plans/knowledge-retrieval.md
// wave 2). Additive to store format v0 (ADR-041): a new FTS5 virtual table
// plus its two metadata tables, following the same "own file, own
// createXSchema, main ddl block never edited" convention file_churn.go
// documents.
//
// SQLite-only, deliberately not part of the Store interface both backends
// implement: docs_fts is an SQLite FTS5 virtual table, and the design
// (§2 decision 1) is explicit that this feature lives in the one existing
// pure-Go store with no second index and no CGO. That is the same shape as
// the physical-purge/VACUUM path (internal/store/purge.go), which callers
// reach via a `st.(*store.SQLiteStore)` type assertion (cmd/fathom/purgecmd.go,
// cmd/fathom/scopecmd.go) rather than a method every Store implementer must
// carry — MemoryStore (session mode, unit tests) has no on-disk file to hold
// an FTS5 shadow-table set, and a from-scratch in-memory re-implementation of
// BM25 ranking would be a second, divergent ranking algorithm the design
// explicitly refuses (§11: "a separate search service or index file outside
// the store"). A MemoryStore-backed session simply has no `search kind:
// docs` results — grep-shaped `search` still works — never a panic: callers
// type-assert and treat "not a SQLiteStore" like "no docs indexed yet".
//
// Column order matters here in a way it does not for the other additive
// tables: SQLite FTS5's bm25() takes weight arguments POSITIONALLY, in
// column declaration order, including UNINDEXED columns (a weight given for
// an unindexed column is accepted but has no effect, since it carries no
// tokens to score). Design §5 calls for `bm25(docs_fts, 3.0, 1.0)` to weight
// heading_path 3:1 over body. For that literal call to do what §5 says, the
// two indexed columns must be declared FIRST, in (heading_path, body)
// order — declaring the UNINDEXED identity columns first (as §4's
// column-list sketch shows them) would instead weight service_id and path,
// which is a no-op since neither is indexed. This file declares
// (heading_path, body) first for that reason; every other column name and
// meaning in §4 is unchanged.
import (
	"context"
	"fmt"
	"strings"
)

// DocUnit is one row of doc_units plus the body text that only ever lives in
// the docs_fts shadow tables (doc_units itself carries no body column, per
// §4). ContentHash and UnitID are computed by the caller (the splitter,
// KR-2, and the hash-gated ingest, KR-8) — this file only persists what it
// is given.
type DocUnit struct {
	UnitID          string
	Path            string
	LineStart       int
	LineEnd         int
	HeadingPath     string
	Kind            string
	ContentHash     string
	IndexedAt       int64
	StaleByLocation bool // archive/ and tmp/ paths (§3), ranks below live docs, never dropped
	Body            string
}

// DocSearchHit is one docs_fts MATCH result: doc_units' metadata joined back
// to the match, plus the raw bm25 score (SQLite convention: LOWER is a
// BETTER match) and an FTS5 snippet() excerpt. RankDocs (KR-4) turns Score
// into a fused rank; this type carries the raw ingredient, not the fusion.
type DocSearchHit struct {
	UnitID          string
	Path            string
	LineStart       int
	LineEnd         int
	HeadingPath     string
	Kind            string
	StaleByLocation bool
	Score           float64
	Snippet         string
}

const docIndexDDL = `
CREATE VIRTUAL TABLE IF NOT EXISTS docs_fts USING fts5(
    heading_path,
    body,
    service_id UNINDEXED,
    path UNINDEXED,
    unit_id UNINDEXED,
    kind UNINDEXED,
    tokenize = 'porter unicode61'
);

CREATE TABLE IF NOT EXISTS doc_units (
    service_id        TEXT    NOT NULL,
    unit_id           TEXT    NOT NULL,
    path              TEXT    NOT NULL,
    line_start        INTEGER NOT NULL DEFAULT 0,
    line_end          INTEGER NOT NULL DEFAULT 0,
    heading_path      TEXT    NOT NULL DEFAULT '',
    kind              TEXT    NOT NULL DEFAULT '',
    content_hash      TEXT    NOT NULL DEFAULT '',
    indexed_at        BIGINT  NOT NULL DEFAULT 0,
    stale_by_location INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (service_id, unit_id)
);

CREATE INDEX IF NOT EXISTS idx_doc_units_service_path
    ON doc_units (service_id, path);

-- doc_links (KR-6, wired in a later commit): target_kind is symbol|path|key
-- (§6); valid_to is set, never deleted, when a link stops holding (the
-- governance-learning validity-window rule §4 cites). Created here now so
-- the whole wave-2 shape lands in one additive commit, per §4's own framing
-- ("docs_fts, doc_units and doc_links, additive to store format v0").
CREATE TABLE IF NOT EXISTS doc_links (
    service_id           TEXT    NOT NULL,
    unit_id               TEXT    NOT NULL,
    target_kind           TEXT    NOT NULL DEFAULT '',
    target                TEXT    NOT NULL DEFAULT '',
    line                  INTEGER NOT NULL DEFAULT 0,
    link_tier             TEXT    NOT NULL DEFAULT '',
    target_hash_at_link   TEXT    NOT NULL DEFAULT '',
    valid_from            BIGINT  NOT NULL DEFAULT 0,
    valid_to              BIGINT  NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_doc_links_service_target
    ON doc_links (service_id, target);
`

func (s *SQLiteStore) createDocIndexSchema() error {
	_, err := s.db.Exec(docIndexDDL)
	return err
}

// PutDocUnits replaces every doc_units/docs_fts row for (serviceID, path) in
// one transaction — the hash-gated ingest (KR-8) calls this only for a file
// whose content hash changed, never for the whole service (that would make
// a docs-only change re-index every file, which §8's cost table forbids).
// An empty units replaces the path's rows with nothing, which is exactly
// what a caller wanting to clear a path (e.g. before removing it, see
// DeleteDocUnitsForPath) can also reach directly.
func (s *SQLiteStore) PutDocUnits(ctx context.Context, serviceID, path string, units []DocUnit) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("PutDocUnits begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM doc_units WHERE service_id = ? AND path = ?`, serviceID, path); err != nil {
		return fmt.Errorf("PutDocUnits delete doc_units: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM docs_fts WHERE service_id = ? AND path = ?`, serviceID, path); err != nil {
		return fmt.Errorf("PutDocUnits delete docs_fts: %w", err)
	}

	if len(units) > 0 {
		unitStmt, err := tx.PrepareContext(ctx, `
INSERT INTO doc_units (service_id, unit_id, path, line_start, line_end, heading_path, kind, content_hash, indexed_at, stale_by_location)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("PutDocUnits prepare doc_units: %w", err)
		}
		defer unitStmt.Close()

		ftsStmt, err := tx.PrepareContext(ctx, `
INSERT INTO docs_fts (heading_path, body, service_id, path, unit_id, kind)
VALUES (?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("PutDocUnits prepare docs_fts: %w", err)
		}
		defer ftsStmt.Close()

		for _, u := range units {
			if u.UnitID == "" {
				return fmt.Errorf("PutDocUnits: unit with empty UnitID for path %q", path)
			}
			stale := 0
			if u.StaleByLocation {
				stale = 1
			}
			if _, err := unitStmt.ExecContext(ctx, serviceID, u.UnitID, path, u.LineStart, u.LineEnd, u.HeadingPath, u.Kind, u.ContentHash, u.IndexedAt, stale); err != nil {
				return fmt.Errorf("PutDocUnits insert doc_units: %w", err)
			}
			if _, err := ftsStmt.ExecContext(ctx, u.HeadingPath, u.Body, serviceID, path, u.UnitID, u.Kind); err != nil {
				return fmt.Errorf("PutDocUnits insert docs_fts: %w", err)
			}
		}
	}

	return tx.Commit()
}

// DeleteDocUnitsForPath removes every doc_units/docs_fts row for
// (serviceID, path) — what the hash-gated ingest (KR-8) calls in the same
// transaction shape when a previously-indexed doc file has been deleted
// from the corpus (§4: "a deleted file's units are removed in the same
// transaction").
func (s *SQLiteStore) DeleteDocUnitsForPath(ctx context.Context, serviceID, path string) error {
	return s.PutDocUnits(ctx, serviceID, path, nil)
}

// DocUnitsForPath returns serviceID's stored doc_units rows for path,
// ordered by line_start — used by the hash-gated ingest to compare a file's
// freshly split units against what is already stored, and by tests.
func (s *SQLiteStore) DocUnitsForPath(ctx context.Context, serviceID, path string) ([]DocUnit, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT unit_id, line_start, line_end, heading_path, kind, content_hash, indexed_at, stale_by_location
FROM doc_units WHERE service_id = ? AND path = ? ORDER BY line_start`, serviceID, path)
	if err != nil {
		return nil, fmt.Errorf("DocUnitsForPath query: %w", err)
	}
	defer rows.Close()
	var out []DocUnit
	for rows.Next() {
		var u DocUnit
		var stale int
		if err := rows.Scan(&u.UnitID, &u.LineStart, &u.LineEnd, &u.HeadingPath, &u.Kind, &u.ContentHash, &u.IndexedAt, &stale); err != nil {
			return nil, fmt.Errorf("DocUnitsForPath scan: %w", err)
		}
		u.Path = path
		u.StaleByLocation = stale != 0
		out = append(out, u)
	}
	return out, rows.Err()
}

// DocIndexedPaths returns every distinct path with rows in doc_units for
// serviceID, sorted. The hash-gated ingest (KR-8) diffs this against a fresh
// corpus walk to find a deleted file: a path indexed before that the walk no
// longer finds, whose units must be removed in the same ingest.
func (s *SQLiteStore) DocIndexedPaths(ctx context.Context, serviceID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT DISTINCT path FROM doc_units WHERE service_id = ? ORDER BY path`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("DocIndexedPaths query: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("DocIndexedPaths scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DocLinkMatch is one doc_units unit with at least one still-holding
// (valid_to = 0) doc_links row naming one of RankDocs' concernSymbols
// targets, and how many distinct such targets it names. KR-4's fusion uses
// MatchCount only to re-order a unit already in the BM25 candidate set —
// never to admit a unit the text query itself did not match.
type DocLinkMatch struct {
	UnitID     string
	MatchCount int
}

// DocUnitsLinkedToTargets returns, for serviceID, every doc_units unit_id
// with a still-holding doc_links row naming one of targets, most distinct
// targets first, ties broken by unit_id for determinism. Empty targets
// returns nil without running a query — doc_links is written by KR-6 (not
// yet landed as of this call's introduction in KR-4); reading it before any
// row exists is simply an empty result, never an error.
func (s *SQLiteStore) DocUnitsLinkedToTargets(ctx context.Context, serviceID string, targets []string) ([]DocLinkMatch, error) {
	if len(targets) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(targets))
	args := make([]any, 0, len(targets)+1)
	args = append(args, serviceID)
	for i, t := range targets {
		placeholders[i] = "?"
		args = append(args, t)
	}
	q := fmt.Sprintf(`
SELECT unit_id, COUNT(DISTINCT target) AS n
FROM doc_links
WHERE service_id = ? AND valid_to = 0 AND target IN (%s)
GROUP BY unit_id
ORDER BY n DESC, unit_id ASC`, strings.Join(placeholders, ","))
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("DocUnitsLinkedToTargets: %w", err)
	}
	defer rows.Close()
	var out []DocLinkMatch
	for rows.Next() {
		var m DocLinkMatch
		if err := rows.Scan(&m.UnitID, &m.MatchCount); err != nil {
			return nil, fmt.Errorf("DocUnitsLinkedToTargets scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DocLink is one fresh, resolved span (internal/docindex.LinkDocSpans,
// KR-6) ready to reconcile into doc_links for one unit —
// SyncDocLinksForUnit's input shape. It mirrors internal/docindex.Link
// field-for-field but lives here, never imported from that package, since
// internal/docindex already imports internal/store (KR-1/KR-4/KR-8) and a
// reverse import would cycle.
type DocLink struct {
	TargetKind       string
	Target           string
	Line             int
	LinkTier         string
	TargetHashAtLink string
}

// DocLinkView is one still-open (valid_to = 0) doc_links row joined back
// to its unit's location — everything KR-7's docDrift needs to report a
// possibly-stale span without a second query per row.
type DocLinkView struct {
	UnitID           string
	Path             string
	HeadingPath      string
	TargetKind       string
	Target           string
	Line             int
	LinkTier         string
	TargetHashAtLink string
	ValidFrom        int64
}

// docLinkKey identifies one doc_links row for the purpose of deciding
// whether a fresh link is the SAME link as an already-open one (§4's
// validity-window rule applied per link, not per unit): the tuple a
// human/agent would call "the same span naming the same thing".
type docLinkKey struct {
	kind   string
	target string
	line   int
}

// SyncDocLinksForUnit reconciles unitID's doc_links rows against fresh —
// the set LinkDocSpans (KR-6) computed for the unit's CURRENT body — under
// §4's validity-window rule, applied per link:
//
//   - A fresh link whose (target_kind, target, line) matches an already-open
//     row is left COMPLETELY untouched: its target_hash_at_link and
//     valid_from never move. This is the point — KR-7's docDrift compares
//     that frozen link-time hash against a fresh resolution to notice the
//     target moved; overwriting it on every re-ingest would make drift
//     undetectable.
//   - A fresh link with no matching open row is inserted, valid_from = now,
//     valid_to = 0.
//   - An open row with no matching fresh link (the span was edited out of
//     the doc, or its target no longer resolves) is CLOSED: valid_to = now.
//     Never deleted (§4).
func (s *SQLiteStore) SyncDocLinksForUnit(ctx context.Context, serviceID, unitID string, fresh []DocLink, now int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("SyncDocLinksForUnit begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
SELECT target_kind, target, line FROM doc_links
WHERE service_id = ? AND unit_id = ? AND valid_to = 0`, serviceID, unitID)
	if err != nil {
		return fmt.Errorf("SyncDocLinksForUnit query open: %w", err)
	}
	open := map[docLinkKey]bool{}
	for rows.Next() {
		var k docLinkKey
		if err := rows.Scan(&k.kind, &k.target, &k.line); err != nil {
			rows.Close()
			return fmt.Errorf("SyncDocLinksForUnit scan open: %w", err)
		}
		open[k] = true
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return fmt.Errorf("SyncDocLinksForUnit rows: %w", rowsErr)
	}

	freshKeys := make(map[docLinkKey]bool, len(fresh))
	for _, f := range fresh {
		freshKeys[docLinkKey{f.TargetKind, f.Target, f.Line}] = true
	}

	closeStmt, err := tx.PrepareContext(ctx, `
UPDATE doc_links SET valid_to = ?
WHERE service_id = ? AND unit_id = ? AND target_kind = ? AND target = ? AND line = ? AND valid_to = 0`)
	if err != nil {
		return fmt.Errorf("SyncDocLinksForUnit prepare close: %w", err)
	}
	defer closeStmt.Close()
	for k := range open {
		if freshKeys[k] {
			continue
		}
		if _, err := closeStmt.ExecContext(ctx, now, serviceID, unitID, k.kind, k.target, k.line); err != nil {
			return fmt.Errorf("SyncDocLinksForUnit close: %w", err)
		}
	}

	insertStmt, err := tx.PrepareContext(ctx, `
INSERT INTO doc_links (service_id, unit_id, target_kind, target, line, link_tier, target_hash_at_link, valid_from, valid_to)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)`)
	if err != nil {
		return fmt.Errorf("SyncDocLinksForUnit prepare insert: %w", err)
	}
	defer insertStmt.Close()
	for _, f := range fresh {
		k := docLinkKey{f.TargetKind, f.Target, f.Line}
		if open[k] {
			continue
		}
		if _, err := insertStmt.ExecContext(ctx, serviceID, unitID, f.TargetKind, f.Target, f.Line, f.LinkTier, f.TargetHashAtLink, now); err != nil {
			return fmt.Errorf("SyncDocLinksForUnit insert: %w", err)
		}
	}

	return tx.Commit()
}

// OpenDocLinks returns every still-open (valid_to = 0) doc_links row for
// serviceID, joined to its unit's path and heading path, sorted by path
// then line then target (I2/determinism). KR-7's docDrift is the caller:
// it re-resolves each row's target and compares the fresh hash against
// TargetHashAtLink.
func (s *SQLiteStore) OpenDocLinks(ctx context.Context, serviceID string) ([]DocLinkView, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT dl.unit_id, du.path, du.heading_path, dl.target_kind, dl.target, dl.line, dl.link_tier, dl.target_hash_at_link, dl.valid_from
FROM doc_links dl
JOIN doc_units du ON du.service_id = dl.service_id AND du.unit_id = dl.unit_id
WHERE dl.service_id = ? AND dl.valid_to = 0
ORDER BY du.path, dl.line, dl.target`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("OpenDocLinks: %w", err)
	}
	defer rows.Close()
	var out []DocLinkView
	for rows.Next() {
		var v DocLinkView
		if err := rows.Scan(&v.UnitID, &v.Path, &v.HeadingPath, &v.TargetKind, &v.Target, &v.Line, &v.LinkTier, &v.TargetHashAtLink, &v.ValidFrom); err != nil {
			return nil, fmt.Errorf("OpenDocLinks scan: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// QueryDocUnits runs `docs_fts MATCH query` bounded to serviceID, ordered by
// bm25 (best match first — SQLite's bm25() is a cost, lower is better,
// hence the plain ascending ORDER BY), LIMIT limit (§5: 50 is the caller's
// convention, not enforced here so tests can ask for fewer). A malformed
// FTS5 query string (unbalanced quotes, a bare operator) is returned as an
// error, never a silent empty result — I38's "unreadable is never a healthy
// zero" applies to a query the engine could not run, not only to a file it
// could not read.
func (s *SQLiteStore) QueryDocUnits(ctx context.Context, serviceID, query string, limit int) ([]DocSearchHit, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT du.unit_id, du.path, du.line_start, du.line_end, du.heading_path, du.kind, du.stale_by_location,
       bm25(docs_fts, 3.0, 1.0) AS score,
       snippet(docs_fts, 1, '[', ']', '...', 10)
FROM docs_fts
JOIN doc_units du ON du.service_id = docs_fts.service_id AND du.unit_id = docs_fts.unit_id
WHERE docs_fts.service_id = ? AND docs_fts MATCH ?
ORDER BY score
LIMIT ?`, serviceID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("QueryDocUnits: %w", err)
	}
	defer rows.Close()
	var out []DocSearchHit
	for rows.Next() {
		var h DocSearchHit
		var stale int
		if err := rows.Scan(&h.UnitID, &h.Path, &h.LineStart, &h.LineEnd, &h.HeadingPath, &h.Kind, &stale, &h.Score, &h.Snippet); err != nil {
			return nil, fmt.Errorf("QueryDocUnits scan: %w", err)
		}
		h.StaleByLocation = stale != 0
		out = append(out, h)
	}
	return out, rows.Err()
}
