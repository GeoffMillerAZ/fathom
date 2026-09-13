// Package mcp — discoveries: receipts on the hand-off (RC-25..RC-27,
// docs/vision/28-receipts.md §4).
//
// A discovery is a pointer a tool answer established (a symbol id in the
// same untyped "pkgDir.Name" grammar cmd/fathom's `ctx:` plan annotation
// already uses, or a repo-relative file path) — never rendered text. This
// file extracts that pointer set from a tool's STRUCTURED result
// (resultPointers, called from dispatch.go's loggedToolsCall) and serves
// the pull-form "discoveries" MCP tool that re-resolves them fresh against
// the live index at read time, exactly like `fathom plans context`'s
// ctxIndex does for a plan row's `ctx:` pointers (cmd/fathom/
// planscontext.go) — the two surfaces deliberately share the pointer
// grammar, not any code (different packages, same shape).
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
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
		add(q)
		for _, raw := range asSlice(payload["definitions"]) {
			if m, ok := raw.(map[string]any); ok {
				add(asString(m["file"]))
			}
		}
		for _, key := range []string{"callers", "occurrences"} {
			for _, raw := range asSlice(payload[key]) {
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

// ─── the "discoveries" MCP tool (RC-27) ─────────────────────────────────────

// discoveriesDefaultBudget mirrors cmd/fathom's `fathom discoveries`
// CLI default (RC-26) — the same render, on the agent surface.
const discoveriesDefaultBudget = 4000

// discoveryRow is one ranked, resolved pointer on the wire.
type discoveryRow struct {
	Pointer  string `json:"pointer"`
	Count    int    `json:"count"`
	LastSeen string `json:"last_seen"`
	Resolved bool   `json:"resolved"`
	Line     string `json:"line"`
}

// discoveriesResult is the discoveries tool's payload (also outputschema.go's
// agentResultTypes row).
type discoveriesResult struct {
	Service  string         `json:"service"`
	Since    string         `json:"since,omitempty"`
	Budget   int            `json:"budget"`
	Pointers []discoveryRow `json:"pointers"`
	Rendered string         `json:"rendered"`
	Note     string         `json:"note,omitempty"`
}

// registerDiscoveries wires the "discoveries" tool: the pull form of
// `fathom discoveries` on the agent surface — this repo's recent call-log
// pointers (plus RC-26's own thread-pointer half, CLI-only for now: the
// tool answers from THIS daemon's own call log, which is what an agent
// mid-session actually wants), reduced, ranked by recency then frequency,
// and re-resolved fresh against the live index.
func registerDiscoveries(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "discoveries",
		Description: "What this session (or the planner) already established, resolved fresh — read before exploring. Pulls this repo's recent call-log pointers (symbol ids, files), deduped and ranked by recency then frequency, each re-resolved against the LIVE index — a pointer that no longer resolves is marked stale, never served as fact. Reach for it when you are picking up a hand-off (a subagent, or the start of a new session) and want to know what was already found before spending a call re-finding it. Required: service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"since":   map[string]any{"type": "string", "description": "RFC3339 timestamp; only pointers established at/after it (default: all recorded history)"},
				"budget":  map[string]any{"type": "integer", "description": fmt.Sprintf("Byte budget for the rendered list (default %d)", discoveriesDefaultBudget)},
			},
			"required": []string{"service"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		var p struct {
			Service string `json:"service"`
			Since   string `json:"since"`
			Budget  int    `json:"budget"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "discoveries: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "discoveries: service required"}
		}
		budget := p.Budget
		if budget <= 0 {
			budget = discoveriesDefaultBudget
		}
		var since time.Time
		if p.Since != "" {
			t, err := time.Parse(time.RFC3339, p.Since)
			if err != nil {
				return nil, &rpcError{Code: ErrCodeParams, Message: "discoveries: since: " + err.Error()}
			}
			since = t
		}
		logPath := s.callLogger.Path()
		if logPath == "" {
			return discoveriesResult{Service: p.Service, Budget: budget, Pointers: []discoveryRow{}, Note: "no call log configured on this daemon"}, nil
		}
		agg := aggregateCallLogPointers(logPath, p.Service, since)

		var nodes []core.SymbolNode
		sourceSet := map[string]bool{}
		if backend != nil {
			nodes, _ = backend.ListAllSymbols(ctx, p.Service)
			if sources, err := backend.Sources(ctx, p.Service); err == nil {
				for _, sf := range sources {
					sourceSet[sf.Path] = true
				}
			}
		}

		rows := make([]discoveryRow, 0, len(agg))
		lines := make([]string, 0, len(agg))
		for _, e := range agg {
			line, resolved := renderDiscoveryPointer(e.Pointer, e.Count, nodes, sourceSet)
			rows = append(rows, discoveryRow{Pointer: e.Pointer, Count: e.Count, LastSeen: e.Last.UTC().Format(time.RFC3339), Resolved: resolved, Line: line})
			lines = append(lines, line)
		}
		rendered, _ := budgetDiscoveryLines(lines, budget)
		res := discoveriesResult{Service: p.Service, Budget: budget, Pointers: rows, Rendered: rendered}
		if p.Since != "" {
			res.Since = since.UTC().Format(time.RFC3339)
		}
		if len(rows) == 0 {
			res.Note = "no discoveries recorded for this service in the window"
		}
		return res, nil
	})
}

// discoveryPointerAgg is one pointer's aggregate over the call-log window:
// how many rows established it, and the most recent one.
type discoveryPointerAgg struct {
	Pointer string
	Count   int
	Last    time.Time
}

// aggregateCallLogPointers reads logPath's JSONL rows (callLogEntry's own
// shape) and folds every Pointers entry for rows matching service (when
// non-empty) with ts >= since into a ranked, deduped list: most recent
// first, then most frequent, then lexical (determinism — internal/CLAUDE.md).
// A missing or malformed log degrades to an empty result, never an error —
// discoveries is a best-effort convenience surface, not a gate.
func aggregateCallLogPointers(logPath, service string, since time.Time) []discoveryPointerAgg {
	f, err := os.Open(logPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	agg := map[string]*discoveryPointerAgg{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var row struct {
			TS       string   `json:"ts"`
			Service  string   `json:"service"`
			Pointers []string `json:"pointers"`
		}
		if json.Unmarshal(line, &row) != nil || len(row.Pointers) == 0 {
			continue
		}
		if service != "" && row.Service != service {
			continue
		}
		ts, terr := time.Parse(time.RFC3339Nano, row.TS)
		if terr != nil || ts.Before(since) {
			continue
		}
		for _, p := range row.Pointers {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			e, ok := agg[p]
			if !ok {
				e = &discoveryPointerAgg{Pointer: p}
				agg[p] = e
			}
			e.Count++
			if ts.After(e.Last) {
				e.Last = ts
			}
		}
	}
	out := make([]discoveryPointerAgg, 0, len(agg))
	for _, e := range agg {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Last.Equal(out[j].Last) {
			return out[i].Last.After(out[j].Last)
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Pointer < out[j].Pointer
	})
	return out
}

// citationSuffixRE matches an obligations-style source citation appended to
// a file pointer: "path#Lstart-Lend" (internal/semantic.Obligation.Src's
// own format).
var citationSuffixRE = regexp.MustCompile(`^(.+)#(L\d+-L\d+)$`)

// stripCitation splits a citation-shaped pointer into its file path and the
// line-range suffix; a plain pointer (file or symbol) returns it unchanged
// with an empty citation.
func stripCitation(ptr string) (filePath, citation string) {
	if m := citationSuffixRE.FindStringSubmatch(ptr); m != nil {
		return m[1], m[2]
	}
	return ptr, ""
}

// splitDiscoveryPointer splits a "pkgDir.Name" pointer on its LAST dot —
// the same convention cmd/fathom's splitPointer (planscontext.go) uses.
func splitDiscoveryPointer(ptr string) (pkgHint, name string) {
	if i := strings.LastIndex(ptr, "."); i >= 0 {
		return ptr[:i], ptr[i+1:]
	}
	return "", ptr
}

// findDiscoverySymbol looks for name among nodes, preferring one whose
// directory matches pkgHint but falling back to any name match — the same
// "narrow, else the full set" rule cmd/fathom's storeCtxIndex.resolveSymbol
// applies, so a stale/renamed package hint never hides a symbol that still
// resolves unambiguously by bare name.
func findDiscoverySymbol(nodes []core.SymbolNode, pkgHint, name string) (core.SymbolNode, bool) {
	if pkgHint != "" {
		for _, n := range nodes {
			if n.Name == name && path.Dir(n.FilePath) == pkgHint {
				return n, true
			}
		}
	}
	for _, n := range nodes {
		if n.Name == name {
			return n, true
		}
	}
	return core.SymbolNode{}, false
}

// renderDiscoveryPointer resolves ptr FRESH against nodes/sourceSet (both
// read moments before this call, from the live index) and renders one
// line: "read <path>" for a file, "symbol <ptr> → file:line" for a
// resolved symbol, or "<ptr> — stale (unresolved)" when neither holds —
// never rendered from anything stored at write time (ADR-028).
func renderDiscoveryPointer(ptr string, count int, nodes []core.SymbolNode, sourceSet map[string]bool) (line string, resolved bool) {
	freq := ""
	if count > 1 {
		freq = fmt.Sprintf(" (%d)", count)
	}
	filePath, citation := stripCitation(ptr)
	if sourceSet[filePath] {
		suffix := ""
		if citation != "" {
			suffix = "#" + citation
		}
		return fmt.Sprintf("read %s%s%s", filePath, suffix, freq), true
	}
	pkgHint, name := splitDiscoveryPointer(ptr)
	if n, ok := findDiscoverySymbol(nodes, pkgHint, name); ok {
		return fmt.Sprintf("symbol %s → %s:%d%s", ptr, n.FilePath, n.Line, freq), true
	}
	return fmt.Sprintf("%s — stale (unresolved)%s", ptr, freq), false
}

// budgetDiscoveryLines joins lines up to budget bytes, cutting whole lines
// from the bottom and naming the cut (law 3) — the same elision shape
// cmd/fathom's budgetContextPack uses for ctx: packs.
func budgetDiscoveryLines(lines []string, budget int) (string, int) {
	if len(lines) == 0 {
		return "", 0
	}
	var b strings.Builder
	included := 0
	for _, l := range lines {
		row := l + "\n"
		if budget > 0 && b.Len()+len(row) > budget && included > 0 {
			break
		}
		b.WriteString(row)
		included++
	}
	dropped := len(lines) - included
	if dropped > 0 {
		fmt.Fprintf(&b, "…%d more: fathom discoveries --budget %d\n", dropped, budget*2)
	}
	return b.String(), dropped
}
