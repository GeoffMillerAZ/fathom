package main

// route_issue_shape_test.go — SR-44 (F-25, docs/design/surface-
// reckoning.md §5): the prompt hook routes issue-shaped prompts (a stack
// trace, a failing test name, an error string) to localize
// deterministically, never through the probabilistic POST /route call
// (ADR-042, CV-M11: the LLM-judged route matched its own adjudicated gold
// tool only 45.7% of the time — these three shapes are lexically
// unambiguous and need no judgment call at all).

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
			got := routeLineFor(hc, srv.URL, t.TempDir(), c.prompt, deadline)
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
		got := routeLineFor(hc, srv.URL, t.TempDir(), "where is HandleLogin defined", deadline)
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

// frameFixtureForHook is internal/stackframe/testdata/frames.json, TL-1's
// one shared frame fixture (the localize tests read it too).
type frameFixtureForHook struct {
	Sentinel string `json:"sentinel"`
	Cases    []struct {
		Name string `json:"name"`
		Text string `json:"text"`
	} `json:"cases"`
	NotIssueShaped []string `json:"not_issue_shaped"`
}

func loadFrameFixtureForHook(t *testing.T) frameFixtureForHook {
	t.Helper()
	b, err := os.ReadFile("../../internal/stackframe/testdata/frames.json")
	if err != nil {
		t.Fatal(err)
	}
	var f frameFixtureForHook
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// TestPromptHookRoutesEveryFixtureFrameShapeToLocalize is TL-1's hook row:
// every trace and payload of the shared fixture (V8 colon frames, PHP
// frames, JVM, Python, Go, and the Bugsnag, OTLP, Lambda and New Relic
// payloads) is issue-shaped, the full `fathom prompt-hook` run names
// localize, and the hook output never carries the message, breadcrumb,
// user or metaData sentinel. The negatives stay unrouted.
func TestPromptHookRoutesEveryFixtureFrameShapeToLocalize(t *testing.T) {
	f := loadFrameFixtureForHook(t)
	srv := fakePromptDaemon(t, nil, nil)
	defer srv.Close()
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			if _, _, ok := routeIssueShapedPrompt(c.Text); !ok {
				t.Fatalf("not issue-shaped: %q", c.Text)
			}
			promptHookStdin(t, t.TempDir(), c.Text)
			var out, errBuf bytes.Buffer
			if code := runWithStdin([]string{"prompt-hook", "--url=" + srv.URL}, nil, &out, &errBuf); code != 0 {
				t.Fatalf("prompt-hook exit = %d; stderr=%s", code, errBuf.String())
			}
			if !strings.Contains(out.String(), "localize") {
				t.Errorf("hook output does not name localize: %q", out.String())
			}
			if strings.Contains(out.String(), f.Sentinel) {
				t.Errorf("hook output carries message text: %q", out.String())
			}
		})
	}
	for _, s := range f.NotIssueShaped {
		if _, _, ok := routeIssueShapedPrompt(s); ok {
			t.Errorf("routeIssueShapedPrompt(%q) = ok, want false", s)
		}
	}
}
