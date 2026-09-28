// Package mcp — dispatch layer.
//
// registerBuiltins wires the core MCP protocol methods (initialize, ping,
// tools/list, tools/call, notifications/initialized). Tool handlers are
// registered separately in tools.go via registerTools.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// ToolGroup classifies a tool so the active profile filter can include/exclude
// it from tools/list responses.
type ToolGroup string

const (
	GroupCoding        ToolGroup = "coding"
	GroupInvestigation ToolGroup = "investigation"
	GroupShared        ToolGroup = "shared"
	GroupPlatform      ToolGroup = "platform"
	// GroupModules is the module-host floor tool set (docs/plans/
	// gke-module-host.md GKE-MH-5): list_modules, module_obligations,
	// module_staleness. Visible on bare /rpc (the full registry, same as
	// every other group) and on the named "modules" surface
	// (POST /rpc/modules) — never added to companionTools (/rpc/agent's
	// allowlist stays untouched, doctrine 4).
	GroupModules ToolGroup = "modules"
)

// toolDescriptor is the MCP tool schema entry returned in tools/list.
type toolDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	group       ToolGroup      // unexported; not serialised

	// aliases maps an additive alias parameter name to the primary/native
	// parameter name it stands in for (P1-1: find_symbol's "symbol" alias
	// for "name", references' for "query", query_symbols' for "concern").
	// Unexported, not serialised — schemas document the alias in prose
	// (InputSchema's property description), and this map exists purely so
	// dispatch-level required-field validation (P1-8, see
	// handleToolsCallWithCtx) knows an alias satisfies its primary's
	// "required" entry. nil for tools with no alias.
	aliases map[string]string

	// cost declares the tool's cost class explicitly (design §3.1, RT-55);
	// the zero value takes toolCostClasses' row (costclass.go).
	cost costDecl
}

// toolEntry pairs a descriptor with its handler.
type toolEntry struct {
	desc    toolDescriptor
	handler func(ctx context.Context, args json.RawMessage) (any, *rpcError)
	class   tenancy.Class // the declared cost class (RT-55)
}

// registerTool adds a tool to the server's tool registry. Every handler is
// wrapped with budgetedHandler (SU-3, ADR-041 decision 3) — a single choke
// point, so a handler that forgets its own budget still cannot exceed the
// dispatch-level one.
//
// RT-55 (design §3.1): a tool without a cost class cannot be registered —
// the admission gate would have nothing to admit it in.
func (s *Server) registerTool(desc toolDescriptor, h func(ctx context.Context, args json.RawMessage) (any, *rpcError)) {
	class, ok := resolveCostClass(desc)
	if !ok {
		panic("mcp: tool " + desc.Name + " declares no cost class: add it to toolCostClasses (internal/mcp/costclass.go), testdata/toolcost.json and docs/mcp-tools.md's Cost classes table")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools = append(s.tools, toolEntry{desc: desc, handler: budgetedHandler(h), class: class})
}

// findTool returns the toolEntry for a given name, or (zero, false).
func (s *Server) findTool(name string) (toolEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, te := range s.tools {
		if te.desc.Name == name {
			return te, true
		}
	}
	return toolEntry{}, false
}

// registerBuiltins wires the MCP protocol handlers: initialize, ping,
// tools/list, tools/call, notifications/initialized.
func registerBuiltins(s *Server) {
	s.Register("initialize", s.handleInitialize)
	s.Register("ping", s.handlePing)
	s.Register("tools/list", s.handleToolsList)
	s.Register("tools/call", s.handleToolsCall)
	// notifications/initialized is a notification — the Serve loop swallows it
	// before dispatch; this registration is defensive so it doesn't return
	// -32601 if somehow a client sends it with a non-null id.
	s.Register("notifications/initialized", func(_ context.Context, _ json.RawMessage) (any, *rpcError) {
		return map[string]any{}, nil
	})
}

// ─── built-in handlers ───────────────────────────────────────────────────────

// handleInitialize answers the legacy-era handshake (RP-12, docs/plans/
// reach-parity.md): the client's protocolVersion is echoed when it is in
// supportedProtocolVersions, otherwise the newest supported revision is
// offered (negotiateProtocolVersion) — never an error, so a client on any
// revision learns what this server speaks and can decide for itself.
func (s *Server) handleInitialize(_ context.Context, params json.RawMessage) (any, *rpcError) {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &p) // absent/malformed params negotiate as "" → newest supported
	return map[string]any{
		"protocolVersion": negotiateProtocolVersion(p.ProtocolVersion),
		"capabilities": map[string]any{
			"tools":     map[string]any{"listChanged": false},
			"resources": map[string]any{"listChanged": false},
			"prompts":   map[string]any{"listChanged": false},
		},
		"serverInfo": map[string]any{
			"name":    "fathom",
			"version": "1.0.0",
		},
	}, nil
}

