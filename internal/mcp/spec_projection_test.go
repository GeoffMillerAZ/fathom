package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/sdd"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

type specProjectionBackend struct {
	QueryInterface
	scan func(context.Context, corpus.Budget, func(core.SourceFile) bool) (corpus.Coverage, error)
}

func (b specProjectionBackend) ScanSources(ctx context.Context, _ string, budget corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	return b.scan(ctx, budget, fn)
}

// Unchanged materializing reference from the exact reviewed H1 source. This
// keeps the semantic oracle independent of the new streaming implementation.
func materializedSpecMatrix(ctx context.Context, backend QueryInterface, nodes []core.SymbolNode, specs []string) (sdd.Matrix, corpus.Coverage, error) {
	codeRefs := map[string][]string{}
	for _, n := range nodes {
		if refs := sdd.ExtractRefs(n.Signature + " " + n.Name); len(refs) > 0 {
			codeRefs[n.Name] = refs
		}
	}
	files, cov, err := collectSources(ctx, backend, "svc")
	if errors.Is(err, core.ErrSourcesBusy) {
		return sdd.Matrix{}, corpus.Coverage{}, err
	}
	if err != nil {
		cov = corpus.Coverage{}
	} else {
		for _, f := range files {
			if refs := sdd.ExtractRefs(string(f.Content)); len(refs) > 0 {
				codeRefs[f.Path] = append(codeRefs[f.Path], refs...)
			}
		}
	}
	return sdd.BuildMatrix(specs, codeRefs), cov, nil
}

func deliveredSpecBackend(files []core.SourceFile, terminal corpus.Coverage, err error) specProjectionBackend {
	return specProjectionBackend{scan: func(_ context.Context, budget corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
		if budget.Bytes != 64<<20 || budget.Wall != 5*time.Second {
			panic("spec projection changed its scan budget")
		}
		for _, file := range files {
			if fn(file) {
				panic("spec projection introduced a hit stop")
			}
		}
		return terminal, err
	}}
}

