package toolregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// Tool binds an llmclient.ToolDef (what the model sees) to a Handler (what
// runs when the model calls it). Register the same Tool once and it serves
// both llmclient.Request.Tools and an MCP server's tools/list.
type Tool struct {
	// Def is the model-facing definition: name, description, input schema.
	// Def.Name is the registry key and must be non-empty.
	Def llmclient.ToolDef

	// Handler executes the tool. It receives the dispatch context (which
	// carries the per-tool deadline; see the package doc, "Timeouts") and the
	// raw-JSON input from the tool_use block, and returns the tool output to
	// place in the tool_result block. A non-nil error becomes an is_error
	// tool_result; so does a panic. Handlers must be safe for concurrent use
	// when Dispatch is called concurrently, and should select on ctx.Done().
	Handler func(ctx context.Context, input json.RawMessage) (string, error)

	// Timeout bounds one execution of this tool. Zero means use the
	// registry's Options.DefaultTimeout; negative is rejected by Register.
	Timeout time.Duration

	// Capabilities declares the security-relevant access classes this tool
	// exercises. Use NewCapabilitySet(caps...) to declare specific capabilities,
	// DeclareNone() to explicitly declare no capabilities, or leave nil to mark
	// the tool as having UNKNOWN capabilities. Unknown is DISTINCT from empty:
	// under TrifectaStrict mode, unknown tools are treated as potentially
	// completing the trifecta (fail-closed means suspicion, not trust).
	//
	// The three tracked capabilities are CapReadsUntrusted, CapReadsPrivate,
	// and CapWritesExternal. A toolset holding all three is the "lethal
	// trifecta" — see capability.go and the package doc for enforcement details.
	Capabilities CapabilitySet

	// Effect, when non-nil, declares what the tool does to its environment
	// in the MCP tool-annotation vocabulary (see Effect). nil means
	// UNDECLARED, which a strict effect policy (kits/agent-harness
	// EffectStrict) treats as destructive — fail-closed, exactly like nil
	// Capabilities. Declare &Effect{ReadOnlyHint: true} for a pure read and
	// &Effect{DestructiveHint: true} for an irreversible action. A declared
	// Effect never changes Defs: the model sees the same tool list; only
	// dispatch governance and MCP tools/list annotations read it.
	Effect *Effect
}

// Policy decides whether a tool call may proceed. nil Policy in Options means
// allow all. A non-nil error denies the call; the error text is surfaced to
// the model as an is_error tool_result, so write it for the model to read.
// The input is the raw JSON from the tool_use block, allowing argument-aware
// policies; name-only policies may ignore it.
type Policy func(name string, input json.RawMessage) error

// Options configures a Registry.
type Options struct {
	// DefaultTimeout bounds tool executions whose Tool.Timeout is zero.
	// Zero means no registry-imposed deadline (the caller's context still
	// applies).
	DefaultTimeout time.Duration

	// Policy, when non-nil, is consulted before every handler invocation.
	// See the Policy type doc. nil allows all calls.
	Policy Policy

	// Trifecta configures lethal-trifecta detection and enforcement. Zero
	// value is TrifectaPermissive with no callback. See TrifectaOptions for
	// enforcement modes, override provenance, and telemetry hooks.
	Trifecta TrifectaOptions
}

// Registry holds the registered tools. Create one with New. Safe for
// concurrent use; see the package doc for the full concurrency contract.
type Registry struct {
	mu       sync.RWMutex
	tools    map[string]Tool
	opts     Options
	capUnion CapabilitySet // union of all registered tools' effective capabilities
}

// New returns an empty Registry with the given options.
func New(opts Options) *Registry {
	return &Registry{
		tools:    make(map[string]Tool),
		opts:     opts,
		capUnion: make(CapabilitySet),
	}
}

// TrifectaComplete reports whether the registered toolset's union of declared
// capabilities (with unknown tools treated as all three trifecta capabilities)
// currently completes the lethal trifecta. Safe to call from any goroutine.
func (r *Registry) TrifectaComplete() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.capUnion.completesTrifecta()
}