func (s *Server) handlePing(_ context.Context, _ json.RawMessage) (any, *rpcError) {
	return map[string]any{"pong": true}, nil
}

func (s *Server) handleToolsList(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
	return map[string]any{"tools": s.toolsListEntries(ctx)}, nil
}

// toolsListEntries renders the request's visible tools as the MCP
// tools/list entries: name, description, inputSchema, and — for every
// classified tool — annotations (RP-10). outputSchema (RP-11) is included
// only when the request asked for it (WithContracts / the HTTP
// "?contracts=true" query param, O101/TD-2): the generated JSON Schema was
// ~18.8 kB of a 48.3 kB served payload, paid by every connected session on
// every turn whether or not the client validates structured output against
// it, so it is off by default and available on request. The order is
// pinned to ascending name (RP-12): registration order was already
// deterministic, but a client that hashes or diffs the list (the kit
// client pins per-tool hashes; a doc renderer diffs the whole list)
// deserves an order that does not move when a registration call moves.
func (s *Server) toolsListEntries(ctx context.Context) []any {
	descs := s.visibleToolsCtx(ctx)
	sort.Slice(descs, func(i, j int) bool { return descs[i].Name < descs[j].Name })
	includeSchemas := contractsFrom(ctx)
	tools := make([]any, len(descs))
	for i, d := range descs {
		entry := map[string]any{
			"name":        d.Name,
			"description": d.Description,
			"inputSchema": d.InputSchema,
		}
		if ann := toolAnnotations(d.Name); ann != nil {
			entry["annotations"] = ann
		}
		if includeSchemas {
			if out := outputSchemaFor(d.Name); out != nil {
				entry["outputSchema"] = out
			}
		}
		tools[i] = entry
	}
	return tools
}

// toolsListBytes is the serialised size of this request's tools/list —
// the per-session schema budget every MCP-connected client pays before
// any work happens (TD-2's diet, reported by the capabilities tool as
// schema_bytes so growth is measured, not guessed).
func (s *Server) toolsListBytes(ctx context.Context) int {
	b, err := json.Marshal(map[string]any{"tools": s.toolsListEntries(ctx)})
	if err != nil {
		return 0
	}
	return len(b)
}

// outcomeAbandonedByGovernor is the call outcome of work the tenancy
// ladder's refuse rung cancelled (RT-172, design §2.1).
const outcomeAbandonedByGovernor = "abandoned-by-governor"

// abandonedByGovernor is the error a tools/call answers with when the
// refuse rung cancelled its context (cause tenancy.ErrAbandonedByGovernor):
// whatever the handler returned was computed under a cancelled context and
// may be partial, so it is replaced by the named outcome (I38). nil for
// any other context state.
func abandonedByGovernor(ctx context.Context) *rpcError {
	cause := context.Cause(ctx)
	if cause == nil || !errors.Is(cause, tenancy.ErrAbandonedByGovernor) {
		return nil
	}
	return &rpcError{Code: ErrCodeInternal, Message: cause.Error(), Data: map[string]any{"outcome": outcomeAbandonedByGovernor}}
}

