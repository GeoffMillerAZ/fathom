// Package mcp implements a JSON-RPC 2.0 MCP tool server over stdio.
//
// Wire protocol: newline-delimited JSON (one request per line). Responses are
// written back as single-line JSON objects, one per request. Notifications
// (id == null) are processed but generate no response.
//
// The server is safe for concurrent use: stdout writes are serialized via a
// mutex; each request is dispatched in the goroutine that reads it.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// JSON-RPC 2.0 error codes.
const (
	ErrCodeParse    = -32700
	ErrCodeInvalid  = -32600
	ErrCodeNotFound = -32601
	ErrCodeParams   = -32602
	ErrCodeInternal = -32603
)

// CommitInfo is the MCP-layer commit record returned by ListCommits.
// It is a presentation-layer view of a git commit for tool output.
type CommitInfo = store.CommitInfo

// QueryInterface is the minimal query surface the MCP tools depend on.
// It is satisfied by *client.Client, *query.Engine, or any fake for testing.
type QueryInterface interface {
	QuerySymbols(ctx context.Context, q core.Query) (core.QueryResult, error)
	FindCallers(ctx context.Context, serviceID, symbol string) ([]core.SymbolNode, error)
	FindSymbol(ctx context.Context, serviceID, name string) ([]core.SymbolNode, error)
	ListAllSymbols(ctx context.Context, serviceID string) ([]core.SymbolNode, error)
	// Services returns the distinct service IDs known to the backend.
	// Implementations may return nil/empty when the information is unavailable.
	Services(ctx context.Context) ([]string, error)
	// Sources discovers and fetches every source file for the named service.
	// Implementations may return nil/empty when source fetching is unavailable.
	Sources(ctx context.Context, service string) ([]core.SourceFile, error)
	// IngestService triggers a re-ingest of the named service, refreshing the
	// store.  Implementations that do not support re-ingest may return nil.
	IngestService(ctx context.Context, service string) error
	// ServiceMeta returns the stored ingest metadata (path, indexed commit) for
	// serviceID.  Implementations that do not persist this may return
	// ("", "", false, nil).
	ServiceMeta(ctx context.Context, serviceID string) (path, indexedCommit string, found bool, err error)
	// ListCommits returns commit history for serviceID newest-first.
	// Implementations that do not support commits may return nil, nil.
	ListCommits(ctx context.Context, serviceID string) ([]CommitInfo, error)
	// TypedEdges returns every typed Go call edge (ADR-021 §3) stored for
	// serviceID — direct, interface-dispatch (one row per implementer), and
	// unresolved. Implementations that do not support typed edges (or a
	// service with none yet) return nil, nil.
	TypedEdges(ctx context.Context, serviceID string) ([]core.Edge, error)
}

// rpcRequest is a minimal representation of a JSON-RPC 2.0 request. The id
// field is kept as json.RawMessage so it can be null, a number, or a string.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// rpcResponse is the envelope written back to the client.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError represents the JSON-RPC error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	// Data is JSON-RPC's optional structured error detail (the governor's
	// {"outcome": "abandoned-by-governor"}, RT-172); omitted when nil.
	Data any `json:"data,omitempty"`
}

// Handler is the function signature for an MCP method handler. It receives the
// raw params bytes and returns either a result value (serialised to JSON) or an
// rpcError.
type Handler func(ctx context.Context, params json.RawMessage) (any, *rpcError)

// Server is a JSON-RPC 2.0 MCP server that dispatches requests to Handlers.
// Handlers are registered via Register before the first Serve call.
type Server struct {
	mu       sync.Mutex
	handlers map[string]Handler
	tools    []toolEntry // registered MCP tools (see dispatch.go)
	backend  QueryInterface

	// activeProfile guards which tools appear in tools/list.
	profileMu sync.RWMutex
	profile   string // "", "coding", "investigation", "shared", "platform"

	// strictSurfaces restores the pre-amendment ADR-014 hard gate: off-surface
	// tools/call errors instead of executing with an off_surface log flag.
	// Default false — surfaces are ergonomics + measurement, never security
	// (bare /rpc is the full registry regardless). Keep available for any
	// future multi-tenant shape.
	strictSurfaces bool

	// hostedModules is the module host's hosted-deployment mode (docs/plans/
	// gke-module-host.md GKE-MH-7, critic finding 2): cmd/fathom's serve-http
	// sets this via SetHostedModules exactly when --modules-token-env names a
	// configured token. Unlike strictSurfaces, this DOES make /rpc/modules an
	// actual security boundary (surface.go's hostedModulesTools/
	// hostedModuleBlocked) — the local, loopback-only, unauthenticated daemon
	// is unaffected (default false). Set once, before Serve/ListenAndServe
	// begins accepting concurrent requests — same single-set-before-serving
	// convention as strictSurfaces; no separate mutex.
	hostedModules bool

	// Stats for server_stats meta-tool.
	startTime  time.Time
	reqCount   int64
	reqCountMu sync.Mutex

	// callLogger is wired in by NewServerWithLogger.  May be nil.
	callLogger *CallLogger

	// budgets is the project/user `budgets` block (BA-9), set once via
	// SetBudgets before serving; the zero value means the measured
	// built-in defaults (budget_defaults.go).
	budgets budgetSettings

	// graphCache memoizes each service's call graphs per store watermark
	// (BP-28, I21). Building one cost ~9.7s of every blast_radius query on
	// a ten-thousand-symbol service and nothing about that work is
	// per-query. Never nil after newServer; safe for concurrent use.
	graphCache *callgraph.CachedCallGraph
	// navMemo holds each service's loaded symbols, typed edges and graphs
	// per store watermark; navFlight lets concurrent cold callers share one
	// load (resolved_nav.go loadNavGraphs).
	navMu     sync.Mutex
	navMemo   map[string]navMemoEntry
	navOrder  []string
	navFlight map[string]*navCall

	// gate is the admission gate (design §3.2, RT-56): set once by
	// SetGate before serving; nil admits everything (stdio, tests).
	gate *tenancy.Gate
}

