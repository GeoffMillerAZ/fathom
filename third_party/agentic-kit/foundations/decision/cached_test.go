package decision_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

// cachedInner is a concurrency-safe inner Decider for Cached tests: it
// answers every predicate question under typesafe and a model the test can
// move, and records every Request it is asked.
type cachedInner struct {
	mu      sync.Mutex
	model   string
	invalid map[string]bool // question IDs answered as Invalid
	fail    error
	asked   [][]string
}

func (c *cachedInner) Name() string { return "typesafe" }

func (c *cachedInner) Decide(_ context.Context, req decision.Request) (decision.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.asked = append(c.asked, decorIDs(req))
	if c.fail != nil {
		return decision.Response{}, c.fail
	}
	resp := decision.Response{Provider: "typesafe", Model: c.model, Answers: map[string]decision.Answer{}, Invalid: map[string]string{}, Usage: decision.Usage{InputTokens: 5 * len(req.Questions)}}
	for _, q := range req.Questions {
		if c.invalid[q.ID] {
			resp.Invalid[q.ID] = "out of range"
			continue
		}
		resp.Answers[q.ID] = decorPredicate(q.ID, 0.8)
	}
	return resp, nil
}

func (c *cachedInner) setModel(m string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.model = m
}

func (c *cachedInner) takeAsked() [][]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.asked
	c.asked = nil
	return out
}

