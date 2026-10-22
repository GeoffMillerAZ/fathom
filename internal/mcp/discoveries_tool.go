// Package mcp — discoveries: receipts on the hand-off (RC-25, docs/vision/
// 28-receipts.md §4).
//
// A discovery is a pointer a tool answer established (a symbol id in the
// same untyped "pkgDir.Name" grammar cmd/fathom's `ctx:` plan annotation
// already uses, or a repo-relative file path) — never rendered text. This
// file extracts that pointer set from a tool's STRUCTURED result
// (resultPointers, called from dispatch.go's loggedToolsCall) for the
// agent-surface call-log rows `fathom plans context`'s ctxIndex resolves
// the same way for a plan row's `ctx:` pointers (cmd/fathom/
// planscontext.go) — the two surfaces deliberately share the pointer
// grammar, not any code (different packages, same shape). The pull-form
// "discoveries" MCP tool this file used to also serve was unregistered by
// SR-29 (surface reckoning F-47, RC-27's re-anchor); the SubagentStart
// hook's push-form render (subagentDiscoveries, cmd/fathom/discoveries.go)
// covers the need now.
package mcp

import (
	"encoding/json"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/discoveryptr"
	"path"
	"strings"
)

// maxResultPointers caps resultPointers' extraction per call (RC-25): a
// discovery brief is a pointer list, not a second copy of the result.
const maxResultPointers = 8

// resultPointers extracts a call's top discovery pointers from its
// STRUCTURED result — never from rendered text (doctrine: 21 §4, "no
// transcript mining" applies equally here). args is the tool's raw
// argument object (some tools, e.g. outline, never echo the queried path
// back in their result, so the pointer comes from the request instead).
// Unrecognized tools, and a result that fails to unwrap, return nil — a
// silent no-op, exactly like every other opt-in extraction in
// calllog.go (claimsAttachedFrom, elidedCountFrom).
func resultPointers(toolName string, args json.RawMessage, result any) []string {
	payload, ok := unwrapDiscoveryPayload(result)
	if !ok {
		return nil
	}
	var argp struct {
		Symbol string `json:"symbol"`
		Query  string `json:"query"`
		Name   string `json:"name"`
		File   string `json:"file"`
		Regex  bool   `json:"regex"`
	}
	_ = json.Unmarshal(args, &argp)

	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || len(out) >= maxResultPointers {
			return
		}
		for _, existing := range out {
			if existing == p {
				return
			}
		}
		out = append(out, p)
	}
	addFile := func(loc string) {
		if i := strings.Index(loc, ":"); i >= 0 {
			loc = loc[:i]
		}
		add(loc)
	}

	switch toolName {
	case "find_symbol", "query_symbols":
		for _, key := range []string{"symbols", "nodes"} {
			for _, raw := range asSlice(payload[key]) {
				m, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				add(symbolPointer(asString(m["name"]), asString(m["file_path"])))
			}
		}
	case "symbol_context":
		if def, ok := payload["def"].(map[string]any); ok {
			add(symbolPointer(asString(def["name"]), asString(def["file_path"])))
		} else if argp.Symbol != "" {
			add(argp.Symbol)
		}
	case "references", "find_callers":
		q := argp.Query
		if q == "" {
			q = argp.Symbol
		}
		defs := asSlice(payload["definitions"])
		callerRows := asSlice(payload["callers"])
		occRows := asSlice(payload["occurrences"])
		// MINOR (V-review): the raw query string is a pointer worth
		// re-resolving only when the call actually found something — a
		// zero-result miss has nothing to hand off — AND was a literal
		// symbol/text lookup: `regex: true` means q is a REGEXP, not a
		// resolvable symbol name or file path the discoveries grammar
		// (cmd/fathom's ctx: pointers) can ever re-resolve.
		if !argp.Regex && (len(defs) > 0 || len(callerRows) > 0 || len(occRows) > 0) {
			add(q)
		}
		for _, raw := range defs {
			if m, ok := raw.(map[string]any); ok {
				add(asString(m["file"]))
			}
		}
		for _, rows := range [][]any{callerRows, occRows} {
			for _, raw := range rows {
				if m, ok := raw.(map[string]any); ok {
					addFile(asString(m["loc"]))
				}
			}
		}
	case "obligations":
		add(asString(payload["file"]))
		for _, raw := range asSlice(payload["rules"]) {
			if m, ok := raw.(map[string]any); ok {
				add(asString(m["src"]))
			}
		}
	case "outline":
		add(argp.File)
	case "read_source":
		f := asString(payload["file"])
		if f == "" {
			f = argp.File
		}
		add(f)
	}
	return out
}

