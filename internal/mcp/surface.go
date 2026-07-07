// Package mcp — request-scoped tool surfaces.
//
// A surface is an immutable, per-request view of the tool registry, selected
// by the HTTP path on the shared daemon (POST /rpc/{surface}). It differs from
// the stdio profile in two ways: it is carried in the request context rather
// than mutable process-global state (so one client can never change another
// client's tools/list), and it hard-gates tools/call and batch sub-calls
// rather than only filtering tools/list.
//
// Surfaces: "companion" is a curated allowlist for coding-agent sessions;
// "coding", "investigation", "shared", "platform" mirror the group profiles
// (group + shared tools). The empty surface is the full registry.
package mcp

import (
	"context"
	"sort"
)

// SurfaceCompanion is the curated coding-agent surface name.
const SurfaceCompanion = "companion"

// companionTools is the coding-companion allowlist: the navigation /
// context / freshness hot loop an agentic coding session needs, and nothing
// else. Kept deliberately small; the full registry stays one endpoint away
// at POST /rpc. Changing membership is a contract change — update the pinned
// test in surface_test.go and docs/mcp-tools.md together.
var companionTools = map[string]bool{
	"batch":            true,
	"brief":            true,
	"entrypoints":      true,
	"find_callers":     true,
	"find_symbol":      true,
	"ingest":           true,
	"list_services":    true,
	"outline":          true,
	"pack_context":     true,
	"query_symbols":    true,
	"references":       true,
	"repo_map":         true,
	"staleness_report": true,
	"symbol_context":   true,
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
	case SurfaceCompanion, "coding", "investigation", "shared", "platform":
		return true
	}
	return false
}

// SurfaceNames returns the valid named surfaces, sorted.
func SurfaceNames() []string {
	names := []string{SurfaceCompanion, "coding", "investigation", "shared", "platform"}
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
	case SurfaceCompanion:
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

// visibleToolsCtx returns the descriptors visible to this request: the
// effective surface's tools, minus set_profile over HTTP — every HTTP call to
// it is rejected, so listing its schema would only cost clients tokens.
func (s *Server) visibleToolsCtx(ctx context.Context) []toolDescriptor {
	descs := s.visibleToolsFor(s.effectiveSurface(ctx))
	if !scopeFrom(ctx).viaHTTP {
		return descs
	}
	var out []toolDescriptor
	for _, d := range descs {
		if d.Name != "set_profile" {
			out = append(out, d)
		}
	}
	return out
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
