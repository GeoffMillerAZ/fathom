package tsengine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/sys"
)

// compiledGrammar is one grammar's compiled machine code, shared by every
// module instantiated from it. refs counts parsers being instantiated from
// it right now; an evicted grammar's code is closed when refs reaches zero
// (wazero lets modules already instantiated outlive their compiled module).
type compiledGrammar struct {
	cm      wazero.CompiledModule
	mu      sync.Mutex
	refs    int
	evicted bool
	closed  bool
}

func (cg *compiledGrammar) evict() {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	cg.evicted = true
	cg.closeLocked()
}

func (cg *compiledGrammar) closeLocked() {
	if cg.evicted && cg.refs == 0 && !cg.closed {
		cg.closed = true
		_ = cg.cm.Close(context.Background())
	}
}

// release ends one instantiation's hold (see compiledCache.acquire).
func (cg *compiledGrammar) release() {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	cg.refs--
	cg.closeLocked()
}

func (cg *compiledGrammar) isClosed() bool {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	return cg.closed
}

// compiledCache holds compiled grammars as a memo group (LB-F15, design §9,
// I51): each is charged to the registry's byte budget at its lock row's
// blob_bytes, so the shed trim evicts the least recently used compiled
// grammars and the refuse drop evicts them all. The charge is the module
// size, not the machine code's: wazero does not report the compiled size,
// and measured on arm64 (wazero v1.12.0) the code is 0.46x-1.84x the module
// (javascript 844,395 / 458,741 bytes; typescript 943,867 / 2,037,040).
type compiledCache struct {
	g *memo.Group[string, *compiledGrammar]
}

func newCompiledCache(reg *memo.Registry) *compiledCache {
	g := memo.NewGroupIn[string, *compiledGrammar](reg, "grammar_code", memo.Options{})
	g.SetOnEvict(func(_ string, cg *compiledGrammar) { cg.evict() })
	return &compiledCache{g: g}
}

var defaultCompiled = newCompiledCache(memo.Default)

// acquire returns name's compiled grammar, compiling it on first use (the
// blob is decompressed then — openGrammarBlob), held until release. A
// compiled grammar the registry did not keep (over the limit in force) is
// closed at its release.
func (c *compiledCache) acquire(ctx context.Context, name string) (*compiledGrammar, error) {
	for attempt := 0; attempt < 3; attempt++ {
		cg, err := c.g.Get(ctx, name, func(ctx context.Context) (*compiledGrammar, int64, error) {
			lock, err := embeddedLock()
			if err != nil {
				return nil, 0, err
			}
			row, ok := lock.Row(name)
			if !ok {
				return nil, 0, fmt.Errorf("tsengine: unknown grammar %q (no grammars.lock.json row)", name)
			}
			r, err := ensureRuntime(ctx)
			if err != nil {
				return nil, 0, err
			}
			buf, err := openGrammarBlob(name)
			if err != nil {
				return nil, 0, err
			}
			cm, err := r.CompileModule(ctx, buf)
			if err != nil {
				return nil, 0, fmt.Errorf("tsengine: compile %q: %w", name, err)
			}
			return &compiledGrammar{cm: cm}, row.BlobBytes, nil
		})
		if err != nil {
			return nil, err
		}
		cg.mu.Lock()
		if cg.closed {
			cg.mu.Unlock()
			continue // evicted and closed between Get and here: fetch again
		}
		cg.refs++
		if cur, ok := c.g.Peek(name); !ok || cur != cg {
			cg.evicted = true // not cached: close at release
		}
		cg.mu.Unlock()
		return cg, nil
	}
	return nil, fmt.Errorf("tsengine: compile %q: evicted repeatedly while being fetched", name)
}

// dropCompiled evicts every compiled grammar (tests): the next parser for a
// grammar recompiles it.
func dropCompiled() { defaultCompiled.g.Reset(memo.ReasonIdle) }

// wazeroModuleConfig gives every guest a real monotonic clock (94d9d412): budget.c's
// deadline checks read it, and wazero's default fake clock ticks 1ms per read, which
// turned every ParseTimeout into a count of progress callbacks. Tests inject
// guestNanotime to make the cancel point deterministic.
func wazeroModuleConfig() wazero.ModuleConfig {
	cfg := wazero.NewModuleConfig().WithName("")
	if guestNanotime != nil {
		return cfg.WithNanotime(guestNanotime, sys.ClockResolution(time.Millisecond))
	}
	return cfg.WithSysNanotime()
}
