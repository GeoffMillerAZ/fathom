package embeddingclient_test

import (
	"context"
	"math"
	"sync"
	"testing"

	ec "github.com/geoffmilleraz/agentic-kit/foundations/embedding/embedding-client"
)

// ---------------------------------------------------------------------------
// MockEmbedder tests
// ---------------------------------------------------------------------------

func TestMockEmbedder_Determinism(t *testing.T) {
	m := ec.NewMockEmbedder(8)
	ctx := context.Background()

	v1, err := m.Embed(ctx, "hello world")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	v2, err := m.Embed(ctx, "hello world")
	if err != nil {
		t.Fatalf("Embed second call: %v", err)
	}

	if len(v1) != 8 {
		t.Errorf("expected dim 8, got %d", len(v1))
	}
	for i := range v1 {
		if v1[i] != v2[i] {
			t.Errorf("non-deterministic at index %d: %v != %v", i, v1[i], v2[i])
		}
	}
}

func TestMockEmbedder_DifferentTextsProduceDifferentVectors(t *testing.T) {
	m := ec.NewMockEmbedder(16)
	ctx := context.Background()

	v1, _ := m.Embed(ctx, "foo")
	v2, _ := m.Embed(ctx, "bar")

	equal := true
	for i := range v1 {
		if v1[i] != v2[i] {
			equal = false
			break
		}
	}
	if equal {
		t.Error("different texts produced identical vectors")
	}
}

func TestMockEmbedder_UnitNorm(t *testing.T) {
	m := ec.NewMockEmbedder(32)
	ctx := context.Background()

	v, _ := m.Embed(ctx, "normalised check")
	var sumSq float64
	for _, x := range v {
		sumSq += x * x
	}
	norm := math.Sqrt(sumSq)
	if math.Abs(norm-1.0) > 1e-9 {
		t.Errorf("vector not unit-normalised: norm = %v", norm)
	}
}

func TestMockEmbedder_DefaultDim(t *testing.T) {
	m := ec.NewMockEmbedder(0) // 0 → DefaultMockDim
	v, _ := m.Embed(context.Background(), "dim check")
	if len(v) != ec.DefaultMockDim {
		t.Errorf("expected %d dims, got %d", ec.DefaultMockDim, len(v))
	}
}

