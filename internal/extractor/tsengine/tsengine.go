// Package tsengine is a pure-Go tree-sitter execution engine: tree-sitter's
// C core runtime, statically linked with one grammar's C parser per
// language, compiled to a self-contained wasm32-wasi module and executed via
// wazero (github.com/tetratelabs/wazero) — no CGO, no Emscripten dynamic
// linking, no Docker at runtime (ADR-021 §4, ADR-004; see grammars/
// PROVENANCE.md for exactly how the blobs are built and why this shape was
// chosen over the Emscripten/web-tree-sitter one).
//
// tsengine knows nothing about fathom's domain model (core.SymbolNode, …) —
// it is a thin, generic wrapper exposing tree-sitter's parse + query C API.
// The fathom-specific mapping from query captures to symbols lives in
// internal/extractor's TSExtractor (NewTSExtractor).
//
// Concurrency: mirrors the existing CGO extractors' discipline (see
// internal/extractor/treesitter.WithTree) — a *Grammar is compiled once and
// is safe to share, but each Parser owns its own wazero module instance (its
// own linear memory), so callers should create one Parser per goroutine /
// per Extract call rather than share a live Parser across goroutines.
package tsengine

import (
	"context"
	"embed"
	"encoding/binary"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/sys"
)

// DefaultParseTimeout bounds each potentially-unbounded guest phase (the
// parse, and the query match scan). Chosen far above any healthy corpus
// file's cost, so it only ever fires on genuinely pathological input —
// e.g. `({[` x 50,000 (150KB, a truncated minified bundle's shape), which
// ran 52.9s unbounded on this engine (x200,000: 11+ minutes at 100% CPU;
// the CGO incumbent finishes the same bytes in ~49ms — critic B1). Measured
// attribution: the PARSE of that input completes in ~70ms; the 45s+ goes to
// the QUERY CURSOR's scan of the resulting error tree (see grammars/
// PROVENANCE.md's runtime-behavior note). Without a bound, one such file
// wedges the daemon: uncancellable wasm execution holding the extractor's
// mutex, serializing that language's extraction until restart.
//
// Enforcement is GUEST-side: the blobs export fathom_parse_bounded and
// fathom_query_cursor_exec_bounded (grammars/budget.c), which run
// tree-sitter's own progress-callback cancellation (one CLOCK_MONOTONIC
// read per ~100 operations, via WASI clock_time_get; ~2% corpus cost). The
// host-side alternative — wazero's WithCloseOnContextDone — was implemented
// first and rejected: its termination checks on every loop back-edge
// measured ~3x slower across the whole calibration corpus, busting the
// plan's 1.5x ingest budget for a deadline that pathological files alone
// need. ParseHardBackstop covers what the guest budgets structurally cannot.
const DefaultParseTimeout = 10 * time.Second

// ParseTimeout is the live parse deadline. A package-level variable (not a
// constant) solely so tests can shrink it to exercise the timeout path
// without a 10s wait; production code must not mutate it. Set it before any
// parsing starts — it is read (unsynchronized) at each Parse.
var ParseTimeout = DefaultParseTimeout

// guestNanotime is the monotonic clock budget.c's deadline checks read (WASI
// clock_time_get). Nil — production — is the host's real clock
// (WithSysNanotime). It MUST be set explicitly: wazero's default monotonic
// clock is a FAKE that advances 1ms per reading, under which ParseTimeout
// silently meant "ParseTimeout/1ms progress callbacks" (x100 tree-sitter
// operations) rather than elapsed time — a 300ms budget became ~30,000 query
// ops, ~8s of CPU on the pathological tree (more under load), and the 10s
// default ~1,000,000 ops. Tests inject a deterministic clock here to assert
// the cancel in operations, independent of machine load. Read at NewParser.
var guestNanotime sys.Nanotime

// ParseHardBackstop is the wall-clock bound on each potentially-unbounded
// guest operation (the parse call, and the whole query match loop),
// enforced host-side by abandoning the in-flight wasm work (goroutine +
// module reaper — see bounded). Zero means 2x ParseTimeout, computed at
// each operation.
//
// Why a second bound exists on top of the guest budgets (measured, not
// speculative): the guest budgets only fire when tree-sitter's progress
// callbacks actually run, and their real-time cadence collapses on exactly
// the pathological inputs — during the fix, a 16x-amortized clock check let
// the full 45s pathological query scan run to completion because callbacks
// arrived ~8s apart under load, and a phase whose work runs between checks
// (or a future core whose cadence shifts) would make a guest-only budget
// decorative. The guest budgets are the cheap, surgical first line; this
// backstop guarantees the operation returns within a fixed bound regardless
// of where the time goes. An abandoned call keeps burning one core in the
// background until tree-sitter finishes (bounded by the 2MB extraction
// cap's worst case), after which the reaper closes its module and frees
// everything — degraded and counted, never a wedged daemon.
var ParseHardBackstop time.Duration

