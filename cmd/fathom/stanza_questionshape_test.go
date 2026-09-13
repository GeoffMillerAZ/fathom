// Package main — cmd/fathom/stanza_questionshape_test.go: SB-7
// (docs/plans/strengths-bench.md; docs/vision/27-strengths-bench.md §8).
//
// The home bench (O31, O32) found "index before any grep" over-broad: a
// junk references query and 38 read_source calls came from a stanza that
// told every session to index-first, unconditionally. The stanza fathom
// init writes, the subagent primer, and the SessionStart primer now route
// by QUESTION SHAPE instead — where/who-calls/what-breaks reach for the
// index; RC-23 (docs/vision/28-receipts.md §3.5) names the tool per shape:
// what-governs is `obligations` with no path, is-this-still-true/
// what-was-known is `staleness_report`; a literal string, a config value,
// a log line, or a document-versus-document contradiction reach for grep.
package main

import (
	"strings"
	"testing"
)

// assertRoutesByQuestionShape is the shared assertion for every rendering
// of the routing guidance: the retired blanket phrase is gone, and the
// question-shape vocabulary plus the grep carve-out are present.
func assertRoutesByQuestionShape(t *testing.T, label, text string) {
	t.Helper()
	for _, retired := range []string{"before any Grep/Read sweep", "BEFORE any Grep/Read sweep"} {
		if strings.Contains(text, retired) {
			t.Errorf("%s: still carries the retired blanket phrase %q:\n%s", label, retired, text)
		}
	}
	for _, want := range []string{"question shape", "who-calls", "what-breaks", "what-governs", "staleness_report", "grep"} {
		if !strings.Contains(text, want) {
			t.Errorf("%s: missing question-shape routing phrase %q:\n%s", label, want, text)
		}
	}
}

// TestStanzaRoutesByQuestionShape pins SB-7 across the three renderings the
// row names: the stanza `fathom init` writes, the subagent primer
// (sessionhook_events.go), and the SessionStart primer (sessionhook.go).
func TestStanzaRoutesByQuestionShape(t *testing.T) {
	t.Run("stanza", func(t *testing.T) {
		assertRoutesByQuestionShape(t, "stanza", fathomStanza("svc", false))
	})

	srv := fakeDaemon()
	defer srv.Close()

	t.Run("subagent_primer", func(t *testing.T) {
		primer := subagentPrimer(srv.URL, "fathom", "")
		if primer == "" {
			t.Fatal("subagentPrimer returned \"\" — fakeDaemon must resolve \"fathom\" as an indexed service")
		}
		assertRoutesByQuestionShape(t, "subagent primer", primer)
	})

	t.Run("session_start_primer", func(t *testing.T) {
		assertRoutesByQuestionShape(t, "SessionStart primer", buildSessionContext(srv.URL, "fathom"))
	})
}
