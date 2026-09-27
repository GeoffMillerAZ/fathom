package store

// servicerefs.go — routes and outbound HTTP client calls extracted at
// ingest (RT-89, docs/design/responsible-tenant.md §6): serviceref's
// cross-service hops are joined from these rows, never from a per-call
// read of every service's source. Rows are per file; an ingest replaces
// the rows of the files it read, and a service is complete once one ingest
// has extracted every readable file (service_refs_state).

import (
	"context"
	"fmt"
	"sort"
)

// ServiceRoute is one route registration a file makes.
type ServiceRoute struct {
	Method, Path, Handler, File string
	Line                        int
}

// ServiceHTTPCall is one outbound HTTP client call a file makes.
type ServiceHTTPCall struct {
	Method, Template, RawURL, Symbol, File string
	Line                                   int
}

// ServiceRefs is a service's extracted routes and client calls.
type ServiceRefs struct {
	Routes []ServiceRoute
	Calls  []ServiceHTTPCall
}

var serviceRefsDDL = `
CREATE TABLE IF NOT EXISTS service_routes (
    service_id TEXT    NOT NULL,
    file       TEXT    NOT NULL,
    line       INTEGER NOT NULL DEFAULT 0,
    method     TEXT    NOT NULL DEFAULT '',
    path       TEXT    NOT NULL DEFAULT '',
    handler    TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_service_routes_file ON service_routes (service_id, file);

CREATE TABLE IF NOT EXISTS service_http_calls (
    service_id TEXT    NOT NULL,
    file       TEXT    NOT NULL,
    line       INTEGER NOT NULL DEFAULT 0,
    method     TEXT    NOT NULL DEFAULT '',
    template   TEXT    NOT NULL DEFAULT '',
    raw_url    TEXT    NOT NULL DEFAULT '',
    symbol     TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_service_http_calls_file ON service_http_calls (service_id, file);

-- A service's rows are complete once an ingest extracted every readable
-- file; until then readers name the service as not extracted.
CREATE TABLE IF NOT EXISTS service_refs_state (
    service_id   TEXT    NOT NULL PRIMARY KEY,
    extracted_at INTEGER NOT NULL DEFAULT 0
);
`

// serviceRefTables are the tables a service's refs live in.
var serviceRefTables = []string{"service_routes", "service_http_calls", "service_refs_state"}