func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	if s.callLogger == nil {
		result, rpcErr, _ := s.handleToolsCallMeta(ctx, params)
		return result, rpcErr
	}
	// Extract tool name for logging before dispatching.
	var nameOnly struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(params, &nameOnly) // best-effort; errors handled below
	return s.loggedToolsCall(ctx, nameOnly.Name, params)
}

// loggedToolsCall wraps handleToolsCallMeta with call logging.
func (s *Server) loggedToolsCall(ctx context.Context, toolName string, params json.RawMessage) (any, *rpcError) {
	// Extract service from arguments for the log entry (best-effort).
	var args struct {
		Arguments struct {
			Service string `json:"service"`
		} `json:"arguments"`
	}
	_ = json.Unmarshal(params, &args)

	// RT-179: the clock starts at the request's arrival when the HTTP
	// layer recorded it, so a wait before dispatch is never logged as fast.
	start := time.Now()
	if a := arrivalFrom(ctx); !a.IsZero() && a.Before(start) {
		start = a
	}
	result, rpcErr, meta := s.handleToolsCallMeta(ctx, params)
	latency := time.Since(start)

	// Determine group + off-surface flag from the tool descriptor. The flag is
	// recomputed here (pure: scope + descriptor) rather than threaded through
	// the dispatch return path.
	group := ""
	offSurface := false
	if te, ok := s.findTool(toolName); ok {
		group = string(te.desc.group)
		if sc := scopeFrom(ctx); sc.enforced() && !s.toolOnSurface(sc.surface, te.desc) && !s.strictFor(sc.surface) {
			offSurface = true
		}
	}

	var logErr error
	if rpcErr != nil {
		// Construct a Go error-like message for the log entry without importing errors.
		logErr = &rpcErrWrapper{rpcErr}
	}
	claimsAttached := claimsAttachedFrom(result)
	verdictOutcome := verdictOutcomeFrom(result)
	resultCount, matchedVia := outcomeFrom(result)
	// BA-6: response size is measured from the exact bytes the wrapped MCP
	// envelope would marshal to — a second, tolerant marshal (result is
	// already a plain map by this point) rather than threading the
	// dispatch-time []byte through the return path.
	// PE-20 (docs/design/progress-and-economics.md §5): the same marshaled
	// bytes carry an estimated token count beside their byte count —
	// receiptTokens over the exact []byte responseBytes counted, so the two
	// numbers can never name different responses.
	responseBytes, responseTokens := 0, 0
	if b, err := json.Marshal(result); err == nil {
		responseBytes = len(b)
		responseTokens = receiptTokens(b)
	}
	elidedCount := elidedCountFrom(result)
	// RC-25 (docs/vision/28-receipts.md §4): the pointer-extraction hook —
	// every agent-surface call log row gains the answer's own top
	// pointers, extracted from the structured result before it is
	// rendered, never from text. resultPointers wants the tool's own
	// argument object (its contract, matched by its unit tests) — not
	// this call's {name, arguments} envelope — so unwrap the same
	// "arguments" field args.Arguments.Service above was already read
	// from, best-effort (a malformed envelope leaves it "null", and every
	// extraction branch degrades to "no pointer" on that, never a panic).
	var rawArgs struct {
		Arguments json.RawMessage `json:"arguments"`
	}
	_ = json.Unmarshal(params, &rawArgs)
	pointers := resultPointers(toolName, rawArgs.Arguments, result)
	s.callLogger.log(toolName, args.Arguments.Service, group, s.effectiveSurface(ctx), "", originFrom(ctx), offSurface, claimsAttached, verdictOutcome,
		s.recordCallOutcome(ctx, meta, callOutcome{ResultCount: resultCount, MatchedVia: matchedVia, ResponseBytes: responseBytes, ResponseTokens: responseTokens, ElidedCount: elidedCount, Pointers: pointers}), latency, logErr)
	return result, rpcErr
}

