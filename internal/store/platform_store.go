// Package store — PlatformStore opens many per-service DuckDB stores from a
// directory and provides fleet-wide query helpers.
//
// Each *.duckdb file in the directory is loaded as a service whose ID is the
// filename stem (e.g. "payment-service.duckdb" → service "payment-service").
package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// PlatformStore manages a fleet of per-service DuckDB stores rooted at a
// common directory.  It is safe for concurrent reads; Reload performs an
// atomic swap under a write lock.
type PlatformStore struct {
	mu       sync.RWMutex
	storeDir string
	services map[string]*DuckStore // keyed by service ID
}

// OpenPlatform scans storeDir for *.duckdb files and opens each one as the
// DuckStore for the service whose ID is the filename stem.  It returns an
// error when storeDir cannot be read or any individual store fails to open.
func OpenPlatform(storeDir string) (*PlatformStore, error) {
	ps := &PlatformStore{
		storeDir: storeDir,
		services: make(map[string]*DuckStore),
	}
	if err := ps.load(); err != nil {
		return nil, err
	}
	return ps, nil
}

// load (re)scans storeDir and opens every *.duckdb file that is not yet open.
// It must be called with mu held for writing, or before the store is published.
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
		if !strings.HasSuffix(name, ".duckdb") {
			continue
		}
		stem := strings.TrimSuffix(name, ".duckdb")
		if _, already := ps.services[stem]; already {
			continue // already open; keep existing handle
		}
		path := filepath.Join(ps.storeDir, name)
		ds, err := Open(path)
		if err != nil {
			return fmt.Errorf("OpenPlatform Open %q: %w", path, err)
		}
		ps.services[stem] = ds
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
	snapshot := make(map[string]*DuckStore, len(ps.services))
	for id, ds := range ps.services {
		snapshot[id] = ds
	}
	ps.mu.RUnlock()

	result := make(map[string][]core.Observation, len(snapshot))
	for id, ds := range snapshot {
		obs, err := ds.QueryObservations(ctx, id, concernID)
		if err != nil {
			continue
		}
		if len(obs) > 0 {
			result[id] = obs
		}
	}
	return result
}

// Reload re-scans storeDir for new *.duckdb files and opens any that were not
// previously loaded.  Existing stores are not re-opened or closed.  The swap
// is atomic under the write lock so concurrent reads see a consistent view.
func (ps *PlatformStore) Reload() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.load()
}

// Close closes every per-service DuckStore.  After Close returns the
// PlatformStore must not be used.
func (ps *PlatformStore) Close() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for _, ds := range ps.services {
		_ = ds.Close()
	}
	ps.services = nil
}
