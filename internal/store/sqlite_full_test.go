package store

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Large, scattered call lists must spill their statement journals without losing the
// batch's ordinary Go observations. Cover both an empty store and existing call indexes.
func TestPutObservationsLargeCallLists(t *testing.T) {
	if shape := os.Getenv("FATHOM_LARGE_CALLS_SHAPE"); shape != "" {
		testPutObservationsLargeCallLists(t, shape)
		return
	}
	// SQLite's statement-journal configuration must run before its first open.
	for _, shape := range []string{"single", "many"} {
		t.Run(shape, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestPutObservationsLargeCallLists$", "-test.v")
			cmd.Env = append(os.Environ(), "FATHOM_LARGE_CALLS_SHAPE="+shape)
			out, err := cmd.CombinedOutput()
			t.Logf("%s", out)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func testPutObservationsLargeCallLists(t *testing.T, shape string) {
	t.Helper()
	s, err := NewSQLiteStore(filepath.Join(t.TempDir(), "calls.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !stmtSpillApplied {
		t.Fatal("statement-journal spill configuration was not applied before SQLite opened")
	}
	defer func() {
		if out, err := exec.Command("df", "-k", os.TempDir()).CombinedOutput(); err == nil {
			t.Logf("TMPDIR free space after insert:\n%s", out)
		}
	}()
	// With a 4 MiB spill threshold, these shapes fail just above 56k calls
	// (many) and 57k calls (single, second batch). Keep a small margin.
	count := 58_000
	perObservation := count
	if shape == "many" {
		count = 57_000
		perObservation = 1000
	}
	obs := []core.Observation{{ServiceID: "svc", SourceRef: "ordinary.go", SourceKind: core.KindGoFunc, Symbol: "Ordinary", Calls: []string{"pkg.Small"}}}
	for start := 0; start < count; start += perObservation {
		n := min(perObservation, count-start)
		calls := make([]string, n)
		for i := range calls {
			// A fixed permutation scatters index updates. Sorted, append-only keys
			// touch too few existing pages to reproduce a large statement journal.
			calls[i] = fmt.Sprintf("pkg.Callee%016x", uint64(start+i)*0x9e3779b97f4a7c15)
		}
		obs = append(obs, core.Observation{ServiceID: "svc", SourceRef: "large.model", Symbol: fmt.Sprintf("Symbol%d", start), Calls: calls})
	}
	for batch := 1; batch <= 2; batch++ {
		if err := s.PutObservations(context.Background(), obs); err != nil {
			t.Fatalf("shape=%s batch=%d calls=%d: %v", shape, batch, count, err)
		}
		var rows, ordinary, jsonCalls, indexedCalls int
		if err := s.db.QueryRow(`SELECT count(*), sum(source_ref = 'ordinary.go'), sum(json_array_length(calls))
FROM observations WHERE service_id = 'svc'`).Scan(&rows, &ordinary, &jsonCalls); err != nil {
			t.Fatal(err)
		}
		if err := s.db.QueryRow(`SELECT count(*) FROM observation_calls WHERE service_id = 'svc'`).Scan(&indexedCalls); err != nil {
			t.Fatal(err)
		}
		if rows != batch*len(obs) || ordinary != batch || jsonCalls != batch*(count+1) || indexedCalls != jsonCalls {
			t.Fatalf("batch=%d: rows=%d ordinary=%d JSON calls=%d indexed calls=%d", batch, rows, ordinary, jsonCalls, indexedCalls)
		}
	}
}
