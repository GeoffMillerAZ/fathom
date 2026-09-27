package fusion

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestStatMemoBoundedAndForgotten is RT-102 (design §7.2) under the RT-M
// memory goals: the in-memory stat record (until file_stats is wired) holds
// at most statMemoMax records across the fleet — the least recently
// ingested service is evicted first, never the one just ingested — a
// removed or renamed service's records are forgotten, and the ladder can
// drop them all (the cost is one full read per service, not a wrong answer).
func TestStatMemoBoundedAndForgotten(t *testing.T) {
	prevMax, prevRacy := statMemoMax, racyWindow
	statMemoMax, racyWindow = 25, 0
	t.Cleanup(func() { statMemoMax, racyWindow = prevMax, prevRacy })

	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{DisableTypedEdges: true})
	ingest := func(svc string, files int) {
		t.Helper()
		dir := t.TempDir()
		for i := 0; i < files; i++ {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.md", i)), []byte(fmt.Sprintf("# %s %d\n", svc, i)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: svc, Path: dir}); err != nil {
			t.Fatal(err)
		}
	}
	ingest("a", 10)
	ingest("b", 10)
	if got := p.StatMemoRecords(); got != 20 {
		t.Fatalf("two services of 10 files: %d records, want 20", got)
	}
	ingest("c", 10) // 30 > 25: a (least recent) goes
	if got := p.StatMemoRecords(); got > statMemoMax {
		t.Fatalf("%d records held, over the %d bound", got, statMemoMax)
	}
	if p.hasStatMemo("a") || !p.hasStatMemo("b") || !p.hasStatMemo("c") {
		t.Fatalf("eviction order wrong: a=%v b=%v c=%v; want the least recently ingested (a) evicted", p.hasStatMemo("a"), p.hasStatMemo("b"), p.hasStatMemo("c"))
	}
	ingest("huge", 40) // alone over the bound: kept (it is the one just ingested), the rest go
	if !p.hasStatMemo("huge") || p.hasStatMemo("b") || p.hasStatMemo("c") {
		t.Fatal("a service over the bound on its own must evict every other and keep itself")
	}

	p.ForgetStats("huge")
	if p.hasStatMemo("huge") || p.StatMemoRecords() != 0 {
		t.Fatal("ForgetStats left the service's records")
	}
	ingest("d", 5)
	p.DropStatMemo()
	if p.StatMemoRecords() != 0 {
		t.Fatal("DropStatMemo left records")
	}
}

func (p *Pipeline) hasStatMemo(service string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.statMemo[service]
	return ok
}
