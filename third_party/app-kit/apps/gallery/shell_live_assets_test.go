package gallery

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestShellLiveAssetsServeAndRevalidate(t *testing.T) {
	mux := http.NewServeMux()
	registerShellLiveAssets(mux)
	for _, asset := range []struct{ path, body string }{
		{"/static/shell-live.js", shellLiveJS},
		{"/static/shell-live-entry.js", shellLiveEntryJS},
		{"/static/datastar-v1.0.3.js", shellLiveDatastarJS},
		{"/static/datastar-v1.0.3.LICENSE.txt", shellLiveDatastarLicense},
	} {
		t.Run(asset.path, func(t *testing.T) {
			etag := fmt.Sprintf(`"%x"`, sha256.Sum256([]byte(asset.body)))
			mime := "application/javascript; charset=utf-8"
			if strings.HasSuffix(asset.path, ".txt") {
				mime = "text/plain; charset=utf-8"
			}
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, httptest.NewRequest(method, asset.path, nil))
				if w.Code != http.StatusOK {
					t.Fatalf("%s status = %d", method, w.Code)
				}
				for key, want := range map[string]string{
					"Content-Type":           mime,
					"X-Content-Type-Options": "nosniff",
					"Cache-Control":          "public, max-age=0, must-revalidate",
					"ETag":                   etag,
					"Content-Length":         strconv.Itoa(len(asset.body)),
				} {
					if got := w.Header().Get(key); got != want {
						t.Errorf("%s %s = %q, want %q", method, key, got, want)
					}
				}
				wantBody := asset.body
				if method == http.MethodHead {
					wantBody = ""
				}
				if w.Body.String() != wantBody {
					t.Errorf("%s did not preserve the expected asset bytes/body semantics", method)
				}
				if asset.path == "/static/datastar-v1.0.3.js" && w.Header().Get("Link") != `</static/datastar-v1.0.3.LICENSE.txt>; rel="license"` {
					t.Error("vendored client must link its embedded license notice")
				}

				for _, validator := range []string{etag, "W/" + etag} {
					r := httptest.NewRequest(method, asset.path, nil)
					r.Header.Set("If-None-Match", validator)
					w := httptest.NewRecorder()
					mux.ServeHTTP(w, r)
					if w.Code != http.StatusNotModified || w.Body.Len() != 0 || w.Header().Get("ETag") != etag {
						t.Errorf("%s matching validator %q: status=%d bytes=%d etag=%q", method, validator, w.Code, w.Body.Len(), w.Header().Get("ETag"))
					}
				}
			}

			r := httptest.NewRequest(http.MethodGet, asset.path, nil)
			r.Header.Set("If-None-Match", `"obsolete-client"`)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != http.StatusOK || w.Body.String() != asset.body {
				t.Error("stale validator did not return the current complete asset")
			}
			for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions} {
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, httptest.NewRequest(method, asset.path, nil))
				if w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
					t.Errorf("%s status=%d allow=%q", method, w.Code, w.Header().Get("Allow"))
				}
			}
		})
	}
}

func TestShellLiveDatastarPinAndLicense(t *testing.T) {
	const wantBytes = 33538
	const wantSHA256 = "5d6b7794a50a83d82da962aec5e382f5ae83ac7afbc751f903f7a9c6bd433c65"
	if got := len(shellLiveDatastarJS); got != wantBytes {
		t.Fatalf("Datastar v1.0.3 bundle bytes = %d, want unchanged upstream %d", got, wantBytes)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256([]byte(shellLiveDatastarJS))); got != wantSHA256 {
		t.Fatalf("Datastar v1.0.3 SHA-256 = %s, want %s", got, wantSHA256)
	}
	license, err := os.ReadFile("assets/datastar/v1.0.3/LICENSE.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(license) != shellLiveDatastarLicense {
		t.Fatal("compiled Gallery must retain the complete upstream license")
	}
	for _, notice := range []string{
		"Copyright © Star Federation",
		"Permission is hereby granted, free of charge, to any person obtaining a copy",
		"The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.",
		`THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND`,
		"IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE",
	} {
		if !strings.Contains(string(license), notice) {
			t.Errorf("vendored Datastar license lacks retained notice %q", notice)
		}
	}
}

func TestShellLiveOptionalScriptBudgets(t *testing.T) {
	rawBytes, gzipBytes := 0, 0
	for _, asset := range []struct{ name, body string }{
		{"observer", shellLiveJS},
		{"entry", shellLiveEntryJS},
		{"datastar-v1.0.3", shellLiveDatastarJS},
	} {
		if len(asset.body) == 0 {
			t.Fatalf("%s script is empty", asset.name)
		}
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		if _, err := writer.Write([]byte(asset.body)); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		rawBytes += len(asset.body)
		gzipBytes += compressed.Len()
		t.Logf("%s: raw=%d gzip=%d", asset.name, len(asset.body), compressed.Len())
	}
	// Sum independently compressed responses, as the browser fetches three assets.
	if rawBytes > 48<<10 || gzipBytes > 20<<10 {
		t.Fatalf("optional observer scripts exceed agreed budgets: raw=%d/49152 gzip=%d/20480", rawBytes, gzipBytes)
	}
	t.Logf("optional observer scripts total: raw=%d/49152 gzip=%d/20480", rawBytes, gzipBytes)
}
