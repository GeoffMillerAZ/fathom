package resultcache

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestGet_MissOnEmpty(t *testing.T) {
	c := New()
	_, ok := c.Get("missing")
	if ok {
		t.Fatal("expected miss on empty cache")
	}
	s := c.Stats()
	if s.Hits != 0 || s.Misses != 1 {
		t.Fatalf("stats: want {0,1} got {%d,%d}", s.Hits, s.Misses)
	}
}

func TestGetOrCompute_StoresAndReturns(t *testing.T) {
	c := New()
	calls := 0
	fn := func() (any, error) {
		calls++
		return "hello", nil
	}

	v, err := c.GetOrCompute("k", fn)
	if err != nil {
		t.Fatal(err)
	}
	if v != "hello" {
		t.Fatalf("want hello got %v", v)
	}
	if calls != 1 {
		t.Fatalf("fn should have been called once, got %d", calls)
	}

	// Second call must hit the cache.
	v2, err2 := c.GetOrCompute("k", fn)
	if err2 != nil {
		t.Fatal(err2)
	}
	if v2 != "hello" {
		t.Fatalf("want hello got %v", v2)
	}
	if calls != 1 {
		t.Fatalf("fn should not be called again, got %d", calls)
	}

	s := c.Stats()
	if s.Hits != 1 || s.Misses != 1 {
		t.Fatalf("stats: want {1,1} got {%d,%d}", s.Hits, s.Misses)
	}
}

func TestGet_HitAfterCompute(t *testing.T) {
	c := New()
	_, _ = c.GetOrCompute("x", func() (any, error) { return 42, nil })
	v, ok := c.Get("x")
	if !ok {
		t.Fatal("expected hit")
	}
	if v.(int) != 42 {
		t.Fatalf("want 42 got %v", v)
	}
}

func TestGetOrCompute_ErrorNotStored(t *testing.T) {
	c := New()
	boom := errors.New("boom")
	_, err := c.GetOrCompute("e", func() (any, error) { return nil, boom })
	if !errors.Is(err, boom) {
		t.Fatalf("want boom err got %v", err)
	}
	_, ok := c.Get("e")
	if ok {
		t.Fatal("errored result must not be cached")
	}
}

func TestInvalidate_ClearsAll(t *testing.T) {
	c := New()
	_, _ = c.GetOrCompute("a", func() (any, error) { return 1, nil })
	_, _ = c.GetOrCompute("b", func() (any, error) { return 2, nil })

	c.Invalidate()

	_, ok := c.Get("a")
	if ok {
		t.Fatal("a should be gone after Invalidate")
	}
	_, ok = c.Get("b")
	if ok {
		t.Fatal("b should be gone after Invalidate")
	}
	s := c.Stats()
	// After Invalidate stats reset; the two Get calls above each add a miss.
	if s.Hits != 0 || s.Misses != 2 {
		t.Fatalf("post-invalidate stats: want {0,2} got {%d,%d}", s.Hits, s.Misses)
	}
}

func TestConcurrent_RaceDetector(t *testing.T) {
	c := New()
	var wg sync.WaitGroup
	var computeCount int64
	const goroutines = 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "shared"
			_, _ = c.GetOrCompute(key, func() (any, error) {
				atomic.AddInt64(&computeCount, 1)
				return "value", nil
			})
			c.Get(key)
		}(i)
	}
	wg.Wait()

	// At least one compute must have happened; cache must hold the value.
	v, ok := c.Get("shared")
	if !ok {
		t.Fatal("value should be cached after concurrent writes")
	}
	if v != "value" {
		t.Fatalf("unexpected value %v", v)
	}
}

func TestConcurrent_Invalidate(t *testing.T) {
	c := New()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = c.GetOrCompute("k", func() (any, error) { return 1, nil })
		}()
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Invalidate()
		}()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Stats()
		}()
	}
	wg.Wait()
}
