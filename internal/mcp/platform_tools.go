// Package mcp — fleet/platform MCP tool registrations.
//
// registerPlatformTools wires four tools in GroupPlatform:
//
//	list_services        — services with metadata (languages, symbol counts)
//	query_cross_service  — keyword search across all services
//	fleet_drift          — config-style version/kind drift across services
//	cross_repo_callers   — callers of a symbol across all services via Calls edges
//	platform_callers     — alias for cross_repo_callers
package mcp

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/configdrift"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// FleetBackend is the minimal interface the platform tools require.  It is
// satisfied by *store.PlatformStore (after adding thin adapter shims) or by a
// fake for testing.
type FleetBackend interface {
	// Services returns the sorted list of service IDs in the fleet.
	Services() []string
	// SymbolsFor returns all SymbolNodes for service.
	SymbolsFor(service string) []core.SymbolNode
	// ObservationsFor returns all core.Observations for service.
	ObservationsFor(service string) []core.Observation
}

// registerPlatformTools registers the five fleet-level tools into s. The
// QueryInterface (may be nil) supplies each service's recorded corpus path
// for list_services — the authoritative repo↔service identity the hooks
// resolve against (dogfood O15: directory-basename guessing silently broke
// injection when the dir name and service ID diverged).
func registerPlatformTools(s *Server, backend QueryInterface, fleet FleetBackend) {
	registerListServices(s, backend, fleet)
	registerQueryCrossService(s, fleet)
	registerFleetDrift(s, fleet)
	registerCrossRepoCallers(s, fleet, "cross_repo_callers", "Find all callers of a symbol across every service in the fleet via stored call edges.")
}

// ─── list_services ────────────────────────────────────────────────────────────

func registerListServices(s *Server, backend QueryInterface, fleet FleetBackend) {
	s.registerTool(toolDescriptor{
		Name:        "list_services",
		Description: "List all services in the fleet with metadata: detected languages, symbol counts, and each service's recorded corpus path (the authoritative repo↔service mapping).",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		group: GroupPlatform,
	}, func(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
		if fleet == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no fleet backend configured"}
		}
		svcs := fleet.Services()
		entries := make([]any, 0, len(svcs))
		for _, svc := range svcs {
			nodes := fleet.SymbolsFor(svc)
			langs := langSet(nodes)
			entry := map[string]any{
				"service":      svc,
				"symbol_count": len(nodes),
				"languages":    langs,
			}
			// Recorded corpus path (ServiceMeta) — empty/omitted when the
			// backend is absent or the service has no recorded root (e.g.
			// remote corpora). Always absolute post-O10.
			if backend != nil {
				if path, _, found, err := backend.ServiceMeta(ctx, svc); err == nil && found && path != "" {
					entry["path"] = path
				}
			}
			entries = append(entries, entry)
		}
		return map[string]any{"services": entries, "count": len(svcs)}, nil
	})
}

// ─── query_cross_service ──────────────────────────────────────────────────────

