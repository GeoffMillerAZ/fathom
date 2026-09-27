// Package mcp — request-scoped tool surfaces.
//
// A surface is an immutable, per-request view of the tool registry, selected
// by the HTTP path on the shared daemon (POST /rpc/{surface}). It is carried
// in the request context rather than mutable process-global state, so one
// client can never change another client's tools/list.
//
// Gate semantics are execute-and-flag, not hard-gate (ADR-014 amendment,
// 2026-07-16, surfaces week-1): an off-surface tools/call or batch sub-call
// still executes — an error only teaches an agent to abandon the surface,
// which looks identical to no-demand in the call log. The call is logged with
// an off_surface flag instead, so demand is measurable. Surfaces are
// ergonomics and measurement, never a security boundary — bare /rpc already
// serves the full, unauthenticated registry. The pre-amendment hard gate
// (off-surface calls return a "not on the surface" error) stays available
// behind Server.strictSurfaces (see SetStrictSurfaces) for any future
// multi-tenant shape.
//
// Surfaces: "agent" is the primary curated allowlist for coding-agent
// sessions, served at POST /rpc/agent; "companion" is a compatible alias
// (identical allowlist, POST /rpc/companion); "modules" is the module host's
// scoped set. The empty surface is the full registry.
//
// The four group surfaces ("coding", "investigation", "shared", "platform")
// were DELETED 2026-08-26 (ADR-014 amendment 2, docs/plans/honesty-fixes.md
// HF-5). They were reachable only by typing the URL — no client
// registration, stanza, skill, or doc ever pointed at one — so their zero
// calls proved exposure-zero, not demand-zero (law 8 forbids reading that as
// a kill clock). They are removed on law 4's ground instead: a surface no
// wired client can reach is registry weight with no consumer. The group
// TAXONOMY (GroupCoding, GroupInvestigation, …) is untouched; only the
// path-scoped surfaces named after it are gone.
package mcp

import (
	"context"
	"sort"
)

// SurfaceCompanion is the curated coding-agent surface name.
const SurfaceCompanion = "companion"

// SurfaceAgent is the primary agent surface (docs/vision/05, ADR-014
// amendment): the same allowlist as companion, at POST /rpc/agent — the path
// the user-scope MCP registration points at.
const SurfaceAgent = "agent"

// SurfaceModules is the module-host scoped surface (docs/plans/
// gke-module-host.md GKE-MH-5), served at POST /rpc/modules: the
// GroupModules floor tool set (list_modules, module_obligations,
// module_staleness) plus GroupShared, exactly like every other group
// surface (platform/coding/investigation) — a new scoping boundary, not a
// new capability contract (the plan's design section's doctrine-4
// argument).
const SurfaceModules = "modules"

// companionTools is the coding-companion allowlist: the navigation /
// context / freshness hot loop an agentic coding session needs, and nothing
// else. Kept deliberately small; the full registry stays one endpoint away
// at POST /rpc. Changing membership is a contract change — update the pinned
// test in surface_test.go and docs/mcp-tools.md together.
//
// ADR-035 decision 1 (docs/vision/29-verbs-not-lookups.md §3 wave A, plan
// VB-1/VB-2) adds the four verbs promotedVerbs() names plus
// safe_delete_check, its read-only sibling: the agent surface leads with
// the answers grep cannot give, not another lookup pitched on token
// savings. Wave G (plan VB-26) adds a fifth promoted verb, localize.
var companionTools = func() map[string]bool {
	m := map[string]bool{
		"capabilities":     true,
		"discoveries":      true,
		"entrypoints":      true,
		"find_callers":     true,
		"find_symbol":      true,
		"ingest":           true,
		"list_services":    true,
		"memory_search":    true,
		"obligations":      true,
		"outline":          true,
		"pack_context":     true,
		"query_symbols":    true,
		"read_source":      true,
		"references":       true,
		"repo_map":         true,
		"search":           true,
		"staleness_report": true,
		"symbol_context":   true,
		"verify_plan":      true,
	}
	for _, v := range promotedVerbs() {
		m[v.Name] = true
	}
	m["safe_delete_check"] = true
	return m
}()

// hostedModulesTools is /rpc/modules' EXPLICIT allowlist while hosted mode
// is active (Server.hostedModules, set by cmd/fathom's serve-http exactly
// when --modules-token-env names a configured token — docs/plans/
// gke-module-host.md GKE-MH-7, critic finding 2). Deliberately NOT
// "GroupModules ∪ GroupShared" (the non-hosted default every other named
// surface uses): GroupShared bundles tools bound to the FLEET backend
// (repo_map, pack_context, staleness_report, find_symbol, …, all keyed by
// an arbitrary caller-supplied "service" — a modules-token holder pivoting
// through one of those would read the FLEET's own indexed services, not
// just the mounted modules) and tools with real side effects (ingest
// triggers a store write; compile_layer spends against a wired LLM; batch
// can smuggle any other tool's call). A hosted module host serves ONLY the
// module-host's own floor tools plus two harmless, dependency-free meta
// tools. Every other GroupShared tool is deliberately absent here — update
// this list and surface_test.go together, the same discipline
// companionTools already documents.
//
//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
var hostedModulesTools = map[string]bool{
	"list_modules":       true,
	"module_obligations": true,
	"module_query":       true,
	"module_staleness":   true,
	"capabilities":       true,
	"server_stats":       true,
}

