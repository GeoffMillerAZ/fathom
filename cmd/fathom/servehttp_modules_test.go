package main

// servehttp_modules_test.go — the module host's serve-http wiring
// (docs/plans/gke-module-host.md): --module mounts a store read-only for
// obligations' module bindings, and a non-conforming/unreadable store
// aborts startup loudly (GKE-MH-1). The /rpc/modules surface, its four
// tools and hosted-modules mode (--modules-token-env) were removed by
// SR-17 (surface reckoning F-68).

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

// TestRunServeHTTP_ModulesTokenEnvIsGone is SR-17 (surface reckoning F-68):
// hosted-modules mode is removed, so --modules-token-env is no longer a
// flag — a script still passing it fails loudly instead of believing it
// configured an auth boundary.
func TestRunServeHTTP_ModulesTokenEnvIsGone(t *testing.T) {
	withIsolatedConfigHome(t)
	storePath := filepath.Join(t.TempDir(), "fleet.sqlite")
	t.Setenv("FATHOM_TEST_TOKEN_VAR", "secret")

	var out, errBuf strings.Builder
	code := runServeHTTP([]string{"--store", storePath, "--modules-token-env", "FATHOM_TEST_TOKEN_VAR"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("serve-http accepted --modules-token-env; SR-17 removed hosted-modules mode")
	}
	if !strings.Contains(errBuf.String(), "modules-token-env") {
		t.Fatalf("stderr = %q, want the unknown-flag error naming modules-token-env", errBuf.String())
	}
}

// TestServeHTTPModulesPathIsGone: with a module store mounted (the shape
// that used to serve /rpc/modules), the path now 404s like any unknown
// surface, and the full registry lists none of the four module tools.
func TestServeHTTPModulesPathIsGone(t *testing.T) {
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
	handler := newHTTPHandlerLogged(mcp.NewServerWithModules(c, nil, nil, h), c, "127.0.0.1:0", nil, testDiscardLogger())

	req := httptest.NewRequest(http.MethodPost, "/rpc/modules", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("POST /rpc/modules = %d, want 404 (SR-17 removed the surface)", rr.Code)
	}

	resp := postRPCPath(t, handler, "/rpc", rpcBody(t, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]any{},
	}))
	raw, _ := json.Marshal(resp)
	for _, gone := range []string{"list_modules", "module_obligations", "module_query", "module_staleness"} {
		if strings.Contains(string(raw), `"`+gone+`"`) {
			t.Errorf("the full registry still lists %s", gone)
		}
	}
}
