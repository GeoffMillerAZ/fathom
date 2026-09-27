package mcp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// fullDiskIngestFake's inline ingest fails with SQLITE_FULL.
type fullDiskIngestFake struct{ refreshFake }

func (f *fullDiskIngestFake) IngestService(context.Context, string) error {
	f.ingests++
	return errors.New("fusion: commit: database or disk is full (13)")
}

// TestAutoRefreshWriteErrorIsStoreDistress (RT-41, §2.1): auto-refresh's
// inline ingest failing on SQLITE_FULL sets store distress.
func TestAutoRefreshWriteErrorIsStoreDistress(t *testing.T) {
	tenancy.ResetLifecycleStateForTest()
	t.Cleanup(tenancy.ResetLifecycleStateForTest)
	fake := &fullDiskIngestFake{refreshFake: refreshFake{path: "/repo", indexed: "old"}}
	ar := newAR(t, fake, "new")
	ensureSettled(t, ar, context.Background(), "svc")
	if fake.ingests != 1 {
		t.Fatalf("ingests = %d, want 1", fake.ingests)
	}
	if why, on := tenancy.StoreDistressNow(time.Now()); !on {
		t.Fatalf("an inline ingest's SQLITE_FULL did not set store distress (%q)", why)
	}
}
