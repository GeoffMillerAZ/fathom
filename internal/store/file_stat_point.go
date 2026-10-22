package store

import (
	"context"
	"database/sql"
	"errors"
)

// FileStatFor reads one existing file-stat receipt without listing the corpus.
func (s *SQLiteStore) FileStatFor(ctx context.Context, service, path string) (FileStat, bool, error) {
	var row FileStat
	var ino int64
	err := s.db.QueryRowContext(ctx, `SELECT path, size, mtime_ns, ino, content_hash, lang, excluded FROM file_stats WHERE service_id = ? AND path = ?`, service, path).Scan(&row.Path, &row.Size, &row.MtimeNS, &ino, &row.ContentHash, &row.Lang, &row.Excluded)
	if errors.Is(err, sql.ErrNoRows) || isNoSuchTable(err) {
		return FileStat{}, false, nil
	}
	row.Ino = uint64(ino)
	rows, err := filterScoped(ctx, s.gate, service, []FileStat{row}, err, func(f FileStat) string { return f.Path })
	if err != nil || len(rows) == 0 {
		return FileStat{}, false, err
	}
	return rows[0], true, nil
}

// FileStatFor reads one existing file-stat receipt without listing the corpus.
func (m *MemoryStore) FileStatFor(ctx context.Context, service, path string) (FileStat, bool, error) {
	m.mu.RLock()
	var found *FileStat
	for _, row := range m.fileStats[service] {
		if row.Path == path {
			copy := row
			found = &copy
			break
		}
	}
	m.mu.RUnlock()
	if found == nil {
		return FileStat{}, false, nil
	}
	rows, err := filterScoped(ctx, m.gate, service, []FileStat{*found}, nil, func(f FileStat) string { return f.Path })
	if err != nil || len(rows) == 0 {
		return FileStat{}, false, err
	}
	return rows[0], true, nil
}

// SetFileStatContentHash points an existing file_stats row's content_hash at
// hash (a one-file refresh keeping its receipt in step with the fingerprint).
// It reports false when the service has no row for path: a refresh never
// creates file_stats rows, only a whole ingest does.
func (s *SQLiteStore) SetFileStatContentHash(ctx context.Context, service, path, hash string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	res, err := s.db.ExecContext(context.WithoutCancel(ctx), `UPDATE file_stats SET content_hash = ? WHERE service_id = ? AND path = ?`, hash, service, path)
	if err != nil {
		if isNoSuchTable(err) {
			return false, nil
		}
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// SetFileStatContentHash is the MemoryStore twin of the SQLite point update.
func (m *MemoryStore) SetFileStatContentHash(_ context.Context, service, path, hash string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rows := m.fileStats[service]
	for i := range rows {
		if rows[i].Path == path {
			rows[i].ContentHash = hash
			return true, nil
		}
	}
	return false, nil
}