// TrifectaOverrideRecord returns the override that authorized trifecta
// completion, or nil if no override was supplied. The returned pointer is safe
// to read; the TrifectaOverride struct is immutable after New.
func (r *Registry) TrifectaOverrideRecord() *TrifectaOverride {
	return r.opts.Trifecta.Override
}

// Register adds t to the registry. It returns an error when the tool name is
// empty (or whitespace), the name is already registered, the Handler is nil,
// or the Timeout is negative. The Tool is stored by value; do not mutate
// Def.InputSchema's backing bytes after registering.
//
// Trifecta enforcement: if the registry was created with TrifectaStrict mode
// and registering t would complete the lethal trifecta (CapReadsUntrusted +
// CapReadsPrivate + CapWritesExternal across the registered toolset), Register
// returns an *ErrTrifectaBlocked unless a valid TrifectaOverride is present in
// Options.Trifecta. Tools with nil Capabilities (unknown) are treated as
// potentially exercising all three capabilities under strict mode. The
// TrifectaOptions.OnComplete callback is invoked in both strict and permissive
// modes when trifecta completion occurs (strictly before the strict-mode
// refusal, to allow logging even of blocked attempts).
func (r *Registry) Register(t Tool) error {
	name := t.Def.Name
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("toolregistry: Register: tool name must be non-empty")
	}
	if t.Handler == nil {
		return fmt.Errorf("toolregistry: Register: tool %q has a nil Handler", name)
	}
	if t.Timeout < 0 {
		return fmt.Errorf("toolregistry: Register: tool %q has negative Timeout %v", name, t.Timeout)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("toolregistry: Register: tool %q already registered", name)
	}

	// Compute what the capability union would become if this tool is added.
	// Unknown capabilities (nil CapabilitySet) are treated as all three
	// trifecta capabilities under strict mode — fail-closed means suspicion.
	effectiveCaps := t.Capabilities
	if effectiveCaps == nil && r.opts.Trifecta.Mode == TrifectaStrict {
		effectiveCaps = NewCapabilitySet(CapReadsUntrusted, CapReadsPrivate, CapWritesExternal)
	}
	candidate := r.capUnion.union(effectiveCaps)

	if candidate.completesTrifecta() && !r.capUnion.completesTrifecta() {
		// This tool completes the trifecta. Fire the callback (for both modes),
		// then enforce strict mode if active.
		if r.opts.Trifecta.OnComplete != nil {
			r.opts.Trifecta.OnComplete(name)
		}
		if r.opts.Trifecta.Mode == TrifectaStrict {
			// An override must be present and valid to proceed.
			overrideOK := r.opts.Trifecta.Override != nil &&
				r.opts.Trifecta.Override.validate() == nil
			if !overrideOK {
				return &ErrTrifectaBlocked{
					ToolName:          name,
					CapabilitiesUnion: formatCapSet(candidate),
				}
			}
		}
	}

	r.tools[name] = t
	r.capUnion = candidate
	return nil
}

// formatCapSet formats a CapabilitySet as a human-readable string for error
// messages. The order is fixed so messages are deterministic.
func formatCapSet(s CapabilitySet) string {
	var parts []string
	for _, c := range []Capability{CapReadsUntrusted, CapReadsPrivate, CapWritesExternal} {
		if s.Has(c) {
			parts = append(parts, string(c))
		}
	}
	return strings.Join(parts, " + ")
}

// MustRegister is Register but panics on error. Intended for program-startup
// tool tables where a registration failure is a programming bug.
func (r *Registry) MustRegister(t Tool) {
	if err := r.Register(t); err != nil {
		panic(err)
	}
}

// Get returns the registered Tool by name and whether it exists.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// Names returns the registered tool names, sorted.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Defs returns the registered tool definitions sorted by name — ready for
// llmclient.Request.Tools or an MCP tools/list response. The slice is freshly
// allocated on every call; the ToolDefs share json.RawMessage backing bytes
// with the registered tools.
func (r *Registry) Defs() []llmclient.ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]llmclient.ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Def)
	}
	slices.SortFunc(defs, func(a, b llmclient.ToolDef) int {
		return strings.Compare(a.Name, b.Name)
	})
	return defs
}