// requestScope is the per-request surface selection carried in context.
type requestScope struct {
	surface string // "" = full registry
	viaHTTP bool   // true when the request arrived over the shared HTTP daemon
}

type scopeKey struct{}

// WithRequestScope returns a context carrying the request's surface selection.
// The HTTP layer attaches it per request; stdio requests carry no scope and
// fall back to the process profile.
func WithRequestScope(ctx context.Context, surface string, viaHTTP bool) context.Context {
	return context.WithValue(ctx, scopeKey{}, requestScope{surface: surface, viaHTTP: viaHTTP})
}

// scopeFrom extracts the request scope; the zero scope means stdio.
func scopeFrom(ctx context.Context) requestScope {
	sc, _ := ctx.Value(scopeKey{}).(requestScope)
	return sc
}

type originKey struct{}

// WithOrigin returns a context carrying the caller-declared channel origin
// (the X-Fathom-Origin header): "edit-hook", "session-hook", "stop-hook",
// "verify-cli" — empty for agent tool choice. The call log records it so
// demand numbers and kill clocks separate plumbing from agents (GI-6).
// Self-declared and trusted: this is measurement, not security.
func WithOrigin(ctx context.Context, origin string) context.Context {
	if origin == "" {
		return ctx
	}
	return context.WithValue(ctx, originKey{}, origin)
}

// OriginConsole is the origin the Observatory console stamps on its own
// reads of the daemon (the model's list_services, the fleet page's registry
// read): the console reading itself is not usage. The call log records the
// row with this origin, so it stays auditable, and every usage reader —
// tool tables, call totals, fleet and impact counts, the in-memory surface
// traffic server_stats reports — leaves it out (IsConsoleRead).
const OriginConsole = "console"

// IsConsoleRead reports whether a call-log row's origin is the console
// reading itself, never usage.
// The daemon's own in-process console calls (OriginConsoleInProcess,
// RT-61) are console reads too.
func IsConsoleRead(origin string) bool {
	return origin == OriginConsole || origin == OriginConsoleInProcess
}

// OriginOf is ctx's caller-declared origin ("" = agent/unknown).
func OriginOf(ctx context.Context) string { return originFrom(ctx) }

// originFrom extracts the caller-declared origin ("" = agent/unknown).
func originFrom(ctx context.Context) string {
	o, _ := ctx.Value(originKey{}).(string)
	return o
}

type contractsKey struct{}

// WithContracts returns a context requesting outputSchema entries on
// tools/list (O101/TD-2 diet): default false, so the served payload stays
// diet-sized (no generated JSON Schema per tool) unless a client that
// validates envelopes explicitly opts in. The HTTP layer sets this from the
// "?contracts=true" query parameter on POST /rpc and /rpc/{surface}
// (cmd/fathom's servehttp.go); stdio callers get the diet unconditionally
// unless they call this directly. The schemas themselves are unaffected —
// they stay in the registry (outputschema.go's agentResultTypes) and are
// generated on demand by outputSchemaFor regardless of this flag.
func WithContracts(ctx context.Context, include bool) context.Context {
	if !include {
		return ctx
	}
	return context.WithValue(ctx, contractsKey{}, true)
}

// contractsFrom reports whether this request asked for outputSchema entries
// on tools/list (see WithContracts). Default false.
func contractsFrom(ctx context.Context) bool {
	v, _ := ctx.Value(contractsKey{}).(bool)
	return v
}

// enforced reports whether tools/call must be hard-gated for this request:
// only named surfaces over HTTP gate calls. The stdio profile stays advisory
// (a single client owns the process), and the bare HTTP endpoint is the full
// registry by definition.
func (sc requestScope) enforced() bool {
	return sc.viaHTTP && sc.surface != ""
}

// ValidSurface reports whether name is a registerable surface for
// POST /rpc/{surface}. The empty name is not valid as a path segment; the
// bare /rpc endpoint is the full surface.
func ValidSurface(name string) bool {
	switch name {
	case SurfaceCompanion, SurfaceAgent, SurfaceModules:
		return true
	}
	return false
}

// SurfaceNames returns the valid named surfaces, sorted.
func SurfaceNames() []string {
	names := []string{SurfaceCompanion, SurfaceAgent, SurfaceModules}
	sort.Strings(names)
	return names
}

