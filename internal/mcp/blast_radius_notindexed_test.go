package mcp_test

// blast_radius_notindexed_test.go — O1071: blast_radius on a file the
// index does not hold answered an empty success (zero symbols, zero
// tests), indistinguishable from a file nothing depends on. A changed file
// the index has no symbol in and the corpus does not list is named.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

func blastCall(t *testing.T, srv *mcp.Server, files ...string) map[string]any {
	t.Helper()
	return roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "blast_radius",
			"arguments": map[string]any{"service": "svc", "changed_files": files},
		},
	})
}

func TestBlastRadius_FileNotIndexedIsAnError(t *testing.T) {
	srv := newExtSrv()
	for _, f := range []string{"pkg/nope.go", "/abs/pkg/a.go"} {
		resp := blastCall(t, srv, f)
		e, ok := resp["error"].(map[string]any)
		if !ok {
			t.Fatalf("blast_radius(%s): no error (result %v), want not-indexed", f, resp["result"])
		}
		msg, _ := e["message"].(string)
		if !strings.Contains(msg, "not indexed") || !strings.Contains(msg, f) || !strings.Contains(msg, "repo-relative") {
			t.Fatalf("blast_radius(%s): error %q, want it to name the file as not indexed and the repo-relative form", f, msg)
		}
	}
}

func TestBlastRadius_SomeFilesNotIndexedAreNamed(t *testing.T) {
	srv := newExtSrv()
	resp := blastCall(t, srv, "pkg/a.go", "pkg/nope.go")
	if resp["error"] != nil {
		t.Fatalf("one indexed file: error %v, want an answer", resp["error"])
	}
	p := xtraPayload(t, resp)
	ni, _ := p["not_indexed"].([]any)
	if len(ni) != 1 || ni[0] != "pkg/nope.go" {
		t.Fatalf("not_indexed = %v, want [pkg/nope.go]", p["not_indexed"])
	}
	if syms, _ := p["symbols"].([]any); len(syms) == 0 {
		t.Fatalf("the indexed file's radius was dropped: %v", p)
	}
	if s, _ := p["summary"].(string); !strings.Contains(s, "not indexed: pkg/nope.go") {
		t.Fatalf("summary %q does not name the unindexed file", s)
	}
}
