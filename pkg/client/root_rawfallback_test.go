package client_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/client"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// O1452 at the client: once the repository root's guide changes after the
// compile, the root raw fallback serves its current sections; while the
// guide is compiled it adds nothing.
func TestRootRawFallbackObligations_UncompiledRootGuide(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	agents := filepath.Join(dir, "AGENTS.md")
	for path, body := range map[string]string{
		agents:                              "# Guidance\n\n- Run the gate before pushing.\n",
		filepath.Join(dir, "pkg", "foo.go"): "package pkg\nfunc Foo() {}\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}
	c.WithLLM(llmclient.NewMockClient(
		llmclient.MockEntry{Response: llmclient.Response{Text: `{"claims":[{"rule":"Run the gate before pushing.","scope":["pkg/*.go"],"cite_start":3,"cite_end":3}]}`}},
	), "mock-model", 0, 0)
	if run, err := c.CompileLayer(ctx, "svc", "obligations", false); err != nil || run.SectionsCompiled == 0 {
		t.Fatalf("compile: run=%+v err=%v", run, err)
	}
	if rows, _, ok, err := c.RootRawFallbackObligations(ctx, "svc", 2); err != nil || ok || len(rows) != 0 {
		t.Fatalf("compiled root guide: rows=%+v ok=%v err=%v, want nothing", rows, ok, err)
	}
	if err := os.WriteFile(agents, []byte("# Guidance\n\n- Run the gate before pushing.\n- Deploy through the task, never cp.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, why, ok, err := c.RootRawFallbackObligations(ctx, "svc", 2)
	if err != nil || !ok || len(rows) != 1 {
		t.Fatalf("stale root guide: rows=%+v ok=%v err=%v, want its one changed section", rows, ok, err)
	}
	r := rows[0]
	if r.Tier != "raw" || r.Layer != "obligations" || !strings.HasPrefix(r.Src, "AGENTS.md#") || r.Rule != "Run the gate before pushing.\nDeploy through the task, never cp." || r.Why != why {
		t.Errorf("row = %+v, want the AGENTS.md section raw from obligations carrying %q", r, why)
	}
	if rows, _, _, _ := c.RootRawFallbackObligations(ctx, "svc", 0); len(rows) != 0 {
		t.Errorf("max 0: rows = %+v, want none", rows)
	}
}
