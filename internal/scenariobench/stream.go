// Stream parsing shared by ClaudeHarness (SB-14): reading a `claude -p
// --output-format stream-json --verbose` transcript (one JSON event per
// line) into the harness-independent SessionResult shape (types.go). The
// approach mirrors internal/contextbench's transcript.go/metrics.go, which
// this package does not import — the two benches have different result
// shapes and this package must stay buildable and testable on its own.
package scenariobench

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ToolSearchToolName is the harness's deferred-schema loader as it appears
// in a stream transcript (Claude Code 2.1.x defers MCP tool schemas).
const ToolSearchToolName = "ToolSearch"

// editToolNames are the tool_use names ParseSessionStream treats as an
// edit for FirstEditTurn / FilesEdited / box-check detection.
var editToolNames = map[string]bool{"Edit": true, "Write": true, "MultiEdit": true}

// readToolNames are the tool_use names ParseSessionStream treats as a file
// read for FilesRead/BytesRead: the Read tool (file_path, absolute) and
// fathom's read_source MCP tool (file, repo-relative — internal/mcp/
// readsource.go).
var readToolNames = map[string]bool{"Read": true, "mcp__fathom__read_source": true}

// boxCheckRE pulls a plan-grammar-v2-shaped {id: ..., symbol: ..., package:
// ...} annotation off a checklist line, when one is present. Every key is
// optional; a line with none still counts as a box check, just with no
// row/symbol/package attribution.
var (
	boxCheckRE = regexp.MustCompile(`\{([^}]*)\}`)
	boxCheckKV = regexp.MustCompile(`(\w+)\s*:\s*"?([^,"}]*)"?`)
)

// streamEvent is the subset of a stream-json line this parser reads.
type streamEvent struct {
	Type    string `json:"type"`
	Message struct {
		Content []streamContentBlock `json:"content"`
		Usage   struct {
			InputTokens              int64 `json:"input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
	// The terminal `{"type":"result",...}` event carries the same shape as
	// `claude -p --output-format json`'s envelope.
	IsError      bool    `json:"is_error"`
	NumTurns     int     `json:"num_turns"`
	DurationMS   int64   `json:"duration_ms"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int64 `json:"input_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
	} `json:"usage"`
}

type streamContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"` // tool_result: string or []block
}

// parsedStream is what one pass over a transcript accumulates, in the
// vocabulary ParseSessionStream's caller (ClaudeHarness.RunSession) turns
// into a SessionResult — kept here, not exported, so ParseSessionStream
// itself stays the one public entry point per SB-14.
type parsedStream struct {
	turns             int
	toolSearchTurns   int
	firstEditTurn     int
	tokensSoFar       int64
	tokensToFirstEdit int64
	filesRead         map[string]bool
	filesEdited       map[string]bool
	bytesRead         int64
	boxChecks         []BoxCheck
	toolCalls         map[string]int
	toolByID          map[string]string // tool_use id -> tool name, for attributing tool_result bytes
	payloadBytes      int64
	sawResult         bool
	result            streamEvent
}

// ParseSessionStream reads a captured `claude -p --output-format
// stream-json --verbose` transcript at path and derives a SessionResult
// (SB-14): turns, ToolSearch turns, tokens, cost, the first edit turn and
// the tokens spent to reach it, files read (with bytes) via the Read tool
// and fathom's read_source, files edited via Edit/Write/MultiEdit, box
// checks (an edit that adds "- [x]" to a markdown file), tool-call counts
// and total tool-result payload bytes. Session/Role/RowID and RawPath are
// left zero here — ClaudeHarness.RunSession fills them from the request it
// made, and relativizes file paths to the worktree.
func ParseSessionStream(path string) (SessionResult, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return SessionResult{}, fmt.Errorf("reading session stream %s: %w", path, err)
	}
	return parseSessionStreamBytes(raw), nil
}

func parseSessionStreamBytes(transcript []byte) SessionResult {
	ps := &parsedStream{
		filesRead:   map[string]bool{},
		filesEdited: map[string]bool{},
		toolCalls:   map[string]int{},
		toolByID:    map[string]string{},
	}
	checkedLines := map[string]map[string]bool{} // file -> "- [x] ..." lines already seen

	sc := bufio.NewScanner(bytes.NewReader(transcript))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var ev streamEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "assistant":
			ps.handleAssistant(ev, checkedLines)
		case "user":
			ps.handleUser(ev)
		case "result":
			ps.sawResult = true
			ps.result = ev
		}
	}
	return ps.toSessionResult()
}

func (ps *parsedStream) handleAssistant(ev streamEvent, checkedLines map[string]map[string]bool) {
	ps.turns++
	turn := ps.turns
	sawToolSearch := false
	turnTokens := ev.Message.Usage.InputTokens + ev.Message.Usage.CacheCreationInputTokens +
		ev.Message.Usage.CacheReadInputTokens + ev.Message.Usage.OutputTokens
	for _, c := range ev.Message.Content {
		if c.Type != "tool_use" {
			continue
		}
		ps.toolCalls[c.Name]++
		if c.ID != "" {
			ps.toolByID[c.ID] = c.Name
		}
		if c.Name == ToolSearchToolName {
			sawToolSearch = true
		}
		if readToolNames[c.Name] {
			if p := toolInputPath(c.Name, c.Input); p != "" {
				ps.filesRead[p] = true
			}
		}
		if editToolNames[c.Name] {
			path := toolInputString(c.Input, "file_path")
			if path != "" {
				ps.filesEdited[path] = true
			}
			if ps.firstEditTurn == 0 {
				ps.firstEditTurn = turn
				ps.tokensToFirstEdit = ps.tokensSoFar
			}
			ps.recordBoxChecks(c, path, turn, checkedLines)
		}
	}
	if sawToolSearch {
		ps.toolSearchTurns++
	}
	ps.tokensSoFar += turnTokens
}

