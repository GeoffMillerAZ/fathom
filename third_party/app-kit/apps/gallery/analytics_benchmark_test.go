package gallery

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	analyticsBenchmarkHTMLCeiling      = 32 << 10
	analyticsBenchmarkLocalTimeCeiling = 5 * time.Millisecond
	analyticsBenchmarkLocalAllocLimit  = 512 << 10
)

type analyticsBenchmarkCase struct {
	name, target, contentType string
	status                    int
	want, absent              []string
	html                      bool
}

type analyticsBenchmarkWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newAnalyticsBenchmarkWriter() *analyticsBenchmarkWriter {
	return &analyticsBenchmarkWriter{header: make(http.Header)}
}

func (w *analyticsBenchmarkWriter) Header() http.Header { return w.header }

func (w *analyticsBenchmarkWriter) WriteHeader(status int) { w.status = status }

func (w *analyticsBenchmarkWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(body)
}

func (w *analyticsBenchmarkWriter) Reset() {
	clear(w.header)
	w.body.Reset()
	w.status = 0
}

// BenchmarkAnalyticsHTTPHandler measures the real analytics route after mux
// construction. It includes query validation, fixture construction/validation,
// HTML or CSV rendering, headers, and copying output into a reusable in-memory
// response buffer. It excludes request construction, network transport, browser
// parsing/layout, and first-growth allocation of the response buffer.
//
// Exploratory ceilings were declared before measurement for this bounded fixture
// on a Mac16,10 (10 logical CPUs, 32 GiB): <=5 ms/op and <=512 KiB allocated/op.
// They are reported for comparison, not enforced as universal CI timing gates.
func BenchmarkAnalyticsHTTPHandler(b *testing.B) {
	handler := analyticsRouteHandler()
	cases := []analyticsBenchmarkCase{
		{
			name: "complete_overview", target: "/analytics?period=2026-09&compare=previous&category=all&state=complete",
			status: http.StatusOK, contentType: "text/html", html: true,
			want: []string{"The selected full-month counts total 5 failed operations.", "Exact values"},
		},
		{
			name: "partial_API_without_comparison", target: "/analytics?period=2026-09&compare=none&category=api&state=partial",
			status: http.StatusOK, contentType: "text/html", html: true,
			want: []string{"No complete-period change", "Partial, lower bound"}, absent: []string{"compared with August 2026"},
		},
		{
			name: "missing_worker", target: "/analytics?period=2026-09&compare=previous&category=worker&state=missing",
			status: http.StatusOK, contentType: "text/html", html: true,
			want: []string{"All selected counts are missing, not zero.", "Missing, not zero"},
		},
		{
			name: "stale_detail", target: "/analytics/gateway-api?period=2026-09&compare=previous&category=all&state=stale",
			status: http.StatusOK, contentType: "text/html", html: true,
			want: []string{"Gateway API", "source observation is stale"},
		},
		{
			name: "CSV_export", target: "/analytics/export.csv?period=2026-09&compare=previous&category=all&state=complete",
			status: http.StatusOK, contentType: "text/csv",
			want: []string{"series,selection_period", "current,2026-09", "previous,2026-09"},
		},
		{
			name: "invalid_query_recovery", target: "/analytics?period=2026-09&compare=previous&category=all&state=zero",
			status: http.StatusBadRequest, contentType: "text/html", html: true,
			want: []string{"These filters could not be read.", "Return to a valid report"}, absent: []string{"state=zero"},
		},
	}

	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			writer := newAnalyticsBenchmarkWriter()
			handler.ServeHTTP(writer, request)
			verifyAnalyticsBenchmarkResponse(b, test, writer)
			expectedBytes := writer.body.Len()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				writer.Reset()
				handler.ServeHTTP(writer, request)
			}
			b.StopTimer()
			b.ReportMetric(float64(analyticsBenchmarkLocalTimeCeiling.Nanoseconds()), "local-time-ceiling-ns/op")
			b.ReportMetric(float64(analyticsBenchmarkLocalAllocLimit), "local-allocation-ceiling-B/op")
			if writer.status != test.status || writer.body.Len() != expectedBytes {
				b.Fatalf("final response changed: status=%d bytes=%d, want %d/%d", writer.status, writer.body.Len(), test.status, expectedBytes)
			}
		})
	}
}

func verifyAnalyticsBenchmarkResponse(b *testing.B, test analyticsBenchmarkCase, writer *analyticsBenchmarkWriter) {
	b.Helper()
	if writer.status != test.status {
		b.Fatalf("status = %d, want %d", writer.status, test.status)
	}
	if contentType := writer.header.Get("Content-Type"); !strings.HasPrefix(contentType, test.contentType) {
		b.Fatalf("Content-Type = %q, want prefix %q", contentType, test.contentType)
	}
	body := writer.body.String()
	if body == "" {
		b.Fatal("handler returned an empty body")
	}
	for _, want := range test.want {
		if !strings.Contains(body, want) {
			b.Fatalf("body missing semantic canary %q", want)
		}
	}
	for _, absent := range test.absent {
		if strings.Contains(body, absent) {
			b.Fatalf("body contains forbidden canary %q", absent)
		}
	}
	if test.html && writer.body.Len() > analyticsBenchmarkHTMLCeiling {
		b.Fatalf("HTML body = %d bytes, exceeds %d-byte fixture ceiling", writer.body.Len(), analyticsBenchmarkHTMLCeiling)
	}
}
