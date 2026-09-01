package embed_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/twplus/embed"
)

func l2(v embed.Vector) float64 {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	return math.Sqrt(s)
}

func TestNormalize(t *testing.T) {
	v := embed.Normalize(embed.Vector{3, 4}) // 3-4-5 triangle → unit length
	if got := l2(v); math.Abs(got-1) > 1e-6 {
		t.Errorf("normalized length = %v, want 1", got)
	}
	// A zero vector must survive without producing NaNs.
	z := embed.Normalize(embed.Vector{0, 0, 0})
	for i, x := range z {
		if math.IsNaN(float64(x)) {
			t.Fatalf("zero vector produced NaN at %d", i)
		}
	}
}

func TestOllamaEmbed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("path = %q, want /api/embed", r.URL.Path)
		}
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "test-model" {
			t.Errorf("model = %q", req.Model)
		}
		// Return an un-normalized vector per input; the client must normalize.
		out := make([][]float32, len(req.Input))
		for i := range req.Input {
			out[i] = []float32{3, 4, 0}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": out})
	}))
	defer srv.Close()

	o := embed.NewOllama(srv.URL, "test-model")
	vecs, err := o.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("got %d vectors, want 2", len(vecs))
	}
	if math.Abs(l2(vecs[0])-1) > 1e-6 {
		t.Errorf("client did not normalize: length = %v", l2(vecs[0]))
	}
	if o.Dims() != 3 {
		t.Errorf("dims = %d, want 3", o.Dims())
	}
}

func TestOllamaErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"model not found"}`))
	}))
	defer srv.Close()
	if _, err := embed.NewOllama(srv.URL, "missing").Embed(context.Background(), []string{"x"}); err == nil {
		t.Fatal("expected error on non-200 response")
	}
}

func TestOllamaCountMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": [][]float32{{1, 0}}})
	}))
	defer srv.Close()
	// Two inputs, one embedding back → must error rather than misalign the index.
	if _, err := embed.NewOllama(srv.URL, "m").Embed(context.Background(), []string{"a", "b"}); err == nil {
		t.Fatal("expected error on count mismatch")
	}
}
