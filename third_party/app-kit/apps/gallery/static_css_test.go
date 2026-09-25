package gallery

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestGalleryCSSRevalidation(t *testing.T) {
	css, err := assets.ReadFile("ui/app.css")
	if err != nil {
		t.Fatal(err)
	}
	etag := fmt.Sprintf(`"%x"`, sha256.Sum256(css))
	h := New()
	for _, tc := range []struct {
		name, method, validator string
		status                  int
	}{
		{"first", "GET", "", 200},
		{"match", "GET", etag, 304},
		{"weak-match", "GET", "W/" + etag, 304},
		{"list-match", "GET", `"old", W/` + etag, 304},
		{"wildcard", "GET", "*", 304},
		{"stale", "GET", `"old"`, 200},
		{"malformed", "GET", "not-an-etag", 200},
		{"head", "HEAD", "", 200},
		{"head-match", "HEAD", etag, 304},
		// Preserve the route's historical non-read method behavior; do not
		// accidentally apply conditional GET semantics to a POST or OPTIONS.
		{"post", "POST", etag, 200},
		{"options", "OPTIONS", etag, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "/static/app.css", nil)
			r.Header.Set("If-None-Match", tc.validator)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
			read := tc.method == "GET" || tc.method == "HEAD"
			if read {
				if w.Header().Get("ETag") != etag || w.Header().Get("Cache-Control") != "public, max-age=0, must-revalidate" {
					t.Errorf("missing exact byte validator/revalidation policy: %v", w.Header())
				}
			} else if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("ETag") != "" {
				t.Errorf("non-read response became reusable: %v", w.Header())
			}
			if w.Header().Get("Last-Modified") != "" {
				t.Error("embedded bytes must not get a fabricated modification time")
			}
			if tc.method == "HEAD" || tc.status == 304 {
				if w.Body.Len() != 0 {
					t.Errorf("body = %d bytes, want none", w.Body.Len())
				}
			} else if !bytes.Equal(w.Body.Bytes(), css) {
				t.Fatal("stylesheet bytes changed")
			}
			if read && tc.status == 200 && w.Header().Get("Content-Length") != strconv.Itoa(len(css)) {
				t.Errorf("Content-Length = %q", w.Header().Get("Content-Length"))
			}
			if tc.status == 200 && w.Header().Get("Content-Type") != "text/css; charset=utf-8" {
				t.Errorf("Content-Type = %q", w.Header().Get("Content-Type"))
			}
		})
	}
}

func TestGalleryCSSValidatorCannotCacheThemeDocuments(t *testing.T) {
	source, err := os.ReadFile("../../cue/tokens/geoff-synthwave/geoff-synthwave.cue")
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), source...)
	lab, err := NewThemeLab("http://127.0.0.1:8765", source, "synthetic", "synthetic-kit")
	if err != nil {
		t.Fatal(err)
	}
	h := NewWithOptions(Options{ThemeLab: lab})
	for _, path := range []string{"/theme-lab", "/theme-lab/preview?specimen=forms&scheme=dark"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Host, r.RemoteAddr = "127.0.0.1:8765", "127.0.0.1:12345"
		r.Header.Set("If-None-Match", "*")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 || w.Body.Len() == 0 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("ETag") != "" {
			t.Errorf("%s: document policy changed: %d %v", path, w.Code, w.Header())
		}
		if !strings.Contains(w.Header().Get("Content-Security-Policy"), "script-src 'none'") {
			t.Error("script prohibition lost")
		}
		if strings.Contains(path, "/preview") && !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") {
			t.Error("preview sandbox lost")
		}
	}
	if !bytes.Equal(source, before) {
		t.Fatal("theme source mutated")
	}
}

func TestGalleryCSSReplacementInvalidatesOldValidator(t *testing.T) {
	css := []byte("body { color: red }")
	first := staticCSSHandler(css)
	css[14] = 'b' // Mutating the caller's slice must not change the first handler.
	second := staticCSSHandler([]byte("body { color: blue }"))
	request := func(h http.HandlerFunc, tag string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/static/app.css", nil)
		r.Header.Set("If-None-Match", tag)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	old := request(first, "")
	if old.Code != 200 || old.Body.String() != "body { color: red }" {
		t.Fatal("stylesheet snapshot changed with caller's bytes")
	}
	current := request(second, old.Header().Get("ETag"))
	if current.Code != 200 || current.Body.String() != "body { color: blue }" || current.Header().Get("ETag") == old.Header().Get("ETag") {
		t.Fatal("old validator reused the previous build's stylesheet")
	}
	match := request(second, current.Header().Get("ETag"))
	if match.Code != 304 || match.Body.Len() != 0 {
		t.Fatal("current bytes did not revalidate without a body")
	}
}