func registerQueryCrossService(s *Server, fleet FleetBackend) {
	s.registerTool(toolDescriptor{
		Name:        "query_cross_service",
		Description: "Search for a keyword across every service in the fleet. Returns matching symbol names and their service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "Case-insensitive substring to search for in symbol names and claims",
				},
			},
			"required": []string{"keyword"},
		},
		group: GroupPlatform,
	}, func(_ context.Context, args json.RawMessage) (any, *rpcError) {
		if fleet == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no fleet backend configured"}
		}
		var p struct {
			Keyword string `json:"keyword"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "query_cross_service: invalid args"}
		}
		if p.Keyword == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "query_cross_service: keyword required"}
		}
		kw := strings.ToLower(p.Keyword)

		type hit struct {
			Service  string `json:"service"`
			Symbol   string `json:"symbol"`
			FilePath string `json:"file_path,omitempty"`
			Kind     string `json:"kind,omitempty"`
		}
		var hits []hit
		for _, svc := range fleet.Services() {
			for _, n := range fleet.SymbolsFor(svc) {
				if strings.Contains(strings.ToLower(n.Name), kw) {
					hits = append(hits, hit{
						Service:  svc,
						Symbol:   n.Name,
						FilePath: n.FilePath,
						Kind:     string(n.Kind),
					})
				}
			}
		}
		// Also search observation claims.
		for _, svc := range fleet.Services() {
			for _, obs := range fleet.ObservationsFor(svc) {
				if strings.Contains(strings.ToLower(obs.Claim), kw) &&
					!strings.Contains(strings.ToLower(obs.Symbol), kw) {
					hits = append(hits, hit{
						Service: svc,
						Symbol:  obs.Symbol,
						Kind:    string(obs.SourceKind),
					})
				}
			}
		}
		// Sort for determinism.
		sort.Slice(hits, func(i, j int) bool {
			if hits[i].Service != hits[j].Service {
				return hits[i].Service < hits[j].Service
			}
			return hits[i].Symbol < hits[j].Symbol
		})
		// Convert to []any.
		result := make([]any, len(hits))
		for i, h := range hits {
			result[i] = map[string]any{
				"service":   h.Service,
				"symbol":    h.Symbol,
				"file_path": h.FilePath,
				"kind":      h.Kind,
			}
		}
		return map[string]any{"matches": result, "count": len(hits)}, nil
	})
}

// ─── fleet_drift ──────────────────────────────────────────────────────────────

// registerFleetDrift reports version/kind drift across the fleet using the
// configdrift engine.  It derives per-service "config" maps from each
// service's SymbolNodes: for every symbol, the key is the symbol name and the
// value is the kind string.  Services that share the same symbol name but
// disagree on kind are reported as drifting.
func registerFleetDrift(s *Server, fleet FleetBackend) {
	s.registerTool(toolDescriptor{
		Name:        "fleet_drift",
		Description: "Report version/kind drift across services: symbols that appear in multiple services with conflicting kinds.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		group: GroupPlatform,
	}, func(_ context.Context, _ json.RawMessage) (any, *rpcError) {
		if fleet == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no fleet backend configured"}
		}
		svcs := fleet.Services()

		// Build configs map: service → (symbolName → kindString).
		configs := make(map[string]map[string]string, len(svcs))
		allKeys := make(map[string]struct{})
		for _, svc := range svcs {
			m := make(map[string]string)
			for _, n := range fleet.SymbolsFor(svc) {
				if n.Name == "" {
					continue
				}
				m[n.Name] = string(n.Kind)
				allKeys[n.Name] = struct{}{}
			}
			configs[svc] = m
		}

		// Collect keys.
		keys := make([]string, 0, len(allKeys))
		for k := range allKeys {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		items := configdrift.Drift(configs, keys)
		result := make([]any, len(items))
		for i, item := range items {
			result[i] = map[string]any{
				"key":      item.Key,
				"service":  item.Service,
				"value":    item.Value,
				"majority": item.Majority,
			}
		}
		return map[string]any{"drift": result, "count": len(items)}, nil
	})
}

// ─── cross_repo_callers / platform_callers ────────────────────────────────────

func registerCrossRepoCallers(s *Server, fleet FleetBackend, name, description string) {
	s.registerTool(toolDescriptor{
		Name:        name,
		Description: description,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"symbol": map[string]any{
					"type":        "string",
					"description": "Symbol name (bare or SymbolKey) to search for callers across the fleet",
				},
			},
			"required": []string{"symbol"},
		},
		group: GroupPlatform,
	}, func(_ context.Context, args json.RawMessage) (any, *rpcError) {
		if fleet == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no fleet backend configured"}
		}
		var p struct {
			Symbol string `json:"symbol"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: name + ": invalid args"}
		}
		if p.Symbol == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: name + ": symbol required"}
		}

		// fleetCallers (crossservice.go) is this tool's own matching logic,
		// extracted so symbol_context/find_callers's cross-service
		// stitching (P1-2) can reuse it verbatim instead of a second
		// implementation drifting apart from this one.
		hits := fleetCallers(fleet, p.Symbol)
		result := make([]any, len(hits))
		for i, h := range hits {
			result[i] = map[string]any{
				"service":   h.Service,
				"symbol":    h.Symbol,
				"file_path": h.FilePath,
			}
		}
		return map[string]any{"callers": result, "count": len(hits)}, nil
	})
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// matchesSymbol reports whether callee (a SymbolKey or bare name) matches the
// search target.  Handles three callee formats:
//
//   - Exact: "FormatCents" == "FormatCents"
//   - SymbolKey: "svc::pkg.Name" — strip the "svc::" prefix, then match tail
//     exactly or by bare name after the last dot.
//   - Plain qualified: "pkg.Name" — as emitted by the Go extractor for
//     cross-package calls; match exactly or by bare name after the last dot.
func matchesSymbol(callee, target string) bool {
	if callee == target {
		return true
	}
	// Determine the "tail" to match against: strip any "service::" prefix first.
	tail := callee
	if idx := strings.LastIndex(callee, "::"); idx >= 0 {
		tail = callee[idx+2:]
	}
	// tail is now either "pkg.Name" or "Name".
	if tail == target {
		return true
	}
	// Match bare name: target "FormatCents" against tail "output.FormatCents".
	if dotIdx := strings.LastIndex(tail, "."); dotIdx >= 0 {
		if tail[dotIdx+1:] == target {
			return true
		}
		// Also allow target to be a qualified suffix: "output.FormatCents" matches
		// tail "output.FormatCents" (already handled above by tail==target), but
		// also handle target "pkg.Name" matching tail "pkg.Name" via HasSuffix for
		// deeper paths.
		if strings.HasSuffix(tail, "."+target) {
			return true
		}
	}
	return false
}

