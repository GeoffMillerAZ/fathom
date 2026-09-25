package receipts

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReceiptPagePayloadAndScriptBudget(t *testing.T) {
	handler, err := New()
	if err != nil {
		t.Fatal(err)
	}
	maximum := 0
	for _, period := range []string{"2024-09", "2024-10", "2024-11"} {
		for _, channel := range []string{"all", "app", "mobile"} {
			for _, compare := range []string{"previous", "none"} {
				for _, stale := range []bool{false, true} {
					for _, theme := range []string{"light", "dark"} {
						selected := selection{Period: period, Channel: channel, Compare: compare, Stale: stale}
						response := httptest.NewRecorder()
						request := httptest.NewRequest(http.MethodGet, reportURL("/", selected, theme), nil)
						request.Host = "127.0.0.1:8766"
						handler.ServeHTTP(response, request)
						if response.Code != http.StatusOK {
							t.Fatalf("selection %+v: %d", selected, response.Code)
						}
						body := response.Body.String()
						maximum = max(maximum, len(body))
						if len(body) > 32<<10 {
							t.Fatalf("HTML exceeds 32KiB: %d bytes for %+v", len(body), selected)
						}
						for _, forbidden := range []string{"<script", "javascript:", "onclick=", "onchange=", "http-equiv="} {
							if strings.Contains(strings.ToLower(body), forbidden) {
								t.Errorf("unexpected scripting surface %q", forbidden)
							}
						}
						if !strings.Contains(body, `id="receipts-main" tabindex="-1"`) {
							t.Fatal("shared skip-link focus target missing")
						}
						if !strings.Contains(body, `2024-12-05T12:00:00Z`) || !strings.Contains(body, `not freshness today`) {
							t.Fatal("fixed fixture clock disclosure missing")
						}
						if period == "2024-11" && (!strings.Contains(body, "2024-11-21T00:00:00Z") || !strings.Contains(body, "not yet known")) {
							t.Fatal("partial period needs actual interval and unknown monthly total")
						}
					}
				}
			}
		}
	}
	t.Logf("maximum HTML across 72 overview selections: %d bytes; budget %d", maximum, 32<<10)
}

func TestMoneyCents(t *testing.T) {
	for cents, want := range map[int64]string{0: "$0.00", 1: "$0.01", -1: "-$0.01", 155000: "$1,550.00", -15000: "-$150.00", maxAmountCents: "$10,000,000.00"} {
		if got := moneyCents(cents); got != want {
			t.Errorf("moneyCents(%d) = %q; want %q", cents, got, want)
		}
	}
}

func TestReportTruthStates(t *testing.T) {
	handler, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		path, want, refused string
	}{
		{"/?period=2024-10&channel=mobile", `<dd data-total="current">Not known</dd>`, `<dd data-total="current">$0.00</dd>`},
		{"/product/atlas?period=2024-10", `<dd data-total="current">$0.00</dd>`, "Current monthly total (USD)"},
		{"/product/atlas", "Product net receipts fell $600.00", "Current monthly total (USD)"},
		{"/?period=2024-11", "November 2024 (partial, before 21 Nov 00:00 UTC)", "Net receipts fell"},
		{"/?period=2024-10", "Missing: <span", `data-freshness="current"`},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, sample.path, nil)
		request.Host = "127.0.0.1:8766"
		handler.ServeHTTP(response, request)
		body := response.Body.String()
		if response.Code != http.StatusOK || !strings.Contains(body, sample.want) || strings.Contains(body, sample.refused) {
			t.Errorf("%s: status=%d, wanted %q, refused %q", sample.path, response.Code, sample.want, sample.refused)
		}
	}
}

// Includes response allocation and request parsing; excludes one-time CUE load,
// process startup and network transport. Run with -benchmem to record the P11 gate.
func BenchmarkReceiptsHTTP(b *testing.B) {
	handler, err := New()
	if err != nil {
		b.Fatal(err)
	}
	for _, sample := range []struct{ name, path string }{
		{"overview", "/"},
		{"detail", "/product/atlas"},
		{"missing", "/?period=2024-10"},
		{"partial", "/?period=2024-11"},
		{"export", "/export.csv"},
	} {
		b.Run(sample.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				response := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodGet, sample.path, nil)
				request.Host = "127.0.0.1:8766"
				handler.ServeHTTP(response, request)
				if response.Code != http.StatusOK {
					b.Fatalf("status %d", response.Code)
				}
			}
		})
	}
}
