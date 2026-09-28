package main

// route_issue_shape_test.go — SR-44 (F-25, docs/design/surface-
// reckoning.md §5): the prompt hook routes issue-shaped prompts (a stack
// trace, a failing test name, an error string) to localize
// deterministically, never through the probabilistic POST /route call
// (ADR-042, CV-M11: the LLM-judged route matched its own adjudicated gold
// tool only 45.7% of the time — these three shapes are lexically
// unambiguous and need no judgment call at all).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestPromptHookRoutesIssueShapesToLocalize is SR-44's reachable_from
// (routeIssueShapedPrompt, package cmd/fathom).
func TestPromptHookRoutesIssueShapesToLocalize(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_ = json.NewEncoder(w).Encode(map[string]any{
			"shape": "where", "verb": "find_symbol", "confidence": 0.99, "band": "proceed", "provider": "llm",
		})
	}))
	defer srv.Close()
	hc := &http.Client{Timeout: deadlineRemaining(deadline)}

	cases := []struct{ name, prompt string }{
		{
			"stack trace (Go panic)",
			"panic: runtime error: invalid memory address or nil pointer dereference\n\ngoroutine 1 [running]:\nmain.main()\n\t/app/main.go:42 +0x1a",
		},
		{
			"failing test name alone",
			"TestForgottenFeature/nested_case",
		},
		{
			"failing test, go test summary shape",
			"--- FAIL: TestForgottenFeature (0.00s)\n    forgotten_test.go:12: expected true, got false",
		},
		{
			"error string",
			"TypeError: cannot read property 'foo' of undefined",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			called = false
			got := routeLineFor(hc, srv.URL, c.prompt, deadline)
			if called {
				t.Errorf("issue-shaped prompt %q reached POST /route — must route deterministically, no HTTP call", c.prompt)
			}
			if !strings.Contains(got, "localize") {
				t.Errorf("routeLineFor(%q) = %q, want a line naming localize", c.prompt, got)
			}
		})
	}

	t.Run("non-issue prompt still asks the daemon", func(t *testing.T) {
		called = false
		got := routeLineFor(hc, srv.URL, "where is HandleLogin defined", deadline)
		if !called {
			t.Error("a non-issue-shaped prompt must still ask POST /route")
		}
		if !strings.Contains(got, "find_symbol") {
			t.Errorf("routeLineFor = %q, want the daemon's own line naming find_symbol", got)
		}
	})
}

// TestRouteIssueShapedPrompt_NoFalsePositiveOnOrdinaryQuestion pins the
// conservative side: an ordinary question that merely mentions a word
// like "error" in prose, with no structural marker, is not issue-shaped.
func TestRouteIssueShapedPrompt_NoFalsePositiveOnOrdinaryQuestion(t *testing.T) {
	for _, prompt := range []string{
		"how should we handle errors in this handler?",
		"what governs error-wrapping conventions here?",
		"can you explain what this function does",
	} {
		if _, _, ok := routeIssueShapedPrompt(prompt); ok {
			t.Errorf("routeIssueShapedPrompt(%q) = ok, want false (ordinary prose, no structural marker)", prompt)
		}
	}
}
