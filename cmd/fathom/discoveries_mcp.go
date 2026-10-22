package main

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/discoveryptr"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

func mcpDiscoveryTool(name string) string {
	tool, ok := strings.CutPrefix(name, "mcp__fathom__")
	if !ok && strings.HasPrefix(name, "mcp__plugin_") {
		_, tool, ok = strings.Cut(name, "_fathom__")
	}
	if ok && (tool == "symbol_context" || tool == "find_symbol" || tool == "read_source") {
		return tool
	}
	return ""
}

// recordMCPDiscovery tags completed lookups at the only seam that knows their
// harness owner. Daemon headers alone cannot distinguish a parent from a child
// sharing its MCP connection. Use the existing observation log, never a cache
// or an extra daemon call; hook-origin requests do not fire harness tool hooks.
func recordMCPDiscovery(in harness.HookInput) {
	if in.SessionID == "" || hookDeadlineSpent() {
		return
	}
	var args struct {
		Service string `json:"service"`
	}
	if json.Unmarshal([]byte(in.ToolInput), &args) != nil || args.Service == "" {
		return
	}
	var result any
	if json.Unmarshal([]byte(in.ToolResponse), &result) != nil {
		return
	}
	// Harnesses may expose MCP content blocks directly or inside the result
	// object. Structured content, when present, is the original typed result.
	if blocks, ok := result.([]any); ok {
		result = map[string]any{"content": blocks}
	}
	payload, ok := result.(map[string]any)
	if !ok || payload["isError"] == true {
		return
	}
	if structured, ok := payload["structuredContent"].(map[string]any); ok {
		payload = structured
	} else if blocks, ok := payload["content"].([]any); ok {
		if len(blocks) == 0 {
			return
		}
		block, _ := blocks[0].(map[string]any)
		text, _ := block["text"].(string)
		if json.Unmarshal([]byte(text), &payload) != nil {
			return
		}
	}
	tool := mcpDiscoveryTool(in.ToolName)
	ranges := mcp.ResultRangePointers(tool, payload)
	root := hookMapRoot(resolveProjDir(in.Cwd))
	if tool == "find_symbol" || (tool == "symbol_context" && len(ranges) == 0) {
		// A locator returns a span rather than source text. Snapshot that
		// local span at completion; never present it as a returned snippet.
		raw, _ := json.Marshal(payload)
		type location struct {
			Path   string          `json:"file_path"`
			Line   int             `json:"line"`
			End    int             `json:"end_line"`
			Status json.RawMessage `json:"citation_status"`
		}
		var v struct {
			Symbols []location `json:"symbols"`
			Def     location   `json:"def"`
		}
		if json.Unmarshal(raw, &v) != nil {
			return
		}
		if tool == "symbol_context" {
			v.Symbols = []location{v.Def}
		}
		for _, loc := range v.Symbols {
			if hookDeadlineSpent() || len(ranges) >= subagentDiscoveriesMaxPointers {
				break
			}
			if len(loc.Status) > 0 && string(loc.Status) != "null" {
				continue
			}
			if loc.End == 0 {
				loc.End = loc.Line
			}
			if p, _, ok := discoveryptr.Read(root, loc.Path, loc.Line, loc.End); ok {
				ranges = append(ranges, p)
			}
		}
	}
	var checked []discoveryptr.Pointer
	var pointers []string
	for _, p := range ranges {
		if hookDeadlineSpent() {
			return
		}
		if discoveryptr.NativeInstructions(p.Path) {
			continue
		}
		if _, ok := p.Current(root); !ok {
			continue
		}
		checked = append(checked, p)
		pointers = append(pointers, p.Ref())
	}
	recordObservedDiscovery(root, observedDiscoveryRow{
		SchemaVersion: observedDiscoverySchemaVersion,
		TS:            nowFunc().UTC().Format(time.RFC3339Nano),
		Service:       args.Service, SessionID: in.SessionID, AgentID: in.AgentID,
		Cwd: in.Cwd, Tool: "lookup", Pointers: pointers, RangePointers: checked,
	})
}
