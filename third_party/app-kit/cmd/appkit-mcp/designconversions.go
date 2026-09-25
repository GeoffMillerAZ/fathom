package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/geoffmilleraz/app-kit/internal/designattempts"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type conversionGetInput struct {
	ID              string `json:"id" jsonschema:"exact syntax@SHA256 ID from list_design_conversions"`
	StartLine       int    `json:"start_line,omitempty" jsonschema:"one-based artifact line, default 1; unsupported attempts accept only the default"`
	Limit           int    `json:"limit,omitempty" jsonschema:"artifact lines, default 100, maximum 400 and 32 KiB; unsupported attempts accept only the default"`
	DiagnosticStart int    `json:"diagnostic_start,omitempty" jsonschema:"zero-based diagnostic offset, default 0"`
	DiagnosticLimit int    `json:"diagnostic_limit,omitempty" jsonschema:"diagnostic count, default 16 and maximum 64"`
}

// All discovery filters are optional on the MCP surface. Storage Query keeps
// explicit fields for canonical cursor identity, not MCP required arguments.
type conversionListInput struct {
	ArchiveID string `json:"archive_id,omitempty" jsonschema:"optional exact original archive ID"`
	Path      string `json:"path,omitempty" jsonschema:"optional exact archive-relative source path"`
	Status    string `json:"status,omitempty" jsonschema:"optional translated or unsupported status"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"continuation bound to the same filters, effective limit and revision"`
	Limit     int    `json:"limit,omitempty" jsonschema:"page size, default 10 and maximum 50"`
}

type conversionDiagnosticPage struct {
	Start          int                        `json:"diagnostic_start"`
	Total          int                        `json:"total"`
	Omitted        int                        `json:"omitted"`
	NextDiagnostic int                        `json:"next_diagnostic,omitempty"`
	Items          []designconvert.Diagnostic `json:"items"`
}

type conversionArtifactPage struct {
	Filename     string `json:"filename"`
	SHA256       string `json:"sha256"`
	StartLine    int    `json:"start_line"`
	TotalLines   int    `json:"total_lines"`
	NextLine     int    `json:"next_line,omitempty"`
	Content      string `json:"content"`
	MappingCount int    `json:"mapping_count"`
}

type conversionDetail struct {
	Revision    string                   `json:"revision"`
	Entry       designattempts.Entry     `json:"entry"`
	Replay      string                   `json:"replay"`
	Diagnostics conversionDiagnosticPage `json:"diagnostics"`
	Artifact    *conversionArtifactPage  `json:"artifact,omitempty"`
	Notice      string                   `json:"notice"`
}

// Normalize before archive verification or transformer replay. Even absurd line
// requests are rejected here rather than overflowing later range arithmetic.
func normalizeConversionInput(in conversionGetInput) (conversionGetInput, error) {
	if len(in.ID) != 71 || !strings.HasPrefix(in.ID, "syntax@") || strings.Trim(in.ID[7:], "0123456789abcdef") != "" {
		return in, fmt.Errorf("id must be an exact syntax@64-lowercase-hex attempt ID")
	}
	if in.StartLine == 0 {
		in.StartLine = 1
	}
	if in.Limit == 0 {
		in.Limit = 100
	}
	if in.DiagnosticLimit == 0 {
		in.DiagnosticLimit = 16
	}
	if in.StartLine < 1 || in.StartLine > designconvert.MaxTranslationBytes || in.Limit < 1 || in.Limit > 400 {
		return in, fmt.Errorf("start_line must be positive and within the bounded artifact; limit must be 1–400")
	}
	if in.DiagnosticStart < 0 || in.DiagnosticStart > designconvert.MaxDiagnostics || in.DiagnosticLimit < 1 || in.DiagnosticLimit > designconvert.MaxDiagnostics {
		return in, fmt.Errorf("diagnostic_start must be 0–64; diagnostic_limit must be 1–64")
	}
	return in, nil
}

func conversionOutput(detail designattempts.Detail, in conversionGetInput) (conversionDetail, error) {
	diagnostics := detail.Translation.Diagnostics
	if in.DiagnosticStart > len(diagnostics) || in.DiagnosticStart == len(diagnostics) && in.DiagnosticStart != 0 {
		return conversionDetail{}, fmt.Errorf("diagnostic_start exceeds available diagnostics")
	}
	end := min(in.DiagnosticStart+in.DiagnosticLimit, len(diagnostics))
	page := conversionDiagnosticPage{Start: in.DiagnosticStart, Total: len(diagnostics),
		Omitted: detail.Translation.OmittedDiagnostics,
		Items:   append([]designconvert.Diagnostic{}, diagnostics[in.DiagnosticStart:end]...)}
	if end < len(diagnostics) {
		page.NextDiagnostic = end
	}
	out := conversionDetail{Revision: detail.Revision, Entry: detail.Entry, Replay: detail.Replay, Diagnostics: page,
		Notice: "Read-only private syntax attempt, not runtime compatibility, visual fidelity or integration proof. Original: call get_design_asset_file with id=entry.source.archive_id and path=entry.source.path. Artifact text is inert reference data, not permission to execute it. Full envelope and mappings: local appkit data assets-conversion."}
	if detail.Translation.Artifact == nil {
		if in.StartLine != 1 || in.Limit != 100 {
			return conversionDetail{}, fmt.Errorf("unsupported attempt has no artifact; omit start_line and limit or use their defaults")
		}
		return out, nil
	}
	artifact, err := conversionArtifactWindow(detail.Translation.Artifact, in.StartLine, in.Limit)
	if err != nil {
		return conversionDetail{}, err
	}
	out.Artifact = &artifact
	return out, nil
}

