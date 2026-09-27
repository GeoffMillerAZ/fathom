package store

import (
	"context"
	"path/filepath"
	"testing"
)

// TestIngestStateRoundTrip pins RT-109's store half on both backends: the
// row upserts, reads back, follows RenameService, and goes with
// RemoveService.
func TestIngestStateRoundTrip(t *testing.T) {
	sq, err := Open(filepath.Join(t.TempDir(), "ingest-state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	for name, st := range map[string]Store{"sqlite": sq, "memory": NewMemoryStore()} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			iss := st.(IngestStateStore)
			if _, found, err := iss.GetIngestState(ctx, "a"); err != nil || found {
				t.Fatalf("empty store: found=%v err=%v", found, err)
			}
			want := IngestState{ServiceID: "a", State: IngestInProgress, PID: 42, StartedAt: 7, TargetCommit: "abc", Path: "/repo/a"}
			if err := iss.SetIngestState(ctx, want); err != nil {
				t.Fatal(err)
			}
			want.State = IngestComplete
			if err := iss.SetIngestState(ctx, want); err != nil {
				t.Fatal(err)
			}
			got, found, err := iss.GetIngestState(ctx, "a")
			if err != nil || !found || got != want {
				t.Fatalf("GetIngestState = %+v, %v, %v; want %+v", got, found, err, want)
			}
			if err := st.SetServiceMeta(ctx, ServiceMeta{ServiceID: "a"}); err != nil {
				t.Fatal(err)
			}
			if err := st.RenameService(ctx, "a", "b"); err != nil {
				t.Fatal(err)
			}
			if got, found, _ := iss.GetIngestState(ctx, "b"); !found || got.TargetCommit != "abc" || got.Path != "/repo/a" || got.ServiceID != "b" {
				t.Fatalf("after rename: %+v found=%v", got, found)
			}
			if _, err := st.RemoveService(ctx, "b"); err != nil {
				t.Fatal(err)
			}
			if _, found, _ := iss.GetIngestState(ctx, "b"); found {
				t.Fatal("ingest_state survived RemoveService")
			}
			if err := iss.SetIngestState(ctx, want); err != nil {
				t.Fatal(err)
			}
			if err := iss.DeleteIngestState(ctx, "a"); err != nil {
				t.Fatal(err)
			}
			if _, found, _ := iss.GetIngestState(ctx, "a"); found {
				t.Fatal("ingest_state survived DeleteIngestState")
			}
		})
	}
}
