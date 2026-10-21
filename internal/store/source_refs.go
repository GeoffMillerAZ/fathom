package store

// Source reference facts are scoped to an indexed ingest generation. A nil
// Refs means the bytes were not examined; an empty non-nil slice means they
// were examined and contained no references. Neither is inferred from symbols.

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

const SourceRefExtractorVersion = "sdd-default-refs-v1"
const SourceRefScopeIngestGeneration = "ingest_generation"

type SourceRefFact struct {
	Path             string
	SourceHash       string // SHA-256 of the exact fetched bytes, independent of extractor epoch
	ExtractorVersion string
	Refs             []string
	Excluded         string
}

type SourceRefProjection struct {
	ServiceID                string
	Scope                    string // ingest_generation on a read
	CurrentWorktreeFreshness string // unknown on a read
	Watermark                string
	Root                     string
	Policy                   string
	ExtractorVersion         string
	MembershipHash           string
	ProducedAt               int64
	Complete                 bool
	Rows                     []SourceRefFact
}

type SourceRefProjectionReader interface {
	ReadSourceRefProjection(context.Context, string) (SourceRefProjection, bool, error)
}

// SourceRefProjectionBackfillReader is ingest-only: call after Discover and
// before the first content write. It may read the previous sealed generation
// while the new run has already marked ingest_state in_progress.
type SourceRefProjectionBackfillReader interface {
	ReadSourceRefProjectionForBackfill(context.Context, string) (SourceRefProjection, bool, error)
}

type SourceRefProjectionInvalidator interface {
	InvalidateSourceRefProjection(context.Context, string) error
}

