package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestModuleListForDoctor pins PC-11: "tool not found" for list_modules
// becomes the real reason (no module store mounted), a served tool yields
// module → path, and any other RPC failure is passed through unchanged.
func TestModuleListForDoctor(t *testing.T) {
	hc := &http.Client{}

	t.Run("unregistered tool names the real reason", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"tool not found: list_modules"}}`))
		}))
		defer ts.Close()
		_, err := moduleListForDoctor(hc, ts.URL)
		if !errors.Is(err, errModulesUnserved) {
			t.Fatalf("err = %v, want errModulesUnserved", err)
		}
		if strings.Contains(err.Error(), "tool not found") || !strings.Contains(err.Error(), "--module") {
			t.Errorf("reason must explain the mount, not repeat 'tool not found': %v", err)
		}
		// The binding rows carry that reason into their SKIP.
		root := t.TempDir()
		writeTestBindings(t, root, `bindings: [{module: "m"}]`)
		c := checkBindingModulesMounted(hc, ts.URL, root, true)
		if c.Status != doctorSkipped || !strings.Contains(c.Reason, "no module store was mounted") {
			t.Errorf("binding_modules_mounted = %s %q", c.Status, c.Reason)
		}
	})
	t.Run("served tool lists modules", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeRPCText(w, `{"modules":[{"module":"m","path":"/m.sqlite"},{"module":"m","path":"/dup.sqlite"}]}`)
		}))
		defer ts.Close()
		paths, err := moduleListForDoctor(hc, ts.URL)
		if err != nil {
			t.Fatal(err)
		}
		if paths["m"] != "/m.sqlite" {
			t.Errorf("paths = %v, want first path kept per module", paths)
		}
	})
	t.Run("other failures pass through", func(t *testing.T) {
		_, err := moduleListForDoctor(hc, "http://127.0.0.1:1")
		if err == nil || errors.Is(err, errModulesUnserved) {
			t.Errorf("connection failure must not be mistaken for an unmounted module host: %v", err)
		}
	})
}
