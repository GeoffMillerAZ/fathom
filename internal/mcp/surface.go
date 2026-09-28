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
// Surfaces: "agent" is the default floor for coding-agent sessions (the
// eight-tool cap the surface reckoning sets, F-86, design §1.1), served at
// POST /rpc/agent — the path the user-scope MCP registration points at.
// "extended" is the opt-in superset (the default eight plus the promoted
// verbs and navigation tools); "companion", the pre-reckoning compatible
// alias, now SERVES THE SAME SET AS "extended" (POST /rpc/companion), so a
// client that pinned it keeps its old tool set minus the reckoning's
// deletions rather than losing tools outright. The empty surface is the
// full registry. ("modules", the module host's scoped set at
// /rpc/modules, was removed with the module MCP tools by SR-17.)
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

// SurfaceExtended is the opt-in surface the surface reckoning adds
// (F-86, design §1.1), served at POST /rpc/extended: the default eight
// (SurfaceAgent) plus the promoted verbs and navigation tools a session
// can reach without paying their schema cost by default. "companion", the
// pre-reckoning name, is a compatible alias onto this same set — not onto
// SurfaceAgent's now-smaller floor — so a client already pinned to
// /rpc/companion keeps its old tool set minus whatever the reckoning's
// other rows delete or merge, rather than silently losing tools when the
// default shrank.
const SurfaceExtended = "extended"

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
// safe_delete_check left this map at SR-24 (F-13, design §4) — it is
// unregistered as a standalone tool, its answer now blast_radius' own
// kind:"delete" (blastRadiusByKind, extra_tools.go), and blast_radius was
// already promoted below via promotedVerbs(). discoveries left this map at
// SR-29 (F-47, design §4) — 0 lifetime calls; the SubagentStart hook's
// push-form render already covers the need.
var companionTools = func() map[string]bool {
	m := map[string]bool{
		"find_callers":     true,
		"find_symbol":      true,
		"ingest":           true,
		"list_services":    true,
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
	return m
}()

// defaultSurfaceTools is the /rpc/agent floor the surface reckoning sets
// (F-86, design §1.1): the eight answers an agent gets before opting into
// anything else, capped by TestDefaultSurfaceIsEight. Membership is a
// contract change — update that test and docs/mcp-tools.md together.
//
// It is a sorted slice, not a package-level map (tenancy lint pkg-map):
// isDefaultSurfaceTool answers membership.
var defaultSurfaceTools = [...]string{
	"blast_radius",
	"find_symbol",
	"obligations",
	"read_source",
	"review_diff",
	"search",
	"symbol_context",
	"verify_plan",
}

// isDefaultSurfaceTool reports whether name is on the /rpc/agent floor.
func isDefaultSurfaceTool(name string) bool {
	for _, n := range defaultSurfaceTools {
		if n == name {
			return true
		}
	}
	return false
}

// extendedTools is /rpc/extended's opt-in set (F-86, design §1.1): the
// default eight plus the navigation/context/freshness tools and the
// remaining promoted verbs an agent can reach without paying their schema
// cost on every session. /rpc/companion, the pre-reckoning compatible
// alias, moves here (toolOnSurface below) instead of staying pinned to the
// now-smaller default, so a client that pinned "companion" keeps its old
// tool set minus the reckoning's own deletions/merges (capabilities,
// discoveries, entrypoints, memory_search, rename_impact and
// safe_delete_check all leave the curated surfaces this way — each by its
// own SR-* row, not this one — because they are either merged into a
// surviving tool or removed outright elsewhere in wave 2).
//
// "status" (design §1.1's merge of capabilities/server_stats/list_sources,
// SR-23) joins the 20 above it — extended now serves 21.
var extendedTools = func() map[string]bool {
	m := map[string]bool{}
	for _, name := range defaultSurfaceTools {
		m[name] = true
	}
	for _, name := range []string{
		"find_callers",
		"findings_near",
		"ingest",
		"list_services",
		"localize",
		"outline",
		"pack_context",
		"query_symbols",
		"references",
		"repo_map",
		"staleness_report",
		"status",
		"test_impact",
	} {
		m[name] = true
	}
	return m
}()

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
	case SurfaceCompanion, SurfaceAgent, SurfaceExtended:
		return true
	}
	return false
}

// SurfaceNames returns the valid named surfaces, sorted.
func SurfaceNames() []string {
	names := []string{SurfaceCompanion, SurfaceAgent, SurfaceExtended}
	sort.Strings(names)
	return names
}

// toolOnSurface reports whether d is visible on the named surface.
// "" is the full registry; agent is the eight-tool default; companion and
// extended share one explicit allowlist (see extendedTools).
func (s *Server) toolOnSurface(surface string, d toolDescriptor) bool {
	switch surface {
	case "":
		return true
	case SurfaceAgent:
		return isDefaultSurfaceTool(d.Name)
	case SurfaceCompanion, SurfaceExtended:
		return extendedTools[d.Name]
	default:
		// Unreachable: ValidSurface admits only the named surfaces above. A
		// path that is not one of them 404s in servehttp before reaching the
		// registry (HF-5 deleted the group surfaces; SR-17 the modules one).
		return false
	}
}

// strictFor reports whether off-surface tools/call must be hard-gated
// (never execute-and-flag) for the given surface: the global
// strictSurfaces toggle (tests, future multi-tenant shapes). Every
// surface's execute-and-flag ergonomics (ADR-014 amendment) otherwise hold.
func (s *Server) strictFor(string) bool {
	return s.strictSurfaces
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

// AgentSurfaceTools returns the pre-reckoning curated allowlist, sorted —
// the full set an agent session can reach across the companion/extended
// surfaces (companionTools; kept for the bench pyramid's committed copy,
// the adoption gauge and the capability probe, none of which this row
// touches). It is no longer what POST /rpc/agent itself serves — that is
// DefaultSurfaceTools now (F-86, design §1.1); use ExtendedSurfaceTools
// for what /rpc/companion and /rpc/extended serve today.
func AgentSurfaceTools() []string {
	out := make([]string, 0, len(companionTools))
	for name := range companionTools {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// DefaultSurfaceTools returns POST /rpc/agent's default allowlist, sorted:
// the eight-tool floor the surface reckoning sets (F-86, design §1.1,
// TestDefaultSurfaceIsEight).
func DefaultSurfaceTools() []string {
	out := make([]string, 0, len(defaultSurfaceTools))
	for _, name := range defaultSurfaceTools {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ExtendedSurfaceTools returns POST /rpc/extended's (and its companion
// alias's) allowlist, sorted: the default eight plus the opt-in set
// (F-86, design §1.1).
func ExtendedSurfaceTools() []string {
	out := make([]string, 0, len(extendedTools))
	for name := range extendedTools {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