// toolOnSurface reports whether d is visible on the named surface.
// "" is the full registry; agent/companion is the explicit allowlist; the
// modules surface serves GroupModules + GroupShared — EXCEPT the modules
// surface while hosted mode is active, which serves ONLY
// hostedModulesTools' explicit allowlist (critic finding 2(a): GroupShared
// carries fleet-backend reads and spend/write tools a modules-token holder
// must never reach). "" (bare /rpc, the full registry) ALSO hides
// GroupModules tools while hosted (finding 2(c): "an unauthenticated path
// to module data must not exist when a token was configured" — bare /rpc
// carries no auth check at all, so the only way to close that path is to
// make GroupModules tools invisible there too; hostedModuleBlocked,
// dispatch.go, additionally refuses to EXECUTE them there even if a caller
// invokes tools/call directly without consulting tools/list).
func (s *Server) toolOnSurface(surface string, d toolDescriptor) bool {
	switch surface {
	case "":
		if s.hostedModules && d.group == GroupModules {
			return false
		}
		return true
	case SurfaceCompanion, SurfaceAgent:
		return companionTools[d.Name]
	case SurfaceModules:
		if s.hostedModules {
			return hostedModulesTools[d.Name]
		}
		return d.group == GroupShared || d.group == GroupModules
	default:
		// Unreachable: ValidSurface admits only the three named surfaces
		// above. A path that is not one of them 404s in servehttp before
		// reaching the registry (HF-5 deleted the group surfaces).
		return false
	}
}

// strictFor reports whether off-surface tools/call must be hard-gated
// (never execute-and-flag) for the given surface: the global
// strictSurfaces toggle (tests, future multi-tenant shapes), OR the
// modules surface specifically while hosted mode is active — "strict
// surfaces ON for that path" is a hosted-mode LAUNCH REQUIREMENT (GKE-MH-7,
// critic finding 2(b)), not a manual opt-in an operator could forget to
// flip; every other surface's execute-and-flag ergonomics (ADR-014
// amendment) are unaffected.
func (s *Server) strictFor(surface string) bool {
	return s.strictSurfaces || (s.hostedModules && surface == SurfaceModules)
}

// hostedModuleBlocked reports whether a GroupModules tool call must be
// refused OUTRIGHT — independent of sc.enforced()/strictSurfaces, and even
// on bare /rpc, any other named surface, or a batch sub-call — because
// hosted mode is active and the request did not arrive scoped to
// SurfaceModules itself. This is the one place in this package where a
// surface becomes an actual SECURITY boundary rather than ergonomics/
// measurement (the package doc's own carve-out: "bare /rpc already serves
// the full, unauthenticated registry" — hostedModuleBlocked is the
// deliberate exception for a network-reachable deployment, docs/plans/
// gke-module-host.md's design section, critic finding 2). The HTTP layer's
// bearer-token middleware (cmd/fathom's modulesAuthMiddleware) already
// gates ENTRY to /rpc/modules; this gates every OTHER path from ever
// reaching a module tool once hosted mode is on, so "no unauthenticated
// path to module data" has no other door.
func (s *Server) hostedModuleBlocked(sc requestScope, d toolDescriptor) bool {
	return s.hostedModules && d.group == GroupModules && sc.surface != SurfaceModules
}

// effectiveSurface resolves the surface for a request: HTTP requests use the
// path-selected surface; stdio requests use the process profile.
func (s *Server) effectiveSurface(ctx context.Context) string {
	if sc := scopeFrom(ctx); sc.viaHTTP {
		return sc.surface
	}
	return s.activeProfile()
}

// visibleToolsFor returns the tool descriptors visible on the named surface.
func (s *Server) visibleToolsFor(surface string) []toolDescriptor {
	s.mu.Lock()
	defer s.mu.Unlock()
	var descs []toolDescriptor
	for _, te := range s.tools {
		if s.toolOnSurface(surface, te.desc) {
			descs = append(descs, te.desc)
		}
	}
	return descs
}

// visibleToolsCtx returns the descriptors visible to this request: the tools
// on the request's effective surface (path-selected over HTTP, the process
// profile over stdio — unreachable today since set_profile was removed, so
// stdio always resolves to the full registry).
func (s *Server) visibleToolsCtx(ctx context.Context) []toolDescriptor {
	return s.visibleToolsFor(s.effectiveSurface(ctx))
}

// promptOnScope reports whether every tool a prompt's workflow references is
// callable under the request scope. Canned prompts are full-surface
// workflows, so a scoped endpoint hides the ones it cannot honour rather than
// advertising steps that would be gated.
func (s *Server) promptOnScope(ctx context.Context, def *promptDef) bool {
	sc := scopeFrom(ctx)
	if !sc.enforced() {
		return true
	}
	for _, name := range def.tools {
		te, ok := s.findTool(name)
		if !ok || !s.toolOnSurface(sc.surface, te.desc) {
			return false
		}
	}
	return true
}

// AgentSurfaceTools returns the agent/companion surface's tool allowlist,
// sorted — the set an agent session can actually reach. Consumers (the
// adoption gauge, IO-10/O6) partition zero-call evidence with it so
// "unused on this surface" and "not exposed on this surface" stop reading
// as the same remedy.
func AgentSurfaceTools() []string {
	out := make([]string, 0, len(companionTools))
	for name := range companionTools {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
