package modulehost_test

// host_test.go — Host mount/unmount lifecycle (docs/plans/gke-module-host.md
// GKE-MH-1/2): a mount either succeeds cleanly or leaves the mounted set
// untouched; an unmount closes the store and removes it from the set with
// no stale reference surviving the call, and the remaining modules keep
// serving.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/modulehost"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// buildModuleStore creates a conforming module store at dir/name with one
// seeded observation for serviceID, and returns its path.
func buildModuleStore(t *testing.T, dir, name, serviceID string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	if err := st.SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: serviceID, IndexedCommit: "deadbeef"}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	if err := st.PutObservations(context.Background(), []core.Observation{{
		ServiceID: serviceID, SourceRef: "a.go", SourceKind: core.KindGoFunc,
		EvidenceTier: core.TierVerified, Symbol: "Foo",
	}}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return path
}

func TestHost_MountAndList(t *testing.T) {
	dir := t.TempDir()
	pathA := buildModuleStore(t, dir, "alpha.sqlite", "svc-a")

	h := modulehost.NewHost(store.LevelL0)
	m, err := h.Mount(pathA, "")
	if err != nil {
		t.Fatalf("Mount: %v", err)
	}
	if m.Name != "alpha" {
		t.Errorf("Mount default name = %q, want %q (derived from the file stem)", m.Name, "alpha")
	}
	if !h.Mounted("alpha") {
		t.Error("Mounted(\"alpha\") = false after a successful Mount")
	}
	if h.Count() != 1 {
		t.Errorf("Count() = %d, want 1", h.Count())
	}
}

func TestHost_MountRefusesLoudlyAndLeavesSetUnchanged(t *testing.T) {
	dir := t.TempDir()
	// A .txt path fails the extension check inside store.MountReadOnly —
	// Mount must surface that failure and NOT add a partial entry.
	h := modulehost.NewHost(store.LevelL0)
	if _, err := h.Mount(filepath.Join(dir, "bogus.txt"), ""); err == nil {
		t.Fatal("Mount of an unsupported extension succeeded; want a refusal")
	}
	if h.Count() != 0 {
		t.Errorf("Count() = %d after a refused Mount, want 0 (never partially mounted)", h.Count())
	}
	if h.Mounted("bogus") {
		t.Error("Mounted(\"bogus\") = true after a refused Mount")
	}
}

func TestHost_MountDuplicateNameRefused(t *testing.T) {
	dir := t.TempDir()
	pathA := buildModuleStore(t, dir, "alpha.sqlite", "svc-a")
	pathB := buildModuleStore(t, dir, "beta.sqlite", "svc-b")

	h := modulehost.NewHost(store.LevelL0)
	if _, err := h.Mount(pathA, "shared-name"); err != nil {
		t.Fatalf("first Mount: %v", err)
	}
	if _, err := h.Mount(pathB, "shared-name"); err == nil {
		t.Fatal("second Mount under a colliding name succeeded; want a refusal")
	}
	if h.Count() != 1 {
		t.Errorf("Count() = %d after a refused duplicate Mount, want 1 (the original mount untouched)", h.Count())
	}
}

func TestHost_UnmountRemovesAndClosesNoStaleReference(t *testing.T) {
	dir := t.TempDir()
	pathA := buildModuleStore(t, dir, "alpha.sqlite", "svc-a")
	pathB := buildModuleStore(t, dir, "beta.sqlite", "svc-b")

	h := modulehost.NewHost(store.LevelL0)
	if _, err := h.Mount(pathA, ""); err != nil {
		t.Fatalf("Mount alpha: %v", err)
	}
	if _, err := h.Mount(pathB, ""); err != nil {
		t.Fatalf("Mount beta: %v", err)
	}

	if err := h.Unmount("alpha"); err != nil {
		t.Fatalf("Unmount: %v", err)
	}
	if h.Mounted("alpha") {
		t.Error("Mounted(\"alpha\") = true after Unmount")
	}
	if h.Count() != 1 {
		t.Errorf("Count() = %d after unmounting one of two, want 1", h.Count())
	}

	// The remaining module keeps serving unaffected.
	infos, err := h.ListModules(context.Background())
	if err != nil {
		t.Fatalf("ListModules after Unmount: %v", err)
	}
	if len(infos) != 1 || infos[0].Module != "beta" {
		t.Fatalf("ListModules after unmounting alpha = %+v, want exactly beta", infos)
	}

	// Unmounting an absent module is a loud error, not a silent no-op.
	if err := h.Unmount("alpha"); err == nil {
		t.Error("second Unmount of an already-unmounted module succeeded; want an error")
	}
}
