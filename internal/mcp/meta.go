// Package mcp — meta-tools, prompts, and resources.
//
// Meta tools (capabilities, server_stats, staleness_report, list_sources)
// are registered in GroupShared.
//
// MCP protocol endpoints:
//   - prompts/list + prompts/get  (4 canned prompts)
//   - resources/list + resources/read
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// currentGitHead shells out to git to return HEAD for path.
// Returns "" when git is unavailable, the path is not a repo, or any error.
// Only called at query time (not hot path).
func currentGitHead(path string) string {
	if path == "" {
		return ""
	}
	var out bytes.Buffer
	cmd := exec.Command("git", "-C", path, "rev-parse", "HEAD")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

// ─── registerMeta ─────────────────────────────────────────────────────────────

// registerMeta wires meta-tools, prompts, and resources into s.
func registerMeta(s *Server, backend QueryInterface) {
	registerCapabilities(s)
	registerServerStats(s)
	registerStalenessReport(s, backend)
	registerListSources(s, backend)
	registerPromptsHandlers(s)
	registerResourcesHandlers(s, backend)
}

// ─── capabilities ─────────────────────────────────────────────────────────────

func registerCapabilities(s *Server) {
	s.registerTool(toolDescriptor{
		Name:        "capabilities",
		Description: "List the tools visible on this request's surface and the active surface/profile.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		group: GroupShared,
	}, func(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
		descs := s.visibleToolsCtx(ctx)
		names := make([]any, len(descs))
		for i, d := range descs {
			names[i] = d.Name
		}
		return map[string]any{
			"tools":          names,
			"active_surface": s.effectiveSurface(ctx),
			"active_profile": s.activeProfile(),
			"tool_count":     len(descs),
		}, nil
	})
}

// ─── server_stats ─────────────────────────────────────────────────────────────

func registerServerStats(s *Server) {
	s.registerTool(toolDescriptor{
		Name:        "server_stats",
		Description: "Return server uptime and total request count. Pass an optional service to also get its typed Go call-edge counts (ADR-021 §3) — direct/interface-dispatch/unresolved — the honest signal for whether a store's typed pass has run at all.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Optional service / repo identifier — adds typed_edges counts for it"},
			},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		uptime := time.Since(s.startTime)
		result := map[string]any{
			"uptime_seconds": uptime.Seconds(),
			"request_count":  s.totalRequests(),
			"started_at":     s.startTime.UTC().Format(time.RFC3339),
		}

		var p struct {
			Service string `json:"service"`
		}
		_ = json.Unmarshal(args, &p) // best-effort: absent/invalid args just skip the service block

		// Typed Go call-edge counters (ADR-021 §3): a Go service whose typed
		// pass never ran (predates ADR-021 §3, or hasn't been re-ingested
		// since) reports all-zero counts here — the visible signal ExtractorEpoch
		// intentionally does NOT force (typed edges are new observations, not
		// a Calls-semantics change, see docs/plans/callgraph-types.md).
		// Note dispatch counts skew high in Go services with many error
		// types: universe/stdlib interface dispatch (error.Error) fans out
		// to every first-party implementer (see internal/fusion/typedcalls.go).
		if p.Service != "" && s.backend != nil {
			edges, err := s.backend.TypedEdges(ctx, p.Service)
			if err == nil {
				counts := map[string]int{"direct": 0, "iface_dispatch": 0, "unresolved": 0}
				for _, e := range edges {
					switch e.Kind {
					case "direct":
						counts["direct"]++
					case "iface-dispatch":
						counts["iface_dispatch"]++
					case "unresolved":
						counts["unresolved"]++
					}
				}
				result["typed_edges"] = map[string]any{
					"service":        p.Service,
					"total":          len(edges),
					"direct":         counts["direct"],
					"iface_dispatch": counts["iface_dispatch"],
					"unresolved":     counts["unresolved"],
				}
			}
		}

		return result, nil
	})
}

// ─── staleness_report ─────────────────────────────────────────────────────────