// unwrapDiscoveryPayload reduces a tools/call result to its plain JSON
// object — mirroring claimsAttachedFrom/elidedCountFrom's two-shape
// handling exactly: a batch sub-call hands resultPointers the raw handler
// map directly, while the top-level dispatch path hands it dispatch.go's
// wrapped MCP envelope ({"content":[{"type":"text","text":"<json>"}]}),
// whose text must be re-parsed to reach the tool's own fields.
func unwrapDiscoveryPayload(result any) (map[string]any, bool) {
	m, ok := result.(map[string]any)
	if !ok {
		return nil, false
	}
	content, ok := m["content"].([]any)
	if !ok {
		return m, true // batch sub-call: already the raw handler result
	}
	if len(content) == 0 {
		return nil, false
	}
	first, ok := content[0].(map[string]any)
	if !ok {
		return nil, false
	}
	text, ok := first["text"].(string)
	if !ok {
		return nil, false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil, false
	}
	return payload, true
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

// symbolPointer renders a resolved symbol as a "pkgDir.Name" pointer — the
// same convention cmd/fathom's ctx: grammar (splitPointer, planscontext.go)
// resolves. A file_path with no directory (a root-level file, or one the
// tool did not carry) yields a bare name, which the resolver already
// treats as "no package hint".
func symbolPointer(name, filePath string) string {
	if name == "" {
		return ""
	}
	if filePath == "" {
		return name
	}
	dir := path.Dir(filePath)
	if dir == "." || dir == "/" {
		return name
	}
	return dir + "." + name
}

// registerDiscoveries (RC-27, docs/vision/28-receipts.md §4) — the pull
// form of the call-log pointer brief — was unregistered by SR-29 (surface
// reckoning F-47): 0 lifetime calls; the SubagentStart hook's push-form
// render (subagentDiscoveries, cmd/fathom/discoveries.go) already covers
// the need. Its own resolution/rendering pipeline (discoveryRow,
// discoveriesResult, aggregateCallLogPointers, renderDiscoveryPointer,
// budgetDiscoveryLines, and their small helpers) went with it — reachable
// solely from the removed handler, per design §2 rule 1.

// ResultRangePointers records the actual returned source, not a later read of
// the same path. The completed-tool observer uses this extraction too, with
// the harness's session/agent identity. Clipped lines fail the live hash check.
func ResultRangePointers(tool string, result any) []discoveryptr.Pointer {
	if tool != "read_source" && tool != "symbol_context" {
		return nil
	}
	payload, ok := unwrapDiscoveryPayload(result)
	if !ok {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	var v struct {
		File string `json:"file"`
		Def  struct {
			File string `json:"file_path"`
		} `json:"def"`
		Lines   []sourceLine `json:"lines"`
		Snippet []sourceLine `json:"snippet"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return nil
	}
	lines := v.Lines
	if tool == "symbol_context" {
		v.File = v.Def.File
		lines = v.Snippet
	}
	if !discoveryptr.PathLike(v.File) || len(lines) == 0 {
		return nil
	}
	texts := make([]string, len(lines))
	for i, l := range lines {
		if l.Line < 1 || l.Line != lines[0].Line+i {
			return nil
		}
		texts[i] = l.Text
	}
	hash, ok := core.HashLines([]byte(strings.Join(texts, "\n")), 1, len(lines))
	if !ok {
		return nil
	}
	return []discoveryptr.Pointer{{Path: v.File, Start: lines[0].Line, End: lines[len(lines)-1].Line, Hash: hash}}
}
