package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/client"
)

// O1151 remainder: the stop and session hooks reach the daemon too, so they
// name their session in X-Fathom-Caller. The session-end, pre-compact and
// config-change hooks make no daemon call (local thread store and config
// audit only) and need nothing.
func TestStopAndSessionHooks_SendPayloadSessionInCallerHeader(t *testing.T) {
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "fathom-guard-stamps")
	t.Cleanup(func() { guardStampDir = orig })
	oldSession, oldOrigin := rpcSession, rpcOrigin
	t.Cleanup(func() { rpcSession, rpcOrigin = oldSession, oldOrigin })

	var mu sync.Mutex
	seen := map[string][]string{} // origin -> session= values
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

	dir := t.TempDir()
	for _, sid := range []string{"sess-one", "sess-two"} {
		var out, errBuf bytes.Buffer
		hookStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"SessionStart","source":"startup"}`, sid, dir))
		runWithStdin([]string{"session-hook", "--url=" + srv.URL, "--dir=" + dir}, os.Stdin, &out, &errBuf)
		if rpcSession != sid {
			t.Errorf("session-hook left rpcSession = %q, want %q", rpcSession, sid)
		}
		rpcSession = ""
		hookStdin(t, fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"Stop"}`, sid, dir))
		runWithStdin([]string{"stop-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
		if rpcSession != sid {
			t.Errorf("stop-hook left rpcSession = %q, want %q", rpcSession, sid)
		}
		rpcSession = ""
	}
	mu.Lock()
	defer mu.Unlock()
	has := map[string]bool{}
	for _, s := range seen["session-hook"] {
		has[s] = true
	}
	if !has["sess-one"] || !has["sess-two"] || has[""] {
		t.Errorf("session-hook: daemon saw session= values %v, want sess-one and sess-two and no empty", seen["session-hook"])
	}
}
