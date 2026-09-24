package fusion_test

// dg4_logger_test.go — DG-4 (diagnostics sweep): Pipeline.SetLogger routes
// the typed-edge pass's own diagnostics through a *dlog.Logger instead of
// a bare fmt.Fprintf(os.Stderr, ...), for the callers that DO construct a
// Pipeline (cmd/fathom's daemon, via pkg/client.Client.WithLogger).

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestPipeline_SetLogger_RoutesTypedEdgeSummaryThroughLogger pins DG-4's
// core contract for internal/fusion: every completed typed-edge pass logs
// a one-line summary (packages/edges/peak-heap/duration) — with SetLogger
// called, that line must go through the injected logger, not stderr.
func TestPipeline_SetLogger_RoutesTypedEdgeSummaryThroughLogger(t *testing.T) {
	dir := writeGoModule(t, map[string]string{
		"go.mod": "module dg4fixture\n\ngo 1.21\n",
		"main.go": `package main

func main() { helper() }
func helper() {}
`,
	})

	st := store.NewMemoryStore()
	pipe := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})

	var buf strings.Builder
	log := dlog.New(&buf, dlog.LevelDebug, time.Now)
	pipe.SetLogger(log)

	ctx := context.Background()
	if _, err := pipe.Ingest(ctx, core.ServiceRef{ID: "dg4fixture", Path: dir}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "fusion: typed-edge pass") {
		t.Errorf("SetLogger was called but the typed-edge summary line never reached it; log:\n%q", out)
	}
	if !strings.Contains(out, "dg4fixture") {
		t.Errorf("summary line missing the service id; log:\n%q", out)
	}
}

// TestPipeline_NoLogger_PreservesOriginalBehavior pins the other half:
// never calling SetLogger must not panic and must not change any RETURNED
// result — every existing embedder/test that constructs a Pipeline
// directly via fusion.New and never calls SetLogger keeps working exactly
// as before this method existed.
func TestPipeline_NoLogger_PreservesOriginalBehavior(t *testing.T) {
	dir := writeGoModule(t, map[string]string{
		"go.mod": "module dg4fixture2\n\ngo 1.21\n",
		"main.go": `package main

func main() { helper() }
func helper() {}
`,
	})

	st := store.NewMemoryStore()
	pipe := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})

	ctx := context.Background()
	rep, err := pipe.Ingest(ctx, core.ServiceRef{ID: "dg4fixture2", Path: dir})
	if err != nil {
		t.Fatalf("Ingest without SetLogger: %v", err)
	}
	if pipe.TypedPassTotal() == 0 {
		t.Error("expected the typed-edge pass to have run at least once")
	}
	if rep.TypedEdgesDirect == 0 {
		t.Errorf("expected at least one direct edge (main -> helper); report = %+v", rep)
	}
	_ = io.Discard
}