// grammarFS holds the compressed blobs and the lock (LB-F12, LB-F13). A
// blob is decompressed only when its grammar is first compiled
// (openGrammarBlob); the lock names every blob the engine will load.
//
//go:embed grammars/*.wasm.gz grammars/grammars.lock.json
var grammarFS embed.FS

// Sizeof constants for tree-sitter's public C structs (tree_sitter/api.h).
// These are part of tree-sitter's stable embedder ABI, not an implementation
// detail of one build — see grammars/PROVENANCE.md for how this was
// verified.
const (
	nodeSize         = 24 // TSNode:  context[4](16) + id(4) + tree(4), wasm32 pointers
	queryMatchSize   = 12 // TSQueryMatch: id(4) + pattern_index(2) + capture_count(2) + captures ptr(4)
	queryCaptureSize = 28 // TSQueryCapture: TSNode(24) + index(4)
)

// MemoryLimitPages caps every grammar module's linear memory (RT-107,
// docs/design/responsible-tenant.md §7.2): 1024 pages of 64 KiB = 64 MiB.
// A parse that would grow past it fails inside the guest — an engine fault,
// counted and disclosed like any other — instead of growing without bound.
const MemoryLimitPages = 1024

// liveModules counts instantiated grammar modules not yet closed (RT-107):
// the regression test's instrument for a module leak.
var liveModules atomic.Int64

// LiveModules is the number of grammar modules instantiated and not yet
// closed in this process.
func LiveModules() int64 { return liveModules.Load() }

// ownedModules counts the live modules a Parser still owns: liveModules
// less the abandoned ones (see bounded) whose reaper has not closed them
// yet. An abandoned runaway burns out on its own schedule — 45 s and more,
// minutes on a loaded 4-vCPU machine — so its reaper's close can land in
// the middle of later, unrelated work; a leak check that brackets work
// counts owned modules, not live ones (CS-8: a full `task test` in a cloud
// VM shape saw TestPolyglotIngestHoldsModulePool read "-1 left open" when
// TestEngine_HardBackstopAbandonsRunawayCall's runaway was reaped mid-test).
var ownedModules atomic.Int64

// instantiations counts every grammar module instantiated in this process,
// monotonically.
var instantiations atomic.Int64

// Instantiations is the number of grammar modules this process has ever
// instantiated: the language-breadth harness re-times a file whose
// extraction instantiated one, so a lazy compile is never charged to
// extraction throughput (design §9, §10).
func Instantiations() int64 { return instantiations.Load() }

// peakModulePages is the largest linear memory (64 KiB pages) any grammar
// module reached after a parse in this process — the language-breadth
// reading's peak_module_pages (design §9, §10).
var peakModulePages atomic.Uint32

// PeakModulePages returns the largest module memory, in 64 KiB pages,
// observed after any parse since the last ResetPeakModulePages.
func PeakModulePages() uint32 { return peakModulePages.Load() }

// ResetPeakModulePages zeroes the peak (a harness measuring one dialect).
func ResetPeakModulePages() { peakModulePages.Store(0) }

func notePages(pages uint32) {
	for {
		cur := peakModulePages.Load()
		if pages <= cur || peakModulePages.CompareAndSwap(cur, pages) {
			return
		}
	}
}

var (
	runtimeOnce sync.Once
	rt          wazero.Runtime
	rtErr       error
)

// ensureRuntime lazily creates the single process-wide wazero Runtime and
// instantiates the WASI shim every grammar module imports (fd_write/
// fd_close from libc startup, clock_time_get for budget.c's deadline checks
// — see PROVENANCE.md; nothing custom to implement). The compilation cache
// in force at that moment (EnableCompilationCache, LB-F26) is fixed for the
// process. Safe to call concurrently.
func ensureRuntime(ctx context.Context) (wazero.Runtime, error) {
	runtimeOnce.Do(func() {
		cacheMu.Lock()
		runtimeMade = true
		dir := cacheDir
		cacheMu.Unlock()
		rt, _, rtErr = newRuntime(ctx, dir)
	})
	return rt, rtErr
}

// compilerSupported reports whether wazero's ahead-of-time compiler engine
// (native codegen, much faster than the interpreter) supports this
// GOOS/GOARCH. Falls back to the portable interpreter engine otherwise —
// correctness never depends on which engine runs the module.
func compilerSupported() bool {
	switch runtime.GOOS {
	case "linux", "darwin", "windows", "freebsd", "android":
	default:
		return false
	}
	switch runtime.GOARCH {
	case "amd64", "arm64":
		return true
	default:
		return false
	}
}

