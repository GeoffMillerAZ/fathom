package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"bytes"

	"github.com/geoffmilleraz/fathom/pkg/client"
)

// O1151: a hook client names the hook payload's session_id in
// X-Fathom-Caller's session= field, so the daemon's /rpc and /route see a
// session (ADR-047; KS-14 keys its killed line by it).
func TestHookClients_SendPayloadSessionInCallerHeader(t *testing.T) {
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "fathom-guard-stamps")
	t.Cleanup(func() { guardStampDir = orig })

	var mu sync.Mutex
	seen := map[string][]string{} // origin -> session= values, in call order
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, ok := client.ParseCallerHeader(r.Header.Get(client.CallerHeaderName)); ok {
			mu.Lock()
			seen[c.Origin] = append(seen[c.Origin], c.Session)
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{}"}]}}`)
	}))
	defer srv.Close()

	dir := touchTree(t, t.TempDir(), "internal/store/backend.go")
	writeGuardCue(t, dir, `guards: [{scopes: ["internal/store/**"], note: "Store schema is spec-governed."}]`)
	for _, sid := range []string{"sess-one", "sess-two"} {
		hookStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"PreToolUse","tool_name":"Edit","tool_input":{"file_path":%q}}`,
			sid, dir, filepath.Join(dir, "internal/store/backend.go")))
		var out, errBuf bytes.Buffer
		runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf)

		hookStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"UserPromptSubmit","prompt":"please refactor internal/store/backend.go"}`, sid, dir))
		out.Reset()
		runWithStdin([]string{"prompt-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, origin := range []string{"edit-hook", "prompt-hook"} {
		got := seen[origin]
		has := map[string]bool{}
		for _, s := range got {
			has[s] = true
		}
		if !has["sess-one"] || !has["sess-two"] || has[""] {
			t.Errorf("%s: daemon saw session= values %v, want both sess-one and sess-two and no empty", origin, got)
		}
	}
}

// The test-run hook sets the session on the same path (its RPCs go through
// originHeader like the others).
func TestOriginHeader_CarriesRPCSession(t *testing.T) {
	old := rpcSession
	t.Cleanup(func() { rpcSession = old })
	rpcSession = "sess-x"
	req, err := originHeader("http://127.0.0.1:1", bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	c, ok := client.ParseCallerHeader(req.Header.Get(client.CallerHeaderName))
	if !ok || c.Session != "sess-x" {
		t.Fatalf("session = %q (ok=%v), want sess-x", c.Session, ok)
	}
}
