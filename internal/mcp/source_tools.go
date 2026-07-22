// Package mcp — source-scanning analyzer tool registrations.
//
// registerSourceTools wires thirteen MCP tools that iterate backend.Sources,
// run per-file analyzers, and return aggregated sorted results:
//
//	shared/security group:
//	  secret_scan        — security.SecretScan
//	  shell_out          — security.ShellOut
//	  http_client_calls  — security.HTTPClientCalls
//
//	coding group:
//	  antipatterns       — antipattern.ScanAll
//	  cognitive          — cognitive.Analyze (Go only; others skipped)
//	  cloc               — cloc.Count aggregated by language
//	  deprecation        — deprecation.Scan
//	  env_keys           — envkeys.Scan
//
//	investigation group:
//	  markers / test_debt       — markers.Scan
//	  routes / api_routes       — routes.Extract (by lang)
//	  endpoint_inventory        — endpoints.Inventory (aggregates all routes)
//	  dependencies / deps       — deps.Detect (by filename)
package mcp

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/analysis/antipattern"
	"github.com/geoffmilleraz/fathom/internal/analysis/cloc"
	"github.com/geoffmilleraz/fathom/internal/analysis/cognitive"
	"github.com/geoffmilleraz/fathom/internal/analysis/deprecation"
	"github.com/geoffmilleraz/fathom/internal/analysis/deps"
	"github.com/geoffmilleraz/fathom/internal/analysis/endpoints"
	"github.com/geoffmilleraz/fathom/internal/analysis/envkeys"
	"github.com/geoffmilleraz/fathom/internal/analysis/markers"
	"github.com/geoffmilleraz/fathom/internal/analysis/routes"
	"github.com/geoffmilleraz/fathom/internal/analysis/security"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// registerSourceTools is called from registerTools (tools.go) and adds the
// source-scanning tools.  backend may be nil; tools that require it return
// ErrCodeInternal when backend == nil.
func registerSourceTools(s *Server, backend QueryInterface) {
	// ── shared/security ──────────────────────────────────────────────────────
	registerSecretScan(s, backend)
	registerShellOut(s, backend)
	registerHTTPClientCalls(s, backend)

	// ── coding ───────────────────────────────────────────────────────────────
	registerAntipatterns(s, backend)
	registerCognitiveTool(s, backend)
	registerCloc(s, backend)
	registerDeprecation(s, backend)
	registerEnvKeys(s, backend)

	// ── investigation / docs ─────────────────────────────────────────────────
	registerMarkers(s, backend, "markers", "Scan source files for annotation comments (TODO, FIXME, HACK, XXX, BUG, DEPRECATED).")
	registerRoutes(s, backend, "routes", "Extract HTTP route registrations from source files.")
	registerEndpointInventory(s, backend)
	registerDependencies(s, backend, "dependencies", "Parse dependency lockfiles and return a SBOM-style dependency list.")
}

// ─── shared helpers ──────────────────────────────────────────────────────────