// Parser is one instantiated grammar module (its own linear memory) bound to
// one language entry point, plus a live TSParser. Not safe for concurrent
// use from multiple goroutines — create one per goroutine.
//
// closed is atomic because abandonment (see bounded) is the one place two
// goroutines interact: the caller marks the Parser closed while the
// abandoned op goroutine may be about to make its next guest call — the
// closed check in call makes that goroutine stop entering the module, after
// which the reaper closes it. All OTHER use is single-goroutine.
type Parser struct {
	ctx       context.Context
	mod       api.Module
	fns       map[string]*api.Function
	langPtr   uint32
	parserPtr uint32
	closed    atomic.Bool
	released  atomic.Bool // the module has been closed and uncounted
	// cached holds values that live inside this module (compiled queries)
	// for the Parser's owner; they die with it (Cached, SetCached).
	cached map[string]any
}

// Cached returns the value SetCached stored under key on this Parser, or
// nil. A pooled Parser's owner keeps its compiled queries here, so they are
// reused with the module and dropped when the pool closes it.
func (p *Parser) Cached(key string) any { return p.cached[key] }

// SetCached stores v under key on this Parser (see Cached).
func (p *Parser) SetCached(key string, v any) {
	if p.cached == nil {
		p.cached = map[string]any{}
	}
	p.cached[key] = v
}

// release uncounts the module once it is closed (RT-107).
func (p *Parser) release() {
	if !p.released.Swap(true) {
		liveModules.Add(-1)
	}
}

// requiredExports is every C function build.sh exports and this package
// calls. Resolved once per Parser at construction so later calls are a plain
// map lookup, and a missing export fails fast with a clear error instead of
// a nil-pointer panic mid-extraction.
var requiredExports = []string{
	"malloc", "free",
	"ts_parser_new", "ts_parser_delete", "ts_parser_set_language",
	"fathom_parse_bounded", "fathom_query_cursor_exec_bounded",
	"fathom_take_cancelled", "ts_parser_reset",
	"ts_tree_root_node", "ts_tree_delete",
	"fathom_node_end_byte",
	"ts_language_abi_version",
	"ts_query_new", "ts_query_delete",
	"ts_query_cursor_new", "ts_query_cursor_delete",
	"ts_query_cursor_exec", "ts_query_cursor_next_match",
	"ts_query_capture_name_for_id",
}

// probeExports are the node-walk functions build.sh exports that only the
// probe (Tree.SExpr, LB-F20) calls. Resolved when present, never required:
// extraction must not fail on a blob built without them.
var probeExports = []string{
	"ts_node_type", "ts_node_is_named", "ts_node_child", "ts_node_child_count",
	"ts_node_named_child_count", "ts_node_field_name_for_child",
}

// NewParser compiles (if needed) and instantiates the named grammar blob,
// resolves entryPoint (e.g. "tree_sitter_typescript") to get the TSLanguage,
// and creates a TSParser bound to it. Call Close when done — it frees the
// underlying wazero module (and with it every allocation this Parser made).
func NewParser(ctx context.Context, grammar, entryPoint string) (*Parser, error) {
	cg, err := defaultCompiled.acquire(ctx, grammar)
	if err != nil {
		return nil, err
	}
	r, err := ensureRuntime(ctx)
	if err != nil {
		cg.release()
		return nil, err
	}
	mod, err := r.InstantiateModule(ctx, cg.cm, wazeroModuleConfig())
	cg.release()
	if err != nil {
		return nil, fmt.Errorf("tsengine: instantiate %q: %w", grammar, err)
	}
	liveModules.Add(1)
	ownedModules.Add(1)
	instantiations.Add(1)
	fail := func(err error) (*Parser, error) {
		mod.Close(ctx)
		liveModules.Add(-1)
		ownedModules.Add(-1)
		return nil, err
	}

	p := &Parser{ctx: ctx, mod: mod, fns: make(map[string]*api.Function, len(requiredExports))}
	for _, name := range requiredExports {
		fn := mod.ExportedFunction(name)
		if fn == nil {
			return fail(fmt.Errorf("tsengine: grammar %q missing required export %q", grammar, name))
		}
		p.fns[name] = &fn
	}
	for _, name := range probeExports {
		if fn := mod.ExportedFunction(name); fn != nil {
			p.fns[name] = &fn
		}
	}

	entryFn := mod.ExportedFunction(entryPoint)
	if entryFn == nil {
		return fail(fmt.Errorf("tsengine: grammar %q missing entry point %q", grammar, entryPoint))
	}
	langRes, err := entryFn.Call(ctx)
	if err != nil {
		return fail(fmt.Errorf("tsengine: call %q: %w", entryPoint, err))
	}
	p.langPtr = uint32(langRes[0])

	parserRes, err := p.call("ts_parser_new")
	if err != nil {
		return fail(err)
	}
	p.parserPtr = uint32(parserRes[0])

	ok, err := p.call("ts_parser_set_language", uint64(p.parserPtr), uint64(p.langPtr))
	if err != nil {
		return fail(err)
	}
	if ok[0] == 0 {
		return fail(fmt.Errorf("tsengine: grammar %q: incompatible language ABI version", grammar))
	}
	return p, nil
}