func TestSpecMatrixProjectionDifferentialCallbacksAndErrors(t *testing.T) {
	files := []core.SourceFile{
		{Path: "collision.cs", Content: []byte("[FR-1] [US-2] [FR-1]"), Generated: true},
		{Path: "collision.cs", Content: []byte("[FR-1] [EP-3]")},
		{Path: "values.json", Content: []byte("{\"key\":\"[NFR-4] [FR-999]\"}")},
		{Path: "notes.md", Content: []byte("\xff[FR-1]\x00 [X-2]")},
	}
	nodes := []core.SymbolNode{
		{Name: "collision.cs", Signature: "[FR-1]"},
		{Name: "same", Signature: "[US-2]"},
		{Name: "same", Signature: "[EP-3]"},
	}
	specs := []string{"[FR-1]", "[US-2]", "[EP-3]", "[NFR-4]", "[FR-0]"}
	full := corpus.Coverage{Scanned: len(files), Total: len(files), BytesRead: 123}
	for _, tc := range []struct {
		name string
		cov  corpus.Coverage
		err  error
	}{
		{"complete", full, nil},
		{"excluded", corpus.Coverage{Scanned: 4, Total: 5, BytesRead: 123, Excluded: []corpus.Excluded{{Path: "unreadable", Reason: corpus.ReasonUnreadable, Err: errors.New("permission denied")}}}, nil},
		{"bytes", corpus.Coverage{Scanned: 4, Total: 8, BytesRead: 123, Stopped: corpus.StopBytes}, nil},
		{"files", corpus.Coverage{Scanned: 4, Total: 8, BytesRead: 123, Stopped: corpus.StopFiles}, nil},
		{"meter", corpus.Coverage{Scanned: 4, Total: 8, BytesRead: 123, Stopped: corpus.StopMeter}, nil},
		{"late-error", full, errors.New("terminal read failed after callbacks")},
		{"late-cancel", corpus.Coverage{Scanned: 4, Total: 8, Stopped: corpus.StopCtx}, context.Canceled},
		{"busy", full, fmt.Errorf("wrapped: %w", core.ErrSourcesBusy)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := deliveredSpecBackend(files, tc.cov, tc.err)
			want, wc, we := materializedSpecMatrix(context.Background(), backend, nodes, specs)
			got, gc, ge := buildSpecMatrix(context.Background(), backend, "svc", nodes, specs)
			if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gc, wc) || !errors.Is(ge, we) {
				t.Fatalf("projection differs from materializing reference: matrix=%v coverage=%+v/%+v err=%v/%v", reflect.DeepEqual(got, want), gc, wc, ge, we)
			}
			if ge == nil {
				plan := verify.Plan{Service: "svc", Tasks: []verify.Task{
					{ID: "covered", SpecIDs: []string{"[FR-1]"}, Claimed: true},
					{ID: "uncovered", SpecIDs: []string{"[FR-0]"}, Claimed: true},
				}}
				gotReport := verifyReportToMap(verify.VerifyPlan(plan, &verifyEvidence{haveMat: true, matrix: got}))
				wantReport := verifyReportToMap(verify.VerifyPlan(plan, &verifyEvidence{haveMat: true, matrix: want}))
				if !reflect.DeepEqual(gotReport, wantReport) || partialNote(gc) != partialNote(wc) {
					t.Fatal("rendered verify report or named partial changed")
				}
			}
			if tc.name == "complete" {
				// Symbol contribution plus BOTH repeated-path callbacks survive.
				wantIDs := []string{"collision.cs", "collision.cs", "collision.cs", "notes.md"}
				if !reflect.DeepEqual(got.Coverage("[FR-1]"), wantIDs) {
					t.Fatalf("callback or symbol contributions deduplicated: %v", got.Coverage("[FR-1]"))
				}
				if !reflect.DeepEqual(got.Coverage("[FR-999]"), []string{"values.json"}) {
					t.Fatal("undeclared/orphan refs were filtered")
				}
			}
			if tc.err != nil && !errors.Is(tc.err, core.ErrSourcesBusy) && len(got.Coverage("[FR-1]")) != 1 {
				t.Fatal("late scan error kept file contributions or lost the symbol")
			}
		})
	}
}

func TestSpecMatrixProjectionEmptyScan(t *testing.T) {
	backend := deliveredSpecBackend(nil, corpus.Coverage{}, nil)
	got, cov, err := buildSpecMatrix(context.Background(), backend, "svc", nil, []string{"[FR-1]"})
	if err != nil || !reflect.DeepEqual(cov, corpus.Coverage{}) || len(got.Coverage("[FR-1]")) != 0 {
		t.Fatalf("empty projection = %v %+v %v", got, cov, err)
	}
}

func TestSpecMatrixProjectionScanSourceFilesLastWins(t *testing.T) {
	files := []core.SourceFile{
		{Path: "same.json", Content: []byte("[FR-1]")},
		{Path: "same.json", Content: []byte("[FR-2]")},
	}
	backend := specProjectionBackend{scan: func(ctx context.Context, budget corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
		return corpus.ScanSourceFiles(ctx, files, budget, fn)
	}}
	got, cov, err := buildSpecMatrix(context.Background(), backend, "svc", nil, []string{"[FR-1]", "[FR-2]"})
	if err != nil || cov.Total != 1 || cov.Scanned != 1 || len(got.Coverage("[FR-1]")) != 0 || !reflect.DeepEqual(got.Coverage("[FR-2]"), []string{"same.json"}) {
		t.Fatalf("scan-level last-wins changed: %+v %v %v", cov, err, got)
	}
}

