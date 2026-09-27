package main

// refreshfile_test.go — SU-5/SU-6 (docs/plans/substrate.md §A, ADR-041
// decisions 3 and 4): POST /refresh-file {service, file}.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestRefreshFileHandler_ReextractsOneFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.go")
	if err := os.WriteFile(target, []byte("package main\n\nfunc Original() int { return 1 }\n"), 0o644); err != nil {
		t.Fatalf("write target.go: %v", err)
	}

	conn := connector.NewLocalConnector()
	ms := store.NewMemoryStore()
	c := client.New(conn, ms)
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "refresh-svc", Path: dir}); err != nil {
		t.Fatalf("initial Ingest: %v", err)
	}
	if err := os.WriteFile(target, []byte("package main\n\nfunc Changed() int { return 2 }\n"), 0o644); err != nil {
		t.Fatalf("rewrite target.go: %v", err)
	}

	srv := httptest.NewServer(refreshFileHandler(c, nil, nil))
	defer srv.Close()

	body, _ := json.Marshal(refreshFileRequest{Service: "refresh-svc", File: "target.go"})
	resp, err := http.Post(srv.URL+"/refresh-file", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /refresh-file: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var out refreshFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !out.Refreshed || out.Symbols != 1 {
		t.Errorf("response = %+v, want Refreshed=true Symbols=1", out)
	}
	if !out.TypedEdgesStale {
		t.Errorf("response.TypedEdgesStale = false for a .go file, want true")
	}
}

func TestRefreshFileHandler_MissingFieldsBadRequest(t *testing.T) {
	conn := connector.NewLocalConnector()
	ms := store.NewMemoryStore()
	c := client.New(conn, ms)
	t.Cleanup(func() { _ = c.Close() })

	srv := httptest.NewServer(refreshFileHandler(c, nil, nil))
	defer srv.Close()

	body, _ := json.Marshal(refreshFileRequest{Service: "", File: ""})
	resp, err := http.Post(srv.URL+"/refresh-file", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for missing service/file", resp.StatusCode)
	}
}

func TestRefreshFileHandler_MethodNotAllowed(t *testing.T) {
	conn := connector.NewLocalConnector()
	ms := store.NewMemoryStore()
	c := client.New(conn, ms)
	t.Cleanup(func() { _ = c.Close() })

	srv := httptest.NewServer(refreshFileHandler(c, nil, nil))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/refresh-file")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405 for GET", resp.StatusCode)
	}
}
