package mcp

// references_symlinkroot_test.go — O39 reproduced end to end: the index
// was built at a service's real root, and the root recorded for it (the
// one `references` re-reads sources from at query time) names the same
// directory through a symlink. Before the fix references returned the
// definition and ZERO occurrences — exact or not — while verify_plan,
// answering from the store alone, still resolved every caller.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func callToolArgs(t *testing.T, s *Server, name string, args map[string]any) map[string]any {
	t.Helper()
	req, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, ok := s.HandleRequest(context.Background(), req)
	if !ok {
		t.Fatalf("%s: no response", name)
	}
	var env struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(resp, &env); err != nil || env.Error != nil || len(env.Result.Content) == 0 {
		t.Fatalf("%s: bad response %s (err=%v)", name, resp, err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(env.Result.Content[0].Text), &payload); err != nil {
		t.Fatalf("%s: payload: %v", name, err)
	}
	return payload
}

func TestReferences_RecordedRootThroughSymlinkStillFindsCallers(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	for rel, body := range map[string]string{
		"go.mod":        "module example.com/o39\n\ngo 1.22\n",
		"pkg/a.go":      "package pkg\n\n// Target is what everyone calls.\nfunc Target() int { return 1 }\n",
		"pkg/b.go":      "package pkg\n\n// Caller calls Target.\nfunc Caller() int {\n\treturn Target() + 1\n}\n",
		"pkg/b_test.go": "package pkg\n\nimport \"testing\"\n\nfunc TestCaller(t *testing.T) {\n\tif Caller() != 2 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n",
	} {
		full := filepath.Join(real, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	ctx := context.Background()
	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	if err := c.IngestServiceAt(ctx, "o39", real); err != nil {
		t.Fatalf("IngestServiceAt: %v", err)
	}
	meta, found, err := st.GetServiceMeta(ctx, "o39")
	if err != nil || !found {
		t.Fatalf("GetServiceMeta: found=%v err=%v", found, err)
	}
	meta.Path = link // the recorded root, differing from the ingest root only by a symlink
	if err := st.SetServiceMeta(ctx, meta); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(c)
	for _, exact := range []bool{true, false} {
		payload := callToolArgs(t, srv, "references", map[string]any{"services": []any{"*"}, "service": "o39", "query": "Target", "exact": exact})
		occs, _ := payload["occurrences"].([]any)
		if len(occs) == 0 {
			t.Errorf("exact=%v: references found no occurrence of Target through the symlinked recorded root: %v", exact, payload["summary"])
			continue
		}
		if exact {
			first, _ := occs[0].(map[string]any)
			if first["loc"] != "pkg/b.go:5" || first["c"] != "r" {
				t.Errorf("exact occurrence = %v, want pkg/b.go:5 confirmed resolved (c=r)", first)
			}
		}
	}
}
