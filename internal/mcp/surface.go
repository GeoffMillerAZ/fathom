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
// Surfaces: "agent" is the primary 15-tool allowlist for coding-agent
// sessions, served at POST /rpc/agent; "companion" is a compatible alias
// (identical allowlist, POST /rpc/companion). "coding", "investigation",
// "shared", "platform" mirror the group taxonomy (group + shared tools). The
// empty surface is the full registry.
package mcp

import (
	"context"
	"sort"
)

// SurfaceCompanion is the curated coding-agent surface name.
const SurfaceCompanion = "companion"

// SurfaceAgent is the primary 15-tool agent surface (docs/vision/05, ADR-014
// amendment): the same allowlist as companion, at POST /rpc/agent — the path
// the user-scope MCP registration points at.
const SurfaceAgent = "agent"

// companionTools is the coding-companion allowlist: the navigation /
// context / freshness hot loop an agentic coding session needs, and nothing
// else. Kept deliberately small; the full registry stays one endpoint away
// at POST /rpc. Changing membership is a contract change — update the pinned
// test in surface_test.go and docs/mcp-tools.md together.
var companionTools = map[string]bool{
	"capabilities":     true,
	"entrypoints":      true,
	"find_callers":     true,
	"find_symbol":      true,
	"ingest":           true,
	"list_services":    true,
	"obligations":      true,
	"outline":          true,
	"pack_context":     true,
	"query_symbols":    true,
	"references":       true,
	"repo_map":         true,
	"staleness_report": true,
	"symbol_context":   true,
	"verify_plan":      true,
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

// originFrom extracts the caller-declared origin ("" = agent/unknown).
func originFrom(ctx context.Context) string {
	o, _ := ctx.Value(originKey{}).(string)
	return o
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
	case SurfaceCompanion, SurfaceAgent, "coding", "investigation", "shared", "platform":
		return true
	}
	return false
}

// SurfaceNames returns the valid named surfaces, sorted.
func SurfaceNames() []string {
	names := []string{SurfaceCompanion, SurfaceAgent, "coding", "investigation", "shared", "platform"}
	sort.Strings(names)
	return names
}

// toolOnSurface reports whether d is visible on the named surface.
// "" is the full registry; companion is the explicit allowlist; group
// surfaces show the group's tools plus shared tools.
func toolOnSurface(surface string, d toolDescriptor) bool {
	switch surface {
	case "":
		return true
	case SurfaceCompanion, SurfaceAgent:
		return companionTools[d.Name]
	default:
		return d.group == GroupShared || string(d.group) == surface
	}
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
		if toolOnSurface(surface, te.desc) {
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
		if !ok || !toolOnSurface(sc.surface, te.desc) {
			return false
		}
	}
	return true
}