func TestCachedKeysByTemplateAndContentHashStampsModel(t *testing.T) {
	ctx := context.Background()
	q1, q2, q3 := decorQuestion("q1"), decorQuestion("q2"), decorQuestion("q3")
	// q1again is q1's template under another ID: the same cache entry.
	q1again := q1
	q1again.ID = "q1-again"

	t.Run("the key is a template digest without the ID plus the content hash", func(t *testing.T) {
		if decision.TemplateDigest(q1, "v1") != decision.TemplateDigest(q1again, "v1") {
			t.Error("the question ID changed the template digest")
		}
		if decision.TemplateDigest(q1, "v1") == decision.TemplateDigest(q1, "v2") {
			t.Error("the template version did not change the digest")
		}
		if decision.TemplateDigest(q1, "v1") == decision.TemplateDigest(q2, "v1") {
			t.Error("different Instructions share a digest")
		}
		alias := q1
		alias.Kind = decision.Kind("noul")
		if decision.TemplateDigest(q1, "v1") != decision.TemplateDigest(alias, "v1") {
			t.Error("the noul alias changed the digest of a predicate")
		}
		sum := sha256.Sum256([]byte("the diff"))
		if got, want := decision.ContentHash("the diff"), "sha256:"+hex.EncodeToString(sum[:]); got != want {
			t.Errorf("ContentHash = %q, want %q", got, want)
		}
		key := decision.CacheKeyFor(q1, "the diff", "v1")
		if key.Template != decision.TemplateDigest(q1, "v1") || key.Content != decision.ContentHash("the diff") {
			t.Errorf("CacheKeyFor = %+v", key)
		}
	})

	t.Run("hits are served, misses batched into one call, entries stamped, and a new model re-grades", func(t *testing.T) {
		inner := &cachedInner{model: "jev-1"}
		store := decision.NewMemoryCacheStore()
		var observed []decision.CachedReport
		c, err := decision.NewCached(inner, decision.CachedOptions{
			Store: store, TemplateVersion: "v1",
			Observe: func(r decision.CachedReport) { observed = append(observed, r) },
		})
		if err != nil {
			t.Fatalf("NewCached() error = %v", err)
		}
		if got := c.Name(); got != "cached" {
			t.Errorf("Name() = %q, want cached", got)
		}

		// 1. Cold: every question misses, in one inner call.
		resp, report, err := c.DecideReport(ctx, decision.Request{State: "diff-A", Questions: []decision.Question{q1, q2}})
		if err != nil {
			t.Fatal(err)
		}
		if got := inner.takeAsked(); !reflect.DeepEqual(got, [][]string{{"q1", "q2"}}) {
			t.Errorf("inner asked %v, want one call [q1 q2]", got)
		}
		if len(report.Hits) != 0 || !reflect.DeepEqual(report.Misses, []string{"q1", "q2"}) {
			t.Errorf("hits %v misses %v", report.Hits, report.Misses)
		}
		if resp.Provider != "typesafe" || resp.Model != "jev-1" {
			t.Errorf("labeled %q/%q", resp.Provider, resp.Model)
		}
		wantKeys := []decision.CacheKey{decision.CacheKeyFor(q1, "diff-A", "v1"), decision.CacheKeyFor(q2, "diff-A", "v1")}
		if wantKeys[1].Template < wantKeys[0].Template {
			wantKeys[0], wantKeys[1] = wantKeys[1], wantKeys[0]
		}
		if got := store.Keys(); !reflect.DeepEqual(got, wantKeys) {
			t.Errorf("store keys = %v, want the two sorted keys %v", got, wantKeys)
		}
		entry, ok, err := store.Get(ctx, decision.CacheKeyFor(q1, "diff-A", "v1"))
		if err != nil || !ok || entry.Provider != "typesafe" || entry.Model != "jev-1" {
			t.Errorf("q1 entry = %+v (ok %v, err %v), want stamped typesafe/jev-1", entry, ok, err)
		}

		// 2. Same template under another ID hits; the new question misses
		// and is the only one sent.
		resp, report, err = c.DecideReport(ctx, decision.Request{State: "diff-A", Questions: []decision.Question{q1again, q3}})
		if err != nil {
			t.Fatal(err)
		}
		if got := inner.takeAsked(); !reflect.DeepEqual(got, [][]string{{"q3"}}) {
			t.Errorf("inner asked %v, want [[q3]]", got)
		}
		if !reflect.DeepEqual(report.Hits, []string{"q1-again"}) || !reflect.DeepEqual(report.Misses, []string{"q3"}) {
			t.Errorf("hits %v misses %v", report.Hits, report.Misses)
		}
		if got := resp.Answers["q1-again"]; got.ID != "q1-again" || got.Noul != 0.8 {
			t.Errorf("hit answer = %+v, want it re-keyed to q1-again", got)
		}
		if src := report.Sources["q1-again"]; !src.Cached || src.Model != "jev-1" {
			t.Errorf("hit source = %+v, want Cached under jev-1", src)
		}
		if src := report.Sources["q3"]; src.Cached {
			t.Errorf("fresh source = %+v, want not Cached", src)
		}
		// Hits are kept apart from proof: Fresh keeps only what the
		// provider answered in this call.
		fresh := report.Fresh(resp)
		if _, ok := fresh.Answers["q1-again"]; ok || len(fresh.Answers) != 1 || fresh.Model != "jev-1" {
			t.Errorf("Fresh = %+v, want only q3", fresh)
		}

		// 3. All hits: no inner call at all.
		resp, report, err = c.DecideReport(ctx, decision.Request{State: "diff-A", Questions: []decision.Question{q1, q2}})
		if err != nil {
			t.Fatal(err)
		}
		if got := inner.takeAsked(); len(got) != 0 {
			t.Errorf("inner asked %v on an all-hit call", got)
		}
		if resp.Provider != "typesafe" || resp.Model != "jev-1" || resp.Usage != (decision.Usage{}) || len(report.Hits) != 2 {
			t.Errorf("all-hit Response %+v, report %+v", resp, report)
		}
		if len(report.Fresh(resp).Answers) != 0 {
			t.Error("Fresh of an all-hit call is not empty")
		}

		// 4. Other content: the content hash changes, so it misses.
		if _, report, _ = c.DecideReport(ctx, decision.Request{State: "diff-B", Questions: []decision.Question{q1}}); len(report.Misses) != 1 {
			t.Errorf("new State: misses %v, want [q1]", report.Misses)
		}
		inner.takeAsked()

		// 5. The model moves: a miss reveals it, and the stale hits are
		// re-asked so the Response never mixes models.
		inner.setModel("jev-2")
		resp, report, err = c.DecideReport(ctx, decision.Request{State: "diff-A", Questions: []decision.Question{q1, q2, decorQuestion("q9")}})
		if err != nil {
			t.Fatal(err)
		}
		if got := inner.takeAsked(); !reflect.DeepEqual(got, [][]string{{"q9"}, {"q1", "q2"}}) {
			t.Errorf("inner asked %v, want [[q9] [q1 q2]]", got)
		}
		if resp.Provider != "typesafe" || resp.Model != "jev-2" || len(resp.Answers) != 3 {
			t.Errorf("after the move: %+v", resp)
		}
		if len(report.Hits) != 0 || !reflect.DeepEqual(report.Regraded, []string{"q1", "q2"}) {
			t.Errorf("hits %v regraded %v, want none and [q1 q2]", report.Hits, report.Regraded)
		}
		if entry, _, _ := store.Get(ctx, decision.CacheKeyFor(q1, "diff-A", "v1")); entry.Model != "jev-2" {
			t.Errorf("q1 entry stamped %q, want jev-2", entry.Model)
		}
		// ...and the new stamp now hits.
		if _, report, _ = c.DecideReport(ctx, decision.Request{State: "diff-A", Questions: []decision.Question{q1}}); len(report.Hits) != 1 {
			t.Errorf("after re-grade: hits %v, want [q1]", report.Hits)
		}

		// 6. An entry stamped with the old model is a miss once the stamp
		// has moved, even with no other miss in the call to reveal it: q3
		// was stored under jev-1 in step 2 and is asked again under jev-2,
		// never served stale.
		inner.takeAsked()
		resp, report, err = c.DecideReport(ctx, decision.Request{State: "diff-A", Questions: []decision.Question{q3}})
		if err != nil {
			t.Fatal(err)
		}
		if got := inner.takeAsked(); !reflect.DeepEqual(got, [][]string{{"q3"}}) {
			t.Errorf("inner asked %v, want [[q3]]: a jev-1 entry was served under jev-2", got)
		}
		if len(report.Hits) != 0 || !reflect.DeepEqual(report.Misses, []string{"q3"}) || resp.Model != "jev-2" {
			t.Errorf("hits %v misses %v model %q, want a jev-2 miss for q3", report.Hits, report.Misses, resp.Model)
		}
		if len(observed) != 7 {
			t.Errorf("Observe called %d times, want 7", len(observed))
		}
	})

	t.Run("an entry under another model is a miss for a fresh Cached", func(t *testing.T) {
		store := decision.NewMemoryCacheStore()
		old := &cachedInner{model: "jev-1"}
		c1, _ := decision.NewCached(old, decision.CachedOptions{Store: store})
		if _, err := c1.Decide(ctx, decision.Request{State: "s", Questions: []decision.Question{q1}}); err != nil {
			t.Fatal(err)
		}
		better := &cachedInner{model: "jev-2"}
		c2, _ := decision.NewCached(better, decision.CachedOptions{Store: store})
		resp, report, err := c2.DecideReport(ctx, decision.Request{State: "s", Questions: []decision.Question{q1}})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Hits) != 0 || resp.Model != "jev-2" {
			t.Errorf("hits %v model %q, want a jev-2 miss", report.Hits, resp.Model)
		}
	})

	t.Run("invalid answers and inner errors are never cached", func(t *testing.T) {
		store := decision.NewMemoryCacheStore()
		inner := &cachedInner{model: "jev-1", invalid: map[string]bool{"q2": true}}
		c, _ := decision.NewCached(inner, decision.CachedOptions{Store: store})
		resp, err := c.Decide(ctx, decision.Request{State: "s", Questions: []decision.Question{q1, q2}})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Invalid["q2"] != "out of range" || store.Len() != 1 {
			t.Errorf("Invalid %v, store holds %d; want q2 invalid and only q1 stored", resp.Invalid, store.Len())
		}
		boom := errors.New("boom")
		inner.fail = boom
		if _, err := c.Decide(ctx, decision.Request{State: "t", Questions: []decision.Question{q1}}); !errors.Is(err, boom) {
			t.Errorf("err = %v, want boom", err)
		}
		if store.Len() != 1 {
			t.Errorf("store holds %d after an error, want 1", store.Len())
		}
	})

	t.Run("store failures are disclosed and never fail the call", func(t *testing.T) {
		inner := &cachedInner{model: "jev-1"}
		c, _ := decision.NewCached(inner, decision.CachedOptions{Store: failingStore{}})
		req := decision.Request{State: "s", Questions: []decision.Question{q1}}
		// Cold, the stamp is unknown, so nothing is looked up: only the
		// put fails. Warm, the get fails too and reads as a miss.
		for i, wantErrs := range []int{1, 2} {
			resp, report, err := c.DecideReport(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			if len(resp.Answers) != 1 || len(report.StoreErrors) != wantErrs || len(report.Misses) != 1 {
				t.Errorf("call %d: answers %d, misses %v, store errors %v; want 1 answer, 1 miss and %d failures", i, len(resp.Answers), report.Misses, report.StoreErrors, wantErrs)
			}
		}
	})
}

