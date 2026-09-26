// Package mcp — fleet/platform MCP tool registrations.
//
// registerPlatformTools wires three tools in GroupPlatform:
//
//	list_services        — services with metadata (languages, symbol counts)
//	fleet_drift          — config-style version/kind drift across services
//	cross_repo_callers   — callers of a symbol across all services via Calls edges
package mcp

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

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
	// UnsupportedLanguagesFor returns, by language display name (e.g.
	// "kotlin"), the count of files for service whose extension names a
	// known programming language fathom has no registered extractor for
	// (O69, connector.UnsupportedLanguageForPath) — the Long Code Arena
	// bench run's bumble-tech/appyx finding: a real Kotlin repo indexed as
	// near-empty with no signal anywhere that Kotlin was the reason.
	// Implementations that cannot compute this (no source-fetching
	// capability) return nil.
	UnsupportedLanguagesFor(service string) map[string]int
}

// registerPlatformTools registers the five fleet-level tools into s. The
// QueryInterface (may be nil) supplies each service's recorded corpus path
// for list_services — the authoritative repo↔service identity the hooks
// resolve against (dogfood O15: directory-basename guessing silently broke
// injection when the dir name and service ID diverged).
func registerPlatformTools(s *Server, backend QueryInterface, fleet FleetBackend) {
	registerListServices(s, backend, fleet)
	registerFleetDrift(s, fleet)
	registerCrossRepoCallers(s, fleet, "cross_repo_callers", "Find all callers of a symbol across every service in the fleet via stored call edges.")
}

// ─── list_services ────────────────────────────────────────────────────────────

// ServiceSummary is one list_services row's identity: the service, its
// symbol count, the languages its symbols come from, and its recorded
// corpus root ("" when none is recorded).
type ServiceSummary struct {
	Service     string
	SymbolCount int
	Languages   []string
	Path        string
}

// ServiceSummaries computes list_services' identity fields in process. The
// tool handler and the daemon-hosted console both call it, so the console
// shows exactly what list_services returns without a loopback HTTP round
// trip — which timed out under load and rendered project pages as "store
// unreadable" — and without writing a call-log row for a page render.
func ServiceSummaries(ctx context.Context, backend QueryInterface, fleet FleetBackend) []ServiceSummary {
	if fleet == nil {
		return nil
	}
	svcs := fleet.Services()
	out := make([]ServiceSummary, 0, len(svcs))
	for _, svc := range svcs {
		nodes := fleet.SymbolsFor(svc)
		sum := ServiceSummary{Service: svc, SymbolCount: len(nodes), Languages: langStrings(nodes)}
		if backend != nil {
			if path, _, found, err := backend.ServiceMeta(ctx, svc); err == nil && found && path != "" {
				sum.Path = path
			}
		}
		out = append(out, sum)
	}
	return out
}

func registerListServices(s *Server, backend QueryInterface, fleet FleetBackend) {
	s.registerTool(toolDescriptor{
		Name:        "list_services",
		Description: "Lists every service in the fleet with languages, symbol counts, and corpus path. Reach for it when you need indexed repos, or an exact service id. Required: none.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		group: GroupPlatform,
	}, func(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
		if fleet == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no fleet backend configured"}
		}
		sums := ServiceSummaries(ctx, backend, fleet)
		entries := make([]any, 0, len(sums))
		for _, sum := range sums {
			svc := sum.Service
			langs := make([]any, len(sum.Languages))
			for i, l := range sum.Languages {
				langs[i] = l
			}
			entry := map[string]any{
				"service":      svc,
				"symbol_count": sum.SymbolCount,
				"languages":    langs,
			}
			// Recorded corpus path (ServiceMeta) — empty/omitted when the
			// backend is absent or the service has no recorded root (e.g.
			// remote corpora). Always absolute post-O10.
			if sum.Path != "" {
				entry["path"] = sum.Path
			}
			// Unsupported languages (O69): a known programming language with
			// no registered extractor, named and counted rather than left
			// silently indistinguishable from a service that classifies
			// every file it has. Omitted when there is nothing to report —
			// the common case stays exactly as quiet as it always was.
			// Typed-edge outcome (wave P3 / O82, .fathom/observations.md):
			// the SAME per-service TypedEdgeStatus doctor's typed_edges
			// row and the ingest response's own typed_edges block read —
			// one source of truth, so list_services can never disagree
			// with either about whether a Go service's graph is typed.
			// known:false (no ingest of this service has run in THIS
			// daemon process) omits the block entirely — never a
			// fabricated clean pass.
			if backend != nil {
				if teb, ok := backend.(typedEdgeStatusBackend); ok {
					if st, known := teb.TypedEdgeStatus(svc); known {
						te := map[string]any{
							"packages_seen":      st.PackagesSeen,
							"packages_processed": st.Packages,
							"edges":              st.EdgesTotal,
							"failed":             st.Failed,
						}
						if st.Reason != "" {
							te["error"] = st.Reason
						}
						if st.Fallback != "" {
							te["fallback"] = st.Fallback
						}
						// DG-9: a discovery gap (a call target file that
						// EXISTS on disk despite being excluded from the
						// connector's allowed set — a likely wedged
						// Discover pass, not a legitimate exclusion) was
						// computed and deduped every run but never left
						// typedcalls.go's own stderr line. Omitted when
						// zero — the common, healthy case stays as quiet
						// as it always was.
						if st.DiscoveryGaps > 0 {
							te["discovery_gaps"] = st.DiscoveryGaps
							if len(st.DiscoveryGapFiles) > 0 {
								te["discovery_gap_files"] = st.DiscoveryGapFiles
							}
						}
						// finding [4]/DG-10: Persisted/RecordedAt exist SPECIFICALLY
						// so a caller can tell a store-replayed (pre-restart) answer
						// apart from one this process just computed -- omitted before
						// this fix, so a restart-stale PASS read identically to a
						// fresh one on this exact surface.
						if st.Persisted {
							te["persisted"] = true
							te["recorded_at"] = st.RecordedAt.UTC().Format(time.RFC3339)
						}
						entry["typed_edges"] = te
					}
				}
			}
			if unsupported := fleet.UnsupportedLanguagesFor(svc); len(unsupported) > 0 {
				names := make([]string, 0, len(unsupported))
				for lang := range unsupported {
					names = append(names, lang)
				}
				sort.Strings(names)
				rows := make([]any, len(names))
				for i, lang := range names {
					rows[i] = map[string]any{"language": lang, "files": unsupported[lang]}
				}
				entry["unsupported_languages"] = rows
			}
			// DG-18: a service whose recorded corpus root has vanished
			// (e.g. a cleaned-up scratch clone) is reported structurally
			// rather than via a live re-walk turning into a bare error —
			// root_missing:true plus a remedy naming how to drop the
			// stale record.
			if frb, ok := fleet.(fleetRootStatusBackend); ok {
				if missing, remedy := frb.RootMissing(svc); missing {
					entry["root_missing"] = true
					entry["remedy"] = remedy
				}
			}
			entries = append(entries, entry)
		}
		return map[string]any{"services": entries, "count": len(sums)}, nil
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
	langs := langStrings(nodes)
	out := make([]any, len(langs))
	for i, l := range langs {
		out[i] = l
	}
	return out
}

// langStrings is langSet's sorted, de-duplicated language list as strings.
func langStrings(nodes []core.SymbolNode) []string {
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
	return langs
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
