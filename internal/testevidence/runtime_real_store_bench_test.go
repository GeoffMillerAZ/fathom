package testevidence

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

var runtimeBenchStore = flag.String("runtime-bench-store", "", "path to an owned closed SQLite copy for the native runtime benchmark")
var runtimeBenchService = flag.String("runtime-bench-service", "clogging-in", "service in the owned runtime benchmark store")

type runtimeBenchResult struct {
	Key string             `json:"key"`
	Run verify.AttestedRun `json:"run"`
}

type runtimeBenchCounts struct {
	Rows, Native, Pass, Fail, Skip, Flake, Unknown, Other, Refusals int
}

// BenchmarkLoadRuntimeRunsRealStore reads the supplied owned SQLite copy on
// every iteration, including native receipt, XML, source and SCIP freshness.
// The hash covers every newest keyed result and its refusal reason. It does
// not run during ordinary tests or benchmarks without -runtime-bench-store.
func BenchmarkLoadRuntimeRunsRealStore(b *testing.B) {
	if *runtimeBenchStore == "" {
		b.Skip("set -runtime-bench-store to an owned closed SQLite copy")
	}
	info, err := os.Stat(*runtimeBenchStore)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		b.Fatalf("runtime benchmark store must be an existing nonempty regular file: %v", err)
	}
	st, err := store.Open(*runtimeBenchStore)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := st.Close(); err != nil {
			b.Error(err)
		}
	})
	rows, err := st.NewestTestCaseRuns(context.Background(), *runtimeBenchService)
	if err != nil {
		b.Fatal(err)
	}
	if len(rows) == 0 {
		b.Fatalf("no newest runs for service %q", *runtimeBenchService)
	}
	keys := make([]string, len(rows))
	native := 0
	for i, row := range rows {
		if row.TestSymbolKey == "" || (i > 0 && rows[i-1].TestSymbolKey >= row.TestSymbolKey) {
			b.Fatalf("newest runs lack sorted unique canonical keys at row %d", i)
		}
		keys[i] = row.TestSymbolKey
		if row.NativeJSON != "" {
			native++
		}
	}
	if native == 0 {
		b.Fatalf("service %q has no native joined runs", *runtimeBenchService)
	}
	rows = nil
	now := func() time.Time { return time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC) }
	var expectedHash [sha256.Size]byte
	var expectedCounts runtimeBenchCounts
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		runs, loadErr := LoadRuntimeRuns(ctx, st, *runtimeBenchService, now)
		cancel()
		if loadErr != nil {
			b.Fatal(loadErr)
		}
		got, resultErr := runs.NewestAttestedRuns(keys)
		if resultErr != nil || len(got) != len(keys) {
			b.Fatalf("newest keyed results: got %d of %d: %v", len(got), len(keys), resultErr)
		}
		canonical := make([]runtimeBenchResult, len(keys))
		counts := runtimeBenchCounts{Rows: len(keys), Native: native}
		for j, key := range keys {
			run, ok := got[key]
			if !ok {
				b.Fatalf("missing runtime key %q", key)
			}
			canonical[j] = runtimeBenchResult{Key: key, Run: run}
			switch run.Status {
			case "pass":
				counts.Pass++
			case "fail":
				counts.Fail++
			case "skip":
				counts.Skip++
			case "flake":
				counts.Flake++
			case "unknown":
				counts.Unknown++
			default:
				counts.Other++
			}
			if run.UnknownReason != "" {
				counts.Refusals++
			}
		}
		encoded, encodeErr := json.Marshal(canonical)
		if encodeErr != nil {
			b.Fatal(encodeErr)
		}
		hash := sha256.Sum256(encoded)
		if i == 0 {
			expectedHash, expectedCounts = hash, counts
		} else if hash != expectedHash || counts != expectedCounts {
			b.Fatalf("runtime results changed at iteration %d: hash=%x counts=%+v; first hash=%x counts=%+v", i, hash, counts, expectedHash, expectedCounts)
		}
	}
	b.StopTimer()
	b.Logf("runtime_results_sha256=%x counts=%+v", expectedHash, expectedCounts)
}