func TestMockEmbedder_EmbedBatch(t *testing.T) {
	m := ec.NewMockEmbedder(8)
	ctx := context.Background()
	texts := []string{"alpha", "beta", "gamma"}

	vecs, err := m.EmbedBatch(ctx, texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(vecs) != len(texts) {
		t.Fatalf("expected %d vectors, got %d", len(texts), len(vecs))
	}

	// Each should match the single-embed result.
	for i, txt := range texts {
		single, _ := m.Embed(ctx, txt)
		for j := range single {
			if single[j] != vecs[i][j] {
				t.Errorf("batch[%d][%d] mismatch: %v vs %v", i, j, vecs[i][j], single[j])
			}
		}
	}
}

// ---------------------------------------------------------------------------
// CachingEmbedder — hit / miss counters
// ---------------------------------------------------------------------------

func TestCachingEmbedder_HitMiss(t *testing.T) {
	mock := ec.NewMockEmbedder(4)
	cache := ec.NewCachingEmbedder(mock)
	ctx := context.Background()

	// First call — miss.
	_, err := cache.Embed(ctx, "test")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if cache.Misses() != 1 {
		t.Errorf("expected 1 miss, got %d", cache.Misses())
	}
	if cache.Hits() != 0 {
		t.Errorf("expected 0 hits, got %d", cache.Hits())
	}

	// Second call for same text — hit.
	_, err = cache.Embed(ctx, "test")
	if err != nil {
		t.Fatalf("Embed second: %v", err)
	}
	if cache.Hits() != 1 {
		t.Errorf("expected 1 hit, got %d", cache.Hits())
	}
	if cache.Misses() != 1 {
		t.Errorf("misses should still be 1, got %d", cache.Misses())
	}
}

func TestCachingEmbedder_ReturnsSameVector(t *testing.T) {
	mock := ec.NewMockEmbedder(8)
	cache := ec.NewCachingEmbedder(mock)
	ctx := context.Background()

	v1, _ := cache.Embed(ctx, "repeat")
	v2, _ := cache.Embed(ctx, "repeat")

	for i := range v1 {
		if v1[i] != v2[i] {
			t.Errorf("cached vector differs at index %d", i)
		}
	}
}

// ---------------------------------------------------------------------------
// CachingEmbedder — batch behaviour
// ---------------------------------------------------------------------------

func TestCachingEmbedder_BatchDeduplication(t *testing.T) {
	mock := ec.NewMockEmbedder(4)
	cache := ec.NewCachingEmbedder(mock)
	ctx := context.Background()

	// "a" appears twice; the inner Embedder should only be called once for it.
	texts := []string{"a", "b", "a"}
	vecs, err := cache.EmbedBatch(ctx, texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(vecs) != 3 {
		t.Fatalf("expected 3 results, got %d", len(vecs))
	}

	// vecs[0] and vecs[2] must be identical (same text "a").
	for i := range vecs[0] {
		if vecs[0][i] != vecs[2][i] {
			t.Errorf("duplicate text gave different vectors at index %d", i)
		}
	}

	// "a" was a miss → should now be cached; second batch call hits.
	_, _ = cache.EmbedBatch(ctx, []string{"a"})
	if cache.Hits() < 1 {
		t.Errorf("expected at least 1 hit after re-embedding cached text, got %d", cache.Hits())
	}
}

func TestCachingEmbedder_BatchPartialCache(t *testing.T) {
	mock := ec.NewMockEmbedder(4)
	cache := ec.NewCachingEmbedder(mock)
	ctx := context.Background()

	// Warm up "x" in the cache.
	_, _ = cache.Embed(ctx, "x")
	missesBefore := cache.Misses()
	hitsBefore := cache.Hits()

	// Batch with one cached and one uncached text.
	vecs, err := cache.EmbedBatch(ctx, []string{"x", "y"})
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vecs))
	}

	if cache.Hits() != hitsBefore+1 {
		t.Errorf("expected 1 new hit, hits=%d before=%d", cache.Hits(), hitsBefore)
	}
	if cache.Misses() != missesBefore+1 {
		t.Errorf("expected 1 new miss, misses=%d before=%d", cache.Misses(), missesBefore)
	}
}

// ---------------------------------------------------------------------------
// CachingEmbedder — concurrency under -race
// ---------------------------------------------------------------------------

func TestCachingEmbedder_Concurrency(t *testing.T) {
	mock := ec.NewMockEmbedder(8)
	cache := ec.NewCachingEmbedder(mock)
	ctx := context.Background()

	const goroutines = 50
	const texts = 5

	words := []string{"alpha", "beta", "gamma", "delta", "epsilon"}

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			text := words[id%texts]
			v, err := cache.Embed(ctx, text)
			if err != nil {
				t.Errorf("goroutine %d: Embed error: %v", id, err)
				return
			}
			if len(v) != 8 {
				t.Errorf("goroutine %d: expected dim 8, got %d", id, len(v))
			}
		}(g)
	}
	wg.Wait()

	totalOps := cache.Hits() + cache.Misses()
	if totalOps != goroutines {
		t.Errorf("expected %d total ops (hits+misses), got %d", goroutines, totalOps)
	}
}

func TestCachingEmbedder_ConcurrentBatch(t *testing.T) {
	mock := ec.NewMockEmbedder(4)
	cache := ec.NewCachingEmbedder(mock)
	ctx := context.Background()

	const goroutines = 20
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			texts := []string{"p", "q", "r"}
			vecs, err := cache.EmbedBatch(ctx, texts)
			if err != nil {
				t.Errorf("goroutine %d: EmbedBatch: %v", id, err)
				return
			}
			if len(vecs) != 3 {
				t.Errorf("goroutine %d: expected 3 vecs, got %d", id, len(vecs))
			}
		}(g)
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Interface compliance
// ---------------------------------------------------------------------------

func TestInterfaceCompliance(t *testing.T) {
	// Verify that both MockEmbedder and CachingEmbedder satisfy the Embedder
	// interface at compile time.
	var _ ec.Embedder = ec.NewMockEmbedder(4)
	var _ ec.Embedder = ec.NewCachingEmbedder(ec.NewMockEmbedder(4))
}