// Close releases the wazero module instance (and every allocation it made).
// After an abandonment (see bounded) this is a no-op — the reaper owns the
// close.
func (p *Parser) Close() error {
	if p.closed.Swap(true) {
		return nil
	}
	ownedModules.Add(-1)
	err := p.mod.Close(p.ctx)
	p.release()
	return err
}

func (p *Parser) call(name string, args ...uint64) ([]uint64, error) {
	// The closed check makes an ABANDONED op goroutine (see bounded) stop
	// entering the module as soon as its in-flight guest call returns, and
	// turns the caller's deferred best-effort frees (Tree.Close, cursor
	// delete, free) into no-op errors instead of concurrent calls into a
	// module another goroutine is still executing in.
	if p.closed.Load() {
		return nil, fmt.Errorf("tsengine: call %s: parser closed", name)
	}
	fn, ok := p.fns[name]
	if !ok {
		return nil, fmt.Errorf("tsengine: internal error: export %q not resolved", name)
	}
	res, err := (*fn).Call(p.ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("tsengine: call %s: %w", name, err)
	}
	return res, nil
}

// bounded runs op under the ParseHardBackstop wall-clock bound, abandoning
// it (and this Parser) on expiry. op wraps the guest work whose runtime is
// a function of untrusted input — the parse call, and the whole query match
// loop (one bounded harness per file per phase, NOT per guest call: a
// goroutine+timer per next_match measured ~2x user-CPU overhead corpus-
// wide). On abandonment the op goroutine keeps running its in-flight guest
// call; the closed flag stops it at its NEXT p.call, and the reaper closes
// the module — releasing all its memory — once op returns. The op goroutine
// only touches op-local state plus construction-time-immutable Parser
// fields, so abandonment does not race with the caller.
func (p *Parser) bounded(what string, op func() (any, error)) (any, error) {
	backstop := ParseHardBackstop
	if backstop <= 0 {
		backstop = 2 * ParseTimeout
	}

	type opResult struct {
		val any
		err error
	}
	ch := make(chan opResult, 1)
	//fathom:tenancy-allow bare-go runs one wasm op against its deadline; the reaper closes the module when a timed-out op returns
	go func() {
		val, err := op()
		ch <- opResult{val: val, err: err}
	}()

	timer := time.NewTimer(backstop)
	defer timer.Stop()
	select {
	case r := <-ch:
		return r.val, r.err
	case <-timer.C:
		// Hard backstop: the guest work is stuck past its budget in a
		// progress-check-free phase. Abandon it — this Parser is dead to
		// callers from here on; the reaper closes the module when the op
		// goroutine finishes in the background.
		if !p.closed.Swap(true) {
			ownedModules.Add(-1) // the reaper's from here, no caller's
		}
		//fathom:tenancy-allow bare-go runs one wasm op against its deadline; the reaper closes the module when a timed-out op returns
		go func() {
			<-ch
			_ = p.mod.Close(context.Background())
			p.release()
		}()
		return nil, fmt.Errorf("tsengine: %s abandoned at %v hard backstop (guest %v budget did not fire — progress-check-free phase); parser must be re-created", what, backstop, ParseTimeout)
	}
}

func (p *Parser) malloc(n uint32) (uint32, error) {
	res, err := p.call("malloc", uint64(n))
	if err != nil {
		return 0, err
	}
	ptr := uint32(res[0])
	if ptr == 0 && n > 0 {
		return 0, fmt.Errorf("tsengine: malloc(%d) returned null", n)
	}
	return ptr, nil
}

func (p *Parser) free(ptr uint32) {
	if ptr == 0 {
		return
	}
	_, _ = p.call("free", uint64(ptr))
}

