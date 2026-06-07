package store

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// seedService creates <dir>/<id>.duckdb and writes obs into it, then closes it.
// The file is left on disk so OpenPlatform can pick it up.
func seedService(t *testing.T, dir, id string, obs []core.Observation) {
	t.Helper()
	path := filepath.Join(dir, id+".duckdb")
	ds, err := Open(path)
	if err != nil {
		t.Fatalf("seedService Open %q: %v", path, err)
	}
	if err := ds.PutObservations(context.Background(), obs); err != nil {
		_ = ds.Close()
		t.Fatalf("seedService PutObservations: %v", err)
	}
	if err := ds.Close(); err != nil {
		t.Fatalf("seedService Close: %v", err)
	}
}

func nowObs(serviceID, symbol string) core.Observation {
	return core.Observation{
		ServiceID:    serviceID,
		ConcernID:    "auth",
		SourceRef:    serviceID + "/main.go",
		SourceKind:   core.KindGoFunc,
		EvidenceTier: core.TierVerified,
		Symbol:       symbol,
		Claim:        "func " + symbol + "()",
		ContentHash:  symbol,
		IngestedAt:   time.Now().UTC(),
	}
}

// ─── OpenPlatform: two services ───────────────────────────────────────────────

func TestOpenPlatform_TwoServices(t *testing.T) {
	dir := t.TempDir()
	seedService(t, dir, "alpha", []core.Observation{nowObs("alpha", "Login")})
	seedService(t, dir, "beta", []core.Observation{nowObs("beta", "Charge")})

	ps, err := OpenPlatform(dir)
	if err != nil {
		t.Fatalf("OpenPlatform: %v", err)
	}
	defer ps.Close()

	// ServiceIDs must be sorted.
	ids := ps.ServiceIDs()
	if len(ids) != 2 {
		t.Fatalf("ServiceIDs() len = %d, want 2; got %v", len(ids), ids)
	}
	if !sort.StringsAreSorted(ids) {
		t.Errorf("ServiceIDs() not sorted: %v", ids)
	}
	for i, want := range []string{"alpha", "beta"} {
		if ids[i] != want {
			t.Errorf("ids[%d] = %q, want %q", i, ids[i], want)
		}
	}

	// Get returns true for known services and false for unknown.
	if _, ok := ps.Get("alpha"); !ok {
		t.Error("Get(alpha) returned false, want true")
	}
	if _, ok := ps.Get("unknown"); ok {
		t.Error("Get(unknown) returned true, want false")
	}
}

// ─── QueryAll ────────────────────────────────────────────────────────────────

func TestPlatformStore_QueryAll(t *testing.T) {
	dir := t.TempDir()
	seedService(t, dir, "svc1", []core.Observation{nowObs("svc1", "Login")})
	seedService(t, dir, "svc2", []core.Observation{nowObs("svc2", "Charge")})

	ps, err := OpenPlatform(dir)
	if err != nil {
		t.Fatalf("OpenPlatform: %v", err)
	}
	defer ps.Close()

	// QueryAll("") should return observations for every service.
	all := ps.QueryAll(context.Background(), "")
	if _, ok := all["svc1"]; !ok {
		t.Error("QueryAll: svc1 missing from results")
	}
	if _, ok := all["svc2"]; !ok {
		t.Error("QueryAll: svc2 missing from results")
	}

	// QueryAll with a concern only present in svc1.
	byAuth := ps.QueryAll(context.Background(), "auth")
	// Both seedService calls used ConcernID="auth" so both should appear.
	if _, ok := byAuth["svc1"]; !ok {
		t.Error("QueryAll(auth): svc1 missing")
	}
}

// ─── Reload ───────────────────────────────────────────────────────────────────

func TestPlatformStore_Reload(t *testing.T) {
	dir := t.TempDir()
	seedService(t, dir, "svc1", []core.Observation{nowObs("svc1", "F")})

	ps, err := OpenPlatform(dir)
	if err != nil {
		t.Fatalf("OpenPlatform: %v", err)
	}
	defer ps.Close()

	if ids := ps.ServiceIDs(); len(ids) != 1 {
		t.Fatalf("before Reload: want 1 service, got %v", ids)
	}

	// Add svc2 to the directory.
	seedService(t, dir, "svc2", []core.Observation{nowObs("svc2", "G")})

	if err := ps.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	ids := ps.ServiceIDs()
	if len(ids) != 2 {
		t.Fatalf("after Reload: want 2 services, got %v", ids)
	}
	if _, ok := ps.Get("svc2"); !ok {
		t.Error("after Reload: Get(svc2) returned false")
	}
}

// ─── edge cases ───────────────────────────────────────────────────────────────

func TestOpenPlatform_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	ps, err := OpenPlatform(dir)
	if err != nil {
		t.Fatalf("OpenPlatform empty dir: %v", err)
	}
	defer ps.Close()
	if ids := ps.ServiceIDs(); len(ids) != 0 {
		t.Errorf("empty dir: ServiceIDs() = %v, want []", ids)
	}
}

func TestOpenPlatform_NonDuckdbFilesIgnored(t *testing.T) {
	dir := t.TempDir()
	// Plant a non-.duckdb file; it must be silently ignored.
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedService(t, dir, "only", []core.Observation{nowObs("only", "X")})

	ps, err := OpenPlatform(dir)
	if err != nil {
		t.Fatalf("OpenPlatform: %v", err)
	}
	defer ps.Close()

	ids := ps.ServiceIDs()
	if len(ids) != 1 || ids[0] != "only" {
		t.Errorf("ServiceIDs() = %v, want [only]", ids)
	}
}
