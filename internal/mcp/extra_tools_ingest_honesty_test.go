package mcp_test

// Daemon-side ingest honesty (docs/plans/instrument-truth.md IT-15, O16):
// when the pipeline refuses to replace a populated index with the
// zero-symbol result of a wedged reader, the `ingest` MCP tool returns a
// JSON-RPC error that names the cause — count, first path, errno — and the
// external-volume hint, never an ok:true over an untouched index. A
// DEGRADED ingest (IT-14: readable files indexed, unreadable ones named)
// is an error too, but it WROTE — so the service-identity cache is busted
// before the error goes out; a refusal wrote nothing and leaves it alone.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

type refusingBackend struct {
	extendedFakeBackend
	calls int
}

func (r *refusingBackend) IngestService(_ context.Context, service string) error {
	r.calls++
	return &fusion.ReadFailuresError{
		Service: service,
		Failures: []fusion.ReadFailure{
			{Path: "internal/a.go", Err: "open /vol/repo/internal/a.go: operation not permitted"},
			{Path: "internal/b.go", Err: "open /vol/repo/internal/b.go: operation not permitted"},
		},
		Refused:  true,
		Existing: 8703,
	}
}

var _ mcp.QueryInterface = (*refusingBackend)(nil)

// degradedBackend indexed every readable file and could not read one — the
// non-refused *ReadFailuresError shape.
type degradedBackend struct {
	extendedFakeBackend
	calls int
}

func (d *degradedBackend) IngestService(_ context.Context, service string) error {
	d.calls++
	return &fusion.ReadFailuresError{
		Service: service,
		Failures: []fusion.ReadFailure{
			{Path: "internal/b.go", Err: "open /vol/repo/internal/b.go: operation not permitted"},
		},
	}
}

var _ mcp.QueryInterface = (*degradedBackend)(nil)

// seedIdentCache points hookcache.IdentDir at a throwaway directory holding
// one cached entry — never the developer's real cache — and returns a probe
// reporting whether the cache still exists.
func seedIdentCache(t *testing.T) func() bool {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ident")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir ident cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "entry.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("seed ident cache: %v", err)
	}
	orig := hookcache.IdentDir
	hookcache.IdentDir = dir
	t.Cleanup(func() { hookcache.IdentDir = orig })
	return func() bool {
		_, err := os.Stat(dir)
		return err == nil
	}
}

func callIngest(t *testing.T, srv *mcp.Server, id int) map[string]any {
	t.Helper()
	return roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: id, Method: "tools/call",
		Params: map[string]any{
			"name":      "ingest",
			"arguments": map[string]any{"service": "svc"},
		},
	})
}

func TestIngestTool_RefusalNamesCauseAsRPCError(t *testing.T) {
	cached := seedIdentCache(t)
	backend := &refusingBackend{extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}}}
	srv := mcp.NewServer(backend)

	resp := callIngest(t, srv, 910)
	if backend.calls != 1 {
		t.Fatalf("IngestService calls = %d, want 1", backend.calls)
	}
	if resp["result"] != nil {
		t.Fatalf("a refused ingest must not return a result: %v", resp["result"])
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected a JSON-RPC error object, got %v", resp)
	}
	msg, _ := errObj["message"].(string)
	for _, want := range []string{
		"refused",
		"2 file(s)",
		"internal/a.go",
		"operation not permitted",
		"8703",
		"tccutil reset SystemPolicyRemovableVolumes",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message must contain %q:\n%s", want, msg)
		}
	}
	if !cached() {
		t.Fatal("a refused ingest wrote nothing; the service-identity cache must be left alone")
	}
}

func TestIngestTool_DegradedIngestBustsIdentCacheAndNamesCause(t *testing.T) {
	cached := seedIdentCache(t)
	backend := &degradedBackend{extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}}}
	srv := mcp.NewServer(backend)

	resp := callIngest(t, srv, 911)
	if backend.calls != 1 {
		t.Fatalf("IngestService calls = %d, want 1", backend.calls)
	}
	if resp["result"] != nil {
		t.Fatalf("a degraded ingest must not return ok:true: %v", resp["result"])
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected a JSON-RPC error object, got %v", resp)
	}
	msg, _ := errObj["message"].(string)
	for _, want := range []string{
		"1 file(s) could not be read",
		"internal/b.go",
		"operation not permitted",
		"left as it was",
		"tccutil reset SystemPolicyRemovableVolumes",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message must contain %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "refused") {
		t.Errorf("a degraded ingest is not a refusal:\n%s", msg)
	}
	if cached() {
		t.Fatal("a degraded ingest indexed every readable file; the service-identity cache must be busted before the error is returned")
	}
}