func (p *Parser) writeBytes(b []byte) (uint32, error) {
	ptr, err := p.malloc(uint32(len(b)))
	if err != nil {
		return 0, err
	}
	if len(b) > 0 && !p.mod.Memory().Write(ptr, b) {
		return 0, fmt.Errorf("tsengine: memory write of %d bytes at %d failed", len(b), ptr)
	}
	return ptr, nil
}

func (p *Parser) readBytes(ptr, n uint32) ([]byte, error) {
	b, ok := p.mod.Memory().Read(ptr, n)
	if !ok {
		return nil, fmt.Errorf("tsengine: memory read of %d bytes at %d failed", n, ptr)
	}
	// Memory.Read returns a view into wazero's live memory buffer; callers
	// that outlive the next allocation must copy it. Copy defensively here
	// since every caller in this package treats the result as immutable.
	out := make([]byte, n)
	copy(out, b)
	return out, nil
}

// Tree is a parsed syntax tree. Call Close when done.
type Tree struct {
	p       *Parser
	treePtr uint32
	src     []byte
}

// Parse parses src and returns the resulting Tree. Never returns a nil Tree
// without an error; a source tree-sitter cannot lex at all is vanishingly
// rare (tree-sitter's error-recovery grammar generation means almost any
// byte string produces *some* tree, possibly full of ERROR nodes) but is
// still surfaced as an error rather than a silent empty result.
//
// The parse runs under the ParseTimeout deadline (critic B1), enforced by
// the guest-side fathom_parse_bounded (grammars/budget.c): tree-sitter's
// progress callback checks CLOCK_MONOTONIC every ~100 parser operations and
// cancels the parse when the budget expires, returning a null tree. The
// Parser itself SURVIVES the abort (tree-sitter's documented cancellation
// contract requires only a reset before the next parse — which this method
// performs at entry anyway); the caller (internal/extractor.TSExtractor)
// still treats any Parse error as an engine fault and re-creates the Parser
// for uniformity with genuine traps.
func (p *Parser) Parse(src []byte) (*Tree, error) {
	// A Parser is reused across many files for the life of the process
	// (see internal/extractor.TSExtractor) — reset defensively before every
	// parse so a prior call that errored or was cancelled never leaks state
	// into this one. Cheap: tree-sitter's own recommended usage pattern for
	// a long-lived TSParser parsing many unrelated documents, and REQUIRED
	// after a cancelled parse.
	if _, err := p.call("ts_parser_reset", uint64(p.parserPtr)); err != nil {
		return nil, err
	}

	srcPtr, err := p.writeBytes(src)
	if err != nil {
		return nil, err
	}
	defer p.free(srcPtr)

	budgetMS := ParseTimeout.Milliseconds()
	if budgetMS < 1 {
		budgetMS = 1
	}

	resAny, err := p.bounded("parse", func() (any, error) {
		return p.call("fathom_parse_bounded", uint64(p.parserPtr), uint64(srcPtr), uint64(len(src)), uint64(budgetMS))
	})
	if err != nil {
		return nil, err
	}
	res := resAny.([]uint64)
	treePtr := uint32(res[0])
	if treePtr == 0 {
		// fathom_parse_bounded returns null exactly when the progress
		// callback cancelled the parse — the ParseTimeout budget expired
		// during a checked phase. The Parser survives (Parse resets at
		// entry).
		return nil, fmt.Errorf("tsengine: parse aborted after %v budget (pathological input?)", ParseTimeout)
	}
	notePages(p.mod.Memory().Size() / 65536)
	return &Tree{p: p, treePtr: treePtr, src: src}, nil
}

// Close releases the tree's wasm-side allocation.
func (t *Tree) Close() {
	_, _ = t.p.call("ts_tree_delete", uint64(t.treePtr))
}

// Root returns a lightweight snapshot of the tree's root node.
func (t *Tree) Root() (Node, error) {
	ptr, node, err := t.rootPtr()
	if ptr != 0 {
		defer t.p.free(ptr)
	}
	return node, err
}

// rootPtr materializes the tree's root TSNode struct in wasm memory and
// returns both the raw pointer (needed by ts_query_cursor_exec) and the
// lightweight Node snapshot. Caller owns freeing the returned pointer.
func (t *Tree) rootPtr() (uint32, Node, error) {
	sret, err := t.p.malloc(nodeSize)
	if err != nil {
		return 0, Node{}, err
	}
	if _, err := t.p.call("ts_tree_root_node", uint64(sret), uint64(t.treePtr)); err != nil {
		t.p.free(sret)
		return 0, Node{}, err
	}
	node, err := t.p.readNode(sret)
	if err != nil {
		t.p.free(sret)
		return 0, Node{}, err
	}
	return sret, node, nil
}

