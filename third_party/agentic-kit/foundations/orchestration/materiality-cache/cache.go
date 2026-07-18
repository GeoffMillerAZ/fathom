package materialitycache

import "time"

// Cache is a deterministic in-memory cache keyed by materiality key.
// The zero-value is not valid; use New.
type Cache struct {
	store map[Key]Entry
	now   func() time.Time
}

// New returns an initialised Cache. If now is nil, time.Now is used.
func New(now func() time.Time) *Cache {
	if now == nil {
		now = time.Now
	}
	return &Cache{
		store: make(map[Key]Entry),
		now:   now,
	}
}

// Get returns the cached entry for key, or a miss result if the key is absent
// or has been invalidated. The returned Entry.Value is a copy; mutating it
// does not affect the cache's internal state.
func (c *Cache) Get(key Key) GetResult {
	entry, ok := c.store[key]
	if !ok {
		return GetResult{Hit: false}
	}
	cp := entry
	cp.Value = copyBytes(entry.Value)
	return GetResult{Hit: true, Entry: cp}
}

// Put stores value under key. If the key already has an entry it is
// overwritten. StoredAt is set to the cache's current time.
func (c *Cache) Put(key Key, value []byte, fingerprint string) {
	c.store[key] = Entry{
		K:           key,
		Value:       copyBytes(value),
		StoredAt:    c.now().UTC(),
		Fingerprint: fingerprint,
	}
}

// Invalidate removes key from the cache. After Invalidate, Get(key) returns a
// miss. Invalidating an absent key is a no-op.
func (c *Cache) Invalidate(key Key) {
	delete(c.store, key)
}

// Len returns the number of entries currently in the cache.
func (c *Cache) Len() int {
	return len(c.store)
}

// copyBytes returns a new slice with the same bytes as b.
func copyBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp
}
