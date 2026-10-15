package store

import (
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// readOnlyDir makes dir read-only for the rest of the test and restores it
// for t.TempDir's cleanup. Root ignores directory permissions, so a
// read-only directory cannot be built as root: the test skips naming why.
func readOnlyDir(t *testing.T, dir string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root: root writes into a 0555 directory, so a read-only directory cannot be built here")
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

// walHeader reports whether the database file at path is in WAL mode: bytes
// 18 and 19 of the header (the file format write and read versions) are 2.
func walHeader(t *testing.T, path string) bool {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	hdr := make([]byte, 20)
	if _, err := io.ReadFull(f, hdr); err != nil {
		t.Fatal(err)
	}
	return hdr[18] == 2 && hdr[19] == 2
}

// TestMountReadOnlyOpensStoreFromReadOnlyDirectory (IXS-12): a WAL-mode
// store closed cleanly (no -wal or -shm left) in a directory nobody may
// write — read-only media, a snapshot mounted read-only — opens and reads
// through MountReadOnly. SQLite opens a WAL-mode file read-only only when
// its -shm and -wal exist, its directory is writable, or the URI says
// immutable=1; mode=ro alone failed here. A -wal beside the file on
// read-only media is frames nobody checkpointed: the mount refuses it by
// name rather than open a stale view.
func TestMountReadOnlyOpensStoreFromReadOnlyDirectory(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "media")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "snapshot.sqlite")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.PutObservations(ctx, []core.Observation{{
		ServiceID: "svc", SourceRef: "a.go", SourceKind: core.KindGoFunc,
		EvidenceTier: core.TierVerified, Symbol: "Foo",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	if !walHeader(t, path) {
		t.Fatal("the store is not in WAL mode: the case this test pins is a WAL-mode store")
	}
	for _, sidecar := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(path + sidecar); err == nil {
			t.Fatalf("closing the store left %s beside it; the case is a cleanly closed store", sidecar)
		}
	}
	readOnlyDir(t, dir)

	mounted, err := MountReadOnly(path, LevelL0)
	if err != nil {
		t.Fatalf("MountReadOnly from a read-only directory: %v", err)
	}
	ids, err := mounted.ListServiceIDs(ctx)
	if err != nil || len(ids) != 1 || ids[0] != "svc" {
		t.Errorf("ListServiceIDs = %v, %v; want [svc]", ids, err)
	}
	if err := mounted.PutObservations(ctx, []core.Observation{{ServiceID: "svc2", SourceRef: "b.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Bar"}}); err == nil {
		t.Error("a write through the read-only mount succeeded")
	}
	_ = mounted.Close()
}

// TestMountReadOnlyRefusesWALOnReadOnlyMedia: a store whose -wal holds
// frames nobody checkpointed, on read-only media, is refused with a named
// reason — never opened without them. A live writer's store in a writable
// directory keeps today's open (another process may still be writing it).
func TestMountReadOnlyRefusesWALOnReadOnlyMedia(t *testing.T) {
	ctx := context.Background()
	live := filepath.Join(t.TempDir(), "live.sqlite")
	st, err := Open(live)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	raw, err := sql.Open("sqlite", "file:"+live+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	if _, err := raw.ExecContext(ctx, "PRAGMA wal_autocheckpoint=0"); err != nil {
		t.Fatal(err)
	}
	if err := st.PutObservations(ctx, []core.Observation{{ServiceID: "svc", SourceRef: "a.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Foo"}}); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(live + "-wal"); err != nil || fi.Size() == 0 {
		t.Fatalf("the live store has no -wal frames to copy (%v)", err)
	}

	// The live store, its writer still open, mounts as before.
	m, err := MountReadOnly(live, LevelL0)
	if err != nil {
		t.Fatalf("MountReadOnly of a live store in a writable directory: %v", err)
	}
	_ = m.Close()

	// Its files copied onto read-only media, -wal included: refused.
	dir := filepath.Join(t.TempDir(), "media")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "snapshot.sqlite")
	for _, suffix := range []string{"", "-wal"} {
		data, err := os.ReadFile(live + suffix)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path+suffix, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	readOnlyDir(t, dir)
	_, err = MountReadOnly(path, LevelL0)
	assertConformanceReason(t, err, ReasonWALOnReadOnlyMedia)
}