func (s *SQLiteStore) InvalidateSourceRefProjection(ctx context.Context, service string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM source_ref_seals WHERE service_id=?`, service)
	return err
}

func (m *MemoryStore) InvalidateSourceRefProjection(_ context.Context, service string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sourceRefProjections, service)
	return nil
}

// SourceRefMembershipHash seals ordered membership and every per-file fact.
// JSON encoding avoids ambiguous concatenations.
func SourceRefMembershipHash(rows []SourceRefFact) string {
	copyRows := append([]SourceRefFact(nil), rows...)
	sort.Slice(copyRows, func(i, j int) bool { return copyRows[i].Path < copyRows[j].Path })
	b, _ := json.Marshal(copyRows)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func validSourceRefProjection(p SourceRefProjection) bool {
	if p.ExtractorVersion != SourceRefExtractorVersion || p.MembershipHash != SourceRefMembershipHash(p.Rows) || p.Policy == "" || p.Root == "" || p.Watermark == "" {
		return false
	}
	excluded := 0
	for i, row := range p.Rows {
		if row.Path == "" || row.ExtractorVersion != p.ExtractorVersion || (i > 0 && p.Rows[i-1].Path >= row.Path) {
			return false
		}
		if row.Excluded == "" && (row.SourceHash == "" || row.Refs == nil) {
			return false
		}
		if row.SourceHash != "" {
			decoded, err := hex.DecodeString(row.SourceHash)
			if err != nil || len(decoded) != sha256.Size {
				return false
			}
		}
		if row.Excluded != "" && row.Refs != nil {
			return false
		}
		if row.Excluded != "" {
			excluded++
		}
		if p.Complete && row.Excluded != "" {
			return false
		}
	}
	return p.Complete == (excluded == 0)
}

// ValidSourceRefProjection is the read-side integrity check for optional
// consumers. It checks the seal and every fact without consulting live files.
func ValidSourceRefProjection(p SourceRefProjection) bool { return validSourceRefProjection(p) }

func writeSourceRefProjectionTx(ctx context.Context, tx *sql.Tx, meta ServiceMeta, files []FileStat, p *SourceRefProjection) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM source_ref_seals WHERE service_id = ?`, meta.ServiceID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM source_ref_facts WHERE service_id = ?`, meta.ServiceID); err != nil {
		return err
	}
	if p == nil {
		return nil
	}
	p.ServiceID, p.Watermark, p.Root = meta.ServiceID, WatermarkOf(meta), meta.Path
	p.MembershipHash = SourceRefMembershipHash(p.Rows)
	if !validSourceRefProjection(*p) {
		return fmt.Errorf("invalid source reference projection")
	}
	if len(files) != len(p.Rows) {
		return fmt.Errorf("source reference membership differs from file_stats")
	}
	paths := make([]FileStat, 0, len(files))
	for _, file := range files {
		paths = append(paths, file)
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].Path < paths[j].Path })
	for i, file := range paths {
		if file.Path != p.Rows[i].Path || (p.Rows[i].Excluded != "" && file.Excluded != p.Rows[i].Excluded) {
			return fmt.Errorf("source reference membership differs from file_stats")
		}
	}
	for _, row := range p.Rows {
		var refs any
		if row.Refs != nil {
			b, _ := json.Marshal(row.Refs)
			refs = string(b)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO source_ref_facts(service_id,path,source_hash,extractor_version,refs_json,excluded) VALUES(?,?,?,?,?,?)`, meta.ServiceID, row.Path, row.SourceHash, row.ExtractorVersion, refs, row.Excluded); err != nil {
			return err
		}
	}
	complete := 0
	if p.Complete {
		complete = 1
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO source_ref_seals(service_id,watermark,root,policy,extractor_version,membership_hash,produced_at,complete) VALUES(?,?,?,?,?,?,?,?)`, meta.ServiceID, p.Watermark, p.Root, p.Policy, p.ExtractorVersion, p.MembershipHash, p.ProducedAt, complete)
	return err
}

// ReadSourceRefProjection returns a compact, self-checked indexed-generation
// snapshot. ok=false covers absent, interrupted, mixed or corrupt generations.
func (s *SQLiteStore) ReadSourceRefProjection(ctx context.Context, service string) (SourceRefProjection, bool, error) {
	return s.readSourceRefProjection(ctx, service, false)
}

func (s *SQLiteStore) ReadSourceRefProjectionForBackfill(ctx context.Context, service string) (SourceRefProjection, bool, error) {
	return s.readSourceRefProjection(ctx, service, true)
}

func (s *SQLiteStore) readSourceRefProjection(ctx context.Context, service string, duringIngest bool) (SourceRefProjection, bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return SourceRefProjection{}, false, err
	}
	defer tx.Rollback()
	var p SourceRefProjection
	var complete int
	err = tx.QueryRowContext(ctx, `SELECT watermark,root,policy,extractor_version,membership_hash,produced_at,complete FROM source_ref_seals WHERE service_id=?`, service).Scan(&p.Watermark, &p.Root, &p.Policy, &p.ExtractorVersion, &p.MembershipHash, &p.ProducedAt, &complete)
	if err == sql.ErrNoRows || isNoSuchTable(err) {
		return SourceRefProjection{}, false, nil
	}
	if err != nil {
		return SourceRefProjection{}, false, err
	}
	p.ServiceID, p.Complete = service, complete == 1
	var commit, path string
	var at int64
	if err := tx.QueryRowContext(ctx, `SELECT indexed_commit,indexed_at,path FROM service_meta WHERE service_id=?`, service).Scan(&commit, &at, &path); err != nil {
		return SourceRefProjection{}, false, err
	}
	if p.Watermark != WatermarkOf(ServiceMeta{IndexedCommit: commit, IndexedAt: at}) || p.Root != path {
		return SourceRefProjection{}, false, nil
	}
	var busy int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ingest_state WHERE service_id=? AND state='in_progress')`, service).Scan(&busy); err != nil && !isNoSuchTable(err) {
		return SourceRefProjection{}, false, err
	}
	if busy != 0 && !duringIngest {
		return SourceRefProjection{}, false, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT path,source_hash,extractor_version,refs_json,excluded FROM source_ref_facts WHERE service_id=? ORDER BY path`, service)
	if isNoSuchTable(err) {
		return SourceRefProjection{}, false, nil
	}
	if err != nil {
		return SourceRefProjection{}, false, err
	}
	for rows.Next() {
		var f SourceRefFact
		var raw sql.NullString
		if err := rows.Scan(&f.Path, &f.SourceHash, &f.ExtractorVersion, &raw, &f.Excluded); err != nil {
			rows.Close()
			return SourceRefProjection{}, false, err
		}
		if raw.Valid {
			if err := json.Unmarshal([]byte(raw.String), &f.Refs); err != nil {
				rows.Close()
				return SourceRefProjection{}, false, err
			}
			if f.Refs == nil {
				f.Refs = []string{}
			}
		}
		p.Rows = append(p.Rows, f)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return SourceRefProjection{}, false, err
	}
	if !validSourceRefProjection(p) {
		return SourceRefProjection{}, false, nil
	}
	stats, err := tx.QueryContext(ctx, `SELECT path,excluded FROM file_stats WHERE service_id=? ORDER BY path`, service)
	if err != nil {
		return SourceRefProjection{}, false, err
	}
	i := 0
	for stats.Next() {
		var path, excluded string
		if err := stats.Scan(&path, &excluded); err != nil {
			stats.Close()
			return SourceRefProjection{}, false, err
		}
		if i >= len(p.Rows) || p.Rows[i].Path != path || (p.Rows[i].Excluded != "" && p.Rows[i].Excluded != excluded) {
			stats.Close()
			return SourceRefProjection{}, false, nil
		}
		i++
	}
	err = stats.Err()
	stats.Close()
	if err != nil {
		return SourceRefProjection{}, false, err
	}
	if i != len(p.Rows) {
		return SourceRefProjection{}, false, nil
	}
	p.Scope, p.CurrentWorktreeFreshness = SourceRefScopeIngestGeneration, "unknown"
	if err := tx.Commit(); err != nil {
		return SourceRefProjection{}, false, err
	}
	if !duringIngest {
		visible, err := filterScoped(ctx, s.gate, service, p.Rows, nil, func(f SourceRefFact) string { return f.Path })
		if err != nil {
			return SourceRefProjection{}, false, err
		}
		if len(visible) != len(p.Rows) {
			return SourceRefProjection{}, false, nil
		}
	}
	return p, true, nil
}

func (m *MemoryStore) ReadSourceRefProjection(ctx context.Context, service string) (SourceRefProjection, bool, error) {
	return m.readSourceRefProjection(ctx, service, false)
}

func (m *MemoryStore) ReadSourceRefProjectionForBackfill(ctx context.Context, service string) (SourceRefProjection, bool, error) {
	return m.readSourceRefProjection(ctx, service, true)
}

func (m *MemoryStore) readSourceRefProjection(ctx context.Context, service string, duringIngest bool) (SourceRefProjection, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.sourceRefProjections[service]
	if !ok {
		return SourceRefProjection{}, false, nil
	}
	meta, mok := m.serviceMeta[service]
	if !mok || p.Watermark != WatermarkOf(meta) || p.Root != meta.Path || !validSourceRefProjection(p) {
		return SourceRefProjection{}, false, nil
	}
	stats := m.fileStats[service]
	if len(stats) != len(p.Rows) {
		return SourceRefProjection{}, false, nil
	}
	for i, f := range stats {
		if f.Path != p.Rows[i].Path || (p.Rows[i].Excluded != "" && f.Excluded != p.Rows[i].Excluded) {
			return SourceRefProjection{}, false, nil
		}
	}
	if state, sok := m.ingestState[service]; sok && state.State == IngestInProgress && !duringIngest {
		return SourceRefProjection{}, false, nil
	}
	p.Rows = append([]SourceRefFact(nil), p.Rows...)
	for i := range p.Rows {
		p.Rows[i].Refs = append([]string(nil), p.Rows[i].Refs...)
		if m.sourceRefProjections[service].Rows[i].Refs != nil && p.Rows[i].Refs == nil {
			p.Rows[i].Refs = []string{}
		}
	}
	p.Scope, p.CurrentWorktreeFreshness = SourceRefScopeIngestGeneration, "unknown"
	if !duringIngest {
		visible, err := filterScoped(ctx, m.gate, service, p.Rows, nil, func(f SourceRefFact) string { return f.Path })
		if err != nil {
			return SourceRefProjection{}, false, err
		}
		if len(visible) != len(p.Rows) {
			return SourceRefProjection{}, false, nil
		}
	}
	return p, true, nil
}