// failingStore fails every Get and Put.
type failingStore struct{}

func (failingStore) Get(context.Context, decision.CacheKey) (decision.CacheEntry, bool, error) {
	return decision.CacheEntry{}, false, errors.New("store down")
}

func (failingStore) Put(context.Context, decision.CacheKey, decision.CacheEntry) error {
	return errors.New("store down")
}

func TestNewCachedRequiresInnerAndStore(t *testing.T) {
	if _, err := decision.NewCached(nil, decision.CachedOptions{Store: decision.NewMemoryCacheStore()}); err == nil {
		t.Error("nil inner: want an error")
	}
	if _, err := decision.NewCached(decision.None{}, decision.CachedOptions{}); err == nil {
		t.Error("nil Store: want an error")
	}
}

func TestCachedDeclaresItsInnerCapabilities(t *testing.T) {
	c, _ := decision.NewCached(decision.None{}, decision.CachedOptions{Store: decision.NewMemoryCacheStore()})
	if got, want := decision.CapabilitiesOf(c), decision.CapabilitiesOf(decision.None{}); !reflect.DeepEqual(got, want) {
		t.Errorf("CapabilitiesOf(cached) = %+v, want the inner's %+v", got, want)
	}
}

func TestCachedConcurrentUse(t *testing.T) {
	inner := &cachedInner{model: "jev-1"}
	store := decision.NewMemoryCacheStore()
	c, err := decision.NewCached(inner, decision.CachedOptions{Store: store, Observe: func(decision.CachedReport) {}})
	if err != nil {
		t.Fatal(err)
	}
	const n = 32
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := decision.Request{State: fmt.Sprintf("s%d", i%4), Questions: []decision.Question{decorQuestion("q1"), decorQuestion("q2")}}
			resp, err := c.Decide(context.Background(), req)
			if err != nil {
				errs <- err
				return
			}
			if len(resp.Answers) != 2 || resp.Model != "jev-1" {
				errs <- fmt.Errorf("goroutine %d: %+v", i, resp)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if store.Len() != 8 {
		t.Errorf("store holds %d entries, want 8 (4 states x 2 templates)", store.Len())
	}
}