// langSet derives the set of detected languages from SymbolNode kinds and
// returns a sorted []any for JSON serialisation.
func langSet(nodes []core.SymbolNode) []any {
	seen := make(map[string]struct{}, 4)
	for _, n := range nodes {
		if lang := kindToLang(n.Kind); lang != "" {
			seen[lang] = struct{}{}
		}
	}
	langs := make([]string, 0, len(seen))
	for l := range seen {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	out := make([]any, len(langs))
	for i, l := range langs {
		out[i] = l
	}
	return out
}

// kindToLang maps a SourceKind to a short language tag.
func kindToLang(k core.SourceKind) string {
	switch {
	case strings.HasPrefix(string(k), "Go"):
		return "go"
	case strings.HasPrefix(string(k), "Python"):
		return "python"
	case strings.HasPrefix(string(k), "JS"):
		return "javascript"
	case strings.HasPrefix(string(k), "Java") && !strings.HasPrefix(string(k), "JavaScript"):
		return "java"
	case strings.HasPrefix(string(k), "Ruby"):
		return "ruby"
	case strings.HasPrefix(string(k), "Terraform"):
		return "terraform"
	case strings.HasPrefix(string(k), "K8s"):
		return "kubernetes"
	// C-family and friends (docs/plans/cpp-toolchain.md CT-6). Order
	// matters: every multi-letter C* prefix must match before the bare-C
	// kinds, and those are matched exactly — a HasPrefix "C" would swallow
	// CSharp, CMake, CUE and Compose alike.
	case strings.HasPrefix(string(k), "Cpp"):
		return "cpp"
	case strings.HasPrefix(string(k), "CSharp"):
		return "csharp"
	case strings.HasPrefix(string(k), "CMake"):
		return "cmake"
	case strings.HasPrefix(string(k), "CUE"):
		return "cue"
	case strings.HasPrefix(string(k), "Rust"):
		return "rust"
	case k == core.KindCFunc, k == core.KindCStruct, k == core.KindCEnum, k == core.KindCTypedef:
		return "c"
	default:
		return ""
	}
}