// Node is a lightweight, already-materialized snapshot of a tree-sitter
// node's position — just enough for symbol extraction by containment
// (start/end byte, start line). It does not keep the underlying wasm-side
// TSNode struct alive; re-fetch via a fresh query/traversal if more is
// needed later.
type Node struct {
	StartByte uint32
	EndByte   uint32
	StartRow  uint32 // 0-based; add 1 for a human line number
}

// Line returns the node's 1-based starting line number.
func (n Node) Line() int { return int(n.StartRow) + 1 }

// Text returns the node's source text, given the same src Parse was called
// with.
func (n Node) Text(src []byte) string {
	if n.EndByte <= n.StartByte || int(n.EndByte) > len(src) {
		return ""
	}
	return string(src[n.StartByte:n.EndByte])
}

// nodeFromBytes builds a Node snapshot from a TSNode struct's raw 24 bytes
// with exactly ONE guest call. start_byte and start_point are literally
// context[0..2] of the struct (node.c: ts_node_new / ts_node_start_byte),
// so they decode host-side for free; only the end byte lives behind the
// subtree pointer and needs the guest — fathom_node_end_byte takes the six
// words as scalars, so no guest-memory allocation is involved at all. This
// is THE hot path of extraction (once per query capture) — the previous
// shape (malloc + write + 3 accessor calls + free per capture) measurably
// busted the corpus ingest budget.
func (p *Parser) nodeFromBytes(b []byte) (Node, error) {
	c0 := binary.LittleEndian.Uint32(b[0:4])
	c1 := binary.LittleEndian.Uint32(b[4:8])
	c2 := binary.LittleEndian.Uint32(b[8:12])
	c3 := binary.LittleEndian.Uint32(b[12:16])
	id := binary.LittleEndian.Uint32(b[16:20])
	tree := binary.LittleEndian.Uint32(b[20:24])
	endRes, err := p.call("fathom_node_end_byte",
		uint64(c0), uint64(c1), uint64(c2), uint64(c3), uint64(id), uint64(tree))
	if err != nil {
		return Node{}, err
	}
	return Node{
		StartByte: c0,
		EndByte:   uint32(endRes[0]),
		StartRow:  c1,
	}, nil
}

// readNode reads a 24-byte TSNode struct at ptr (guest memory) and builds a
// Node snapshot. Frees no wasm memory itself (caller-owned ptr).
func (p *Parser) readNode(ptr uint32) (Node, error) {
	b, err := p.readBytes(ptr, nodeSize)
	if err != nil {
		return Node{}, err
	}
	return p.nodeFromBytes(b)
}

// Query is a compiled tree-sitter query (a .scm pattern set). Call Close
// when done.
type Query struct {
	p   *Parser
	ptr uint32
	// patterns holds each pattern's host-side predicates and #set!
	// settings, indexed like Match.PatternIndex (predicates.go, LB-F2).
	patterns []patternMeta
	// nodeTypes makes Run name each capture's node type (CaptureNodeTypes).
	nodeTypes bool
	// src is the query source, kept for the guard variants (guards.go).
	src string
	// variants are this query compiled without the patterns whose
	// #requires-text! guard a source fails, keyed by the patterns kept
	// (guards.go); guardsOff makes Run ignore the guards (tests).
	variants  map[string]*queryVariant
	guardsOff bool
	// lastPatterns is how many patterns the last Run's guest query held.
	lastPatterns int
}

// CaptureNodeTypes makes Run fill each Capture's Type with its node's
// grammar type: one more guest call per capture, so extraction never asks
// for it. The tags-query oracle does (LB-F46: a Zed outline.scm's @item
// kind is its node's type). It needs the probe export ts_node_type.
func (q *Query) CaptureNodeTypes() { q.nodeTypes = true }

// PatternCount is the number of top-level patterns the host read from the
// query source — every Match.PatternIndex is below it.
func (q *Query) PatternCount() int { return len(q.patterns) }

// PatternSetting returns the value a pattern's `(#set! key "value")`
// directive gives key, or "" (LB-F6 reads injection.language this way).
func (q *Query) PatternSetting(pattern uint16, key string) string {
	if int(pattern) >= len(q.patterns) {
		return ""
	}
	return q.patterns[pattern].settings[key]
}

// CompileQuery compiles src (tree-sitter query-language text) against this
// Parser's language. Returns a descriptive error including the byte offset
// tree-sitter reports on a malformed query — this is a build-time developer
// error (a bad .scm file), never a function of input source, so a plain
// error (not per-file degradation) is appropriate.
func (p *Parser) CompileQuery(src string) (*Query, error) {
	qPtr, err := p.compileRaw(src)
	if err != nil {
		return nil, err
	}
	metas, err := parseQueryPredicates(src)
	if err != nil {
		_, _ = p.call("ts_query_delete", uint64(qPtr))
		return nil, fmt.Errorf("tsengine: query compile failed: %w", err)
	}
	return &Query{p: p, ptr: qPtr, patterns: metas, src: src}, nil
}