func TestSpecMatrixProjectionWallStopAndFinalCallbackCancellation(t *testing.T) {
	backend := specProjectionBackend{scan: func(ctx context.Context, budget corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
		now := time.Unix(0, 0)
		budget.Now = func() time.Time { return now }
		return corpus.ScanFetch(ctx, []string{"a", "b", "c"}, budget,
			func(context.Context, string) ([]byte, error) { return []byte("[FR-1]"), nil },
			func(path string, content []byte) bool {
				stop := fn(core.SourceFile{Path: path, Content: content})
				now = now.Add(3 * time.Second) // deterministic callback work charge
				return stop
			})
	}}
	got, cov, err := buildSpecMatrix(context.Background(), backend, "svc", nil, []string{"[FR-1]"})
	if err != nil || cov.Stopped != corpus.StopTime || cov.Scanned != 2 || !reflect.DeepEqual(got.Coverage("[FR-1]"), []string{"a", "b"}) {
		t.Fatalf("wall stop changed: %+v %v %v", cov, err, got)
	}
	// ScanFetch checks cancellation before each file, not after the final
	// callback. Cancellation within the final delivery must not add a new
	// terminal check, rename the stop, or discard a nil-error complete scan.
	for _, terminalError := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		backend := specProjectionBackend{scan: func(ctx context.Context, budget corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
			cov, err := corpus.ScanFetch(ctx, []string{"final"}, budget,
				func(context.Context, string) ([]byte, error) { return []byte("[FR-1]"), nil },
				func(path string, content []byte) bool {
					cancel()
					return fn(core.SourceFile{Path: path, Content: content})
				})
			if terminalError {
				return cov, context.Canceled
			}
			return cov, err
		}}
		matrix, cov, err := buildSpecMatrix(ctx, backend, "svc", nil, []string{"[FR-1]"})
		if ctx.Err() == nil || err != nil {
			t.Fatalf("final cancellation setup/error changed: %v %v", ctx.Err(), err)
		}
		if terminalError {
			if len(matrix.Coverage("[FR-1]")) != 0 || !reflect.DeepEqual(cov, corpus.Coverage{}) {
				t.Fatal("terminal nonbusy error did not discard all file refs and coverage")
			}
		} else if cov.Stopped != "" || cov.Scanned != 1 || !reflect.DeepEqual(matrix.Coverage("[FR-1]"), []string{"final"}) {
			t.Fatalf("new context check after final callback: %+v %v", cov, matrix)
		}
		cancel()
	}
}

type specProjectionConnector struct{}

func (specProjectionConnector) Discover(_ context.Context, service core.ServiceRef) ([]core.SourceRef, error) {
	return []core.SourceRef{{Service: service.ID, Path: "a.cs"}}, nil
}
func (specProjectionConnector) Fetch(context.Context, core.SourceRef) ([]byte, error) {
	return []byte("[FR-1]"), nil
}
func (specProjectionConnector) Watermark(context.Context, core.ServiceRef) (string, error) {
	return "", nil
}

