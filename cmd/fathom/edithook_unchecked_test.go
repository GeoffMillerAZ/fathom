package main

// edithook_unchecked_test.go — RT-26 at its consumer: the obligations tool
// names a file it did not read for its freshness check (over the 2 MiB
// hot-path cap) as index_unchecked. The edit hooks rendered only
// index_stale, so an oversize edited file got no freshness line at all,
// which reads as current (I38). Both hooks now say the check was skipped,
// and never ask the daemon to refresh it.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const uncheckedReason = "file is 3.0 MiB, over the 2.0 MiB read cap"

func uncheckedDaemon(t *testing.T, dir, rel string, refreshes *int32) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		if rpcRequestName(r) != "obligations" {
			json.NewEncoder(w).Encode(rpcTextResult(map[string]any{"rules": []any{}}))
			return
		}
		json.NewEncoder(w).Encode(rpcTextResult(map[string]any{
			"service": strings.ToLower(filepath.Base(dir)),
			"file":    rel,
			"rules": []any{
				map[string]any{"rule": "Generated data stays generated.", "src": "CLAUDE.md#L1", "tier": "verified"},
			},
			"index_unchecked": uncheckedReason,
		}))
	})
	mux.HandleFunc("/refresh-file", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(refreshes, 1)
		json.NewEncoder(w).Encode(refreshFileResponse{Refreshed: true})
	})
	return httptest.NewServer(mux)
}

func TestEditHookPre_IndexUncheckedNamed(t *testing.T) {
	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [{scopes: ["data/**"], note: "Data is generated."}]`)
	file := filepath.Join(dir, "data", "big.json")
	var refreshes int32
	srv := uncheckedDaemon(t, dir, "data/big.json", &refreshes)
	defer srv.Close()

	hookStdin(t, fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q,"session_id":"sess-unchecked"}`, file, dir))
	var out, errBuf bytes.Buffer
	if code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("exit = %d; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "index freshness not checked for this file ("+uncheckedReason+")") {
		t.Errorf("pre hook did not name the skipped freshness check: %s", out.String())
	}
	if n := atomic.LoadInt32(&refreshes); n != 0 {
		t.Errorf("refresh requested %d time(s) for a file the daemon will not read", n)
	}
}

func TestEditHookPost_IndexUncheckedNamed(t *testing.T) {
	dir := t.TempDir()
	var refreshes int32
	srv := uncheckedDaemon(t, dir, "data/big.json", &refreshes)
	defer srv.Close()
	absFile := filepath.Join(dir, "data", "big.json")
	hookInput := fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, absFile, dir)

	origStdin := os.Stdin
	pr, pw, _ := os.Pipe()
	os.Stdin = pr
	pw.WriteString(hookInput)
	pw.Close()
	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"edit-hook", "--url=" + srv.URL}, pr, &out, &errBuf)
	os.Stdin = origStdin
	pr.Close()
	if code != 0 {
		t.Fatalf("exit = %d; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "index freshness not checked for this file ("+uncheckedReason+")") {
		t.Errorf("post hook did not name the skipped freshness check: %s", out.String())
	}
}
