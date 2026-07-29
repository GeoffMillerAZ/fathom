package main

// servehttp_modules_test.go — the GKE module host's serve-http wiring
// (docs/plans/gke-module-host.md): --module mounts a store read-only onto
// POST /rpc/modules, a non-conforming/unreadable store aborts startup
// loudly (GKE-MH-1), and --modules-token-env gates /rpc/modules behind a
// bearer token (GKE-MH-7) without touching any other route.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/modulehost"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// buildConformingModuleStore creates a conforming module store at dir/name
// carrying one observation for serviceID, and returns its path.
func buildConformingModuleStore(t *testing.T, dir, name, serviceID string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	if err := st.PutObservations(context.Background(), []core.Observation{{
		ServiceID: serviceID, SourceRef: "a.go", SourceKind: core.KindGoFunc,
		EvidenceTier: core.TierVerified, Symbol: "Foo",
	}}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return path
}

// ─── flag wiring: --module ────────────────────────────────────────────────

func TestRunServeHTTP_RefusesNonConformingModuleLoudly(t *testing.T) {
	withIsolatedConfigHome(t)
	storePath := filepath.Join(t.TempDir(), "fleet.sqlite")
	badModule := filepath.Join(t.TempDir(), "bad.txt")

	var out, errBuf strings.Builder
	code := runServeHTTP([]string{"--store", storePath, "--module", badModule}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --module names a non-conforming store")
	}
	if !strings.Contains(errBuf.String(), "mounting module") {
		t.Fatalf("stderr = %q, want a \"mounting module\" refusal naming the failure", errBuf.String())
	}
}

func TestRunServeHTTP_RefusesEmptyModulesTokenEnv(t *testing.T) {
	withIsolatedConfigHome(t)
	storePath := filepath.Join(t.TempDir(), "fleet.sqlite")
	t.Setenv("FATHOM_TEST_EMPTY_TOKEN_VAR", "")

	var out, errBuf strings.Builder
	code := runServeHTTP([]string{"--store", storePath, "--modules-token-env", "FATHOM_TEST_EMPTY_TOKEN_VAR"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --modules-token-env names an empty/unset variable")
	}
	if !strings.Contains(errBuf.String(), "modules-token-env") {
		t.Fatalf("stderr = %q, want the modules-token-env misconfiguration error", errBuf.String())
	}
}

// ─── HTTP wiring: /rpc/modules serves the mounted module ─────────────────

// newModulesTestHandler builds a real mcp.Server wired to a modulehost.Host
// with one mounted module store, wrapped in the HTTP handler under test —
// mirrors newTestHandler (servehttp_test.go) but adds the module surface.
func newModulesTestHandler(t *testing.T, modulesToken string) http.Handler {
	t.Helper()
	fleetStore := filepath.Join(t.TempDir(), "fleet.sqlite")
	c, err := client.NewConstantClient(client.ModeLocal, fleetStore)
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	t.Cleanup(func() { c.Close() })

	modPath := buildConformingModuleStore(t, t.TempDir(), "alpha.sqlite", "svc-a")
	h := modulehost.NewHost(store.LevelL0)
	if _, err := h.Mount(modPath, "alpha"); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	srv := mcp.NewServerWithModules(c, nil, nil, h)
	return newHTTPHandlerWithAuth(srv, modulesToken)
}

func TestServeHTTPModulesSurface_ListsMountedModule(t *testing.T) {
	handler := newModulesTestHandler(t, "")
	resp := postRPCPath(t, handler, "/rpc/modules", rpcBody(t, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "list_modules", "arguments": map[string]any{}},
	}))
	if resp["error"] != nil {
		t.Fatalf("list_modules via /rpc/modules: %v", resp["error"])
	}
	text := resp["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	var payload struct {
		Modules []map[string]any `json:"modules"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("decode: %v (text=%q)", err, text)
	}
	if len(payload.Modules) != 1 || payload.Modules[0]["module"] != "alpha" {
		t.Fatalf("list_modules over HTTP = %+v, want exactly module=alpha", payload.Modules)
	}
}

// ─── auth: /rpc/modules is gated, everything else is not ─────────────────

func TestServeHTTPModulesAuth_401WithoutToken200With(t *testing.T) {
	const token = "s3cr3t"
	handler := newModulesTestHandler(t, token)

	// No Authorization header at all: 401.
	req := httptest.NewRequest(http.MethodPost, "/rpc/modules", rpcBody(t, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]any{},
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("POST /rpc/modules without a token: status = %d, want 401", rr.Code)
	}
	if strings.Contains(rr.Body.String(), token) {
		t.Error("401 response body must never echo the configured token")
	}

	// Wrong token: still 401.
	req2 := httptest.NewRequest(http.MethodPost, "/rpc/modules", rpcBody(t, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{},
	}))
	req2.Header.Set("Authorization", "Bearer wrong-token")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusUnauthorized {
		t.Fatalf("POST /rpc/modules with the wrong token: status = %d, want 401", rr2.Code)
	}

	// Correct token: 200.
	req3 := httptest.NewRequest(http.MethodPost, "/rpc/modules", rpcBody(t, map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "tools/list", "params": map[string]any{},
	}))
	req3.Header.Set("Authorization", "Bearer "+token)
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusOK {
		t.Fatalf("POST /rpc/modules with the correct token: status = %d, want 200 (body=%q)", rr3.Code, rr3.Body.String())
	}
}

func TestServeHTTPModulesAuth_OnlyGatesModulesPath(t *testing.T) {
	const token = "s3cr3t"
	handler := newModulesTestHandler(t, token)

	// Bare /rpc (full, unscoped surface) and /rpc/agent stay unauthenticated
	// even when --modules-token-env is set — the token gates ONLY
	// /rpc/modules, never the daemon's existing loopback-only trust
	// boundary.
	for _, path := range []string{"/rpc", "/rpc/agent"} {
		req := httptest.NewRequest(http.MethodPost, path, rpcBody(t, map[string]any{
			"jsonrpc": "2.0", "id": 4, "method": "tools/list", "params": map[string]any{},
		}))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("POST %s with modules auth configured: status = %d, want 200 (unauthenticated, untouched)", path, rr.Code)
		}
	}
}

func TestModulesAuthMiddleware_DisabledWhenTokenEmpty(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := modulesAuthMiddleware(inner, "")
	req := httptest.NewRequest(http.MethodPost, "/rpc/modules", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("modulesAuthMiddleware with an empty token: status = %d, want 200 (auth disabled by default)", rr.Code)
	}
}