// handleUser reads one "user" event's tool_result blocks: every result adds
// to PayloadBytes (the total discovery cost), and a result whose
// tool_use_id was produced by a read-shaped tool (Read, read_source) also
// adds to BytesRead — the tool_use/tool_result pairing by id (toolByID) is
// what makes the attribution exact rather than a guess from ordering.
func (ps *parsedStream) handleUser(ev streamEvent) {
	for _, c := range ev.Message.Content {
		if c.Type != "tool_result" {
			continue
		}
		body := rawTextBlock(c.Content)
		n := int64(len(body))
		ps.payloadBytes += n
		if readToolNames[ps.toolByID[c.ToolUseID]] {
			ps.bytesRead += n
		}
	}
}

// recordBoxChecks scans an edit tool_use's new content for markdown
// checklist lines ("- [x]") not already recorded for path, and appends one
// BoxCheck per new line. Both Edit (new_string) and MultiEdit (edits[].new_string)
// carry the field this reads; Write's `content` is treated the same way,
// so a checklist authored fresh by Write is still counted the first time
// each line appears in the transcript.
func (ps *parsedStream) recordBoxChecks(c streamContentBlock, path string, turn int, checkedLines map[string]map[string]bool) {
	if !strings.HasSuffix(path, ".md") {
		return
	}
	var texts []string
	switch c.Name {
	case "Edit":
		texts = []string{toolInputString(c.Input, "new_string")}
	case "Write":
		texts = []string{toolInputString(c.Input, "content")}
	case "MultiEdit":
		var body struct {
			Edits []struct {
				NewString string `json:"new_string"`
			} `json:"edits"`
		}
		_ = json.Unmarshal(c.Input, &body)
		for _, e := range body.Edits {
			texts = append(texts, e.NewString)
		}
	}
	if checkedLines[path] == nil {
		checkedLines[path] = map[string]bool{}
	}
	for _, text := range texts {
		for _, ln := range strings.Split(text, "\n") {
			trimmed := strings.TrimSpace(ln)
			if !strings.Contains(trimmed, "- [x]") {
				continue
			}
			if checkedLines[path][trimmed] {
				continue
			}
			checkedLines[path][trimmed] = true
			bc := BoxCheck{Turn: turn, File: path}
			bc.RowID, bc.Symbol, bc.Package = parseBoxCheckAnnotation(trimmed)
			ps.boxChecks = append(ps.boxChecks, bc)
		}
	}
}

// parseBoxCheckAnnotation reads a plan-grammar-v2-shaped {id: ..., symbol:
// ..., package: ...} annotation off a checklist line, if present.
func parseBoxCheckAnnotation(line string) (rowID, symbol, pkg string) {
	m := boxCheckRE.FindStringSubmatch(line)
	if m == nil {
		return "", "", ""
	}
	for _, kv := range boxCheckKV.FindAllStringSubmatch(m[1], -1) {
		key, val := strings.TrimSpace(kv[1]), strings.TrimSpace(kv[2])
		switch key {
		case "id":
			rowID = val
		case "symbol":
			symbol = val
		case "package":
			pkg = val
		}
	}
	return rowID, symbol, pkg
}

func (ps *parsedStream) toSessionResult() SessionResult {
	var r SessionResult
	r.Turns = ps.turns
	r.ToolSearchTurns = ps.toolSearchTurns
	r.FirstEditTurn = ps.firstEditTurn
	r.TokensToFirstEdit = ps.tokensToFirstEdit
	r.FilesRead = sortedKeys(ps.filesRead)
	r.FilesEdited = sortedKeys(ps.filesEdited)
	r.BytesRead = ps.bytesRead
	r.BoxChecks = ps.boxChecks
	r.ToolCalls = ps.toolCalls
	r.PayloadBytes = ps.payloadBytes
	if ps.sawResult {
		u := ps.result.Usage
		r.InputTokens = u.InputTokens
		r.OutputTokens = u.OutputTokens
		r.CacheReadTokens = u.CacheReadInputTokens
		r.CacheCreation = u.CacheCreationInputTokens
		r.TotalTokens = u.InputTokens + u.OutputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
		r.CostUSD = ps.result.TotalCostUSD
		r.WallTime = time.Duration(ps.result.DurationMS) * time.Millisecond
		if ps.result.IsError {
			r.Error = "claude reported is_error for this session"
		}
	} else {
		r.Error = "no terminal result event found in transcript"
	}
	return r
}

func toolInputString(raw json.RawMessage, key string) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// toolInputPath returns the file path a read-shaped tool_use names: Read
// uses "file_path" (absolute); mcp__fathom__read_source uses "file"
// (repo-relative — internal/mcp/readsource.go's registered schema).
func toolInputPath(toolName string, raw json.RawMessage) string {
	switch toolName {
	case "mcp__fathom__read_source":
		return toolInputString(raw, "file")
	default:
		return toolInputString(raw, "file_path")
	}
}

// rawTextBlock flattens a tool_result content field, which is either a
// string or a list of {type:"text", text:…} blocks.
func rawTextBlock(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return string(raw)
	}
	var b strings.Builder
	for _, bl := range blocks {
		b.WriteString(bl.Text)
		b.WriteByte('\n')
	}
	return b.String()
}

func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
