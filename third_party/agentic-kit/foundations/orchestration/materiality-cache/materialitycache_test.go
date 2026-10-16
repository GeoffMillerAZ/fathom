package materialitycache_test

import (
	"bytes"
	"testing"
	"time"

	mc "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/orchestration/materiality-cache"
)

// ---- helpers ---------------------------------------------------------------

func deterministicCache() *mc.Cache {
	epoch := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return mc.New(func() time.Time { return epoch })
}

// ---- Get: miss on empty cache ---------------------------------------------

func TestCache_Get_MissOnEmpty(t *testing.T) {
	c := deterministicCache()
	res := c.Get("nonexistent")
	if res.Hit {
		t.Error("expected miss, got hit")
	}
}

// ---- Put then Get: hit ----------------------------------------------------

func TestCache_PutGet_Hit(t *testing.T) {
	c := deterministicCache()
	c.Put("key-a", []byte("hello"), "fp-a")

	res := c.Get("key-a")
	if !res.Hit {
		t.Fatal("expected hit, got miss")
	}
	if !bytes.Equal(res.Entry.Value, []byte("hello")) {
		t.Errorf("unexpected value: %q", res.Entry.Value)
	}
	if res.Entry.Fingerprint != "fp-a" {
		t.Errorf("unexpected fingerprint: %q", res.Entry.Fingerprint)
	}
	if res.Entry.K != "key-a" {
		t.Errorf("unexpected key echo: %q", res.Entry.K)
	}
}

// ---- Put overwrites existing entry ----------------------------------------

func TestCache_Put_Overwrites(t *testing.T) {
	c := deterministicCache()
	c.Put("k", []byte("v1"), "fp1")
	c.Put("k", []byte("v2"), "fp2")

	res := c.Get("k")
	if !bytes.Equal(res.Entry.Value, []byte("v2")) {
		t.Errorf("expected v2, got %q", res.Entry.Value)
	}
	if res.Entry.Fingerprint != "fp2" {
		t.Errorf("expected fp2, got %q", res.Entry.Fingerprint)
	}
}

// ---- Invalidate: hit becomes miss ----------------------------------------

func TestCache_Invalidate_BecomesMiss(t *testing.T) {
	c := deterministicCache()
	c.Put("k", []byte("v"), "fp")

	if !c.Get("k").Hit {
		t.Fatal("expected hit before invalidate")
	}
	c.Invalidate("k")
	if c.Get("k").Hit {
		t.Error("expected miss after invalidate")
	}
}

// ---- Invalidate absent key is no-op ---------------------------------------

func TestCache_Invalidate_AbsentKeyNoop(t *testing.T) {
	c := deterministicCache()
	// Should not panic.
	c.Invalidate("does-not-exist")
	if c.Len() != 0 {
		t.Errorf("expected 0 entries, got %d", c.Len())
	}
}

// ---- Len tracks Put and Invalidate ----------------------------------------

func TestCache_Len(t *testing.T) {
	c := deterministicCache()
	if c.Len() != 0 {
		t.Errorf("expected 0, got %d", c.Len())
	}
	c.Put("a", nil, "")
	c.Put("b", nil, "")
	if c.Len() != 2 {
		t.Errorf("expected 2, got %d", c.Len())
	}
	c.Invalidate("a")
	if c.Len() != 1 {
		t.Errorf("expected 1, got %d", c.Len())
	}
}

// ---- StoredAt is set by the injected clock --------------------------------

func TestCache_StoredAt_Deterministic(t *testing.T) {
	epoch := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	c := mc.New(func() time.Time { return epoch })
	c.Put("k", []byte("v"), "")
	res := c.Get("k")
	if !res.Entry.StoredAt.Equal(epoch) {
		t.Errorf("expected StoredAt %v, got %v", epoch, res.Entry.StoredAt)
	}
}

// ---- Nil now param defaults to time.Now -----------------------------------

func TestCache_NilNow_Defaults(t *testing.T) {
	c := mc.New(nil)
	before := time.Now().UTC().Add(-time.Second)
	c.Put("k", []byte("v"), "")
	after := time.Now().UTC().Add(time.Second)
	res := c.Get("k")
	if res.Entry.StoredAt.Before(before) || res.Entry.StoredAt.After(after) {
		t.Errorf("StoredAt %v out of expected range [%v, %v]", res.Entry.StoredAt, before, after)
	}
}

// ---- Returned Value is a copy (mutation safety) --------------------------

func TestCache_Get_ReturnsCopy(t *testing.T) {
	c := deterministicCache()
	orig := []byte("original")
	c.Put("k", orig, "")

	res := c.Get("k")
	res.Entry.Value[0] = 'X'

	res2 := c.Get("k")
	if res2.Entry.Value[0] != 'o' {
		t.Error("mutating returned value changed cache internals")
	}
}

// ---- Multiple keys: independent entries ----------------------------------

func TestCache_MultipleKeys_Independent(t *testing.T) {
	c := deterministicCache()
	c.Put("alpha", []byte("a"), "fa")
	c.Put("beta", []byte("b"), "fb")
	c.Put("gamma", []byte("c"), "fc")

	c.Invalidate("beta")

	if !c.Get("alpha").Hit {
		t.Error("alpha should still be present")
	}
	if c.Get("beta").Hit {
		t.Error("beta should be invalidated")
	}
	if !c.Get("gamma").Hit {
		t.Error("gamma should still be present")
	}
	if c.Len() != 2 {
		t.Errorf("expected 2 entries, got %d", c.Len())
	}
}

// ---- Put with nil value --------------------------------------------------

func TestCache_Put_NilValue(t *testing.T) {
	c := deterministicCache()
	c.Put("nil-val", nil, "")
	res := c.Get("nil-val")
	if !res.Hit {
		t.Fatal("expected hit for nil-value entry")
	}
	if res.Entry.Value != nil {
		t.Errorf("expected nil value, got %v", res.Entry.Value)
	}
}
