package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestConsoleTestPortfolio proves TQ-8 (docs/plans/test-intelligence.md;
// design doc §3 T1, T2): /tests renders one repository's risk rank and
// test portfolio (renderTestPortfolio), and its JSON model carries the
// same facts; without a repository it asks for one instead of walking the
// fleet.
func TestConsoleTestPortfolio(t *testing.T) {
	f := newConsoleFixture(t)

	code, _, body := f.get(t, "/tests")
	if code != http.StatusOK {
		t.Fatalf("/tests = %d", code)
	}
	if !strings.Contains(body, "Choose a repository") || !strings.Contains(body, `href="/tests?svc=payment-service"`) {
		t.Errorf("/tests without a repository must offer the chooser")
	}

	env := f.pageJSON(t, "/tests?svc=payment-service")
	var m testPortfolioModel
	if err := json.Unmarshal(env.Model, &m); err != nil {
		t.Fatal(err)
	}
	if m.Err != "" {
		t.Fatalf("model error: %s", m.Err)
	}
	if m.Risk == nil || m.Portfolio == nil {
		t.Fatalf("model lacks risk or portfolio: %+v", m)
	}
	if m.Risk.Watermark == "" || len(m.Risk.Rows) == 0 {
		t.Fatalf("risk rank empty: %+v (centrality stored at ingest?)", m.Risk)
	}
	for _, r := range m.Risk.Rows {
		if len(r.Factors) != 5 {
			t.Errorf("%s: %d factors", r.Symbol, len(r.Factors))
		}
	}

	code, _, html := f.get(t, "/tests?svc=payment-service")
	if code != http.StatusOK {
		t.Fatalf("/tests?svc= = %d", code)
	}
	for _, want := range []string{`id="tests-risk"`, `id="tests-portfolio"`, `id="tests-gaps"`, "Untested and dangerous", "Test portfolio", m.Risk.Rows[0].Symbol} {
		if !strings.Contains(html, want) {
			t.Errorf("/tests?svc=payment-service lacks %q", want)
		}
	}
	for _, pt := range m.Portfolio.Tests {
		if pt.Purpose == "" || !strings.Contains(html, pt.Purpose) {
			t.Errorf("test %s: purpose %q not rendered", pt.Test, pt.Purpose)
		}
	}
	// Role utilities only: no palette colours, no dark: variants (app-kit
	// doctrine, cmd/fathom/AGENTS.md).
	page := renderTestPortfolio(m)
	for _, bad := range []string{"dark:", "text-red-", "bg-gray-", "style="} {
		if strings.Contains(page, bad) {
			t.Errorf("rendered portfolio carries %q", bad)
		}
	}
}