func TestSpecMatrixProjectionSourcesScopeContentionAndRelease(t *testing.T) {
	c := client.New(specProjectionConnector{}, store.NewMemoryStore())
	specs := []string{"[FR-1]"}
	s1, done1 := client.WithSourcesScope(context.Background(), nil)
	s2, done2 := client.WithSourcesScope(context.Background(), nil)
	defer done1()
	defer done2()
	for _, ctx := range []context.Context{s1, s2} {
		if matrix, _, err := buildSpecMatrix(ctx, c, "svc", nil, specs); err != nil || len(matrix.Coverage("[FR-1]")) != 1 {
			t.Fatalf("scoped projection: %v %v", matrix, err)
		}
	}
	if _, _, err := buildSpecMatrix(client.WithHookOrigin(context.Background()), c, "svc", nil, specs); !errors.Is(err, core.ErrSourcesBusy) {
		t.Fatalf("busy hook projection = %v, want named refusal", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	matrix, cov, err := buildSpecMatrix(cancelled, c, "svc", nil, specs)
	if err != nil || len(matrix.Coverage("[FR-1]")) != 0 || !reflect.DeepEqual(cov, corpus.Coverage{}) {
		t.Fatal("cancelled slot wait changed existing nonbusy best-effort behavior")
	}
	done1()
	done1()
	if matrix, _, err := buildSpecMatrix(client.WithHookOrigin(context.Background()), c, "svc", nil, specs); err != nil || len(matrix.Coverage("[FR-1]")) != 1 {
		t.Fatalf("slot not released at scope completion: %v %v", matrix, err)
	}
}

const projectionFixtureBytes = 32 << 20

func specProjectionAllocationFixture() ([]core.SourceFile, specProjectionBackend) {
	files := make([]core.SourceFile, 32)
	for i := range files {
		content := bytes.Repeat([]byte{'x'}, 1<<20)
		copy(content[8190:], "[FR-1]")
		files[i] = core.SourceFile{Path: fmt.Sprintf("file-%02d.json", i), Content: content}
	}
	backend := specProjectionBackend{scan: func(ctx context.Context, budget corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
		return corpus.ScanSourceFiles(ctx, files, budget, fn)
	}}
	return files, backend
}

func requireSpecProjectionFixture(tb testing.TB, matrix sdd.Matrix, cov corpus.Coverage, err error) {
	tb.Helper()
	if err != nil || cov.Stopped != "" || cov.Scanned != 32 || cov.Total != 32 || cov.BytesRead != projectionFixtureBytes || len(matrix.Coverage("[FR-1]")) != 32 {
		tb.Fatalf("allocation win cannot reduce corpus: %+v err=%v refs=%d", cov, err, len(matrix.Coverage("[FR-1]")))
	}
}

// Fixed proposed RED/GREEN threshold: input setup and materializing oracle
// are outside the observation. Baseline's full-file string copies exceed8MiB.
// This has not been executed or calibrated as a production memory bound.
func TestSpecMatrixProjectionAllocationBound(t *testing.T) {
	files, backend := specProjectionAllocationFixture()
	specs := []string{"[FR-1]"}
	want, wc, we := materializedSpecMatrix(context.Background(), backend, nil, specs)
	requireSpecProjectionFixture(t, want, wc, we)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	got, cov, err := buildSpecMatrix(context.Background(), backend, "svc", nil, specs)
	runtime.ReadMemStats(&after)
	requireSpecProjectionFixture(t, got, cov, err)
	if !reflect.DeepEqual(got, want) {
		t.Fatal("allocation target changed the matrix")
	}
	if used := after.TotalAlloc - before.TotalAlloc; used >= projectionFixtureBytes/4 {
		t.Fatalf("extraction overhead = %d bytes, want <8MiB for32MiB delivered", used)
	}
	runtime.KeepAlive(files)
}

func BenchmarkBuildSpecMatrixProjection(b *testing.B) {
	files, backend := specProjectionAllocationFixture()
	b.ReportAllocs()
	b.SetBytes(projectionFixtureBytes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matrix, cov, err := buildSpecMatrix(context.Background(), backend, "svc", nil, []string{"[FR-1]"})
		requireSpecProjectionFixture(b, matrix, cov, err)
	}
	b.StopTimer()
	b.ReportMetric(32, "files/op")
	b.ReportMetric(projectionFixtureBytes, "delivered-B/op")
	runtime.KeepAlive(files)
}

// Synthetic retained-heap observation, deliberately separate from allocation
// and from daemon footprint. Explicit GC is confined to this test workload.
// It reports the in-flight sample without claiming a peak or setting policy.
func TestSpecMatrixProjectionRetainedWorkload(t *testing.T) {
	var initial, during runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initial)
	backend := specProjectionBackend{scan: func(_ context.Context, _ corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
		for i := 0; i < 32; i++ {
			content := bytes.Repeat([]byte{'x'}, 1<<20)
			copy(content, "[FR-1]")
			fn(core.SourceFile{Path: fmt.Sprintf("file-%02d.json", i), Content: content})
			content = nil
			if i == 30 {
				runtime.GC()
				runtime.ReadMemStats(&during)
			}
		}
		return corpus.Coverage{Scanned: 32, Total: 32, BytesRead: projectionFixtureBytes}, nil
	}}
	matrix, cov, err := buildSpecMatrix(context.Background(), backend, "svc", nil, []string{"[FR-1]"})
	requireSpecProjectionFixture(t, matrix, cov, err)
	t.Logf("synthetic in-flight sampled heap delta after31callbacks: %d bytes (not process peak)", int64(during.HeapAlloc)-int64(initial.HeapAlloc))
	runtime.KeepAlive(matrix)
}