// DropCaches drops the server's derived caches — the call graphs and the
// navigation memo — at the tenancy ladder's refuse rung (RT-34). Loads in
// flight finish for their callers and are not stored past the next one.
func (s *Server) DropCaches() {
	s.graphCache.Clear()
	s.navMu.Lock()
	s.navMemo = make(map[string]navMemoEntry) // never nil: a flight in progress stores into it
	s.navOrder = nil
	s.navMu.Unlock()
}

// NewServer constructs a Server wired to backend. If backend is nil the server
// can still serve initialize / ping / tools/list / set_profile; tool calls
// that require the backend will return ErrCodeInternal.
func NewServer(backend QueryInterface) *Server {
	return newServer(backend, nil, nil, nil)
}

// NewServerWithLogger constructs a Server that writes JSONL call-log entries to
// the provided CallLogger.  Pass nil for logger to disable logging (same as
// NewServer).
func NewServerWithLogger(backend QueryInterface, logger *CallLogger) *Server {
	return newServer(backend, nil, logger, nil)
}

// NewServerWithFleet constructs a Server wired to both a per-service
// QueryInterface backend and a fleet-wide FleetBackend.  Either may be nil.
func NewServerWithFleet(backend QueryInterface, fleet FleetBackend) *Server {
	return newServer(backend, fleet, nil, nil)
}

// NewServerWithFleetAndLogger constructs a Server wired to a per-service
// backend, a fleet backend, and a JSONL call logger.  Any may be nil.
func NewServerWithFleetAndLogger(backend QueryInterface, fleet FleetBackend, logger *CallLogger) *Server {
	return newServer(backend, fleet, logger, nil)
}

// NewServerWithModules constructs a Server wired to a per-service backend, a
// fleet backend, a JSONL call logger, and the module host (docs/plans/
// gke-module-host.md GKE-MH-5). modules registers the /rpc/modules-scoped
// tool set (list_modules, module_obligations, module_staleness) — pass nil
// (equivalent to NewServerWithFleetAndLogger) when no --module store was
// mounted; a genuinely non-nil *modulehost.Host registers the tools. Any
// other parameter may also be nil.
func NewServerWithModules(backend QueryInterface, fleet FleetBackend, logger *CallLogger, modules moduleBackend) *Server {
	return newServer(backend, fleet, logger, modules)
}

func newServer(backend QueryInterface, fleet FleetBackend, logger *CallLogger, modules moduleBackend) *Server {
	s := &Server{
		handlers:   make(map[string]Handler),
		backend:    backend,
		startTime:  time.Now(),
		callLogger: logger,
		graphCache: callgraph.NewCachedCallGraph(0),
	}
	registerBuiltins(s)
	registerTools(s, backend, fleet, modules)
	registerPlatformTools(s, backend, fleet)
	registerMemoryTools(s, backend)
	if modules != nil {
		registerModuleTools(s, modules)
	}
	return s
}

// GraphCacheStats returns the call-graph memo's current size and its
// process-lifetime hit/miss counts (BP-28, I21) — see
// callgraph.CachedCallGraph.Stats. Consumed by cmd/fathom's /metrics as
// fathom_graph_memo_entries (O108): a daemon running --auto-refresh should
// hold this at or below the cache's capacity (8 by default) for its whole
// lifetime, never climbing without bound.
func (s *Server) GraphCacheStats() (entries, hits, misses int) {
	return s.graphCache.Stats()
}

// AutoRefreshTotal returns the process-lifetime count of auto-refresh
// re-ingests this server's backend has triggered (O108), or 0 when the
// backend does not wrap autorefresh.go's watcher (e.g. --auto-refresh=false,
// or a test fake). Consumed by cmd/fathom's /metrics as
// fathom_autorefresh_total.
func (s *Server) AutoRefreshTotal() int64 {
	if b, ok := s.backend.(autoRefreshStatsBackend); ok {
		return b.AutoRefreshTotal()
	}
	return 0
}

// Register adds a handler for the given JSON-RPC method name.
func (s *Server) Register(method string, h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = h
}