// recordCallOutcome completes a call-log outcome bundle with the admission
// columns (RT-64): class, queued_ms, bytes_read, outcome and the caller.
func (s *Server) recordCallOutcome(ctx context.Context, meta callMeta, out callOutcome) callOutcome {
	out.Class = meta.class.String()
	if meta.outcome == "" {
		out.Class = ""
	}
	out.QueuedMS = meta.queued.Milliseconds()
	out.BytesRead = meta.bytesRead
	out.Outcome = meta.outcome
	if c, ok := callerFrom(ctx); ok {
		out.Caller = &callLogCaller{PID: c.PID, PPID: c.PPID, Session: c.Session, Test: c.Test}
	}
	return out
}

// validateRequired checks args against desc's declared InputSchema
// "required" list (P1-8). A field is satisfied when it is present in args,
// OR when an alias registered for it in desc.aliases is present (P1-1's
// interaction — see the comment at the call site in
// handleToolsCallWithCtx). Malformed args (not a JSON object) are reported
// as ErrCodeParams here rather than deferred to the handler, so every tool
// gets the same "invalid args" shape regardless of whether it declares any
// required fields.
func validateRequired(desc toolDescriptor, args json.RawMessage) *rpcError {
	required, _ := desc.InputSchema["required"].([]string)
	if len(required) == 0 {
		return nil
	}
	present := map[string]json.RawMessage{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &present); err != nil {
			return &rpcError{Code: ErrCodeParams, Message: desc.Name + ": invalid args"}
		}
	}
	for _, field := range required {
		if _, ok := present[field]; ok {
			continue
		}
		satisfied := false
		for alias, primary := range desc.aliases {
			if primary != field {
				continue
			}
			if _, ok := present[alias]; ok {
				satisfied = true
				break
			}
		}
		if !satisfied {
			return &rpcError{Code: ErrCodeParams, Message: desc.Name + ": missing required parameter \"" + field + "\""}
		}
	}
	return nil
}

// rpcErrWrapper adapts *rpcError to the error interface for logging.
type rpcErrWrapper struct{ e *rpcError }

func (w *rpcErrWrapper) Error() string { return w.e.Message }

func (s *Server) handleToolsCallWithCtx(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	result, rpcErr, _ := s.handleToolsCallMeta(ctx, params)
	return result, rpcErr
}

