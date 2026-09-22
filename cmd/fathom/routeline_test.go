package main

// routeline_test.go — DP-10 (docs/plans/decision-port.md): `fathom
// prompt-hook`'s POST /route line.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// fakeRouteDaemon serves POST /route with body (marshaled as-is) and
// status (default 200 when zero). A nil body writes nothing (a malformed/
// empty response).
func fakeRouteDaemon(t *testing.T, status int, body any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != 0 {
			w.WriteHeader(status)
		}
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

// TestPromptHook_RouteLineBands pins DP-10's reachable_from (routeLine,
// package cmd/fathom): a proceed-band response renders one firm line, a
// confirm-band response renders one hedged line, and every other case —
// unsure band, verb "none", provider "none", a non-200 status, or a
// malformed body — renders nothing at all. Also covers combineWithRouteLine
// (the pure byte-budget-drop helper routeLine's caller, cmdPromptHook,
// folds the line through): a route line that would push the combined text
// over the hook's existing byte budget (harness.CodexAdditionalContextCap)
// is dropped, never truncating the obligations text it would have joined.
func TestPromptHook_RouteLineBands(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)

	t.Run("proceed band: firm line", func(t *testing.T) {
		srv := fakeRouteDaemon(t, 0, map[string]any{
			"shape": "issue", "verb": "localize", "confidence": 0.93, "band": "proceed", "provider": "llm",
		})
		defer srv.Close()
		hc := &http.Client{Timeout: deadlineRemaining(deadline)}
		got := routeLine(hc, srv.URL, "TypeError: x is undefined", deadline)
		want := "fathom route: this prompt is issue-shaped — open with localize (confidence 0.93)."
		if got != want {
			t.Errorf("routeLine = %q, want %q", got, want)
		}
	})

	t.Run("confirm band: hedged line", func(t *testing.T) {
		srv := fakeRouteDaemon(t, 0, map[string]any{
			"shape": "issue", "verb": "localize", "confidence": 0.71, "band": "confirm", "provider": "llm",
		})
		defer srv.Close()
		hc := &http.Client{Timeout: deadlineRemaining(deadline)}
		got := routeLine(hc, srv.URL, "the sync sometimes fails", deadline)
		want := "fathom route: this looks issue-shaped; localize is the likely opening move (confidence 0.71, unconfirmed)."
		if got != want {
			t.Errorf("routeLine = %q, want %q", got, want)
		}
	})

	t.Run("unsure band: nothing", func(t *testing.T) {
		srv := fakeRouteDaemon(t, 0, map[string]any{
			"shape": "", "verb": "", "confidence": 0.2, "band": "unsure", "provider": "llm",
		})
		defer srv.Close()
		hc := &http.Client{Timeout: deadlineRemaining(deadline)}
		if got := routeLine(hc, srv.URL, "hmm", deadline); got != "" {
			t.Errorf("routeLine = %q, want \"\" (unsure band)", got)
		}
	})

	t.Run("verb none: nothing", func(t *testing.T) {
		srv := fakeRouteDaemon(t, 0, map[string]any{
			"shape": "none", "verb": "none", "confidence": 0.95, "band": "proceed", "provider": "llm",
		})
		defer srv.Close()
		hc := &http.Client{Timeout: deadlineRemaining(deadline)}
		if got := routeLine(hc, srv.URL, "thanks!", deadline); got != "" {
			t.Errorf("routeLine = %q, want \"\" (verb none)", got)
		}
	})

	t.Run("provider none: nothing", func(t *testing.T) {
		srv := fakeRouteDaemon(t, 0, map[string]any{
			"shape": "", "verb": "", "confidence": 0, "band": "unsure", "provider": "none",
		})
		defer srv.Close()
		hc := &http.Client{Timeout: deadlineRemaining(deadline)}
		if got := routeLine(hc, srv.URL, "anything", deadline); got != "" {
			t.Errorf("routeLine = %q, want \"\" (provider none)", got)
		}
	})

	t.Run("response carries an error field: nothing", func(t *testing.T) {
		srv := fakeRouteDaemon(t, 0, map[string]any{
			"shape": "", "verb": "", "confidence": 0, "band": "unsure", "provider": "none", "error": "decider unreachable",
		})
		defer srv.Close()
		hc := &http.Client{Timeout: deadlineRemaining(deadline)}
		if got := routeLine(hc, srv.URL, "anything", deadline); got != "" {
			t.Errorf("routeLine = %q, want \"\" (response carries an error)", got)
		}
	})

	t.Run("non-200 status: nothing", func(t *testing.T) {
		srv := fakeRouteDaemon(t, http.StatusInternalServerError, map[string]any{"error": "boom"})
		defer srv.Close()
		hc := &http.Client{Timeout: deadlineRemaining(deadline)}
		if got := routeLine(hc, srv.URL, "anything", deadline); got != "" {
			t.Errorf("routeLine = %q, want \"\" (500 status)", got)
		}
	})

	t.Run("malformed body: nothing", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("{not json"))
		}))
		defer srv.Close()
		hc := &http.Client{Timeout: deadlineRemaining(deadline)}
		if got := routeLine(hc, srv.URL, "anything", deadline); got != "" {
			t.Errorf("routeLine = %q, want \"\" (malformed body)", got)
		}
	})

	t.Run("deadline already spent: nothing, no request made", func(t *testing.T) {
		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			_ = json.NewEncoder(w).Encode(map[string]any{
				"shape": "issue", "verb": "localize", "confidence": 0.99, "band": "proceed", "provider": "llm",
			})
		}))
		defer srv.Close()
		spent := time.Now().Add(-1 * time.Second)
		hc := &http.Client{Timeout: time.Second}
		if got := routeLine(hc, srv.URL, "anything", spent); got != "" {
			t.Errorf("routeLine = %q, want \"\" (deadline already spent)", got)
		}
		if called {
			t.Error("routeLine made a request past its own deadline — must skip, not call")
		}
	})
}

