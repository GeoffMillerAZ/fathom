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
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed grammars/javascript.wasm grammars/typescript.wasm grammars/python.wasm grammars/ruby.wasm
var grammarFS embed.FS

// blobPath maps a grammar name (the registry key extractors ask for) to its
// embedded .wasm file. One blob may serve more than one entry point (e.g.
// "typescript" exports both tree_sitter_typescript and tree_sitter_tsx).
var blobPath = map[string]string{
	"javascript": "grammars/javascript.wasm",
	"typescript": "grammars/typescript.wasm",
	"python":     "grammars/python.wasm",
	"ruby":       "grammars/ruby.wasm",
}

// Sizeof constants for tree-sitter's public C structs (tree_sitter/api.h).
// These are part of tree-sitter's stable embedder ABI, not an implementation
// detail of one build — see grammars/PROVENANCE.md for how this was
// verified.
const (
	nodeSize         = 24 // TSNode:  context[4](16) + id(4) + tree(4), wasm32 pointers
	queryMatchSize   = 12 // TSQueryMatch: id(4) + pattern_index(2) + capture_count(2) + captures ptr(4)
	queryCaptureSize = 28 // TSQueryCapture: TSNode(24) + index(4)
)

var (
	runtimeOnce sync.Once
	rt          wazero.Runtime
	rtErr       error

	compileMu sync.Mutex
	compiled  = map[string]wazero.CompiledModule{}
)

// ensureRuntime lazily creates the single process-wide wazero Runtime and
// instantiates the WASI shim every grammar module imports (fd_write/fd_seek/
// fd_close only — see PROVENANCE.md; nothing custom to implement). Safe to
// call concurrently.
func ensureRuntime(ctx context.Context) (wazero.Runtime, error) {
	runtimeOnce.Do(func() {
		cfg := wazero.NewRuntimeConfig()
		if compilerSupported() {
			cfg = wazero.NewRuntimeConfigCompiler()
		} else {
			cfg = wazero.NewRuntimeConfigInterpreter()
		}
		rt = wazero.NewRuntimeWithConfig(ctx, cfg)
		if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
			rtErr = fmt.Errorf("tsengine: instantiate WASI shim: %w", err)
		}
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

// compile returns the cached wazero.CompiledModule for the named blob,
// compiling it on first use. Compilation is CPU-bound and safe to do once
// per process; CompiledModule itself is safe for concurrent InstantiateModule
// calls.
func compile(ctx context.Context, name string) (wazero.CompiledModule, error) {
	path, ok := blobPath[name]
	if !ok {
		return nil, fmt.Errorf("tsengine: unknown grammar %q", name)
	}

	compileMu.Lock()
	defer compileMu.Unlock()
	if cm, ok := compiled[name]; ok {
		return cm, nil
	}

	r, err := ensureRuntime(ctx)
	if err != nil {
		return nil, err
	}
	buf, err := grammarFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tsengine: read embedded blob %q: %w", path, err)
	}
	cm, err := r.CompileModule(ctx, buf)
	if err != nil {
		return nil, fmt.Errorf("tsengine: compile %q: %w", name, err)
	}
	compiled[name] = cm
	return cm, nil
}

// Parser is one instantiated grammar module (its own linear memory) bound to
// one language entry point, plus a live TSParser. Not safe for concurrent
// use from multiple goroutines — create one per goroutine.
type Parser struct {
	ctx       context.Context
	mod       api.Module
	fns       map[string]*api.Function
	langPtr   uint32
	parserPtr uint32
	closed    bool
}

// requiredExports is every C function build.sh exports and this package
// calls. Resolved once per Parser at construction so later calls are a plain
// map lookup, and a missing export fails fast with a clear error instead of
// a nil-pointer panic mid-extraction.
var requiredExports = []string{
	"malloc", "free",
	"ts_parser_new", "ts_parser_delete", "ts_parser_set_language",
	"ts_parser_parse_string", "ts_parser_reset",
	"ts_tree_root_node", "ts_tree_delete",
	"ts_node_start_byte", "ts_node_end_byte", "ts_node_start_point",
	"ts_language_abi_version",
	"ts_query_new", "ts_query_delete",
	"ts_query_cursor_new", "ts_query_cursor_delete",
	"ts_query_cursor_exec", "ts_query_cursor_next_match",
	"ts_query_capture_name_for_id",
}

// NewParser compiles (if needed) and instantiates the named grammar blob,
// resolves entryPoint (e.g. "tree_sitter_typescript") to get the TSLanguage,
// and creates a TSParser bound to it. Call Close when done — it frees the
// underlying wazero module (and with it every allocation this Parser made).
func NewParser(ctx context.Context, grammar, entryPoint string) (*Parser, error) {
	cm, err := compile(ctx, grammar)
	if err != nil {
		return nil, err
	}
	r, err := ensureRuntime(ctx)
	if err != nil {
		return nil, err
	}
	mod, err := r.InstantiateModule(ctx, cm, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		return nil, fmt.Errorf("tsengine: instantiate %q: %w", grammar, err)
	}

	p := &Parser{ctx: ctx, mod: mod, fns: make(map[string]*api.Function, len(requiredExports))}
	for _, name := range requiredExports {
		fn := mod.ExportedFunction(name)
		if fn == nil {
			mod.Close(ctx)
			return nil, fmt.Errorf("tsengine: grammar %q missing required export %q", grammar, name)
		}
		p.fns[name] = &fn
	}

	entryFn := mod.ExportedFunction(entryPoint)
	if entryFn == nil {
		mod.Close(ctx)
		return nil, fmt.Errorf("tsengine: grammar %q missing entry point %q", grammar, entryPoint)
	}
	langRes, err := entryFn.Call(ctx)
	if err != nil {
		mod.Close(ctx)
		return nil, fmt.Errorf("tsengine: call %q: %w", entryPoint, err)
	}
	p.langPtr = uint32(langRes[0])

	parserRes, err := p.call("ts_parser_new")
	if err != nil {
		mod.Close(ctx)
		return nil, err
	}
	p.parserPtr = uint32(parserRes[0])

	ok, err := p.call("ts_parser_set_language", uint64(p.parserPtr), uint64(p.langPtr))
	if err != nil {
		mod.Close(ctx)
		return nil, err
	}
	if ok[0] == 0 {
		mod.Close(ctx)
		return nil, fmt.Errorf("tsengine: grammar %q: incompatible language ABI version", grammar)
	}
	return p, nil
}

// Close releases the wazero module instance (and every allocation it made).
func (p *Parser) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	return p.mod.Close(p.ctx)
}

