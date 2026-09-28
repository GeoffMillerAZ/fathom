package tsengine

import (
	"context"
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func testPool(t *testing.T, max int) (*parserPool, *fakeClock) {
	t.Helper()
	clk := &fakeClock{t: time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)}
	p := newParserPool(max, 2*time.Minute, clk.now)
	p.noTimer = true
	t.Cleanup(p.closeAll)
	return p, clk
}

func acquire(t *testing.T, p *parserPool, key string) *Lease {
	t.Helper()
	l, err := p.acquire(context.Background(), key, "python", "tree_sitter_python")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// TestParserPoolLRUCap (LB-F15, design §9): the pool never holds more than
// its cap of live modules; acquiring past it closes the least recently
// used idle module, and a released lease is reused rather than
// re-instantiated.
func TestParserPoolLRUCap(t *testing.T) {
	p, clk := testPool(t, 2)
	base := LiveModules()

	a := acquire(t, p, "a")
	pa := a.Parser()
	a.Release()
	clk.t = clk.t.Add(time.Second)
	b := acquire(t, p, "b")
	b.Release()
	if got := LiveModules() - base; got != 2 {
		t.Fatalf("live %d, want 2", got)
	}
	clk.t = clk.t.Add(time.Second)
	a2 := acquire(t, p, "a")
	if a2.Parser() != pa {
		t.Fatal("a released parser was not reused")
	}
	a2.Release()
	clk.t = clk.t.Add(time.Second)
	c := acquire(t, p, "c") // b is now the least recently used
	c.Release()
	if got := LiveModules() - base; got != 2 {
		t.Fatalf("live %d after a third key, want the cap 2", got)
	}
	if p.has("b") || !p.has("a") || !p.has("c") {
		t.Fatalf("LRU evicted the wrong module: a=%v b=%v c=%v", p.has("a"), p.has("b"), p.has("c"))
	}
}

// Leased modules are never closed under their holder; past the cap the
// surplus is closed as soon as it is released.
func TestParserPoolLeasedNeverEvicted(t *testing.T) {
	p, _ := testPool(t, 2)
	base := LiveModules()
	a, b := acquire(t, p, "a"), acquire(t, p, "b")
	c := acquire(t, p, "c") // all leased: a transient third module
	if got := LiveModules() - base; got != 3 {
		t.Fatalf("live %d with three leases, want 3", got)
	}
	if _, err := a.Parser().Parse([]byte("x = 1\n")); err != nil {
		t.Fatalf("a leased parser was closed under its holder: %v", err)
	}
	c.Release()
	if got := LiveModules() - base; got != 2 {
		t.Fatalf("live %d after releasing the surplus, want 2", got)
	}
	a.Release()
	b.Release()
}

func TestParserPoolIdleClose(t *testing.T) {
	p, clk := testPool(t, 6)
	base := LiveModules()
	a := acquire(t, p, "a")
	a.Release()
	b := acquire(t, p, "b") // still leased: not idle
	clk.t = clk.t.Add(2*time.Minute + time.Second)
	p.sweep()
	if got := LiveModules() - base; got != 1 {
		t.Fatalf("live %d after the idle sweep, want 1 (the leased one)", got)
	}
	b.Release()
	clk.t = clk.t.Add(time.Minute)
	p.sweep()
	if got := LiveModules() - base; got != 1 {
		t.Fatalf("b closed after one idle minute: live %d", got)
	}
	clk.t = clk.t.Add(2 * time.Minute)
	p.sweep()
	if got := LiveModules() - base; got != 0 {
		t.Fatalf("live %d after b idled two minutes, want 0", got)
	}
}

func TestParserPoolDiscardAndCloseOwner(t *testing.T) {
	p, _ := testPool(t, 6)
	base := LiveModules()
	a := acquire(t, p, "x|a")
	a.Discard() // an engine fault: the module is closed, never reused
	if got := LiveModules() - base; got != 0 {
		t.Fatalf("a discarded module is still live: %d", got)
	}
	b := acquire(t, p, "x|b")
	b.Release()
	c := acquire(t, p, "x|c") // held while its owner closes
	d := acquire(t, p, "y|d")
	d.Release()
	p.closePrefix("x|")
	if got := LiveModules() - base; got != 2 {
		t.Fatalf("live %d after closing owner x, want 2 (x's leased c, y's d)", got)
	}
	c.Release() // its owner closed: released means closed
	if got := LiveModules() - base; got != 1 {
		t.Fatalf("live %d after releasing a closed owner's lease, want 1", got)
	}
	if !p.has("y|d") {
		t.Fatal("closing owner x closed owner y's module")
	}
}

// Values cached on a parser (a compiled query) die with its module.
func TestParserCachedValues(t *testing.T) {
	p, _ := testPool(t, 6)
	l := acquire(t, p, "q")
	q, err := l.Parser().CompileQuery("(identifier) @id")
	if err != nil {
		t.Fatal(err)
	}
	l.Parser().SetCached("tags", q)
	l.Release()
	l2 := acquire(t, p, "q")
	if l2.Parser().Cached("tags") != q {
		t.Fatal("a reused parser lost its cached query")
	}
	l2.Discard()
	l3 := acquire(t, p, "q")
	defer l3.Release()
	if l3.Parser().Cached("tags") != nil {
		t.Fatal("a fresh parser carried a dead module's query")
	}
}
