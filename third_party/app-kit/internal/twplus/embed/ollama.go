package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultOllamaURL is the local Ollama daemon's default address. The endpoint
// stays loopback by design — embeddings are derived from licensed content.
const DefaultOllamaURL = "http://127.0.0.1:11434"

// Ollama embeds text via a local Ollama daemon's POST /api/embed. It is the
// only network-touching part of the semantic path and is never reached unless
// `appkit data embed` (offline) or a hybrid query with an embedder is run.
type Ollama struct {
	BaseURL   string
	ModelName string
	dims      int // 0 until the first Embed call learns it from the response
	HTTP      *http.Client
}

// NewOllama returns a client for model on baseURL (DefaultOllamaURL if empty).
func NewOllama(baseURL, model string) *Ollama {
	if baseURL == "" {
		baseURL = DefaultOllamaURL
	}
	return &Ollama{
		BaseURL:   baseURL,
		ModelName: model,
		HTTP:      &http.Client{Timeout: 60 * time.Second},
	}
}

func (o *Ollama) Model() string { return o.ModelName }
func (o *Ollama) Dims() int     { return o.dims }

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Error      string      `json:"error,omitempty"`
}

// Embed sends texts to /api/embed in one request and returns one normalized
// vector each. It validates that the count and dimensionality are consistent so
// a misbehaving backend fails loudly rather than corrupting the index.
func (o *Ollama) Embed(ctx context.Context, texts []string) ([]Vector, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(embedRequest{Model: o.ModelName, Input: texts})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed: ollama unreachable at %s (is it running? `ollama serve`): %w", o.BaseURL, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed: ollama %s: %s", resp.Status, snippet(raw))
	}
	var er embedResponse
	if err := json.Unmarshal(raw, &er); err != nil {
		return nil, fmt.Errorf("embed: decode response: %w", err)
	}
	if er.Error != "" {
		return nil, fmt.Errorf("embed: ollama: %s", er.Error)
	}
	if len(er.Embeddings) != len(texts) {
		return nil, fmt.Errorf("embed: got %d embeddings for %d inputs", len(er.Embeddings), len(texts))
	}
	out := make([]Vector, len(er.Embeddings))
	for i, e := range er.Embeddings {
		if len(e) == 0 {
			return nil, fmt.Errorf("embed: empty vector at index %d", i)
		}
		if o.dims == 0 {
			o.dims = len(e)
		}
		if len(e) != o.dims {
			return nil, fmt.Errorf("embed: inconsistent dims: got %d, expected %d", len(e), o.dims)
		}
		out[i] = Normalize(Vector(e))
	}
	return out, nil
}

func snippet(b []byte) string {
	const max = 200
	if len(b) > max {
		return string(b[:max]) + "…"
	}
	return string(b)
}
