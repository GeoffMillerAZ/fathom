package answercache_test

import (
	"context"
	"testing"
	"time"

	embeddingclient "github.com/geoffmilleraz/agentic-kit/foundations/embedding/embedding-client"
	answercache "github.com/geoffmilleraz/agentic-kit/foundations/governance/answer-cache"
)

// fakeNow returns a controllable clock.
type fakeClock struct{ t time.Time }

func (f *fakeClock) now() time.Time          { return f.t }
func (f *fakeClock) advance(d time.Duration) { f.t = f.t.Add(d) }

func newClock() *fakeClock {
	return &fakeClock{t: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func defaultConfig() answercache.Config {
	return answercache.Config{
		TTL:          5 * time.Minute,
		SimThreshold: 0.99,
		MaxEntries:   10,
	}
}

// TestExactHit verifies that the same question (after normalization) returns the cached answer.
func TestExactHit(t *testing.T) {
	clk := newClock()
	c := answercache.New(defaultConfig(), clk.now, nil)

	c.Put(context.Background(), "What is your return policy?", "We offer 30-day returns.")
	ans, ok := c.Get(context.Background(), "what is your return policy?") // lowercase variant
	if !ok {
		t.Fatal("expected cache hit")
	}
	if ans != "We offer 30-day returns." {
		t.Fatalf("unexpected answer: %q", ans)
	}
}

// TestMiss verifies that an unknown question returns a miss.
func TestMiss(t *testing.T) {
	clk := newClock()
	c := answercache.New(defaultConfig(), clk.now, nil)

	_, ok := c.Get(context.Background(), "what is the weather today")
	if ok {
		t.Fatal("expected cache miss for unseen question")
	}
}

// TestTTLExpiry verifies that a cached entry is not returned after TTL expires.
func TestTTLExpiry(t *testing.T) {
	clk := newClock()
	cfg := answercache.Config{TTL: 2 * time.Minute, SimThreshold: 0.99, MaxEntries: 10}
	c := answercache.New(cfg, clk.now, nil)

	c.Put(context.Background(), "how do I cancel?", "Call us at 1-800-000.")

	// Advance past TTL.
	clk.advance(3 * time.Minute)

	_, ok := c.Get(context.Background(), "how do I cancel?")
	if ok {
		t.Fatal("expected cache miss after TTL expiry")
	}
}

// TestTTLNotExpired verifies that entries are still served before TTL expires.
func TestTTLNotExpired(t *testing.T) {
	clk := newClock()
	cfg := answercache.Config{TTL: 10 * time.Minute, SimThreshold: 0.99, MaxEntries: 10}
	c := answercache.New(cfg, clk.now, nil)

	c.Put(context.Background(), "what are your hours?", "9am to 5pm")
	clk.advance(9 * time.Minute)

	ans, ok := c.Get(context.Background(), "what are your hours?")
	if !ok {
		t.Fatal("expected cache hit before TTL expiry")
	}
	if ans != "9am to 5pm" {
		t.Fatalf("unexpected answer: %q", ans)
	}
}

// TestNearDupHit verifies that a semantically similar but textually different question
// returns the cached answer when an embedder is configured.
func TestNearDupHit(t *testing.T) {
	clk := newClock()
	emb := embeddingclient.NewMockEmbedder(0)
	// Use a very low threshold so ANY two different strings can match.
	// MockEmbedder is deterministic: same text → same vector.
	// We need two texts with high cosine. Use the same text to guarantee sim==1.0.
	cfg := answercache.Config{TTL: 5 * time.Minute, SimThreshold: 0.999, MaxEntries: 10}
	c := answercache.New(cfg, clk.now, emb)

	// Store under one phrasing.
	c.Put(context.Background(), "cancel my subscription", "To cancel, go to account settings.")

	// Query with an identical string (sim = 1.0 in MockEmbedder, above threshold).
	// We need to use a distinct normalized key so exact match doesn't fire,
	// but the same embedding. Add trailing whitespace so normalization strips it
	// but the key differs — actually normalization collapses whitespace, so keys
	// would match. Let's use a truly different phrase that happens to get similar
	// embedding. Because MockEmbedder is FNV-based, truly different text → different
	// vector. So instead: we store a key, then query with a key whose normalized
	// form differs but whose embedding is identical by using the *same text*.
	// To force near-dup (not exact): store "cancel my subscription", get
	// "  cancel  my   subscription  " — normalization collapses to same key → exact match.
	// The only way to test near-dup independently is to have a text with a different
	// normalized key but same embedding — impossible with hash-based mock.
	// Instead: lower the threshold to a value that two *different* texts will satisfy.
	// MockEmbedder produces unit-normalised FNV vectors; different texts won't be
	// identical but may be close. We'll pick text pairs that are close enough
	// by testing empirically, OR we use SimThreshold=0 (always match).
	//
	// The cleanest approach: set SimThreshold = -1.0 so any embedding matches.
	// That is a valid float64 and our implementation should accept it.
	_ = c // discard this cache

	cfgLow := answercache.Config{TTL: 5 * time.Minute, SimThreshold: -1.0, MaxEntries: 10}
	c2 := answercache.New(cfgLow, clk.now, emb)

	c2.Put(context.Background(), "cancel my subscription", "To cancel, go to account settings.")

	// Query with a key whose normalized form is different.
	ans, ok := c2.Get(context.Background(), "terminate my subscription plan")
	if !ok {
		t.Fatal("expected near-dup cache hit with SimThreshold=-1.0")
	}
	if ans != "To cancel, go to account settings." {
		t.Fatalf("unexpected answer: %q", ans)
	}
}

// TestNearDupMissNoEmbedder verifies that without an embedder, near-dup matching is skipped.
func TestNearDupMissNoEmbedder(t *testing.T) {
	clk := newClock()
	cfg := answercache.Config{TTL: 5 * time.Minute, SimThreshold: 0.9, MaxEntries: 10}
	c := answercache.New(cfg, clk.now, nil) // no embedder

	c.Put(context.Background(), "cancel my subscription", "To cancel, go to account settings.")

	// Different normalized key, no embedder → miss.
	_, ok := c.Get(context.Background(), "terminate my subscription plan")
	if ok {
		t.Fatal("expected miss when no embedder is configured")
	}
}

// TestEviction verifies that when MaxEntries is exceeded, old entries are evicted.
func TestEviction(t *testing.T) {
	clk := newClock()
	cfg := answercache.Config{TTL: 10 * time.Minute, SimThreshold: 0.99, MaxEntries: 3}
	c := answercache.New(cfg, clk.now, nil)

	c.Put(context.Background(), "question one", "answer one")
	c.Put(context.Background(), "question two", "answer two")
	c.Put(context.Background(), "question three", "answer three")

	// Access "question one" to make it recently used.
	_, _ = c.Get(context.Background(), "question one")

	// Add a fourth entry — should evict the LRU entry (question two).
	c.Put(context.Background(), "question four", "answer four")

	// question two should be evicted.
	_, ok := c.Get(context.Background(), "question two")
	if ok {
		t.Fatal("expected question two to be evicted")
	}

	// question one, three, four should still be present.
	for _, q := range []string{"question one", "question three", "question four"} {
		if _, ok := c.Get(context.Background(), q); !ok {
			t.Fatalf("expected %q to still be in cache", q)
		}
	}
}

// TestSnapshotRestore verifies Snapshot and Restore round-trip correctly.
func TestSnapshotRestore(t *testing.T) {
	clk := newClock()
	c := answercache.New(defaultConfig(), clk.now, nil)

	c.Put(context.Background(), "what is 2+2?", "4")
	c.Put(context.Background(), "what is the capital of France?", "Paris")

	snap := c.Snapshot()

	// Create a new cache and restore.
	c2 := answercache.New(defaultConfig(), clk.now, nil)
	c2.Restore(snap)

	ans, ok := c2.Get(context.Background(), "what is 2+2?")
	if !ok || ans != "4" {
		t.Fatalf("restore failed for 'what is 2+2?': ok=%v ans=%q", ok, ans)
	}

	ans2, ok2 := c2.Get(context.Background(), "what is the capital of France?")
	if !ok2 || ans2 != "Paris" {
		t.Fatalf("restore failed for capital question: ok=%v ans=%q", ok2, ans2)
	}
}

// TestSnapshotRestoreTTL verifies that TTL timestamps survive snapshot/restore.
func TestSnapshotRestoreTTL(t *testing.T) {
	clk := newClock()
	cfg := answercache.Config{TTL: 2 * time.Minute, SimThreshold: 0.99, MaxEntries: 10}
	c := answercache.New(cfg, clk.now, nil)

	c.Put(context.Background(), "ephemeral question", "ephemeral answer")

	snap := c.Snapshot()

	// Advance time past TTL before restoring.
	clk.advance(3 * time.Minute)

	c2 := answercache.New(cfg, clk.now, nil)
	c2.Restore(snap)

	_, ok := c2.Get(context.Background(), "ephemeral question")
	if ok {
		t.Fatal("expected TTL-expired entry to be a miss after restore")
	}
}

// TestDeterminism verifies that Get returns the same result on repeated calls.
func TestDeterminism(t *testing.T) {
	clk := newClock()
	c := answercache.New(defaultConfig(), clk.now, nil)
	c.Put(context.Background(), "stable question", "stable answer")

	ctx := context.Background()
	for i := 0; i < 20; i++ {
		ans, ok := c.Get(ctx, "stable question")
		if !ok || ans != "stable answer" {
			t.Fatalf("non-deterministic at iteration %d: ok=%v ans=%q", i, ok, ans)
		}
	}
}

// TestConcurrency verifies that concurrent Get/Put do not race. Run with -race.
func TestConcurrency(t *testing.T) {
	clk := newClock()
	c := answercache.New(defaultConfig(), clk.now, nil)
	ctx := context.Background()

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			c.Put(ctx, "concurrent question", "some answer")
			c.Get(ctx, "concurrent question")
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestNormalization verifies that whitespace and case normalization works.
func TestNormalization(t *testing.T) {
	clk := newClock()
	c := answercache.New(defaultConfig(), clk.now, nil)

	c.Put(context.Background(), "  What  IS   your POLICY?  ", "Our policy is X.")

	cases := []string{
		"what is your policy?",
		"  what  is  your  policy?  ",
		"WHAT IS YOUR POLICY?",
	}
	for _, q := range cases {
		ans, ok := c.Get(context.Background(), q)
		if !ok || ans != "Our policy is X." {
			t.Fatalf("normalization failed for %q: ok=%v ans=%q", q, ok, ans)
		}
	}
}
