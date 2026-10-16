package httpembedder_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	embeddingclient "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/embedding/embedding-client"
	httpembedder "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/embedding/http-embedder"
)

// embeddingsHandler is a minimal OpenAI-compatible /embeddings server for
// tests. It records every request body it receives.
type embeddingsHandler struct {
	t *testing.T

	dim int // dimension of generated vectors

	mu       sync.Mutex
	requests []embedRequest // every decoded request, in order
	auths    []string       // Authorization header per request
}

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

func (h *embeddingsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/embeddings" {
		h.t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
		return
	}
	var req embedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.t.Errorf("decode request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	h.mu.Lock()
	h.requests = append(h.requests, req)
	h.auths = append(h.auths, r.Header.Get("Authorization"))
	h.mu.Unlock()

	type datum struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	}
	resp := struct {
		Data []datum `json:"data"`
	}{}
	for i, text := range req.Input {
		resp.Data = append(resp.Data, datum{Index: i, Embedding: vectorFor(text, h.dim)})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *embeddingsHandler) snapshot() ([]embedRequest, []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	reqs := make([]embedRequest, len(h.requests))
	copy(reqs, h.requests)
	auths := make([]string, len(h.auths))
	copy(auths, h.auths)
	return reqs, auths
}

// vectorFor generates a deterministic vector for text: element i is
// float64(len(text)*100 + i).
func vectorFor(text string, dim int) []float64 {
	v := make([]float64, dim)
	for i := range v {
		v[i] = float64(len(text)*100 + i)
	}
	return v
}

