// Package httpembedder is an HTTP embedding backend for OpenAI-compatible
// APIs: it POSTs to {BaseURL}/embeddings, which covers OpenAI, Ollama, vLLM,
// OpenRouter, and any other server speaking the same wire format.
//
// Composition: Client implements embeddingclient.Embedder and the
// embeddingclient.DimensionedEmbedder capability (dimension inferred from the
// first successful response). Wrap it with embeddingclient.NewCachingEmbedder
// for caching; failures map onto this package's sentinel errors (see
// errors.go) for retry policy.
//
// Concurrency: Client is safe for concurrent use — it is immutable after New
// apart from an atomically updated inferred dimension, and *http.Client is
// itself concurrency-safe.
//
// Secrecy contract: the API key travels only in the Authorization header and
// is never written into error strings, logs, or events; server-echoed bodies
// are redacted before inclusion in errors.
package httpembedder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"

	embeddingclient "github.com/geoffmilleraz/agentic-kit/foundations/embedding/embedding-client"
)

// maxErrSnippet caps how much of an error response body is quoted in an
// error string.
const maxErrSnippet = 256

// maxErrBodyRead caps how many bytes of a non-200 response body are read at
// all.
const maxErrBodyRead = 4096

// Client is an OpenAI-compatible HTTP Embedder. Construct with New; the zero
// value is not usable.
type Client struct {
	baseURL  string // normalised, no trailing slash
	model    string
	apiKey   string // never included in any error string
	maxBatch int
	http     *http.Client

	// dims is the vector dimensionality inferred from the first successful
	// response; 0 until then.
	dims atomic.Int64
}

// Compile-time interface checks.
var (
	_ embeddingclient.Embedder            = (*Client)(nil)
	_ embeddingclient.DimensionedEmbedder = (*Client)(nil)
)

// New validates cfg and returns a ready Client. The API key is resolved once,
// here: Config.APIKey wins, otherwise the environment variable named by
// Config.APIKeyEnv is read; both empty means no Authorization header is sent.
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("httpembedder: Config.BaseURL is required")
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("httpembedder: Config.BaseURL %q must be a valid http(s) URL", cfg.BaseURL)
	}
	if cfg.Model == "" {
		return nil, errors.New("httpembedder: Config.Model is required")
	}
	if cfg.Timeout < 0 {
		return nil, errors.New("httpembedder: Config.Timeout must not be negative")
	}
	if cfg.MaxBatch < 0 {
		return nil, errors.New("httpembedder: Config.MaxBatch must not be negative")
	}

	key := cfg.APIKey
	if key == "" && cfg.APIKeyEnv != "" {
		key = os.Getenv(cfg.APIKeyEnv)
	}

	hc := cfg.HTTPClient
	if hc == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = DefaultTimeout
		}
		hc = &http.Client{Timeout: timeout}
	}

	maxBatch := cfg.MaxBatch
	if maxBatch == 0 {
		maxBatch = DefaultMaxBatch
	}

	return &Client{
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		model:    cfg.Model,
		apiKey:   key,
		maxBatch: maxBatch,
		http:     hc,
	}, nil
}

// Dimensions returns the vector dimensionality inferred from the first
// successful response, or 0 before any request has succeeded. Implements
// embeddingclient.DimensionedEmbedder.
func (c *Client) Dimensions() int { return int(c.dims.Load()) }

// Embed returns the embedding vector for a single text string.
func (c *Client) Embed(ctx context.Context, text string) (embeddingclient.Vector, error) {
	vecs, err := c.embedChunk(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

// EmbedBatch returns embedding vectors for texts, parallel to the input.
// Slices longer than MaxBatch are split into sequential chunked requests; a
// failure in any chunk fails the whole call. An empty input returns an empty
// slice without any HTTP request.
func (c *Client) EmbedBatch(ctx context.Context, texts []string) ([]embeddingclient.Vector, error) {
	out := make([]embeddingclient.Vector, 0, len(texts))
	for start := 0; start < len(texts); start += c.maxBatch {
		end := min(start+c.maxBatch, len(texts))
		vecs, err := c.embedChunk(ctx, texts[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, vecs...)
	}
	return out, nil
}

// embedRequest is the OpenAI-compatible request body.
type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embedResponse is the subset of the OpenAI-compatible response body the
// client consumes.
type embedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

// apiErrorBody is the OpenAI-compatible error envelope.
type apiErrorBody struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// embedChunk performs one POST {baseURL}/embeddings call for up to MaxBatch
// texts and returns vectors parallel to texts.
func (c *Client) embedChunk(ctx context.Context, texts []string) ([]embeddingclient.Vector, error) {
	payload, err := json.Marshal(embedRequest{Model: c.model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("httpembedder: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("httpembedder: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpembedder: post %s/embeddings: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrBodyRead))
		return nil, c.statusError(resp.StatusCode, body)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("httpembedder: read response: %w", err)
	}

	var er embedResponse
	if err := json.Unmarshal(body, &er); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedResponse, err)
	}
	if len(er.Data) != len(texts) {
		return nil, fmt.Errorf("%w: got %d embeddings for %d inputs", ErrMalformedResponse, len(er.Data), len(texts))
	}

	// Place by the response's index field when the indexes form a valid
	// permutation of the inputs (the OpenAI contract); otherwise fall back to
	// positional order for servers that omit index (it then decodes as 0 for
	// every element).
	byIndex := true
	seen := make([]bool, len(texts))
	for _, d := range er.Data {
		if d.Index < 0 || d.Index >= len(texts) || seen[d.Index] {
			byIndex = false
			break
		}
		seen[d.Index] = true
	}

	out := make([]embeddingclient.Vector, len(texts))
	for pos, d := range er.Data {
		if len(d.Embedding) == 0 {
			return nil, fmt.Errorf("%w: empty embedding in response", ErrMalformedResponse)
		}
		i := pos
		if byIndex {
			i = d.Index
		}
		out[i] = embeddingclient.Vector(d.Embedding)
	}

	c.dims.CompareAndSwap(0, int64(len(out[0])))
	return out, nil
}

// statusError builds the typed error for a non-200 response. The server
// message is preferred from the OpenAI error envelope, truncated, and
// redacted so the API key can never leak into an error string even if the
// backend echoes it.
func (c *Client) statusError(status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	var ae apiErrorBody
	if json.Unmarshal(body, &ae) == nil && ae.Error.Message != "" {
		msg = ae.Error.Message
	}
	if len(msg) > maxErrSnippet {
		msg = msg[:maxErrSnippet] + "..."
	}
	return fmt.Errorf("%w: status %d: %s", classify(status), status, c.redact(msg))
}

// redact removes the API key from s. It is applied to every server-provided
// string before that string is embedded in an error.
func (c *Client) redact(s string) string {
	if c.apiKey == "" {
		return s
	}
	return strings.ReplaceAll(s, c.apiKey, "[REDACTED]")
}