// handleToolsCallMeta dispatches one tools/call and reports its admission
// (class, queue, bytes, outcome) for the call log. meta.outcome is "" only
// for a call rejected before admission (unknown tool, bad params).
func (s *Server) handleToolsCallMeta(ctx context.Context, params json.RawMessage) (any, *rpcError, callMeta) {
	var meta callMeta
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: ErrCodeParams, Message: "invalid tools/call params"}, meta
	}
	if p.Name == "" {
		return nil, &rpcError{Code: ErrCodeParams, Message: "tools/call: name required"}, meta
	}
	te, ok := s.findTool(p.Name)
	if !ok {
		return nil, &rpcError{Code: ErrCodeNotFound, Message: "tool not found: " + p.Name}, meta
	}
	sc := scopeFrom(ctx)
	// Hosted-modules security gate (GKE-MH-7, critic finding 2): checked
	// BEFORE — and independent of — sc.enforced(), because bare /rpc
	// deliberately has enforced()==false (it is the full, unauthenticated
	// registry by design) and would otherwise dispatch a GroupModules tool
	// with no gate at all. hostedModuleBlocked only ever fires when hosted
	// mode is active (a --modules-token-env was configured); the local,
	// loopback-only daemon never reaches this branch.
	if s.hostedModuleBlocked(sc, te.desc) {
		return nil, &rpcError{Code: ErrCodeNotFound, Message: "tool " + p.Name + " requires the " + SurfaceModules + " surface (hosted mode is active)"}, meta
	}
	// Execute-and-flag (ADR-014 amendment): an off-surface call runs and is
	// LOGGED as off_surface — an error teaches agents to abandon the surface,
	// which looks identical to no-demand in the log. Hard gating stays
	// available behind strictSurfaces for any future multi-tenant shape (and
	// is ALWAYS on for the modules surface while hosted — strictFor).
	if sc.enforced() && !s.toolOnSurface(sc.surface, te.desc) && s.strictFor(sc.surface) {
		return nil, &rpcError{Code: ErrCodeNotFound, Message: "tool " + p.Name + " is not on the " + sc.surface + " surface; POST /rpc serves the full surface"}, meta
	}
	// P1-8: enforce the declared InputSchema "required" fields before
	// invoking the handler, so the served contract matches what tools/list
	// advertises (previously only individually-written handler checks
	// enforced anything — C4-5's audit found at least one tool,
	// license_policy, that declared a field required and never checked it).
	//
	// Interaction with P1-1's parameter aliases: a handler may accept an
	// ADDITIVE alias for its primary argument (e.g. find_symbol's "symbol"
	// for "name"). validateRequired treats a present alias as satisfying
	// its primary's requirement (te.desc.aliases, populated at
	// registration) — so a call that only supplies the alias is NOT
	// rejected here for "missing" the native name; the handler resolves
	// which value wins (and errors loudly if both are given with different
	// values) after this gate passes. Alias resolution therefore never
	// needs to run before this check — it is encoded declaratively in
	// aliases and consulted here, rather than mutating p.Arguments.
	if rpcErr := validateRequired(te.desc, p.Arguments); rpcErr != nil {
		return nil, rpcErr, meta
	}

	// RT-56 (design §3.3): admit before the handler runs, so a refused
	// tree or fleet call costs nothing. The handler runs under the
	// ticket's ctx: the class deadline, the in-flight registration the
	// refuse rung cancels, and the byte meter.
	meta.class = costClassOf(te, p.Arguments)
	ctx, tk, busy := s.admitToolCall(ctx, te, p.Arguments, true)
	if busy != nil {
		meta.outcome = string(tenancy.OutcomeRefused)
		meta.queued = sinceArrival(ctx) // RT-179: the wait before the refusal
		return nil, busy, meta
	}

	// SP-3, made real at the only layer every response passes through: the
	// collector rides the ctx into the handler, and whatever the filters
	// dropped is rendered into the envelope. The V-review found the
	// collector channel INERT in deployment — ApplySuppressions produced
	// disclosures and no shipped surface ever listened, so every filtered
	// response in production was silent: "a silent filter is
	// indistinguishable from a recall bug", the exact condition the row
	// forbids, while the row sat checked on internal-object tests.
	ctx, collector := scope.WithCollector(ctx)
	// RT-150: a read whose auto-refresh was deferred says so.
	ctx, refreshed := withRefreshNotes(ctx)

	result, rpcErr := runHandler(ctx, te, p.Arguments)
	// RT-57 (§3.4): the answer names itself — abandoned, partial or ok.
	result, rpcErr, partial, settled := s.settleCall(ctx, te.desc.Name, tk, result, rpcErr)
	settled.class = meta.class
	meta = settled
	if rpcErr != nil {
		return nil, rpcErr, meta
	}
	// Wrap result in MCP content envelope.
	b, err := json.Marshal(result)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "marshal result: " + err.Error()}, meta
	}
	envelope := map[string]any{
		"content": []any{
			map[string]any{"type": "text", "text": string(b)},
		},
	}
	// RP-11: the same result as structuredContent (MCP 2025-06-18+) so a
	// client can validate it against the advertised outputSchema without
	// re-parsing the text block. Only object-shaped results qualify —
	// 2025-11-25 restricts structuredContent to a JSON object — and the
	// text block stays the canonical copy older clients read.
	if len(b) > 0 && b[0] == '{' {
		envelope["structuredContent"] = json.RawMessage(b)
	}
	if suppressed := collector.Render(); suppressed != nil {
		envelope["suppressed"] = suppressed
	}
	if r := refreshed.render(); r != nil {
		envelope["refresh"] = r
	}
	if partial != nil {
		envelope["budget"] = partial
	}
	return envelope, nil, meta
}
