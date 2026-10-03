package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// CS-2 (docs/plans/cloud-sessions.md): `serve-mcp --url` is a stdio front
// for a running serve-http daemon. A cloud VM runs one daemon (the hooks
// talk to it on 127.0.0.1:7575) and the harness spawns this front, so the
// session's MCP tools and its hooks read one store with one writer.

// TestServeMCPURLForwardsToTheDaemon drives the front against a real
// serve-http handler: initialize and tools/list come back over stdio, the
// notification gets no line, and no store is opened by the front itself.
func TestServeMCPURLForwardsToTheDaemon(t *testing.T) {
	daemon := httptest.NewServer(newTestHandler(t))
	t.Cleanup(daemon.Close)

	in := buildMCPInput(t, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	var out, errBuf bytes.Buffer
	code := runServeMCP([]string{"--url", daemon.URL + "/rpc/agent", "--wait", "5s"}, in, &out, &errBuf)
	if code != 0 {
		t.Fatalf("serve-mcp --url exited %d; stderr=%q", code, errBuf.String())
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 response lines (initialize, tools/list; the notification answers nothing), got %d: %q", len(lines), out.String())
	}
	responses := parseAllResponses(t, &out)
	if si, _ := responses[0]["result"].(map[string]any)["serverInfo"].(map[string]any); si["name"] != "fathom" {
		t.Fatalf("initialize did not come back through the front: %v", responses[0])
	}
	tools, _ := responses[1]["result"].(map[string]any)["tools"].([]any)
	if len(tools) == 0 {
		t.Fatalf("tools/list came back empty through the front: %v", responses[1])
	}
	if !strings.Contains(errBuf.String(), "serve-mcp: stdio front for "+daemon.URL+"/rpc/agent") {
		t.Fatalf("the front must name the daemon it forwards to on stderr, got %q", errBuf.String())
	}
}

// TestServeMCPURLCarriesTheNegotiatedProtocolVersion: after initialize the
// front sends the negotiated revision on every request, as a Streamable
// HTTP client does, and forwards the request body byte for byte.
func TestServeMCPURLCarriesTheNegotiatedProtocolVersion(t *testing.T) {
	var mu sync.Mutex
	var bodies, versions []string
	daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			_, _ = io.WriteString(w, "ok")
			return
		}
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		versions = append(versions, r.Header.Get("MCP-Protocol-Version"))
		mu.Unlock()
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.Unmarshal(b, &req)
		if len(req.ID) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "initialize" {
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+string(req.ID)+`,"result":{"protocolVersion":"2025-06-18"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+string(req.ID)+`,"result":{}}`)
	}))
	t.Cleanup(daemon.Close)

	initLine := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`
	call := `{"jsonrpc":"2.0","id":"two","method":"tools/list"}`
	var out, errBuf bytes.Buffer
	code := runServeMCP([]string{"--url", daemon.URL + "/rpc/agent"}, strings.NewReader(initLine+"\n\n"+call+"\n"), &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, errBuf.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 2 || bodies[0] != initLine || bodies[1] != call {
		t.Fatalf("the front must forward each non-blank line verbatim, got %q", bodies)
	}
	if versions[0] != "" || versions[1] != "2025-06-18" {
		t.Fatalf("MCP-Protocol-Version: want none on initialize and the negotiated one after, got %q", versions)
	}
	if !strings.Contains(out.String(), `"id":"two"`) {
		t.Fatalf("a string id must come back as sent, got %q", out.String())
	}
}

// TestServeMCPURLAnswersEveryRequestWhenTheDaemonIsDown: a request with an
// id always gets a line — a JSON-RPC error naming the daemon — so the
// harness never waits on a call nobody will answer; a notification still
// gets nothing, and the front exits 0 at end of input.
func TestServeMCPURLAnswersEveryRequestWhenTheDaemonIsDown(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	url := dead.URL + "/rpc/agent"
	dead.Close() // nothing listens there now

	in := strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"tools/list"}` + "\n" + `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n")
	var out, errBuf bytes.Buffer
	start := time.Now()
	code := runServeMCP([]string{"--url", url, "--wait", "300ms"}, in, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, errBuf.String())
	}
	if time.Since(start) > 20*time.Second {
		t.Fatalf("a dead daemon must not hang the front (took %s)", time.Since(start))
	}
	responses := parseAllResponses(t, &out)
	if len(responses) != 1 {
		t.Fatalf("want exactly one error line (the notification answers nothing), got %q", out.String())
	}
	if responses[0]["id"] != float64(7) {
		t.Fatalf("the error must carry the request's id, got %v", responses[0])
	}
	e, _ := responses[0]["error"].(map[string]any)
	if msg, _ := e["message"].(string); !strings.Contains(msg, url) {
		t.Fatalf("the error must name the daemon URL, got %v", responses[0])
	}
	if !strings.Contains(errBuf.String(), "did not answer /healthz") {
		t.Fatalf("a daemon that never came up must be said on stderr, got %q", errBuf.String())
	}
}

// TestServeMCPURLAndStoreAreExclusive: the front opens no store, so naming
// one beside --url is a mistake said by name, never a silent choice.
func TestServeMCPURLAndStoreAreExclusive(t *testing.T) {
	store := filepath.Join(t.TempDir(), "test.sqlite")
	var out, errBuf bytes.Buffer
	code := runServeMCP([]string{"--url", "http://127.0.0.1:1/rpc/agent", "--store", store}, strings.NewReader(""), &out, &errBuf)
	if code == 0 || !strings.Contains(errBuf.String(), "--store and --url are exclusive") {
		t.Fatalf("want a refusal naming both flags, got exit %d stderr=%q", code, errBuf.String())
	}
}