func newClient(t *testing.T, cfg httpembedder.Config) *httpembedder.Client {
	t.Helper()
	c, err := httpembedder.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// ---------------------------------------------------------------------------
// Construction and validation
// ---------------------------------------------------------------------------

func TestNew_Validation(t *testing.T) {
	tests := []struct {
		name string
		cfg  httpembedder.Config
	}{
		{"missing BaseURL", httpembedder.Config{Model: "m"}},
		{"missing Model", httpembedder.Config{BaseURL: "http://localhost:1"}},
		{"BaseURL without scheme", httpembedder.Config{BaseURL: "localhost:8080/v1", Model: "m"}},
		{"BaseURL with bad scheme", httpembedder.Config{BaseURL: "ftp://host/v1", Model: "m"}},
		{"negative Timeout", httpembedder.Config{BaseURL: "http://localhost:1", Model: "m", Timeout: -time.Second}},
		{"negative MaxBatch", httpembedder.Config{BaseURL: "http://localhost:1", Model: "m", MaxBatch: -1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := httpembedder.New(tt.cfg); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestNew_Valid(t *testing.T) {
	c := newClient(t, httpembedder.Config{BaseURL: "http://localhost:11434/v1", Model: "nomic-embed-text"})
	if c.Dimensions() != 0 {
		t.Errorf("Dimensions() before any request = %d, want 0", c.Dimensions())
	}
}

// ---------------------------------------------------------------------------
// Success paths
// ---------------------------------------------------------------------------

func TestEmbed_Success(t *testing.T) {
	h := &embeddingsHandler{t: t, dim: 3}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := newClient(t, httpembedder.Config{
		BaseURL: srv.URL,
		Model:   "test-model",
		APIKey:  "sk-test-key",
	})

	got, err := c.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	want := vectorFor("hello", 3)
	if len(got) != len(want) {
		t.Fatalf("got dim %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	reqs, auths := h.snapshot()
	if len(reqs) != 1 {
		t.Fatalf("server saw %d requests, want 1", len(reqs))
	}
	if reqs[0].Model != "test-model" {
		t.Errorf("request model = %q, want %q", reqs[0].Model, "test-model")
	}
	if len(reqs[0].Input) != 1 || reqs[0].Input[0] != "hello" {
		t.Errorf("request input = %v, want [hello]", reqs[0].Input)
	}
	if auths[0] != "Bearer sk-test-key" {
		t.Errorf("Authorization = %q, want %q", auths[0], "Bearer sk-test-key")
	}
}

func TestEmbedBatch_Chunking(t *testing.T) {
	h := &embeddingsHandler{t: t, dim: 2}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := newClient(t, httpembedder.Config{
		BaseURL:  srv.URL + "/", // trailing slash must be normalised
		Model:    "test-model",
		MaxBatch: 2,
	})

	texts := []string{"a", "bb", "ccc", "dddd", "eeeee"}
	vecs, err := c.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(vecs) != len(texts) {
		t.Fatalf("got %d vectors, want %d", len(vecs), len(texts))
	}
	for i, text := range texts {
		want := vectorFor(text, 2)
		for j := range want {
			if vecs[i][j] != want[j] {
				t.Errorf("vecs[%d][%d] = %v, want %v (text %q)", i, j, vecs[i][j], want[j], text)
			}
		}
	}

	reqs, _ := h.snapshot()
	wantChunks := [][]string{{"a", "bb"}, {"ccc", "dddd"}, {"eeeee"}}
	if len(reqs) != len(wantChunks) {
		t.Fatalf("server saw %d requests, want %d", len(reqs), len(wantChunks))
	}
	for i, want := range wantChunks {
		if fmt.Sprint(reqs[i].Input) != fmt.Sprint(want) {
			t.Errorf("chunk %d = %v, want %v", i, reqs[i].Input, want)
		}
	}
}

func TestEmbedBatch_Empty(t *testing.T) {
	h := &embeddingsHandler{t: t, dim: 2}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := newClient(t, httpembedder.Config{BaseURL: srv.URL, Model: "m"})
	vecs, err := c.EmbedBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("EmbedBatch(nil): %v", err)
	}
	if len(vecs) != 0 {
		t.Errorf("got %d vectors, want 0", len(vecs))
	}
	if reqs, _ := h.snapshot(); len(reqs) != 0 {
		t.Errorf("server saw %d requests, want 0", len(reqs))
	}
}

func TestDimensionInference(t *testing.T) {
	h := &embeddingsHandler{t: t, dim: 7}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := newClient(t, httpembedder.Config{BaseURL: srv.URL, Model: "m"})

	if got, ok := embeddingclient.DimensionsOf(c); ok || got != 0 {
		t.Errorf("DimensionsOf before first response = (%d, %v), want (0, false)", got, ok)
	}

	if _, err := c.Embed(context.Background(), "probe"); err != nil {
		t.Fatalf("Embed: %v", err)
	}

	if got := c.Dimensions(); got != 7 {
		t.Errorf("Dimensions() = %d, want 7", got)
	}
	if got, ok := embeddingclient.DimensionsOf(c); !ok || got != 7 {
		t.Errorf("DimensionsOf after first response = (%d, %v), want (7, true)", got, ok)
	}
}

func TestResponseOutOfOrder(t *testing.T) {
	// Server returns data entries in reverse index order; the client must
	// place them by index.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[
			{"index":1,"embedding":[2.0]},
			{"index":0,"embedding":[1.0]}
		]}`)
	}))
	defer srv.Close()

	c := newClient(t, httpembedder.Config{BaseURL: srv.URL, Model: "m"})
	vecs, err := c.EmbedBatch(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if vecs[0][0] != 1.0 || vecs[1][0] != 2.0 {
		t.Errorf("vectors not reordered by index: %v", vecs)
	}
}

func TestResponseWithoutIndexFallsBackPositional(t *testing.T) {
	// A server that omits index entirely (all decode to 0) must fall back to
	// positional ordering rather than failing.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[
			{"embedding":[1.0]},
			{"embedding":[2.0]}
		]}`)
	}))
	defer srv.Close()

	c := newClient(t, httpembedder.Config{BaseURL: srv.URL, Model: "m"})
	vecs, err := c.EmbedBatch(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if vecs[0][0] != 1.0 || vecs[1][0] != 2.0 {
		t.Errorf("positional fallback wrong: %v", vecs)
	}
}

// ---------------------------------------------------------------------------
// Auth header behaviour
// ---------------------------------------------------------------------------

func TestAPIKeyFromEnv(t *testing.T) {
	const envName = "HTTPEMBEDDER_TEST_KEY"
	t.Setenv(envName, "env-secret-key")

	h := &embeddingsHandler{t: t, dim: 2}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := newClient(t, httpembedder.Config{BaseURL: srv.URL, Model: "m", APIKeyEnv: envName})
	if _, err := c.Embed(context.Background(), "x"); err != nil {
		t.Fatalf("Embed: %v", err)
	}

	_, auths := h.snapshot()
	if auths[0] != "Bearer env-secret-key" {
		t.Errorf("Authorization = %q, want %q", auths[0], "Bearer env-secret-key")
	}
}

func TestExplicitKeyWinsOverEnv(t *testing.T) {
	const envName = "HTTPEMBEDDER_TEST_KEY"
	t.Setenv(envName, "env-key")

	h := &embeddingsHandler{t: t, dim: 2}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := newClient(t, httpembedder.Config{BaseURL: srv.URL, Model: "m", APIKey: "explicit-key", APIKeyEnv: envName})
	if _, err := c.Embed(context.Background(), "x"); err != nil {
		t.Fatalf("Embed: %v", err)
	}

	_, auths := h.snapshot()
	if auths[0] != "Bearer explicit-key" {
		t.Errorf("Authorization = %q, want %q", auths[0], "Bearer explicit-key")
	}
}

func TestKeylessSendsNoAuthHeader(t *testing.T) {
	h := &embeddingsHandler{t: t, dim: 2}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := newClient(t, httpembedder.Config{BaseURL: srv.URL, Model: "m"})
	if _, err := c.Embed(context.Background(), "x"); err != nil {
		t.Fatalf("Embed: %v", err)
	}

	_, auths := h.snapshot()
	if auths[0] != "" {
		t.Errorf("Authorization = %q, want empty for keyless config", auths[0])
	}
}

// ---------------------------------------------------------------------------
// Error mapping
// ---------------------------------------------------------------------------

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"429 rate limited", http.StatusTooManyRequests, `{"error":{"message":"slow down"}}`, httpembedder.ErrRateLimited},
		{"401 auth", http.StatusUnauthorized, `{"error":{"message":"bad key"}}`, httpembedder.ErrAuth},
		{"403 auth", http.StatusForbidden, `{"error":{"message":"forbidden"}}`, httpembedder.ErrAuth},
		{"500 server", http.StatusInternalServerError, `{"error":{"message":"boom"}}`, httpembedder.ErrServer},
		{"503 server", http.StatusServiceUnavailable, "overloaded", httpembedder.ErrServer},
		{"400 invalid", http.StatusBadRequest, `{"error":{"message":"bad input"}}`, httpembedder.ErrInvalidRequest},
		{"404 invalid", http.StatusNotFound, "no such route", httpembedder.ErrInvalidRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				fmt.Fprint(w, tt.body)
			}))
			defer srv.Close()

			c := newClient(t, httpembedder.Config{BaseURL: srv.URL, Model: "m"})
			_, err := c.Embed(context.Background(), "x")
			if !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want errors.Is(err, %v)", err, tt.want)
			}
			// The batch path maps identically.
			_, err = c.EmbedBatch(context.Background(), []string{"x", "y"})
			if !errors.Is(err, tt.want) {
				t.Errorf("batch error = %v, want errors.Is(err, %v)", err, tt.want)
			}
		})
	}
}

func TestMalformedResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"not JSON", `<html>gateway error</html>`},
		{"wrong count", `{"data":[{"index":0,"embedding":[1.0]}]}`}, // 1 vector for 2 inputs
		{"empty embedding", `{"data":[{"index":0,"embedding":[]},{"index":1,"embedding":[1.0]}]}`},
		{"empty body", ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tt.body)
			}))
			defer srv.Close()

			c := newClient(t, httpembedder.Config{BaseURL: srv.URL, Model: "m"})
			_, err := c.EmbedBatch(context.Background(), []string{"a", "b"})
			if !errors.Is(err, httpembedder.ErrMalformedResponse) {
				t.Errorf("error = %v, want errors.Is(err, ErrMalformedResponse)", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Secret hygiene
// ---------------------------------------------------------------------------

// TestErrorsNeverContainAPIKey drives every error path against a server that
// maliciously echoes the Authorization header back in its responses, and
// asserts the API key never appears in any returned error string.
func TestErrorsNeverContainAPIKey(t *testing.T) {
	const secret = "sk-super-secret-key-12345"

	statuses := []int{
		http.StatusTooManyRequests,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusBadRequest,
		http.StatusInternalServerError,
	}
	for _, status := range statuses {
		t.Run(fmt.Sprintf("status %d", status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				auth := r.Header.Get("Authorization")
				w.WriteHeader(status)
				// Echo the raw key and the full header back in the error body.
				fmt.Fprintf(w, `{"error":{"message":"rejected credential %s (header %s)"}}`, secret, auth)
			}))
			defer srv.Close()

			c := newClient(t, httpembedder.Config{BaseURL: srv.URL, Model: "m", APIKey: secret})
			_, err := c.Embed(context.Background(), "x")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			msg := err.Error()
			if strings.Contains(msg, secret) {
				t.Errorf("error string leaks the API key: %q", msg)
			}
			if !strings.Contains(msg, "[REDACTED]") {
				t.Errorf("expected redaction marker in error string: %q", msg)
			}
		})
	}

	// Malformed-but-200 responses that embed the key must also be safe: the
	// JSON decode error quotes only offsets, never the body, and our own
	// wrapping adds no header material.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "garbage %s garbage", secret)
	}))
	defer srv.Close()
	c := newClient(t, httpembedder.Config{BaseURL: srv.URL, Model: "m", APIKey: secret})
	_, err := c.Embed(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("malformed-response error leaks the API key: %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Interface compliance and composition
// ---------------------------------------------------------------------------

func TestInterfaceCompliance(t *testing.T) {
	var _ embeddingclient.Embedder = (*httpembedder.Client)(nil)
	var _ embeddingclient.DimensionedEmbedder = (*httpembedder.Client)(nil)
}

func TestComposesWithCachingEmbedder(t *testing.T) {
	h := &embeddingsHandler{t: t, dim: 4}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := newClient(t, httpembedder.Config{BaseURL: srv.URL, Model: "m"})
	cached := embeddingclient.NewCachingEmbedder(c)
	ctx := context.Background()

	if _, err := cached.Embed(ctx, "repeat"); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if _, err := cached.Embed(ctx, "repeat"); err != nil {
		t.Fatalf("Embed (cached): %v", err)
	}

	if reqs, _ := h.snapshot(); len(reqs) != 1 {
		t.Errorf("server saw %d requests, want 1 (second resolved from cache)", len(reqs))
	}
	if cached.Hits() != 1 || cached.Misses() != 1 {
		t.Errorf("hits=%d misses=%d, want 1/1", cached.Hits(), cached.Misses())
	}
	// The caching wrapper forwards the inferred dimension.
	if got, ok := embeddingclient.DimensionsOf(cached); !ok || got != 4 {
		t.Errorf("DimensionsOf(cached) = (%d, %v), want (4, true)", got, ok)
	}
}
