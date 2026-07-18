package embeddingclient

import (
	"context"
	"encoding/binary"
	"hash/fnv"
	"math"
	"sync"
	"sync/atomic"
)

// DefaultMockDim is the vector dimension produced by MockEmbedder when no
// explicit dimension is configured.
const DefaultMockDim = 16

// MockEmbedder is a deterministic, network-free Embedder for use in tests.
// Given the same text it always returns the same vector, making assertions
// stable across runs. Vectors are derived from a 64-bit FNV hash of the input
// text spread over Dim dimensions.
//
// MockEmbedder is safe for concurrent use.
type MockEmbedder struct {
	// Dim is the number of dimensions in every returned vector.
	// Defaults to DefaultMockDim when zero.
	Dim int

	callCount atomic.Int64
	mu        sync.Mutex
	calls     []string
}

// NewMockEmbedder returns a MockEmbedder with the given dimension. Pass 0 to
// use DefaultMockDim.
func NewMockEmbedder(dim int) *MockEmbedder {
	if dim <= 0 {
		dim = DefaultMockDim
	}
	return &MockEmbedder{Dim: dim}
}

// Embed returns a deterministic vector for text.
func (m *MockEmbedder) Embed(_ context.Context, text string) (Vector, error) {
	m.callCount.Add(1)
	m.mu.Lock()
	m.calls = append(m.calls, text)
	m.mu.Unlock()
	return m.vectorFor(text), nil
}

// EmbedBatch returns deterministic vectors for each element of texts.
func (m *MockEmbedder) EmbedBatch(_ context.Context, texts []string) ([]Vector, error) {
	m.callCount.Add(int64(len(texts)))
	m.mu.Lock()
	m.calls = append(m.calls, texts...)
	m.mu.Unlock()

	out := make([]Vector, len(texts))
	for i, t := range texts {
		out[i] = m.vectorFor(t)
	}
	return out, nil
}

// Dimensions returns the configured vector dimensionality, implementing the
// DimensionedEmbedder capability interface. It mirrors vectorFor: a
// non-positive Dim reports DefaultMockDim.
func (m *MockEmbedder) Dimensions() int {
	if m.Dim <= 0 {
		return DefaultMockDim
	}
	return m.Dim
}

// CallCount returns the total number of individual text embeddings requested
// (each element of a batch counts as one).
func (m *MockEmbedder) CallCount() int64 { return m.callCount.Load() }

// Calls returns a snapshot of all texts that have been embedded so far.
func (m *MockEmbedder) Calls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(m.calls))
	copy(cp, m.calls)
	return cp
}

// vectorFor deterministically maps text to a unit-normalised Vector by seeding
// a sequence of FNV-1a hashes, one per dimension.
func (m *MockEmbedder) vectorFor(text string) Vector {
	dim := m.Dim
	if dim <= 0 {
		dim = DefaultMockDim
	}
	v := make(Vector, dim)
	var sumSq float64
	for i := range v {
		h := fnv.New64a()
		// Mix text with dimension index so each dimension gets a distinct value.
		h.Write([]byte(text))
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(i))
		h.Write(buf[:])
		// Map uint64 hash to [-1, 1].
		raw := float64(h.Sum64()) / float64(math.MaxUint64)
		val := raw*2 - 1
		v[i] = val
		sumSq += val * val
	}
	// L2-normalise so dot-product == cosine similarity for free.
	if sumSq > 0 {
		norm := math.Sqrt(sumSq)
		for i := range v {
			v[i] /= norm
		}
	}
	return v
}
