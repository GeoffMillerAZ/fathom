// Package mcp — source-scanning analyzer tool registrations.
//
// registerSourceTools wires the source-scanning MCP tools that iterate
// backend.Sources, run per-file analyzers, and return aggregated sorted
// results:
//
//	shared/security group:
//	  secret_scan        — security.SecretScan
//	  shell_out          — security.ShellOut
//	  http_client_calls  — security.HTTPClientCalls
//
//	coding group:
//	  cloc               — cloc.Count aggregated by language
package mcp

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/analysis/cloc"
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
	registerCloc(s, backend)
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
		var generatedFiles, generatedBytes int
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
			// Generated files (ADR-021 §2 / TI-5) get their own bucket, same
			// mechanics as the binary bucket above: excluded from by_lang and
			// from total, so "Code generated ... DO NOT EDIT" output never
			// inflates the code-you-write line count.
			if f.Generated {
				generatedFiles++
				generatedBytes += len(f.Content)
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
			"generated": map[string]any{
				"files": generatedFiles,
				"bytes": generatedBytes,
			},
		}, nil
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
