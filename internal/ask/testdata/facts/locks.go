package facts

import "sync"

type counter struct {
	name string // declared before mu: not guarded
	mu   sync.Mutex
	n    int
	hits map[string]int
}

// want: unchecked=- wrapped=- executes_only=- locked=yes threaded=-
// A deferred Unlock is not between the Lock and the write.
func (c *counter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

// want: unchecked=- wrapped=- executes_only=- locked=no threaded=-
func (c *counter) IncLate() {
	c.n++
	c.mu.Lock()
	c.mu.Unlock()
}

// want: unchecked=- wrapped=- executes_only=- locked=no threaded=-
// The index write on a guarded field comes after the Unlock.
func (c *counter) IncAfterUnlock() {
	c.mu.Lock()
	c.n = 1
	c.mu.Unlock()
	c.hits["x"] = 2
}

// want: unchecked=- wrapped=- executes_only=- locked=- threaded=-
// Tricky negative: name is declared before mu, so it is not guarded.
func (c *counter) Rename(s string) {
	c.name = s
}

// want: unchecked=- wrapped=- executes_only=- locked=- threaded=-
func (c *counter) Get() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// want: unchecked=- wrapped=- executes_only=- locked=no threaded=-
// Tricky negative: the lock taken is another value's mutex.
func (c *counter) LocksOther(o *counter) {
	o.mu.Lock()
	c.n = 2
	o.mu.Unlock()
}

type table struct {
	mu   sync.RWMutex
	rows []string
}

// want: unchecked=- wrapped=- executes_only=- locked=no threaded=-
// RLock is not enough for a write.
func (t *table) AddUnderRLock(s string) {
	t.mu.RLock()
	t.rows = append(t.rows, s)
	t.mu.RUnlock()
}

// want: unchecked=- wrapped=- executes_only=- locked=yes threaded=-
func (t *table) Add(s string) {
	t.mu.Lock()
	t.rows = append(t.rows, s)
	t.mu.Unlock()
}

// want: unchecked=- wrapped=- executes_only=- locked=- threaded=-
// The only write is inside a function literal, which is ignored.
func (t *table) AddLater(s string) func() {
	return func() { t.rows = append(t.rows, s) }
}

type embedded struct {
	sync.Mutex
	v int
}

// want: unchecked=- wrapped=- executes_only=- locked=yes threaded=-
// An embedded mutex: the promoted Lock.
func (e *embedded) Set(v int) {
	e.Lock()
	e.v = v
	e.Unlock()
}

// want: unchecked=- wrapped=- executes_only=- locked=- threaded=-
// Not a method: the fact applies to a receiver's fields only.
func setCounter(c *counter) {
	c.n = 3
}

type plain struct{ a, b int }

// want: unchecked=- wrapped=- executes_only=- locked=- threaded=-
func (p *plain) Set() {
	p.a = 1
}
