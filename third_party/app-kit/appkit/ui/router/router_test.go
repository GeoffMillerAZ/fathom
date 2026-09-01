package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestScript_NonEmpty verifies that Script() returns a non-empty string —
// the embed is wired and the build sees the file.
func TestScript_NonEmpty(t *testing.T) {
	s := Script()
	if s == "" {
		t.Fatal("Script() returned empty string; go:embed may be broken")
	}
}

// TestScript_ContainsAPISymbols verifies that the embedded source exposes
// the public API surface the kit documents.
func TestScript_ContainsAPISymbols(t *testing.T) {
	s := Script()
	for _, want := range []string{
		"AppKitRouter",
		"createRouter",
		"navigate",
		"popstate",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("Script() is missing expected symbol %q", want)
		}
	}
}

// TestHandler_StatusAndContentType verifies that Handler() responds with
// HTTP 200 and the correct Content-Type for a JavaScript asset.
func TestHandler_StatusAndContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/router.js", nil)

	Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Handler() status = %d; want %d", rec.Code, http.StatusOK)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "javascript") {
		t.Errorf("Handler() Content-Type = %q; want a javascript content-type", ct)
	}
}

// TestHandler_BodyContainsAppKitRouter verifies that the response body
// includes the public global name consumers depend on.
func TestHandler_BodyContainsAppKitRouter(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/router.js", nil)

	Handler().ServeHTTP(rec, req)

	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if !strings.Contains(string(body), "AppKitRouter") {
		t.Errorf("Handler() body missing \"AppKitRouter\"; got %d bytes", len(body))
	}
}

// TestHandler_CacheControl verifies that a long-lived cache header is set
// so static hosts can cache the asset aggressively.
func TestHandler_CacheControl(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/router.js", nil)

	Handler().ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc == "" {
		t.Error("Handler() missing Cache-Control header")
	}
}

// TestScriptAndHandlerConsistency verifies that Script() and Handler() serve
// the same source — they must not diverge (both embed the same var).
func TestScriptAndHandlerConsistency(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/router.js", nil)
	Handler().ServeHTTP(rec, req)

	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if string(body) != Script() {
		t.Error("Handler() body and Script() differ; they must be identical")
	}
}
