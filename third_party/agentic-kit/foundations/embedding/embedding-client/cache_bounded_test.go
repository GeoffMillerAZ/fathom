package embeddingclient_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"

	ec "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/embedding/embedding-client"
)

// ---------------------------------------------------------------------------
// Bounded mode — capacity resolution
// ---------------------------------------------------------------------------

func TestCachingEmbedder_CapacityResolution(t *testing.T) {
	tests := []struct {
		name string
		c    *ec.CachingEmbedder
		want int
	}{
		{"plain constructor defaults to bounded", ec.NewCachingEmbedder(ec.NewMockEmbedder(4)), ec.DefaultMaxEntries},
		{"zero MaxEntries means default", ec.NewCachingEmbedderWithOptions(ec.NewMockEmbedder(4), ec.CacheOptions{MaxEntries: 0}), ec.DefaultMaxEntries},
		{"positive MaxEntries is exact", ec.NewCachingEmbedderWithOptions(ec.NewMockEmbedder(4), ec.CacheOptions{MaxEntries: 7}), 7},
		{"minus one means unbounded", ec.NewCachingEmbedderWithOptions(ec.NewMockEmbedder(4), ec.CacheOptions{MaxEntries: -1}), -1},
		{"any negative means unbounded", ec.NewCachingEmbedderWithOptions(ec.NewMockEmbedder(4), ec.CacheOptions{MaxEntries: -42}), -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.MaxEntries(); got != tt.want {
				t.Errorf("MaxEntries() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Bounded mode — LRU eviction
// ---------------------------------------------------------------------------

func TestCachingEmbedder_LRUEviction(t *testing.T) {
	mock := ec.NewMockEmbedder(4)
	cache := ec.NewCachingEmbedderWithOptions(mock, ec.CacheOptions{MaxEntries: 2})
	ctx := context.Background()

	// Fill: a, b. LRU order (most→least recent): b, a.
	_, _ = cache.Embed(ctx, "a")
	_, _ = cache.Embed(ctx, "b")
	if cache.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", cache.Len())
	}

	// c evicts a (the least recently used).
	_, _ = cache.Embed(ctx, "c")
	if cache.Len() != 2 {
		t.Fatalf("Len() after eviction = %d, want 2", cache.Len())
	}

	// b is still cached → hit.
	hitsBefore := cache.Hits()
	_, _ = cache.Embed(ctx, "b")
	if cache.Hits() != hitsBefore+1 {
		t.Errorf("expected hit for retained entry b, hits=%d before=%d", cache.Hits(), hitsBefore)
	}

	// a was evicted → miss.
	missesBefore := cache.Misses()
	_, _ = cache.Embed(ctx, "a")
	if cache.Misses() != missesBefore+1 {
		t.Errorf("expected miss for evicted entry a, misses=%d before=%d", cache.Misses(), missesBefore)
	}
}

func TestCachingEmbedder_LRUPromotionOnHit(t *testing.T) {
	mock := ec.NewMockEmbedder(4)
	cache := ec.NewCachingEmbedderWithOptions(mock, ec.CacheOptions{MaxEntries: 2})
	ctx := context.Background()

	_, _ = cache.Embed(ctx, "a")
	_, _ = cache.Embed(ctx, "b")
	// Touch a so it becomes most recently used; b is now LRU.
	_, _ = cache.Embed(ctx, "a")

	// c evicts b, not a.
	_, _ = cache.Embed(ctx, "c")

	hitsBefore := cache.Hits()
	_, _ = cache.Embed(ctx, "a")
	if cache.Hits() != hitsBefore+1 {
		t.Errorf("a should have been retained after promotion; hits=%d before=%d", cache.Hits(), hitsBefore)
	}

	missesBefore := cache.Misses()
	_, _ = cache.Embed(ctx, "b")
	if cache.Misses() != missesBefore+1 {
		t.Errorf("b should have been evicted; misses=%d before=%d", cache.Misses(), missesBefore)
	}
}

func TestCachingEmbedder_CountersUnderEviction(t *testing.T) {
	mock := ec.NewMockEmbedder(4)
	cache := ec.NewCachingEmbedderWithOptions(mock, ec.CacheOptions{MaxEntries: 1})
	ctx := context.Background()

	_, _ = cache.Embed(ctx, "a") // miss
	_, _ = cache.Embed(ctx, "b") // miss, evicts a
	_, _ = cache.Embed(ctx, "b") // hit
	_, _ = cache.Embed(ctx, "a") // miss (was evicted)

	if cache.Hits() != 1 {
		t.Errorf("Hits() = %d, want 1", cache.Hits())
	}
	if cache.Misses() != 3 {
		t.Errorf("Misses() = %d, want 3", cache.Misses())
	}
	if cache.Len() != 1 {
		t.Errorf("Len() = %d, want 1", cache.Len())
	}
}

func TestCachingEmbedder_Unbounded(t *testing.T) {
	mock := ec.NewMockEmbedder(4)
	cache := ec.NewCachingEmbedderWithOptions(mock, ec.CacheOptions{MaxEntries: -1})
	ctx := context.Background()

	const n = 100
	for i := 0; i < n; i++ {
		_, _ = cache.Embed(ctx, "text-"+strconv.Itoa(i))
	}
	if cache.Len() != n {
		t.Errorf("unbounded cache Len() = %d, want %d", cache.Len(), n)
	}
}

func TestCachingEmbedder_BatchEviction(t *testing.T) {
	mock := ec.NewMockEmbedder(4)
	cache := ec.NewCachingEmbedderWithOptions(mock, ec.CacheOptions{MaxEntries: 2})
	ctx := context.Background()

	// A batch larger than the cap still returns correct, complete results.
	texts := []string{"a", "b", "c", "d"}
	vecs, err := cache.EmbedBatch(ctx, texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(vecs) != 4 {
		t.Fatalf("expected 4 vectors, got %d", len(vecs))
	}
	for i, v := range vecs {
		if len(v) != 4 {
			t.Errorf("vecs[%d] has dim %d, want 4", i, len(v))
		}
	}
	if cache.Len() != 2 {
		t.Errorf("Len() after over-cap batch = %d, want 2", cache.Len())
	}
	if cache.Misses() != 4 {
		t.Errorf("Misses() = %d, want 4", cache.Misses())
	}
}

// ---------------------------------------------------------------------------
// Batch dedup correctness regression
// ---------------------------------------------------------------------------

// TestCachingEmbedder_BatchDuplicateOrdering pins a former bug: with input
// ["a","a","b"] the old index bookkeeping mapped position 2 ("b") to the
// vector for "a".
func TestCachingEmbedder_BatchDuplicateOrdering(t *testing.T) {
	tests := []struct {
		name  string
		texts []string
	}{
		{"dup before distinct", []string{"a", "a", "b"}},
		{"dup after distinct", []string{"a", "b", "a"}},
		{"interleaved dups", []string{"x", "y", "x", "y", "z"}},
		{"all same", []string{"q", "q", "q"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := ec.NewMockEmbedder(4)
			cache := ec.NewCachingEmbedder(mock)
			ctx := context.Background()

			got, err := cache.EmbedBatch(ctx, tt.texts)
			if err != nil {
				t.Fatalf("EmbedBatch: %v", err)
			}

			// Reference vectors from an independent mock with the same
			// deterministic hash — no cache in the way.
			ref := ec.NewMockEmbedder(4)
			for i, text := range tt.texts {
				want, _ := ref.Embed(ctx, text)
				for j := range want {
					if got[i][j] != want[j] {
						t.Fatalf("position %d (%q): wrong vector", i, text)
					}
				}
			}
		})
	}
}

// errBatchEmbedder returns the wrong number of vectors from EmbedBatch to
// exercise the defensive length check.
type errBatchEmbedder struct{}

func (errBatchEmbedder) Embed(context.Context, string) (ec.Vector, error) {
	return ec.Vector{1}, nil
}

func (errBatchEmbedder) EmbedBatch(_ context.Context, texts []string) ([]ec.Vector, error) {
	return make([]ec.Vector, len(texts)+1), nil
}

func TestCachingEmbedder_InnerBatchLengthMismatch(t *testing.T) {
	cache := ec.NewCachingEmbedder(errBatchEmbedder{})
	_, err := cache.EmbedBatch(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("expected error for mismatched inner EmbedBatch length, got nil")
	}
}

// ---------------------------------------------------------------------------
// Bounded mode — concurrency under -race
// ---------------------------------------------------------------------------

func TestCachingEmbedder_BoundedConcurrency(t *testing.T) {
	mock := ec.NewMockEmbedder(8)
	cache := ec.NewCachingEmbedderWithOptions(mock, ec.CacheOptions{MaxEntries: 3})
	ctx := context.Background()

	words := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta"}
	const goroutines = 60

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			text := words[id%len(words)]
			if id%2 == 0 {
				v, err := cache.Embed(ctx, text)
				if err != nil {
					t.Errorf("goroutine %d: Embed: %v", id, err)
					return
				}
				if len(v) != 8 {
					t.Errorf("goroutine %d: dim %d, want 8", id, len(v))
				}
				return
			}
			vecs, err := cache.EmbedBatch(ctx, []string{text, words[(id+1)%len(words)]})
			if err != nil {
				t.Errorf("goroutine %d: EmbedBatch: %v", id, err)
				return
			}
			if len(vecs) != 2 {
				t.Errorf("goroutine %d: got %d vecs, want 2", id, len(vecs))
			}
		}(g)
	}
	wg.Wait()

	if cache.Len() > 3 {
		t.Errorf("Len() = %d exceeds cap 3", cache.Len())
	}
	if cache.Hits()+cache.Misses() == 0 {
		t.Error("expected nonzero hit+miss activity")
	}
}

// ---------------------------------------------------------------------------
// DimensionedEmbedder capability
// ---------------------------------------------------------------------------

// plainEmbedder implements only Embedder, not DimensionedEmbedder.
type plainEmbedder struct{}

func (plainEmbedder) Embed(context.Context, string) (ec.Vector, error) {
	return ec.Vector{0}, nil
}

func (plainEmbedder) EmbedBatch(_ context.Context, texts []string) ([]ec.Vector, error) {
	out := make([]ec.Vector, len(texts))
	for i := range out {
		out[i] = ec.Vector{0}
	}
	return out, nil
}

// zeroDimEmbedder implements DimensionedEmbedder but does not yet know its
// dimension.
type zeroDimEmbedder struct{ plainEmbedder }

func (zeroDimEmbedder) Dimensions() int { return 0 }

func TestDimensionsOf(t *testing.T) {
	tests := []struct {
		name   string
		e      ec.Embedder
		want   int
		wantOK bool
	}{
		{"mock with explicit dim", ec.NewMockEmbedder(64), 64, true},
		{"mock with default dim", ec.NewMockEmbedder(0), ec.DefaultMockDim, true},
		{"caching wrapper forwards inner dim", ec.NewCachingEmbedder(ec.NewMockEmbedder(32)), 32, true},
		{"plain embedder lacks capability", plainEmbedder{}, 0, false},
		{"caching wrapper over plain embedder", ec.NewCachingEmbedder(plainEmbedder{}), 0, false},
		{"capability present but dim unknown", zeroDimEmbedder{}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ec.DimensionsOf(tt.e)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("DimensionsOf() = (%d, %v), want (%d, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestDimensionedInterfaceCompliance(t *testing.T) {
	var _ ec.DimensionedEmbedder = ec.NewMockEmbedder(4)
	var _ ec.DimensionedEmbedder = ec.NewCachingEmbedder(ec.NewMockEmbedder(4))
}

func TestErrorPropagation(t *testing.T) {
	wantErr := errors.New("backend down")
	failing := failingEmbedder{err: wantErr}
	cache := ec.NewCachingEmbedder(failing)
	ctx := context.Background()

	if _, err := cache.Embed(ctx, "a"); !errors.Is(err, wantErr) {
		t.Errorf("Embed error = %v, want %v", err, wantErr)
	}
	if _, err := cache.EmbedBatch(ctx, []string{"a", "b"}); !errors.Is(err, wantErr) {
		t.Errorf("EmbedBatch error = %v, want %v", err, wantErr)
	}
	// Failed embeds must not populate the cache.
	if cache.Len() != 0 {
		t.Errorf("Len() = %d after failures, want 0", cache.Len())
	}
}

type failingEmbedder struct{ err error }

func (f failingEmbedder) Embed(context.Context, string) (ec.Vector, error) {
	return nil, f.err
}

func (f failingEmbedder) EmbedBatch(context.Context, []string) ([]ec.Vector, error) {
	return nil, f.err
}
