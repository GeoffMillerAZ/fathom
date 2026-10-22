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
