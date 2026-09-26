package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestOriginHeaderCarriesCallerTestFlag (RT-164): every hook/CLI RPC built
// by originHeader carries X-Fathom-Caller, marked test=1 under test.
func TestOriginHeaderCarriesCallerTestFlag(t *testing.T) {
	req, err := originHeader("http://127.0.0.1:1", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	c, ok := client.ParseCallerHeader(req.Header.Get(client.CallerHeaderName))
	if !ok || !c.Test {
		t.Fatalf("X-Fathom-Caller = %q, want a parsed header with test=1", req.Header.Get(client.CallerHeaderName))
	}
}