// compileRaw is ts_query_new over src: the guest query's pointer.
func (p *Parser) compileRaw(src string) (uint32, error) {
	srcPtr, err := p.writeBytes([]byte(src))
	if err != nil {
		return 0, err
	}
	defer p.free(srcPtr)

	errOffPtr, err := p.malloc(4)
	if err != nil {
		return 0, err
	}
	defer p.free(errOffPtr)
	errTypePtr, err := p.malloc(4)
	if err != nil {
		return 0, err
	}
	defer p.free(errTypePtr)

	res, err := p.call("ts_query_new", uint64(p.langPtr), uint64(srcPtr), uint64(len(src)), uint64(errOffPtr), uint64(errTypePtr))
	if err != nil {
		return 0, err
	}
	qPtr := uint32(res[0])
	if qPtr == 0 {
		offBytes, _ := p.readBytes(errOffPtr, 4)
		typeBytes, _ := p.readBytes(errTypePtr, 4)
		offset := binary.LittleEndian.Uint32(offBytes)
		errType := binary.LittleEndian.Uint32(typeBytes)
		return 0, fmt.Errorf("tsengine: query compile failed at byte offset %d (error type %d)", offset, errType)
	}
	return qPtr, nil
}

// Close releases the query's wasm-side allocation.
func (q *Query) Close() {
	for _, v := range q.variants {
		_, _ = q.p.call("ts_query_delete", uint64(v.ptr))
	}
	q.variants = nil
	_, _ = q.p.call("ts_query_delete", uint64(q.ptr))
}

// Capture is one named capture within a Match.
type Capture struct {
	Name string
	Node Node
	// Type is the node's grammar type, set only when the query was asked
	// for it (CaptureNodeTypes).
	Type string
}

// Match is one query match: which pattern fired, and every capture it made.
type Match struct {
	PatternIndex uint16
	Captures     []Capture
}

// Run executes q against t's root node and returns every match, in
// tree-sitter's own (deterministic, document) order. This is the whole
// symbol-finding pass: callers post-process the flat Match list (e.g.
// containment-based qualification) — see internal/extractor.TSExtractor.
//
// The match iteration runs under the ParseTimeout budget (guest-side, via
// fathom_query_cursor_exec_bounded's progress callback) plus the
// ParseHardBackstop per next_match call — this is the load-bearing bound in
// practice: on the pathological `({[` x 50,000 input the parse completes in
// ~70ms but the query cursor's scan of the error tree runs 45s+ unbounded
// (measured; see grammars/budget.c). A guest cancellation makes next_match
// report exhaustion, so the cancelled flag is checked afterwards and
// surfaces as an error — never a silently truncated match set.
func (q *Query) Run(t *Tree) ([]Match, error) {
	p := q.p
	qPtr, remap, none := q.guarded(t.src)
	if none {
		return nil, nil
	}
	rootPtr, _, err := t.rootPtr()
	if err != nil {
		return nil, err
	}
	defer p.free(rootPtr)

	cursorRes, err := p.call("ts_query_cursor_new")
	if err != nil {
		return nil, err
	}
	cursorPtr := uint32(cursorRes[0])
	defer p.call("ts_query_cursor_delete", uint64(cursorPtr))

	budgetMS := ParseTimeout.Milliseconds()
	if budgetMS < 1 {
		budgetMS = 1
	}
	if _, err := p.call("fathom_query_cursor_exec_bounded", uint64(cursorPtr), uint64(qPtr), uint64(rootPtr), uint64(budgetMS)); err != nil {
		return nil, err
	}

	matchBuf, err := p.malloc(queryMatchSize)
	if err != nil {
		return nil, err
	}
	defer p.free(matchBuf)

	// The entire match iteration runs inside ONE bounded harness (not one
	// per next_match — see bounded's doc): a single next_match over a
	// pathological tree can burn 45s+, so the bound must cover in-flight
	// guest work, and per-call harnesses measured ~2x user-CPU corpus-wide.
	matchesAny, err := p.bounded("query match loop", func() (any, error) {
		return q.collectMatches(qPtr, cursorPtr, matchBuf)
	})
	if err != nil {
		return nil, err
	}
	all := matchesAny.([]Match)
	if remap != nil {
		for i := range all {
			all[i].PatternIndex = remap[all[i].PatternIndex]
		}
	}
	return q.filterByPredicates(all, t.src), nil
}

