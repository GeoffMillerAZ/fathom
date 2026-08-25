package reachfix

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestServerServes builds the server statically (newServer registers every
// handler by VALUE — see server.go) and hits it via httptest: it never
// calls a handler directly, so reaching one is proof the value-reference
// edge (VS-6) was wired.
func TestServerServes(t *testing.T) {
	s := newServer()
	ts := httptest.NewServer(s)
	defer ts.Close()

	for _, path := range []string{"/a", "/b", "/c", "/d", "/e"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status = %d, want %d", path, resp.StatusCode, http.StatusOK)
		}
	}
}
