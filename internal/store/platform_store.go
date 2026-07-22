// Package store — PlatformStore opens many per-service stores from a
// directory and provides fleet-wide query helpers.
//
// Each recognized store file (*.duckdb, *.sqlite, *.db) in the directory is
// loaded as a service whose ID is the filename stem (e.g.
// "payment-service.sqlite" → service "payment-service"). A single storeDir
// may hold a mix of backends — a fleet migrating from DuckDB to SQLite one
// service at a time (ADR-020) loads correctly throughout the transition.
package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// PlatformStore manages a fleet of per-service stores (any BackendForPath-
// recognized extension, possibly mixed) rooted at a common directory. It is
// safe for concurrent reads; Reload performs an atomic swap under a write lock.
type PlatformStore struct {
	mu       sync.RWMutex
	storeDir string
	services map[string]Store // keyed by service ID
}

// OpenPlatform scans storeDir for recognized store files (*.duckdb, *.sqlite,
// *.db) and opens each one as the Store for the service whose ID is the
// filename stem. It returns an error when storeDir cannot be read or any
// individual store fails to open.
func OpenPlatform(storeDir string) (*PlatformStore, error) {
	ps := &PlatformStore{
		storeDir: storeDir,
		services: make(map[string]Store),
	}
	if err := ps.load(); err != nil {
		return nil, err
	}
	return ps, nil
}

// load (re)scans storeDir and opens every recognized store file that is not
// yet open. It must be called with mu held for writing, or before the store
// is published.
func (ps *PlatformStore) load() error {
	entries, err := os.ReadDir(ps.storeDir)
	if err != nil {
		return fmt.Errorf("OpenPlatform ReadDir %q: %w", ps.storeDir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		stem, ok := ServiceIDFromStoreFilename(name)
		if !ok {
			continue
		}
		if _, already := ps.services[stem]; already {
			continue // already open; keep existing handle
		}
		path := filepath.Join(ps.storeDir, name)
		s, err := BackendForPath(path)
		if err != nil {
			return fmt.Errorf("OpenPlatform BackendForPath %q: %w", path, err)
		}
		ps.services[stem] = s
	}
	return nil
}

// ServiceIDs returns the sorted list of service IDs known to the platform store.
func (ps *PlatformStore) ServiceIDs() []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	ids := make([]string, 0, len(ps.services))
	for id := range ps.services {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Get returns the Store for the given service ID.  The second return value is
// false when the service is not known.
func (ps *PlatformStore) Get(id string) (Store, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	s, ok := ps.services[id]
	return s, ok
}

// QueryAll queries every service for observations tagged with concernID (pass
// "" to query all observations).  The returned map is keyed by service ID.
// Services that return an error are silently omitted; callers that need error
// details should call Get + QueryObservations individually.
func (ps *PlatformStore) QueryAll(ctx context.Context, concernID string) map[string][]core.Observation {
	ps.mu.RLock()
	// Take a snapshot of the current service map so we can release the lock
	// before doing I/O.
	snapshot := make(map[string]Store, len(ps.services))
	for id, s := range ps.services {
		snapshot[id] = s
	}
	ps.mu.RUnlock()

	result := make(map[string][]core.Observation, len(snapshot))
	for id, s := range snapshot {
		obs, err := s.QueryObservations(ctx, id, concernID)
		if err != nil {
			continue
		}
		if len(obs) > 0 {
			result[id] = obs
		}
	}
	return result
}

// Reload re-scans storeDir for new recognized store files and opens any that
// were not previously loaded.  Existing stores are not re-opened or closed.
// The swap is atomic under the write lock so concurrent reads see a
// consistent view.
func (ps *PlatformStore) Reload() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.load()
}

// Close closes every per-service Store.  After Close returns the
// PlatformStore must not be used.
func (ps *PlatformStore) Close() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for _, ds := range ps.services {
		_ = ds.Close()
	}
	ps.services = nil
}