// collectMatches drains the executed cursor into a Match slice. Runs on the
// bounded harness's op goroutine; touches only op-local state plus the
// Parser's construction-time-immutable fields (via call/readBytes).
func (q *Query) collectMatches(qPtr, cursorPtr, matchBuf uint32) ([]Match, error) {
	p := q.p
	var matches []Match
	// captureNames caches ts_query_capture_name_for_id lookups — the same
	// small set of names (e.g. "definition.function", "name") repeats across
	// every match in a file.
	captureNames := map[uint32]string{}
	for {
		res, err := p.call("ts_query_cursor_next_match", uint64(cursorPtr), uint64(matchBuf))
		if err != nil {
			return nil, err
		}
		if res[0] == 0 {
			// Exhaustion OR guest-side cancellation — the C API cannot tell
			// them apart, so consult the cancelled flag.
			cRes, cErr := p.call("fathom_take_cancelled")
			if cErr != nil {
				return nil, cErr
			}
			if cRes[0] != 0 {
				return nil, fmt.Errorf("tsengine: query aborted after %v budget (pathological tree?)", ParseTimeout)
			}
			break
		}
		mb, err := p.readBytes(matchBuf, queryMatchSize)
		if err != nil {
			return nil, err
		}
		patternIndex := binary.LittleEndian.Uint16(mb[4:6])
		captureCount := binary.LittleEndian.Uint16(mb[6:8])
		capturesPtr := binary.LittleEndian.Uint32(mb[8:12])

		match := Match{PatternIndex: patternIndex, Captures: make([]Capture, 0, captureCount)}
		if captureCount > 0 {
			// One bulk read of the whole capture array, then one guest call
			// per capture (nodeFromBytes) — the hot path.
			capBytes, err := p.readBytes(capturesPtr, uint32(captureCount)*queryCaptureSize)
			if err != nil {
				return nil, err
			}
			for i := uint16(0); i < captureCount; i++ {
				one := capBytes[uint32(i)*queryCaptureSize : uint32(i+1)*queryCaptureSize]
				captureID := binary.LittleEndian.Uint32(one[nodeSize : nodeSize+4])

				name, ok := captureNames[captureID]
				if !ok {
					name, err = q.captureName(qPtr, captureID)
					if err != nil {
						return nil, err
					}
					captureNames[captureID] = name
				}

				node, err := p.nodeFromBytes(one[:nodeSize])
				if err != nil {
					return nil, err
				}
				c := Capture{Name: name, Node: node}
				if q.nodeTypes {
					// The capture's TSNode lives in the cursor's match until
					// the next next_match; ts_node_type reads it in place.
					if c.Type, err = p.cString("ts_node_type", uint64(capturesPtr+uint32(i)*queryCaptureSize)); err != nil {
						return nil, err
					}
				}
				match.Captures = append(match.Captures, c)
			}
		}
		matches = append(matches, match)
	}
	return matches, nil
}

func (q *Query) captureName(qPtr, id uint32) (string, error) {
	lenPtr, err := q.p.malloc(4)
	if err != nil {
		return "", err
	}
	defer q.p.free(lenPtr)

	res, err := q.p.call("ts_query_capture_name_for_id", uint64(qPtr), uint64(id), uint64(lenPtr))
	if err != nil {
		return "", err
	}
	namePtr := uint32(res[0])
	lenBytes, err := q.p.readBytes(lenPtr, 4)
	if err != nil {
		return "", err
	}
	nameLen := binary.LittleEndian.Uint32(lenBytes)
	if nameLen == 0 {
		return "", nil
	}
	nameBytes, err := q.p.readBytes(namePtr, nameLen)
	if err != nil {
		return "", err
	}
	return string(nameBytes), nil
}

// Grammars returns the embedded grammar blob names, sorted — the lock's
// rows (for capabilities/docs surfacing — ADR-021 §4 / XT-5).
func Grammars() []string {
	lock, err := embeddedLock()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(lock.Grammars))
	for _, r := range lock.Grammars {
		names = append(names, r.Blob)
	}
	sort.Strings(names)
	return names
}

// filterByPredicates drops the matches whose pattern's text predicates fail
// (LB-F2), keeping order.
func (q *Query) filterByPredicates(matches []Match, src []byte) []Match {
	out := matches[:0]
	for _, m := range matches {
		if int(m.PatternIndex) < len(q.patterns) {
			if preds := q.patterns[m.PatternIndex].preds; len(preds) > 0 && !evalTextPredicates(preds, m, src) {
				continue
			}
		}
		out = append(out, m)
	}
	return out
}
