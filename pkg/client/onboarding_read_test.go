package client_test

// onboarding_read_test.go — F1 of the waves C–F live proof
// (docs/bench/readings/rt-waves-c-f-2026-09-27.md §6), end to end: a
// service's first ingest runs while reads name the same service (the hook
// traffic every onboarding meets). Before the fix the reads' Discover —
// resolved against the bare service ID, since no service_meta path exists
// yet — replaced the connector's recorded root, and the ingest failed for
// every file ("stat <service>/<path>: no such file or directory").

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestFirstIngestSurvivesConcurrentReads(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	const files = 300
	for i := 0; i < files; i++ {
		src := fmt.Sprintf("package p\n\nfunc F%d() int { return %d }\n", i, i)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%03d.go", i)), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ms := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), ms)

	stop := make(chan struct{})
	var reads atomic.Int64
	var wg sync.WaitGroup
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, _ = c.SourcePaths(ctx, "onboarding-svc") // fails until the meta exists; that is fine
				reads.Add(1)
			}
		}()
	}
	report, err := c.Ingest(ctx, client.ServiceRef{ID: "onboarding-svc", Path: dir})
	close(stop)
	wg.Wait()

	if err != nil {
		t.Fatalf("first ingest under %d concurrent reads failed: %v", reads.Load(), err)
	}
	if report.ReadFailed != 0 || report.Discovered != files || report.Extracted != files {
		t.Fatalf("report = discovered %d extracted %d read-failed %d, want %d/%d/0 (reads during the ingest: %d)",
			report.Discovered, report.Extracted, report.ReadFailed, files, files, reads.Load())
	}
	for i := 0; i < files; i++ {
		if _, found, _ := ms.Fingerprint(ctx, "onboarding-svc", fmt.Sprintf("f%03d.go", i)); !found {
			t.Fatalf("f%03d.go has no fingerprint after the ingest", i)
		}
	}
}
