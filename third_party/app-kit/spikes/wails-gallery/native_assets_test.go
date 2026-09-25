//go:build !server && (darwin || windows)

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPOnlyAssetsDenyBeforeFrameworkRouting(t *testing.T) {
	called := false
	assets := httpOnlyAssets()
	handler := assets.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	for _, path := range []string{"/", "/wails/runtime", "/wails/runtime.js", "/wails/ipc", "/anything"} {
		reply := httptest.NewRecorder()
		handler.ServeHTTP(reply, httptest.NewRequest(http.MethodPost, path, nil))
		if reply.Code != http.StatusNotFound || called {
			t.Fatalf("%s reached framework routing: status=%d called=%v", path, reply.Code, called)
		}
	}
}