func registerStalenessReport(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "staleness_report",
		Description: "Return a staleness summary for a service (source files, last-ingested state, and git commit drift).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "staleness_report: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "staleness_report: service required"}
		}
		// Derive a simple "sources seen" summary from ListAllSymbols.
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		seen := make(map[string]struct{}, len(nodes))
		for _, n := range nodes {
			if n.FilePath != "" {
				seen[n.FilePath] = struct{}{}
			}
		}
		sources := make([]string, 0, len(seen))
		for f := range seen {
			sources = append(sources, f)
		}
		sort.Strings(sources)

		// Fetch commit-drift metadata (best-effort; empty strings if unavailable).
		indexedCommit := ""
		currentCommit := ""
		reingestRecommended := false
		if repoPath, ic, found, metaErr := backend.ServiceMeta(ctx, p.Service); metaErr == nil && found {
			indexedCommit = ic
			// Attempt to resolve current HEAD using the stored path.
			if repoPath != "" {
				currentCommit = currentGitHead(repoPath)
			}
			if indexedCommit != "" && currentCommit != "" && indexedCommit != currentCommit {
				reingestRecommended = true
			}
		}

		resp := map[string]any{
			"service":              p.Service,
			"source_count":         len(sources),
			"sources":              stringsToAny(sources),
			"indexed_commit":       indexedCommit,
			"current_commit":       currentCommit,
			"reingest_recommended": reingestRecommended,
		}
		// Optional: append per-layer claims summary when backend supports it.
		if csb, ok := backend.(claimsSummaryBackend); ok {
			if summary, err := csb.ClaimsSummary(ctx, p.Service); err == nil && len(summary) > 0 {
				resp["layers"] = summary
			}
		}
		return resp, nil
	})
}

// ─── list_sources ─────────────────────────────────────────────────────────────

func registerListSources(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "list_sources",
		Description: "List distinct source file references ingested for a service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "list_sources: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "list_sources: service required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		seen := make(map[string]struct{}, len(nodes))
		for _, n := range nodes {
			if n.FilePath != "" {
				seen[n.FilePath] = struct{}{}
			}
		}
		sources := make([]string, 0, len(seen))
		for f := range seen {
			sources = append(sources, f)
		}
		sort.Strings(sources)
		return map[string]any{"sources": stringsToAny(sources)}, nil
	})
}

// ─── prompts ──────────────────────────────────────────────────────────────────

// promptDef is a canned prompt definition.
type promptDef struct {
	Name        string
	Description string
	Arguments   []promptArg
	// tools lists every tool the template's workflow instructs the model to
	// call; a scoped endpoint hides prompts whose tools it gates.
	tools []string
	// template is called with the arguments map to produce the messages array.
	template func(args map[string]string) []map[string]any
}

