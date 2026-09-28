// Package modulehost implements fathom's GKE module host
// (docs/plans/gke-module-host.md): `serve-http` mounts N conforming expert-
// module stores (docs/spec/store-format-v0.md, docs/spec/
// expert-module-v0.md) READ-ONLY, beside the main fleet store, so
// `obligations` can union in the rules of every module a service binds
// (internal/modserve's ModuleFederator). The scoped /rpc/modules surface
// and its four MCP tools were removed by SR-17 (surface reckoning F-68).
//
// There is no write path to a mounted module, structurally
// (docs/vision/12-governed-knowledge-engine.md §2 law 9): every mount goes
// through store.MountReadOnly, whose SQLite connection is opened in true
// read-only mode — a write attempt through it fails at the SQLite/VFS layer
// itself (TestMountReadOnly_SucceedsAndIsGenuinelyReadOnly,
// internal/store/conformance_test.go), and no method on Host or Module ever
// calls a store.Store write method.
package modulehost

import (
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// Module is one mounted, conforming module store.
type Module struct {
	// Name is the module's identity (what a binding names) — the store filename's
	// stem by default (ModuleNameFromPath), or an explicit override passed
	// to Host.Mount.
	Name      string
	Path      string
	Store     *store.SQLiteStore
	MountedAt time.Time
}

// Host manages N mounted, read-only module stores. Safe for concurrent use:
// Mount/Unmount take a write lock; every query takes its own snapshot
// (a copy of the mounted set's pointers, under a read lock) so a concurrent
// Mount/Unmount never races a fan-out query. Unmount deletes the map entry
// AND closes the store's connection before returning, so no snapshot taken
// AFTER that point can observe the unmounted module — GKE-MH-2's "no stale
// reference to the unmounted store's connection survives the call."
type Host struct {
	mu      sync.RWMutex
	modules map[string]*Module
	level   store.ConformanceLevel
	now     func() time.Time
}

// NewHost returns an empty Host that will require `level` conformance of
// every future Mount (docs/plans/gke-module-host.md's design section: "at
// minimum L0 + Server" — pass store.LevelL0, since Server conformance is a
// behavioral, not schema, requirement — see store.ConformanceLevel's doc).
func NewHost(level store.ConformanceLevel) *Host {
	return &Host{modules: map[string]*Module{}, level: level, now: time.Now}
}

// Mount conformance-checks and opens path read-only (store.MountReadOnly),
// then registers it under name (path's file stem, via ModuleNameFromPath,
// when name == ""). Refuses loudly: on ANY failure — a name collision, or a
// store.MountReadOnly refusal — the Host's mounted set is left exactly as
// it was; Mount never partially registers a module (GKE-MH-1, doctrine 3
// "never silent").
func (h *Host) Mount(path, name string) (*Module, error) {
	if name == "" {
		name = ModuleNameFromPath(path)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, exists := h.modules[name]; exists {
		return nil, fmt.Errorf("modulehost: module %q already mounted (existing path %s, requested %s)", name, existing.Path, path)
	}
	st, err := store.MountReadOnly(path, h.level)
	if err != nil {
		return nil, fmt.Errorf("modulehost: mount %q: %w", name, err)
	}
	m := &Module{Name: name, Path: path, Store: st, MountedAt: h.now()}
	h.modules[name] = m
	return m, nil
}

// Unmount closes and removes the named module. The remaining mounted set
// keeps serving unaffected. Returns an error naming the module when it was
// never mounted (unmounting an absent module is refused loudly, not a
// silent no-op).
func (h *Host) Unmount(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	m, ok := h.modules[name]
	if !ok {
		return fmt.Errorf("modulehost: module %q is not mounted", name)
	}
	delete(h.modules, name)
	return m.Store.Close()
}

// Mounted reports whether name is currently mounted.
func (h *Host) Mounted(name string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.modules[name]
	return ok
}

// Count returns the number of currently mounted modules.
func (h *Host) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.modules)
}

// findModule returns the named module (or false), under a read lock.
func (h *Host) findModule(name string) (*Module, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	m, ok := h.modules[name]
	return m, ok
}

// snapshot returns the currently-mounted modules, sorted by name, for a
// fan-out query to iterate without holding h.mu across store calls (which
// could otherwise block a concurrent Mount/Unmount for the duration of a
// slow query).
func (h *Host) snapshot() []*Module {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*Module, 0, len(h.modules))
	for _, m := range h.modules {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ModuleNameFromPath derives a module's default identity from its store
// file path: the base filename with its store extension stripped (mirrors
// store.ServiceIDFromStoreFilename's own convention for the fleet store, so
// a module named "billing-runbook.sqlite" mounts as "billing-runbook").
func ModuleNameFromPath(path string) string {
	base := filepath.Base(path)
	if stem, ok := store.ServiceIDFromStoreFilename(base); ok {
		return stem
	}
	return base
}