// TestCombineWithRouteLine_ByteBudget pins the pure byte-budget-drop
// helper cmdPromptHook folds routeLine's result through: a route line that
// fits is appended on its own line; a route line whose addition would push
// the combined text past harness.CodexAdditionalContextCap is dropped,
// leaving the obligations text untouched; a route line stands alone when
// obligations resolved nothing at all; an empty route line changes
// nothing.
func TestCombineWithRouteLine_ByteBudget(t *testing.T) {
	route := "fathom route: this prompt is issue-shaped — open with localize (confidence 0.93)."

	t.Run("empty route changes nothing", func(t *testing.T) {
		text, ok := combineWithRouteLine("obligations text", true, "")
		if text != "obligations text" || !ok {
			t.Errorf("got (%q, %v), want (%q, true)", text, ok, "obligations text")
		}
	})

	t.Run("route stands alone when nothing resolved", func(t *testing.T) {
		text, ok := combineWithRouteLine("", false, route)
		if text != route || !ok {
			t.Errorf("got (%q, %v), want (%q, true)", text, ok, route)
		}
	})

	t.Run("route appended when it fits the budget", func(t *testing.T) {
		obligations := "obligations text"
		text, ok := combineWithRouteLine(obligations, true, route)
		want := obligations + "\n" + route
		if text != want || !ok {
			t.Errorf("got (%q, %v), want (%q, true)", text, ok, want)
		}
	})

	t.Run("route dropped when it would exceed the byte budget", func(t *testing.T) {
		// One byte under the cap once the route line's own length is
		// accounted for — appending it would land exactly one byte over.
		obligations := strings.Repeat("x", harness.CodexAdditionalContextCap-len(route))
		text, ok := combineWithRouteLine(obligations, true, route)
		if !ok {
			t.Fatalf("ok = false, want true (obligations text alone is still a valid answer)")
		}
		if text != obligations {
			t.Errorf("text = %d bytes, want the untouched obligations text (%d bytes) — route line must be dropped, not truncate it", len(text), len(obligations))
		}
		if strings.Contains(text, "fathom route:") {
			t.Error("route line leaked into the output despite exceeding the byte budget")
		}
		if len(obligations)+1+len(route) <= harness.CodexAdditionalContextCap {
			t.Fatalf("test fixture is wrong: combined length %s does not actually exceed the cap %d",
				strconv.Itoa(len(obligations)+1+len(route)), harness.CodexAdditionalContextCap)
		}
	})
}
