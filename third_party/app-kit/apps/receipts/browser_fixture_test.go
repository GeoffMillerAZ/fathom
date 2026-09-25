package receipts

import (
	"html"
	"os"
	"strings"
	"testing"
)

func TestBrowserLongLabelFixtureIsBoundedEscapedStaticEvidence(t *testing.T) {
	input, _ := loadCUE(t)
	label := "Synthetic product " + strings.Repeat("long-label-", 24)
	if len(label) < 256 {
		t.Fatalf("stress label length=%d, want at least 256", len(label))
	}
	label += `<not-markup>&"`
	input.Products[0].Label = label

	projected, err := project(input, defaultSelection())
	if err != nil {
		t.Fatal(err)
	}
	body, err := renderReport(projected, "light")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, `<not-markup>`) || strings.Count(body, html.EscapeString(label)) < 3 {
		t.Fatal("long product label was truncated, omitted, or rendered as markup")
	}
	css := string(reportCSS)
	for _, rule := range []string{"overflow-wrap:anywhere", ".receipts-chart-product", ".receipts-evidence tbody th"} {
		if !strings.Contains(css, rule) {
			t.Fatalf("compiled CSS lost long-label wrapping contract %q", rule)
		}
	}
	if len(body) > 32*1024 {
		t.Fatalf("long-label fixture HTML=%d bytes, exceeds 32 KiB task bound", len(body))
	}
}

func TestBrowserHarnessRetainsBoundariesAndNamedEvidence(t *testing.T) {
	source := string(mustReadFile(t, "browser.mjs"))
	for _, contract := range []string{
		"RECEIPTS_BASE_URL", "RECEIPTS_EVIDENCE_DIR", "127.0.0.1", "report.json",
		"1440-light-zoom-200-viewport", "chrome://settings/appearance", "#zoomLevel", "Page.captureScreenshot", "captureBeyondViewport: false", "waitForEvent('download')",
		"No blocked outside, write, or WebSocket request", "256-character product-label",
	} {
		if !strings.Contains(source, contract) {
			t.Fatalf("browser harness lost boundary/evidence contract %q", contract)
		}
	}
	for _, forbidden := range []string{"npm install", "npx ", "localhost", "https://"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("browser harness introduced forbidden ambient behavior %q", forbidden)
		}
	}
}

func TestBrowserHarnessPinsCurrentAnalyticsCSVSchema(t *testing.T) {
	source := string(mustReadFile(t, "browser.mjs"))
	for _, required := range []string{
		"'id', 'label', 'value', 'unit', 'state', 'missing_reason', 'lower', 'upper', 'role', 'role_label'",
		"new Set(header).size !== 35", "value.length !== 35",
		"row.role === expectedRole.role", "row.role_label === expectedRole.role_label",
		"role: 'accent', role_label: ''",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("browser harness lost current CSV role contract %q", required)
		}
	}
	for _, stale := range []string{"header.length !== 33", "value.length !== 33", "33-column export"} {
		if strings.Contains(source, stale) {
			t.Fatalf("browser harness retained stale CSV schema expectation %q", stale)
		}
	}
}

func mustReadFile(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