func (p *Parser) call(name string, args ...uint64) ([]uint64, error) {
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
func (p *Parser) Parse(src []byte) (*Tree, error) {
	// A Parser is reused across many files for the life of the process
	// (see internal/extractor.TSExtractor) — reset defensively before every
	// parse so a prior call that errored or was cancelled never leaks state
	// into this one. Cheap: tree-sitter's own recommended usage pattern for
	// a long-lived TSParser parsing many unrelated documents.
	if _, err := p.call("ts_parser_reset", uint64(p.parserPtr)); err != nil {
		return nil, err
	}

	srcPtr, err := p.writeBytes(src)
	if err != nil {
		return nil, err
	}
	defer p.free(srcPtr)

	res, err := p.call("ts_parser_parse_string", uint64(p.parserPtr), 0, uint64(srcPtr), uint64(len(src)))
	if err != nil {
		return nil, err
	}
	treePtr := uint32(res[0])
	if treePtr == 0 {
		return nil, fmt.Errorf("tsengine: parse failed (no tree produced)")
	}
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

// readNode reads a 24-byte TSNode struct at ptr and calls the exported
// accessors needed to build a Node snapshot, then frees no wasm memory
// itself (caller-owned ptr).
func (p *Parser) readNode(ptr uint32) (Node, error) {
	startByteRes, err := p.call("ts_node_start_byte", uint64(ptr))
	if err != nil {
		return Node{}, err
	}
	endByteRes, err := p.call("ts_node_end_byte", uint64(ptr))
	if err != nil {
		return Node{}, err
	}
	pointSret, err := p.malloc(8)
	if err != nil {
		return Node{}, err
	}
	defer p.free(pointSret)
	if _, err := p.call("ts_node_start_point", uint64(pointSret), uint64(ptr)); err != nil {
		return Node{}, err
	}
	pointBytes, err := p.readBytes(pointSret, 8)
	if err != nil {
		return Node{}, err
	}
	row := binary.LittleEndian.Uint32(pointBytes[0:4])
	return Node{
		StartByte: uint32(startByteRes[0]),
		EndByte:   uint32(endByteRes[0]),
		StartRow:  row,
	}, nil
}

// Query is a compiled tree-sitter query (a .scm pattern set). Call Close
// when done.
type Query struct {
	p   *Parser
	ptr uint32
}

// CompileQuery compiles src (tree-sitter query-language text) against this
// Parser's language. Returns a descriptive error including the byte offset
// tree-sitter reports on a malformed query — this is a build-time developer
// error (a bad .scm file), never a function of input source, so a plain
// error (not per-file degradation) is appropriate.
func (p *Parser) CompileQuery(src string) (*Query, error) {
	srcPtr, err := p.writeBytes([]byte(src))
	if err != nil {
		return nil, err
	}
	defer p.free(srcPtr)

	errOffPtr, err := p.malloc(4)
	if err != nil {
		return nil, err
	}
	defer p.free(errOffPtr)
	errTypePtr, err := p.malloc(4)
	if err != nil {
		return nil, err
	}
	defer p.free(errTypePtr)

	res, err := p.call("ts_query_new", uint64(p.langPtr), uint64(srcPtr), uint64(len(src)), uint64(errOffPtr), uint64(errTypePtr))
	if err != nil {
		return nil, err
	}
	qPtr := uint32(res[0])
	if qPtr == 0 {
		offBytes, _ := p.readBytes(errOffPtr, 4)
		typeBytes, _ := p.readBytes(errTypePtr, 4)
		offset := binary.LittleEndian.Uint32(offBytes)
		errType := binary.LittleEndian.Uint32(typeBytes)
		return nil, fmt.Errorf("tsengine: query compile failed at byte offset %d (error type %d)", offset, errType)
	}
	return &Query{p: p, ptr: qPtr}, nil
}

// Close releases the query's wasm-side allocation.
func (q *Query) Close() {
	_, _ = q.p.call("ts_query_delete", uint64(q.ptr))
}

// Capture is one named capture within a Match.
type Capture struct {
	Name string
	Node Node
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
func (q *Query) Run(t *Tree) ([]Match, error) {
	p := q.p
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

	if _, err := p.call("ts_query_cursor_exec", uint64(cursorPtr), uint64(q.ptr), uint64(rootPtr)); err != nil {
		return nil, err
	}

	matchBuf, err := p.malloc(queryMatchSize)
	if err != nil {
		return nil, err
	}
	defer p.free(matchBuf)

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
		for i := uint16(0); i < captureCount; i++ {
			capAddr := capturesPtr + uint32(i)*queryCaptureSize
			capBytes, err := p.readBytes(capAddr, queryCaptureSize)
			if err != nil {
				return nil, err
			}
			nodeBytes := capBytes[:nodeSize]
			captureID := binary.LittleEndian.Uint32(capBytes[nodeSize : nodeSize+4])

			name, ok := captureNames[captureID]
			if !ok {
				name, err = q.captureName(captureID)
				if err != nil {
					return nil, err
				}
				captureNames[captureID] = name
			}

			nPtr, err := p.writeBytes(nodeBytes)
			if err != nil {
				return nil, err
			}
			node, err := p.readNode(nPtr)
			p.free(nPtr)
			if err != nil {
				return nil, err
			}
			match.Captures = append(match.Captures, Capture{Name: name, Node: node})
		}
		matches = append(matches, match)
	}
	return matches, nil
}

func (q *Query) captureName(id uint32) (string, error) {
	lenPtr, err := q.p.malloc(4)
	if err != nil {
		return "", err
	}
	defer q.p.free(lenPtr)

	res, err := q.p.call("ts_query_capture_name_for_id", uint64(q.ptr), uint64(id), uint64(lenPtr))
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

// Grammars returns the registered grammar blob names (for capabilities/
// docs surfacing — ADR-021 §4 / XT-5).
func Grammars() []string {
	names := make([]string, 0, len(blobPath))
	for name := range blobPath {
		names = append(names, name)
	}
	return names
}