// promptArg describes one argument to a canned prompt.
type promptArg struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// cannedPrompts is the list of fathom's built-in prompts.
var cannedPrompts = []promptDef{
	{
		Name:        "investigate_incident",
		Description: "Guide an investigation into a production incident using fathom tools.",
		Arguments: []promptArg{
			{Name: "service", Description: "Service identifier to investigate", Required: true},
			{Name: "symptom", Description: "Observed symptom or error message", Required: true},
		},
		tools: []string{"entrypoints", "complexity", "call_hierarchy", "community", "staleness_report"},
		template: func(a map[string]string) []map[string]any {
			svc := a["service"]
			symptom := a["symptom"]
			return []map[string]any{
				{
					"role": "user",
					"content": map[string]any{
						"type": "text",
						"text": fmt.Sprintf(
							"I need to investigate an incident in service %q.\n\nSymptom: %s\n\n"+
								"Steps:\n"+
								"1. Use entrypoints(%[1]s) to find HTTP handlers and main functions.\n"+
								"2. Use complexity(%[1]s) to identify high-complexity hotspots.\n"+
								"3. Use call_hierarchy(%[1]s, <suspect_symbol>) to trace call paths.\n"+
								"4. Use community(%[1]s, <symbol>) to find related code.\n"+
								"5. Use staleness_report(%[1]s) to check if data is stale.\n\n"+
								"What do you find?",
							svc, symptom,
						),
					},
				},
			}
		},
	},
	{
		Name:        "review_change",
		Description: "Review a code change using fathom impact-analysis tools.",
		Arguments: []promptArg{
			{Name: "service", Description: "Service identifier where the change was made", Required: true},
			{Name: "change", Description: "Description of the change (e.g. 'added rate limiting to login endpoint')", Required: true},
		},
		tools: []string{"rename_impact", "test_impact", "api_surface", "complexity", "call_path"},
		template: func(a map[string]string) []map[string]any {
			svc := a["service"]
			change := a["change"]
			return []map[string]any{
				{
					"role": "user",
					"content": map[string]any{
						"type": "text",
						"text": fmt.Sprintf(
							"Review this change in service %q: %s\n\n"+
								"Steps:\n"+
								"1. Use rename_impact(%[1]s, <changed_symbol>) to list affected call sites.\n"+
								"2. Use test_impact(%[1]s, <symbol>) to identify tests that cover it.\n"+
								"3. Use api_surface(%[1]s) to check if the public API changed.\n"+
								"4. Use complexity(%[1]s) to see if complexity increased.\n"+
								"5. Use call_path(%[1]s, <entry>, <changed>) to trace reachability.\n\n"+
								"Is the change safe to ship?",
							svc, change,
						),
					},
				},
			}
		},
	},
	{
		Name:        "onboard_service",
		Description: "Generate an onboarding tour of a service for a new engineer.",
		Arguments: []promptArg{
			{Name: "service", Description: "Service identifier to onboard to", Required: true},
		},
		tools: []string{"entrypoints", "api_surface", "list_sources", "type_hierarchy", "god_object"},
		template: func(a map[string]string) []map[string]any {
			svc := a["service"]
			return []map[string]any{
				{
					"role": "user",
					"content": map[string]any{
						"type": "text",
						"text": fmt.Sprintf(
							"Help me understand service %q.\n\n"+
								"Steps:\n"+
								"1. Use entrypoints(%[1]s) to find the main entry points.\n"+
								"2. Use api_surface(%[1]s) to see the exported API.\n"+
								"3. Use list_sources(%[1]s) to enumerate source files.\n"+
								"4. Use type_hierarchy(%[1]s) to understand key interfaces.\n"+
								"5. Use god_object(%[1]s) to flag any over-large types.\n\n"+
								"Provide a summary tour of the service architecture.",
							svc,
						),
					},
				},
			}
		},
	},
	{
		Name:        "plan_requirement",
		Description: "Plan implementation of a new requirement using fathom structural analysis.",
		Arguments: []promptArg{
			{Name: "service", Description: "Service identifier to implement the requirement in", Required: true},
			{Name: "requirement", Description: "Description of the new requirement", Required: true},
		},
		tools: []string{"api_surface", "type_hierarchy", "entrypoints", "complexity", "dead_code"},
		template: func(a map[string]string) []map[string]any {
			svc := a["service"]
			req := a["requirement"]
			return []map[string]any{
				{
					"role": "user",
					"content": map[string]any{
						"type": "text",
						"text": fmt.Sprintf(
							"Plan how to implement this requirement in service %q: %s\n\n"+
								"Steps:\n"+
								"1. Use api_surface(%[1]s) to understand the current public API.\n"+
								"2. Use type_hierarchy(%[1]s) to find interfaces to extend.\n"+
								"3. Use entrypoints(%[1]s) to locate where new behavior should plug in.\n"+
								"4. Use complexity(%[1]s) to identify the safest files to change.\n"+
								"5. Use dead_code(%[1]s) to find any unused stubs to repurpose.\n\n"+
								"Produce a step-by-step implementation plan.",
							svc, req,
						),
					},
				},
			}
		},
	},
}

// promptIndex is a fast-lookup map built once at package init.
var promptIndex map[string]*promptDef

func init() {
	promptIndex = make(map[string]*promptDef, len(cannedPrompts))
	for i := range cannedPrompts {
		promptIndex[cannedPrompts[i].Name] = &cannedPrompts[i]
	}
}

// registerPromptsHandlers wires prompts/list and prompts/get.
func registerPromptsHandlers(s *Server) {
	s.Register("prompts/list", func(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
		items := make([]any, 0, len(cannedPrompts))
		for i := range cannedPrompts {
			p := &cannedPrompts[i]
			if !s.promptOnScope(ctx, p) {
				continue
			}
			args := make([]any, len(p.Arguments))
			for j, a := range p.Arguments {
				args[j] = map[string]any{
					"name":        a.Name,
					"description": a.Description,
					"required":    a.Required,
				}
			}
			items = append(items, map[string]any{
				"name":        p.Name,
				"description": p.Description,
				"arguments":   args,
			})
		}
		return map[string]any{"prompts": items}, nil
	})

	s.Register("prompts/get", func(ctx context.Context, params json.RawMessage) (any, *rpcError) {
		var p struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "prompts/get: invalid params"}
		}
		if p.Name == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "prompts/get: name required"}
		}
		def, ok := promptIndex[p.Name]
		if !ok {
			return nil, &rpcError{Code: ErrCodeNotFound, Message: fmt.Sprintf("prompts/get: unknown prompt %q", p.Name)}
		}
		if !s.promptOnScope(ctx, def) {
			return nil, &rpcError{Code: ErrCodeNotFound, Message: fmt.Sprintf("prompts/get: prompt %q uses tools that are not on the %s surface; POST /rpc serves the full surface", p.Name, scopeFrom(ctx).surface)}
		}
		args := p.Arguments
		if args == nil {
			args = map[string]string{}
		}
		messages := def.template(args)
		return map[string]any{
			"description": def.Description,
			"messages":    messages,
		}, nil
	})
}