// Serve reads newline-delimited JSON-RPC requests from in, dispatches them,
// and writes responses to out. It returns when ctx is cancelled or in reaches
// EOF. A write lock on out is held per response so concurrent callers are safe.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	var outMu sync.Mutex
	writeResp := func(resp rpcResponse) {
		b, err := json.Marshal(resp)
		if err != nil {
			return
		}
		outMu.Lock()
		defer outMu.Unlock()
		out.Write(b) //nolint:errcheck
		out.Write([]byte("\n"))
	}

	scanner := bufio.NewScanner(in)
	// Allow large lines (up to 4 MB per the spec).
	buf := make([]byte, 4*1024*1024)
	scanner.Buffer(buf, cap(buf))

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			return nil // EOF
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			// Parse error — id is unknown, use null.
			writeResp(rpcResponse{
				JSONRPC: "2.0",
				ID:      json.RawMessage("null"),
				Error:   &rpcError{Code: ErrCodeParse, Message: fmt.Sprintf("parse error: %v", err)},
			})
			continue
		}

		// Notifications (id is null) are processed but no response is written.
		isNotification := string(req.ID) == "null" || len(req.ID) == 0
		if isNotification {
			continue
		}

		result, rpcErr := s.dispatch(ctx, req)
		if rpcErr != nil {
			writeResp(rpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   rpcErr,
			})
			continue
		}
		writeResp(rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		})
	}
}

// HandleRequest processes a single raw JSON-RPC message (request bytes) and
// returns the serialised response. If the message is a notification (null id)
// or produces no response, hasResp is false and resp is nil. The method is
// safe for concurrent use and reuses the same dispatch logic as Serve.
func (s *Server) HandleRequest(ctx context.Context, raw []byte) (resp []byte, hasResp bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		b, _ := json.Marshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &rpcError{Code: ErrCodeParse, Message: fmt.Sprintf("parse error: %v", err)},
		})
		return b, true
	}
	// Notifications (null or absent id) produce no response.
	isNotification := string(req.ID) == "null" || len(req.ID) == 0
	if isNotification {
		return nil, false
	}
	result, rpcErr := s.dispatch(ctx, req)
	var envelope rpcResponse
	if rpcErr != nil {
		envelope = rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: rpcErr}
	} else {
		envelope = rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	}
	b, err := json.Marshal(envelope)
	if err != nil {
		return nil, false
	}
	return b, true
}

// dispatch looks up and calls the handler for req.Method.
func (s *Server) dispatch(ctx context.Context, req rpcRequest) (any, *rpcError) {
	s.reqCountMu.Lock()
	s.reqCount++
	s.reqCountMu.Unlock()

	s.mu.Lock()
	h, ok := s.handlers[req.Method]
	s.mu.Unlock()
	if !ok {
		return nil, &rpcError{
			Code:    ErrCodeNotFound,
			Message: fmt.Sprintf("method not found: %s", req.Method),
		}
	}
	return h(ctx, req.Params)
}

// incReqCount increments the request counter (exposed for meta-tools).
func (s *Server) totalRequests() int64 {
	s.reqCountMu.Lock()
	defer s.reqCountMu.Unlock()
	return s.reqCount
}

// SetStrictSurfaces toggles the pre-amendment ADR-014 hard gate: when true, an
// off-surface tools/call (or batch sub-call) returns a "not on the surface"
// error instead of executing with an off_surface log flag. Default false.
// Exposed for tests and for any future multi-tenant deployment shape.
func (s *Server) SetStrictSurfaces(v bool) {
	s.strictSurfaces = v
}

// SetHostedModules toggles hosted-module-host mode (docs/plans/
// gke-module-host.md, critic finding 2): when true, GroupModules tools are
// reachable ONLY via the /rpc/modules-scoped path — /rpc/modules itself
// serves a curated allowlist (hostedModulesTools) instead of the full
// GroupShared set, off-surface calls on /rpc/modules are hard-gated (never
// execute-and-flag, see strictFor), and GroupModules tools are hidden from
// bare /rpc's tools/list AND refused outright if invoked there or via any
// batch sub-call (hostedModuleBlocked). cmd/fathom's serve-http calls this
// exactly when --modules-token-env names a configured token; the local,
// loopback-only, unauthenticated daemon is unaffected (default false).
// Call before Serve/ListenAndServe begins accepting requests — same
// convention as SetStrictSurfaces.
func (s *Server) SetHostedModules(v bool) {
	s.hostedModules = v
}

// setProfile updates the active profile (must be a known surface name or "").
func (s *Server) setProfile(p string) *rpcError {
	if p != "" && !ValidSurface(p) {
		return &rpcError{Code: ErrCodeParams, Message: fmt.Sprintf("unknown profile %q; valid: %s", p, strings.Join(SurfaceNames(), ", "))}
	}
	s.profileMu.Lock()
	defer s.profileMu.Unlock()
	s.profile = p
	return nil
}

// activeProfile returns the current profile (read-safe).
func (s *Server) activeProfile() string {
	s.profileMu.RLock()
	defer s.profileMu.RUnlock()
	return s.profile
}
