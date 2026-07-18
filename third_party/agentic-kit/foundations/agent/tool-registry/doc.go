// Package toolregistry is the single tool primitive for agentic-kit: define a
// tool ONCE — an llmclient.ToolDef plus a Go handler — and expose it
// everywhere it is needed. Defs feeds llmclient.Request.Tools and an MCP
// server's tools/list from the same registration; Dispatch turns the model's
// tool_use blocks into the paired tool_result blocks for the conversation
// loop. The agent-harness kit and MCP server kit are both expected to consume
// this package rather than invent their own tool tables.
//
// Composition: imports the standard library plus two sibling foundations —
// foundations/llmclient for the content-block and tool-definition types, and
// foundations/environment/tool-execution-policy for the optional
// PolicyFromExecutionPolicy adapter. The registry itself performs no I/O;
// handlers may.
//
// Concurrency contract: Registry is safe for concurrent use (guarded by a
// sync.RWMutex); Register, MustRegister, Get, Names, Defs, Dispatch, and
// DispatchAll may be called from any goroutine. Handlers must themselves be
// safe for concurrent use when Dispatch is invoked concurrently. DispatchAll
// runs handlers SEQUENTIALLY, in block order.
//
// # Dispatch returns a ContentBlock, never a Go error
//
// Every failure mode of a tool call is something the MODEL should see and
// react to, so each becomes a BlockToolResult with IsError set: unknown tool,
// policy denial, handler error, handler panic (recovered — a tool panic never
// crashes the agent loop), timeout, and context cancellation. A Go error
// return on Dispatch is reserved for infrastructure failure, of which there
// is currently none; rather than force every call site in the agent loop to
// handle an error that cannot occur, the signature returns only the
// ContentBlock. Should an infrastructure failure class appear later, widening
// the signature is a deliberate breaking change to make then.
//
// # Sequential dispatch
//
// DispatchAll dispatches the tool_use blocks of an assistant turn one at a
// time, in order, and is deliberately NOT parallel: tools have side effects,
// and the model emitted the calls in an order it may depend on (create the
// file, then write to it). Callers that know their tools are independent and
// side-effect-free may fan out over Dispatch themselves — the Registry is
// safe for that.
//
// # Timeouts
//
// Each dispatch derives a context for the handler: Tool.Timeout when set
// (> 0), else Options.DefaultTimeout when set, else no deadline (the caller's
// context still applies). The handler runs in its own goroutine; when the
// deadline fires or the caller's context is cancelled, Dispatch returns the
// is_error tool_result immediately — naming the timeout duration — and the
// handler goroutine is ABANDONED: it keeps running until it observes the
// (cancelled) context, and its eventual result is discarded. The result
// channel is buffered, so an abandoned handler that eventually returns does
// not leak a goroutine permanently; a handler that ignores its context and
// blocks forever leaks its goroutine, which is the price of never stalling
// the agent loop. Well-behaved handlers select on ctx.Done().
//
// # Policy
//
// Options.Policy, when non-nil, is consulted before every handler invocation;
// a non-nil error denies the call and the denial message is surfaced to the
// model as an is_error tool_result. PolicyFromExecutionPolicy adapts a
// foundations/environment/tool-execution-policy PolicySet (which decides on
// tool name and environment, deny-by-default) into a Policy by fixing the
// environment name at construction time.
//
// # Capability taxonomy and lethal-trifecta enforcement
//
// Every Tool may declare zero or more Capability values on its Capabilities
// field. Three capabilities are tracked: CapReadsUntrusted (prompt-injection
// surface), CapReadsPrivate (exfiltration target), and CapWritesExternal
// (exfiltration channel). A toolset that holds all three is the "lethal
// trifecta": an agent that reads untrusted content, has access to private
// data, and can write externally can be steered into exfiltrating private data
// via prompt injection alone — the canonical 2026 agent-security failure.
//
// Tool.Capabilities = nil means UNKNOWN (not declared at all), which is
// distinct from DeclareNone() (explicitly declared as having no relevant
// capabilities). The distinction matters under TrifectaStrict mode: unknown
// tools are treated as potentially exercising all three capabilities (fail-
// closed means suspicion, not trust), while explicitly-declared-empty tools
// are taken at their word.
//
// Options.Trifecta configures enforcement. TrifectaPermissive (the zero value)
// allows trifecta completion but fires TrifectaOptions.OnComplete for audit.
// TrifectaStrict blocks the completing tool's Register call with
// *ErrTrifectaBlocked, unless a TrifectaOverride with AuthorizedBy and Reason
// fields is present — the override records who authorized the risk and why.
//
// Taxonomy is self-declared. A lying tool defeats detection; this is
// defense-in-depth, not a security boundary. The value is making the risk
// visible at registration time — before any token is spent — so operators can
// audit toolsets and require explicit, named authorization when the trifecta
// is unavoidable.
//
// # Schema helpers
//
// ObjectSchema builds the flat JSON Schema objects that cover the vast
// majority of tool inputs: scalar properties (string, number, integer,
// boolean), string enums, and arrays of scalars. It is deliberately minimal —
// anything richer (nested objects, oneOf, formats) is supplied as raw JSON in
// ToolDef.InputSchema; json.RawMessage IS the escape hatch.
package toolregistry