// ─── resources ────────────────────────────────────────────────────────────────

const (
	manifestURI      = "fathom:///manifest"
	serviceURIPrefix = "fathom:///service/"
)

// registerResourcesHandlers wires resources/list and resources/read.
func registerResourcesHandlers(s *Server, backend QueryInterface) {
	s.Register("resources/list", func(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
		resources := []any{
			map[string]any{
				"uri":         manifestURI,
				"name":        "Fathom Manifest",
				"description": "Server manifest: registered tools, active profile, and known services.",
				"mimeType":    "application/json",
			},
		}
		// Add per-service resources when backend is available.
		if backend != nil {
			svcs, err := backend.Services(ctx)
			if err == nil {
				for _, svc := range svcs {
					resources = append(resources, map[string]any{
						"uri":         serviceURIPrefix + svc,
						"name":        "Service: " + svc,
						"description": "Symbol summary for service " + svc,
						"mimeType":    "application/json",
					})
				}
			}
		}
		return map[string]any{"resources": resources}, nil
	})

	s.Register("resources/read", func(ctx context.Context, params json.RawMessage) (any, *rpcError) {
		var p struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "resources/read: invalid params"}
		}
		if p.URI == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "resources/read: uri required"}
		}

		switch {
		case p.URI == manifestURI:
			return readManifest(s, ctx, backend)
		case strings.HasPrefix(p.URI, serviceURIPrefix):
			svcID := strings.TrimPrefix(p.URI, serviceURIPrefix)
			return readServiceResource(ctx, backend, svcID)
		default:
			return nil, &rpcError{Code: ErrCodeNotFound, Message: fmt.Sprintf("resources/read: unknown resource %q", p.URI)}
		}
	})
}

// readManifest returns the fathom:///manifest resource.
func readManifest(s *Server, ctx context.Context, backend QueryInterface) (any, *rpcError) {
	descs := s.visibleToolsCtx(ctx)
	toolNames := make([]string, len(descs))
	for i, d := range descs {
		toolNames[i] = d.Name
	}
	var svcs []string
	if backend != nil {
		var err error
		svcs, err = backend.Services(ctx)
		if err != nil {
			svcs = nil
		}
	}
	payload := map[string]any{
		"server":         "fathom",
		"version":        "1.0.0",
		"tools":          toolNames,
		"active_surface": s.effectiveSurface(ctx),
		"active_profile": s.activeProfile(),
		"services":       svcs,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "manifest marshal: " + err.Error()}
	}
	return map[string]any{
		"contents": []any{
			map[string]any{
				"uri":      manifestURI,
				"mimeType": "application/json",
				"text":     string(b),
			},
		},
	}, nil
}

// readServiceResource returns the fathom:///service/<id> resource.
func readServiceResource(ctx context.Context, backend QueryInterface, svcID string) (any, *rpcError) {
	if backend == nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
	}
	nodes, err := backend.ListAllSymbols(ctx, svcID)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
	}
	// Collect distinct file paths and kinds.
	files := make(map[string]struct{}, len(nodes))
	kinds := make(map[string]int, 16)
	for _, n := range nodes {
		if n.FilePath != "" {
			files[n.FilePath] = struct{}{}
		}
		kinds[string(n.Kind)]++
	}
	fileList := make([]string, 0, len(files))
	for f := range files {
		fileList = append(fileList, f)
	}
	sort.Strings(fileList)

	payload := map[string]any{
		"service":      svcID,
		"symbol_count": len(nodes),
		"file_count":   len(files),
		"files":        fileList,
		"kinds":        kinds,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "service resource marshal: " + err.Error()}
	}
	uri := serviceURIPrefix + svcID
	return map[string]any{
		"contents": []any{
			map[string]any{
				"uri":      uri,
				"mimeType": "application/json",
				"text":     string(b),
			},
		},
	}, nil
}
