package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// slowCompileBackend is the real client with a fake compile that outlives
// the route's write deadline (no model, no store work).
type slowCompileBackend struct {
	*client.Client
	run time.Duration
}

func (b *slowCompileBackend) CompileLayer(ctx context.Context, service, layer string, dryRun bool) (core.CompileRun, error) {
	select {
	case <-time.After(b.run):
	case <-ctx.Done():
		return core.CompileRun{}, ctx.Err()
	}
	return core.CompileRun{RunID: "run-long", ServiceID: service, Layer: layer, SectionsTotal: 4, SectionsCompiled: 4}, nil
}

// EW-36 (the O128 recurrence): compile_layer over /rpc ran 424 s, logged ok,
// and answered 0 bytes. The detach hook lifts the route's write deadline
// for an ingest-class call, but it reached the connection through
// loggingMiddleware's response writer, which did not unwrap: the lift
// silently failed and the 150 s deadline cut the client off mid-answer.
// Reproduced through the real chain (newDaemonServer over the logged
// handler) with the deadline shortened.
func TestCompileLayerLongRunNeverAnswersEmptyBody(t *testing.T) {
	c, err := client.NewConstantClient(client.ModeLocal, filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	srv := mcp.NewServer(&slowCompileBackend{Client: c, run: 900 * time.Millisecond})
	h := newHTTPHandlerLogged(srv, c, "127.0.0.1:0", nil, testDiscardLogger())
	ds := newDaemonServer(h, daemonServerOpts{rpcWriteTimeout: 300 * time.Millisecond})
	ts := httptest.NewUnstartedServer(ds.Handler)
	ts.Start()
	t.Cleanup(ts.Close)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"compile_layer","arguments":{"service":"svc","dry_run":false}}}`
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/rpc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("a compile that outlived the write deadline answered no response: %v", err)
	}
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	if err != nil || len(got) == 0 {
		t.Fatalf("answer body = %q err=%v, want the compile result", got, err)
	}
	if !strings.Contains(string(got), "run-long") {
		t.Fatalf("answer = %s, want the run id", got)
	}
}

// loggingResponseWriter must unwrap so http.ResponseController reaches the
// connection (write-deadline lifts, flushes) through the logging wrapper.
func TestLoggingResponseWriterUnwraps(t *testing.T) {
	rec := httptest.NewRecorder()
	lw := &loggingResponseWriter{ResponseWriter: rec, status: http.StatusOK}
	if lw.Unwrap() != http.ResponseWriter(rec) {
		t.Fatal("Unwrap must return the wrapped writer")
	}
	// An unwrappable writer cannot lift the deadline, and says so.
	if err := liftRPCWriteDeadline(struct{ http.ResponseWriter }{rec}); err == nil {
		t.Fatal("a writer that hides the connection must report ErrNotSupported, not succeed silently")
	}
}