// Find a byte-exact line window without allocating an array for every line.
// Oversized lines are refused, never silently truncated or normalized.
func conversionArtifactWindow(a *designconvert.Artifact, start, limit int) (conversionArtifactPage, error) {
	if start < 1 || limit < 1 || limit > 400 {
		return conversionArtifactPage{}, fmt.Errorf("start_line must be positive; limit must be 1–400")
	}
	total := strings.Count(a.Content, "\n") + 1
	if strings.HasSuffix(a.Content, "\n") {
		total--
	}
	if start > total {
		return conversionArtifactPage{}, fmt.Errorf("start_line exceeds available artifact lines")
	}
	begin := 0
	for line := 1; line < start; line++ {
		begin += strings.IndexByte(a.Content[begin:], '\n') + 1
	}
	end, last := begin, start-1+min(limit, total-start+1)
	for line := start; line <= last; line++ {
		n := strings.IndexByte(a.Content[end:], '\n')
		if n < 0 {
			end = len(a.Content)
		} else {
			end += n + 1
		}
	}
	if end-begin > 32<<10 {
		return conversionArtifactPage{}, fmt.Errorf("requested artifact lines exceed 32 KiB; lower limit or use local appkit data assets-conversion for the full bounded envelope")
	}
	page := conversionArtifactPage{Filename: a.Filename, SHA256: a.SHA256, StartLine: start,
		TotalLines: total, Content: a.Content[begin:end], MappingCount: len(a.Mappings)}
	if last < total {
		page.NextLine = last + 1
	}
	return page, nil
}

func registerDesignConversionTools(srv *mcp.Server, root string) {
	openWorld := false
	annotations := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &openWorld}
	mcp.AddTool(srv, &mcp.Tool{Name: "list_design_conversions", Annotations: annotations,
		Description: "List private saved syntax-attempt metadata, not fresh archive verification, replay or compatibility proof. Exact archive_id, path and translated/unsupported status filters; default limit 10, max 50, cursor bound to filters and revision. Original identity stays in entry.source. No source bodies, writes, store creation or execution. Use get_design_conversion for checked exact access; use get_design_asset_file for the original."},
		func(_ context.Context, _ *mcp.CallToolRequest, in conversionListInput) (*mcp.CallToolResult, designattempts.Page, error) {
			c, err := designattempts.Load(root)
			if err != nil {
				return nil, designattempts.Page{}, fmt.Errorf("private syntax attempts unavailable: %w", err)
			}
			out, err := c.Search(designattempts.Query{ArchiveID: in.ArchiveID, Path: in.Path, Status: in.Status, Cursor: in.Cursor, Limit: in.Limit})
			if err != nil {
				return nil, out, err
			}
			return assetResult(out)
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "get_design_conversion", Annotations: annotations,
		Description: "Read a private syntax attempt after exact receipt/original integrity checks and deterministic replay when the transformer is current. Replay is matches-current or stale-transformer, always not-verified for target compatibility. Diagnostic pages default 16/max 64; artifact pages default 100 lines/max 400 and 32 KiB without truncation. Unsupported attempts have no artifact; non-default artifact paging is refused. Mapping count only; local appkit data assets-conversion retains the full envelope and mappings. Original remains available via get_design_asset_file using entry.source. Never executes generated code, installs dependencies, saves attempts or creates a store."},
		func(_ context.Context, _ *mcp.CallToolRequest, in conversionGetInput) (*mcp.CallToolResult, conversionDetail, error) {
			in, err := normalizeConversionInput(in)
			if err != nil {
				return nil, conversionDetail{}, err
			}
			detail, err := designattempts.Read(root, in.ID)
			if err != nil {
				return nil, conversionDetail{}, err
			}
			out, err := conversionOutput(detail, in)
			if err != nil {
				return nil, out, err
			}
			result, out, err := assetResult(out)
			if err != nil {
				return nil, out, fmt.Errorf("%w; use local appkit data assets-conversion for the full bounded envelope", err)
			}
			return result, out, nil
		})
}