// ReplaceServiceRefs replaces every row of files with refs' rows (refs
// must name only those files) in one transaction; markComplete records the
// service as fully extracted (it never unmarks).
func (s *SQLiteStore) ReplaceServiceRefs(ctx context.Context, serviceID string, files []string, refs ServiceRefs, markComplete bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ReplaceServiceRefs begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, table := range []string{"service_routes", "service_http_calls"} {
		if err := execIn(ctx, tx, `DELETE FROM `+table+` WHERE service_id = ? AND file IN (%s)`, serviceID, files, true); err != nil {
			return fmt.Errorf("ReplaceServiceRefs clear %s: %w", table, err)
		}
	}
	for _, r := range refs.Routes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO service_routes (service_id, file, line, method, path, handler) VALUES (?, ?, ?, ?, ?, ?)`,
			serviceID, r.File, r.Line, r.Method, r.Path, r.Handler); err != nil {
			return fmt.Errorf("ReplaceServiceRefs route: %w", err)
		}
	}
	for _, c := range refs.Calls {
		if _, err := tx.ExecContext(ctx, `INSERT INTO service_http_calls (service_id, file, line, method, template, raw_url, symbol) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			serviceID, c.File, c.Line, c.Method, c.Template, c.RawURL, c.Symbol); err != nil {
			return fmt.Errorf("ReplaceServiceRefs call: %w", err)
		}
	}
	if markComplete {
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO service_refs_state (service_id, extracted_at) VALUES (?, strftime('%s','now'))`, serviceID); err != nil {
			return fmt.Errorf("ReplaceServiceRefs state: %w", err)
		}
	}
	return tx.Commit()
}

// clearServiceRefs drops a service's rows and its complete mark (a purge:
// the next ingest re-extracts every file).
func clearServiceRefs(ctx context.Context, ex execer, serviceID string) error {
	for _, table := range serviceRefTables {
		if _, err := ex.ExecContext(ctx, `DELETE FROM `+table+` WHERE service_id = ?`, serviceID); err != nil && !isNoSuchTable(err) {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	return nil
}

func (s *SQLiteStore) serviceRefsRaw(ctx context.Context, serviceID string) (ServiceRefs, bool, error) {
	var out ServiceRefs
	var complete bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM service_refs_state WHERE service_id = ?)`, serviceID).Scan(&complete); err != nil {
		if isNoSuchTable(err) {
			return out, false, nil
		}
		return out, false, fmt.Errorf("ServiceRefsFor state: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT file, line, method, path, handler FROM service_routes WHERE service_id = ?`, serviceID)
	if err != nil {
		return out, false, fmt.Errorf("ServiceRefsFor routes: %w", err)
	}
	for rows.Next() {
		var r ServiceRoute
		if err := rows.Scan(&r.File, &r.Line, &r.Method, &r.Path, &r.Handler); err != nil {
			rows.Close()
			return out, false, err
		}
		out.Routes = append(out.Routes, r)
	}
	if err := closeRows(rows, "service_routes"); err != nil {
		return out, false, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT file, line, method, template, raw_url, symbol FROM service_http_calls WHERE service_id = ?`, serviceID)
	if err != nil {
		return out, false, fmt.Errorf("ServiceRefsFor calls: %w", err)
	}
	for rows.Next() {
		var c ServiceHTTPCall
		if err := rows.Scan(&c.File, &c.Line, &c.Method, &c.Template, &c.RawURL, &c.Symbol); err != nil {
			rows.Close()
			return out, false, err
		}
		out.Calls = append(out.Calls, c)
	}
	if err := closeRows(rows, "service_http_calls"); err != nil {
		return out, false, err
	}
	sortServiceRefs(&out)
	return out, complete, nil
}

// sortServiceRefs orders rows by (file, line, method, path/template).
func sortServiceRefs(r *ServiceRefs) {
	sort.Slice(r.Routes, func(i, j int) bool {
		a, b := r.Routes[i], r.Routes[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Method != b.Method {
			return a.Method < b.Method
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Handler < b.Handler
	})
	sort.Slice(r.Calls, func(i, j int) bool {
		a, b := r.Calls[i], r.Calls[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Method != b.Method {
			return a.Method < b.Method
		}
		if a.Template != b.Template {
			return a.Template < b.Template
		}
		return a.RawURL < b.RawURL
	})
}

// ServiceRefsFor filters each row by its file.
func (s *SQLiteStore) ServiceRefsFor(ctx context.Context, serviceID string) (ServiceRefs, bool, error) {
	refs, complete, err := s.serviceRefsRaw(ctx, serviceID)
	return filterServiceRefs(ctx, s.gate, serviceID, refs, complete, err)
}

func filterServiceRefs(ctx context.Context, g scopeGate, serviceID string, refs ServiceRefs, complete bool, err error) (ServiceRefs, bool, error) {
	if err != nil {
		return ServiceRefs{}, false, err
	}
	routes, err := filterScoped(ctx, g, serviceID, refs.Routes, nil, func(r ServiceRoute) string { return r.File })
	if err != nil {
		return ServiceRefs{}, false, err
	}
	calls, err := filterScoped(ctx, g, serviceID, refs.Calls, nil, func(c ServiceHTTPCall) string { return c.File })
	return ServiceRefs{Routes: routes, Calls: calls}, complete, err
}

// ---- MemoryStore ----

type memServiceRefs struct {
	byFile   map[string]ServiceRefs
	complete bool
}

// ReplaceServiceRefs is the memory backend's per-file replace.
func (m *MemoryStore) ReplaceServiceRefs(_ context.Context, serviceID string, files []string, refs ServiceRefs, markComplete bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.serviceRefs == nil {
		m.serviceRefs = map[string]*memServiceRefs{}
	}
	sr := m.serviceRefs[serviceID]
	if sr == nil {
		sr = &memServiceRefs{byFile: map[string]ServiceRefs{}}
		m.serviceRefs[serviceID] = sr
	}
	for _, f := range files {
		delete(sr.byFile, f)
	}
	for _, r := range refs.Routes {
		x := sr.byFile[r.File]
		x.Routes = append(x.Routes, r)
		sr.byFile[r.File] = x
	}
	for _, c := range refs.Calls {
		x := sr.byFile[c.File]
		x.Calls = append(x.Calls, c)
		sr.byFile[c.File] = x
	}
	sr.complete = sr.complete || markComplete
	return nil
}

// ServiceRefsFor filters each row by its file.
func (m *MemoryStore) ServiceRefsFor(ctx context.Context, serviceID string) (ServiceRefs, bool, error) {
	m.mu.RLock()
	var out ServiceRefs
	complete := false
	if sr := m.serviceRefs[serviceID]; sr != nil {
		complete = sr.complete
		for _, x := range sr.byFile {
			out.Routes = append(out.Routes, x.Routes...)
			out.Calls = append(out.Calls, x.Calls...)
		}
	}
	m.mu.RUnlock()
	sortServiceRefs(&out)
	return filterServiceRefs(ctx, m.gate, serviceID, out, complete, nil)
}
