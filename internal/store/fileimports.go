package store

// fileimports.go — file_imports (RT-108, docs/design/responsible-tenant.md
// §6, §7.2): the file-resolve pass's parsed import specifiers per (service,
// path), keyed by the file's content fingerprint. An ingest whose file is
// unchanged reuses the row instead of fetching and re-parsing the file
// (the audit's fileresolve-reparses-all-js-py hazard: every JS/TS/Python
// file was read and parsed twice on every ingest). A file whose import
// parse faulted records the fault against its fingerprint, so the same
// bytes are not re-parsed every ingest and the fault stays disclosed.
//
// Additive table in its own exec (createSchema's convention); reached
// through the optional FileImportsStore capability. RemoveService,
// RenameService, PurgeService and PurgePaths cover it.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const fileImportsDDL = `
CREATE TABLE IF NOT EXISTS file_imports (
    service_id   TEXT NOT NULL,
    path         TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    imports_json TEXT NOT NULL DEFAULT '',
    fault        TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (service_id, path)
);
`

// FileImports is one file's parsed imports at one content fingerprint.
type FileImports struct {
	ServiceID   string
	Path        string
	ContentHash string // fusion.FingerprintHash of the bytes parsed
	ImportsJSON string // the parsed specifiers, JSON (fusion owns the shape)
	Fault       string // non-empty: the parse faulted on these bytes
}

// FileImportsStore is the optional store capability behind file_imports,
// plus the per-file observation read the pass uses instead of re-extracting
// unchanged files. Both SQLiteStore and MemoryStore implement it.
type FileImportsStore interface {
	GetFileImports(ctx context.Context, serviceID, path string) (FileImports, bool, error)
	SetFileImports(ctx context.Context, fi FileImports) error
	DeleteFileImports(ctx context.Context, serviceID, path string) error
	// SourceObservations returns the observations anchored at any of refs
	// (an ingest-time read — never suppression-filtered: suppression must
	// not change what gets indexed), ordered by source_ref, line, symbol.
	SourceObservations(ctx context.Context, serviceID string, refs []string) ([]core.Observation, error)
}

func (s *SQLiteStore) createFileImportsSchema() error {
	_, err := s.db.Exec(fileImportsDDL)
	return err
}

// GetFileImports returns (serviceID, path)'s row; found is false when none.
func (s *SQLiteStore) GetFileImports(ctx context.Context, serviceID, path string) (FileImports, bool, error) {
	fi := FileImports{ServiceID: serviceID, Path: path}
	if err := ctx.Err(); err != nil {
		return FileImports{}, false, fmt.Errorf("GetFileImports: %w", err)
	}
	err := s.db.QueryRowContext(context.WithoutCancel(ctx), `
SELECT content_hash, imports_json, fault FROM file_imports WHERE service_id = ? AND path = ?
`, serviceID, path).Scan(&fi.ContentHash, &fi.ImportsJSON, &fi.Fault)
	if errors.Is(err, sql.ErrNoRows) {
		return FileImports{}, false, nil
	}
	if err != nil {
		return FileImports{}, false, fmt.Errorf("GetFileImports: %w", err)
	}
	return fi, true, nil
}

// SetFileImports upserts the row.
func (s *SQLiteStore) SetFileImports(ctx context.Context, fi FileImports) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("SetFileImports: %w", err)
	}
	_, err := s.db.ExecContext(context.WithoutCancel(ctx), `
INSERT INTO file_imports (service_id, path, content_hash, imports_json, fault)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (service_id, path) DO UPDATE SET
    content_hash = excluded.content_hash, imports_json = excluded.imports_json, fault = excluded.fault
`, fi.ServiceID, fi.Path, fi.ContentHash, fi.ImportsJSON, fi.Fault)
	if err != nil {
		return fmt.Errorf("SetFileImports: %w", err)
	}
	return nil
}

// DeleteFileImports removes the row; a missing row is a no-op.
func (s *SQLiteStore) DeleteFileImports(ctx context.Context, serviceID, path string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM file_imports WHERE service_id = ? AND path = ?`, serviceID, path); err != nil {
		return fmt.Errorf("DeleteFileImports: %w", err)
	}
	return nil
}

// sourceObservationsChunk bounds one IN list.
const sourceObservationsChunk = 500

// SourceObservations implements FileImportsStore.
func (s *SQLiteStore) SourceObservations(ctx context.Context, serviceID string, refs []string) ([]core.Observation, error) {
	var out []core.Observation
	for start := 0; start < len(refs); start += sourceObservationsChunk {
		end := min(start+sourceObservationsChunk, len(refs))
		chunk := refs[start:end]
		args := make([]any, 0, len(chunk)+1)
		args = append(args, serviceID)
		for _, r := range chunk {
			args = append(args, r)
		}
		rows, err := s.db.QueryContext(ctx, `
SELECT service_id, concern_id, source_ref, source_kind, evidence_tier,
       symbol, package, claim, content_hash, calls, complexity, line, end_line, ingested_at,
       tool_kind, generated
FROM observations
WHERE service_id = ? AND source_ref IN (`+strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ",")+`)
`, args...)
		if err != nil {
			return nil, fmt.Errorf("SourceObservations: %w", err)
		}
		obs, err := scanObservations(ctx, rows)
		rows.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, obs...)
	}
	sortObservationsBySource(out)
	return out, nil
}

func sortObservationsBySource(out []core.Observation) {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SourceRef != out[j].SourceRef {
			return out[i].SourceRef < out[j].SourceRef
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Symbol < out[j].Symbol
	})
}

type fileImportsKey struct{ service, path string }

// GetFileImports returns (serviceID, path)'s row; found is false when none.
func (m *MemoryStore) GetFileImports(_ context.Context, serviceID, path string) (FileImports, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	fi, ok := m.fileImports[fileImportsKey{serviceID, path}]
	return fi, ok, nil
}

// SetFileImports upserts the row.
func (m *MemoryStore) SetFileImports(_ context.Context, fi FileImports) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fileImports[fileImportsKey{fi.ServiceID, fi.Path}] = fi
	return nil
}

// DeleteFileImports removes the row; a missing row is a no-op.
func (m *MemoryStore) DeleteFileImports(_ context.Context, serviceID, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.fileImports, fileImportsKey{serviceID, path})
	return nil
}

// deleteServiceFileImports drops serviceID's rows (caller holds m.mu).
func (m *MemoryStore) deleteServiceFileImports(serviceID string) {
	for k := range m.fileImports {
		if k.service == serviceID {
			delete(m.fileImports, k)
		}
	}
}

// renameServiceFileImports re-keys oldID's rows to newID (caller holds m.mu).
func (m *MemoryStore) renameServiceFileImports(oldID, newID string) {
	for k, v := range m.fileImports {
		if k.service == oldID {
			delete(m.fileImports, k)
			v.ServiceID = newID
			m.fileImports[fileImportsKey{newID, k.path}] = v
		}
	}
}

// SourceObservations implements FileImportsStore.
func (m *MemoryStore) SourceObservations(_ context.Context, serviceID string, refs []string) ([]core.Observation, error) {
	want := make(map[string]bool, len(refs))
	for _, r := range refs {
		want[r] = true
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []core.Observation
	for _, o := range m.observations {
		if o.ServiceID == serviceID && want[o.SourceRef] {
			out = append(out, o)
		}
	}
	sortObservationsBySource(out)
	return out, nil
}
