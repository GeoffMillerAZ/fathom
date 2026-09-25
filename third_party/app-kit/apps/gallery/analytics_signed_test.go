package gallery

import (
	"encoding/csv"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSignedAnalyticsRoutes(t *testing.T) {
	handler := New()
	for _, path := range []string{analyticsSignedPath, analyticsSignedExportPath} {
		var getBody string
		for _, method := range []string{"GET", "HEAD", "POST"} {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(method, path+"?theme=dark", nil))
			if method == "POST" {
				if w.Code != 405 || w.Header().Get("Allow") != "GET, HEAD" {
					t.Fatal("write not refused")
				}
				continue
			}
			if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Header().Get("Content-Security-Policy"), "unsafe-inline") {
				t.Fatalf("unsafe response %d %v", w.Code, w.Header())
			}
			if method == "HEAD" {
				if w.Body.Len() != 0 {
					t.Fatal("HEAD has body")
				}
				continue
			}
			getBody = w.Body.String()
		}
		if path == analyticsSignedPath {
			for _, want := range []string{"data-analytics-diverging", "Export signed CSV", "Source freshness unknown", "Not reported", "-2 points", "2 points"} {
				if !strings.Contains(getBody, want) {
					t.Errorf("page missing %s", want)
				}
			}
			if strings.Contains(getBody, "<script") {
				t.Fatal("unexpected page script")
			}
		} else {
			r := csv.NewReader(strings.NewReader(getBody))
			r.FieldsPerRecord = -1
			rows, err := r.ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			start := -1
			for i, row := range rows {
				if len(row) == 10 && row[0] == "id" {
					start = i + 1
					break
				}
			}
			if start < 0 || len(rows)-start != 6 {
				t.Fatal("unexpected exported row set")
			}
			for i, want := range []string{"-2", "-1", "0", "1", "2", ""} {
				row := rows[start+i]
				if row[2] != want || row[3] != "points" {
					t.Errorf("row %d: %v", i, row)
				}
			}
			if rows[start+5][4] != "missing" || rows[start+5][5] != "Not reported" {
				t.Fatal("missing exported as measured zero")
			}
		}
	}
	for _, query := range []string{"theme=purple", "theme=light&theme=dark", "period=2026-08", "state=missing", "theme=%zz", "foo=bar", "foo=bar&foo=baz"} {
		for _, path := range []string{analyticsSignedPath, analyticsSignedExportPath} {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("GET", path+"?"+query, nil))
			if w.Code != 400 {
				t.Errorf("unsupported %s selection %q returned %d", path, query, w.Code)
			}
		}
	}
	defaults, _ := parseAnalyticsSelection(nil)
	for _, theme := range []string{"light", "dark", ""} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", analyticsURL(analyticsSignedPath, defaults, theme), nil))
		if w.Code != 200 {
			t.Fatalf("shared-frame appearance link failed for %q", theme)
		}
	}
}