// serviceArg parses {"service": "..."} from args and returns it, or an rpcError.
func serviceArg(args json.RawMessage, tool string) (string, *rpcError) {
	var p struct {
		Service string `json:"service"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", &rpcError{Code: ErrCodeParams, Message: tool + ": invalid args"}
	}
	if p.Service == "" {
		return "", &rpcError{Code: ErrCodeParams, Message: tool + ": service required"}
	}
	return p.Service, nil
}

// fetchSources calls backend.Sources and converts the error to rpcError.
func fetchSources(ctx context.Context, backend QueryInterface, service string) ([]core.SourceFile, *rpcError) {
	files, err := backend.Sources(ctx, service)
	if err != nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "sources: " + err.Error()}
	}
	return files, nil
}

// ─── secret_scan ─────────────────────────────────────────────────────────────

func registerSecretScan(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "secret_scan",
		Description: "Scan source files for secrets: hardcoded tokens, high-entropy strings, AWS keys, JWTs, PEM headers, and credential-named variable assignments.",
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
		service, rpcErr := serviceArg(args, "secret_scan")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var all []security.Finding
		for _, f := range files {
			if f.Lang == core.LangBinary {
				continue // never regex-scan binary content (ADR-021 §1 / LT-2)
			}
			all = append(all, security.SecretScan(f.Path, f.Content)...)
		}
		sortSecurityFindings(all)
		return map[string]any{"findings": securityFindingsToAny(all)}, nil
	})
}

// ─── shell_out ────────────────────────────────────────────────────────────────

func registerShellOut(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "shell_out",
		Description: "Find shell-out call sites: exec.Command/CommandContext in Go, subprocess in Python, child_process in JS/TS.",
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
		service, rpcErr := serviceArg(args, "shell_out")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var all []security.Finding
		for _, f := range files {
			if f.Lang == core.LangBinary {
				continue // never regex-scan binary content (ADR-021 §1 / LT-2)
			}
			all = append(all, security.ShellOut(f.Path, f.Content)...)
		}
		sortSecurityFindings(all)
		return map[string]any{"findings": securityFindingsToAny(all)}, nil
	})
}

// ─── http_client_calls ────────────────────────────────────────────────────────

func registerHTTPClientCalls(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "http_client_calls",
		Description: "Find HTTP client call sites: http.Get/Post/.Do() in Go, fetch/axios/requests in JS/TS/Python.",
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
		service, rpcErr := serviceArg(args, "http_client_calls")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var all []security.Finding
		for _, f := range files {
			if f.Lang == core.LangBinary {
				continue // never regex-scan binary content (ADR-021 §1 / LT-2)
			}
			all = append(all, security.HTTPClientCalls(f.Path, f.Content)...)
		}
		sortSecurityFindings(all)
		return map[string]any{"findings": securityFindingsToAny(all)}, nil
	})
}

// ─── antipatterns ─────────────────────────────────────────────────────────────

func registerAntipatterns(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "antipatterns",
		Description: "Detect coding anti-patterns: magic numbers, panic sites, goroutines, regex-in-loop, wall-clock usage, mutex imbalances.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		service, rpcErr := serviceArg(args, "antipatterns")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var all []antipattern.Finding
		for _, f := range files {
			if f.Lang == core.LangBinary {
				continue // never regex-scan binary content (ADR-021 §1 / LT-2)
			}
			all = append(all, antipattern.ScanAll(f.Path, f.Content)...)
		}
		sortAntipatternFindings(all)
		return map[string]any{"findings": antipatternFindingsToAny(all)}, nil
	})
}

// ─── cognitive ────────────────────────────────────────────────────────────────

func registerCognitiveTool(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "cognitive",
		Description: "Compute gocognit-style cognitive complexity for every Go function across the service. Non-Go files are skipped.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		service, rpcErr := serviceArg(args, "cognitive")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var all []cognitive.FuncCognitive
		for _, f := range files {
			if f.Lang != "go" {
				continue
			}
			results, err := cognitive.Analyze(f.Path, string(f.Content))
			if err != nil {
				continue // skip unparseable files
			}
			all = append(all, results...)
		}
		// Sort descending by score, then ascending by file/func for determinism.
		sort.Slice(all, func(i, j int) bool {
			if all[i].Score != all[j].Score {
				return all[i].Score > all[j].Score
			}
			if all[i].File != all[j].File {
				return all[i].File < all[j].File
			}
			return all[i].Func < all[j].Func
		})
		funcs := make([]any, len(all))
		for i, fc := range all {
			funcs[i] = map[string]any{
				"func":  fc.Func,
				"file":  fc.File,
				"line":  fc.Line,
				"score": fc.Score,
			}
		}
		return map[string]any{"functions": funcs}, nil
	})
}

// ─── cloc ─────────────────────────────────────────────────────────────────────

func registerCloc(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "cloc",
		Description: "Count blank, comment, and code lines per language across the service, plus a grand total.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		service, rpcErr := serviceArg(args, "cloc")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		langMap := make(map[string][]cloc.Stats)
		var binaryFiles, binaryBytes int
		for _, f := range files {
			// Binary files never contribute lines (ADR-021 §1 / LT-2): they
			// are counted in their own files/bytes bucket, excluded from
			// by_lang and from total entirely — a NUL-bearing file can never
			// inflate the code-line count the way an un-detected build
			// artifact used to.
			if f.Lang == core.LangBinary {
				binaryFiles++
				binaryBytes += len(f.Content)
				continue
			}
			lang := f.Lang
			if lang == "" {
				lang = core.LangUnclassified
			}
			s := cloc.Count(lang, string(f.Content))
			langMap[lang] = append(langMap[lang], s)
		}
		aggregated := cloc.Aggregate(langMap)
		byLang := make(map[string]any, len(aggregated))
		var total cloc.Stats
		for lang, s := range aggregated {
			byLang[lang] = map[string]any{
				"blank":   s.Blank,
				"comment": s.Comment,
				"code":    s.Code,
				"total":   s.Total(),
			}
			total.Blank += s.Blank
			total.Comment += s.Comment
			total.Code += s.Code
		}
		return map[string]any{
			"by_lang": byLang,
			"total": map[string]any{
				"blank":   total.Blank,
				"comment": total.Comment,
				"code":    total.Code,
				"total":   total.Total(),
			},
			"binary": map[string]any{
				"files": binaryFiles,
				"bytes": binaryBytes,
			},
		}, nil
	})
}

// ─── deprecation ─────────────────────────────────────────────────────────────

func registerDeprecation(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "deprecation",
		Description: "Scan source files for usage of deprecated stdlib symbols (ioutil, os.SEEK_*, etc.). Uses the built-in default list.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		service, rpcErr := serviceArg(args, "deprecation")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var all []deprecation.Finding
		for _, f := range files {
			if f.Lang == core.LangBinary {
				continue // never regex-scan binary content (ADR-021 §1 / LT-2)
			}
			all = append(all, deprecation.Scan(f.Path, string(f.Content), nil)...)
		}
		sort.Slice(all, func(i, j int) bool {
			if all[i].File != all[j].File {
				return all[i].File < all[j].File
			}
			if all[i].Line != all[j].Line {
				return all[i].Line < all[j].Line
			}
			return all[i].Symbol < all[j].Symbol
		})
		result := make([]any, len(all))
		for i, f := range all {
			result[i] = map[string]any{
				"file":   f.File,
				"line":   f.Line,
				"symbol": f.Symbol,
			}
		}
		return map[string]any{"findings": result}, nil
	})
}

// ─── env_keys ─────────────────────────────────────────────────────────────────

func registerEnvKeys(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "env_keys",
		Description: "Extract environment-variable read sites from source files (Go, JS/TS, Python patterns).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		service, rpcErr := serviceArg(args, "env_keys")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var all []envkeys.EnvKey
		for _, f := range files {
			if f.Lang == core.LangBinary {
				continue // never regex-scan binary content (ADR-021 §1 / LT-2)
			}
			all = append(all, envkeys.Scan(f.Path, string(f.Content))...)
		}
		sort.Slice(all, func(i, j int) bool {
			if all[i].File != all[j].File {
				return all[i].File < all[j].File
			}
			if all[i].Line != all[j].Line {
				return all[i].Line < all[j].Line
			}
			return all[i].Name < all[j].Name
		})
		// Deduplicate (same name may appear in multiple files; keep all per-file).
		result := make([]any, len(all))
		for i, k := range all {
			result[i] = map[string]any{
				"name": k.Name,
				"file": k.File,
				"line": k.Line,
			}
		}
		return map[string]any{"keys": result}, nil
	})
}

// ─── markers / test_debt ──────────────────────────────────────────────────────

func registerMarkers(s *Server, backend QueryInterface, name, description string) {
	s.registerTool(toolDescriptor{
		Name:        name,
		Description: description,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		service, rpcErr := serviceArg(args, name)
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var all []markers.Marker
		for _, f := range files {
			if f.Lang == core.LangBinary {
				continue // never regex-scan binary content (ADR-021 §1 / LT-2)
			}
			all = append(all, markers.Scan(f.Path, string(f.Content))...)
		}
		sort.Slice(all, func(i, j int) bool {
			if all[i].File != all[j].File {
				return all[i].File < all[j].File
			}
			if all[i].Line != all[j].Line {
				return all[i].Line < all[j].Line
			}
			return string(all[i].Kind) < string(all[j].Kind)
		})
		result := make([]any, len(all))
		for i, m := range all {
			result[i] = map[string]any{
				"file":   m.File,
				"line":   m.Line,
				"kind":   string(m.Kind),
				"text":   m.Text,
				"ticket": m.Ticket,
			}
		}
		return map[string]any{"markers": result}, nil
	})
}

// ─── routes / api_routes ──────────────────────────────────────────────────────

func registerRoutes(s *Server, backend QueryInterface, name, description string) {
	s.registerTool(toolDescriptor{
		Name:        name,
		Description: description,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		service, rpcErr := serviceArg(args, name)
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var all []routes.Route
		for _, f := range files {
			lang := routesLang(f.Lang)
			if lang == "" {
				continue
			}
			all = append(all, routes.Extract(lang, f.Path, f.Content)...)
		}
		// routes.Extract already sorts per file; sort the aggregate.
		sort.Slice(all, func(i, j int) bool {
			if all[i].Method != all[j].Method {
				return all[i].Method < all[j].Method
			}
			if all[i].Path != all[j].Path {
				return all[i].Path < all[j].Path
			}
			if all[i].File != all[j].File {
				return all[i].File < all[j].File
			}
			return all[i].Line < all[j].Line
		})
		result := make([]any, len(all))
		for i, r := range all {
			result[i] = map[string]any{
				"method":  r.Method,
				"path":    r.Path,
				"handler": r.Handler,
				"file":    r.File,
				"line":    r.Line,
			}
		}
		return map[string]any{"routes": result}, nil
	})
}

// routesLang maps a SourceFile.Lang to the lang tag accepted by routes.Extract.
// Returns "" for unsupported languages.
func routesLang(lang string) string {
	switch lang {
	case "go":
		return "go"
	case "js", "ts":
		return "js"
	case "py":
		return "python"
	}
	return ""
}

// ─── endpoint_inventory ───────────────────────────────────────────────────────

func registerEndpointInventory(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "endpoint_inventory",
		Description: "Aggregate HTTP route registrations across all source files into a deduplicated endpoint inventory.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		service, rpcErr := serviceArg(args, "endpoint_inventory")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var allRoutes []routes.Route
		for _, f := range files {
			lang := routesLang(f.Lang)
			if lang == "" {
				continue
			}
			allRoutes = append(allRoutes, routes.Extract(lang, f.Path, f.Content)...)
		}
		inv := endpoints.Inventory(allRoutes)
		result := make([]any, len(inv))
		for i, ep := range inv {
			handlers := make([]any, len(ep.Handlers))
			for j, h := range ep.Handlers {
				handlers[j] = h
			}
			files := make([]any, len(ep.Files))
			for j, fi := range ep.Files {
				files[j] = fi
			}
			lines := make([]any, len(ep.Lines))
			for j, l := range ep.Lines {
				lines[j] = l
			}
			result[i] = map[string]any{
				"method":   ep.Method,
				"path":     ep.Path,
				"handlers": handlers,
				"files":    files,
				"lines":    lines,
			}
		}
		return map[string]any{"endpoints": result}, nil
	})
}

// ─── dependencies / deps ──────────────────────────────────────────────────────

func registerDependencies(s *Server, backend QueryInterface, name, description string) {
	s.registerTool(toolDescriptor{
		Name:        name,
		Description: description,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		service, rpcErr := serviceArg(args, name)
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var all []deps.Dependency
		for _, f := range files {
			parsed, err := deps.Detect(f.Path, f.Content)
			if err != nil {
				continue // skip non-lockfile sources
			}
			all = append(all, parsed...)
		}
		// Sort by (Ecosystem, Name, Version) for determinism.
		sort.Slice(all, func(i, j int) bool {
			if all[i].Ecosystem != all[j].Ecosystem {
				return all[i].Ecosystem < all[j].Ecosystem
			}
			if all[i].Name != all[j].Name {
				return all[i].Name < all[j].Name
			}
			return all[i].Version < all[j].Version
		})
		result := make([]any, len(all))
		for i, d := range all {
			result[i] = map[string]any{
				"name":      d.Name,
				"version":   d.Version,
				"direct":    d.Direct,
				"ecosystem": d.Ecosystem,
			}
		}
		return map[string]any{"dependencies": result}, nil
	})
}

// ─── serialisation helpers ───────────────────────────────────────────────────

// securityFindingsToAny converts []security.Finding to []any for JSON.
// Returns an empty slice (never nil) so JSON encodes to [].
func securityFindingsToAny(fs []security.Finding) []any {
	if len(fs) == 0 {
		return []any{}
	}
	out := make([]any, len(fs))
	for i, f := range fs {
		out[i] = map[string]any{
			"file":    f.File,
			"line":    f.Line,
			"rule":    f.Rule,
			"message": f.Message,
		}
	}
	return out
}

// sortSecurityFindings sorts []security.Finding by (File, Line, Rule).
func sortSecurityFindings(fs []security.Finding) {
	sort.Slice(fs, func(i, j int) bool {
		if fs[i].File != fs[j].File {
			return fs[i].File < fs[j].File
		}
		if fs[i].Line != fs[j].Line {
			return fs[i].Line < fs[j].Line
		}
		return fs[i].Rule < fs[j].Rule
	})
}

// antipatternFindingsToAny converts []antipattern.Finding to []any for JSON.
func antipatternFindingsToAny(fs []antipattern.Finding) []any {
	if len(fs) == 0 {
		return []any{}
	}
	out := make([]any, len(fs))
	for i, f := range fs {
		out[i] = map[string]any{
			"file":    f.File,
			"line":    f.Line,
			"rule":    f.Rule,
			"message": f.Message,
		}
	}
	return out
}

// sortAntipatternFindings sorts []antipattern.Finding by (File, Line, Rule).
func sortAntipatternFindings(fs []antipattern.Finding) {
	sort.Slice(fs, func(i, j int) bool {
		if fs[i].File != fs[j].File {
			return fs[i].File < fs[j].File
		}
		if fs[i].Line != fs[j].Line {
			return fs[i].Line < fs[j].Line
		}
		return fs[i].Rule < fs[j].Rule
	})
}
