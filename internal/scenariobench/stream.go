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
	Type       string `json:"type"`
	MCPServers []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"mcp_servers"`
	// Tools is the init event's own available-tool list — counted for
	// MCP-namespaced entries (MCPTools), never stored verbatim.
	Tools []string `json:"tools"`
	// HookEvent is the "system"/"hook_response" event's own hook event
	// name ("SessionStart", "PostToolUse", ...) — docs/bench/scenarios.md's
	// clean-room smoke test found the real shape carries this as
	// "hook_event" (distinct from "hook_name", which is "Event:matcher",
	// e.g. "SessionStart:startup").
	HookEvent string `json:"hook_event"`
	// HookName is the same "system"/"hook_response" event's own
	// "Event:matcher" name (e.g. "PreToolUse:Grep",
	// "PreToolUse:mcp__fathom__find_symbol") — distinct from HookEvent,
	// which names only the bare event ("PreToolUse"). handleHookResponse
	// (wave n13) uses it for HooksFiredByName's own per-tool-class
	// breakdown.
	HookName string `json:"hook_name"`
	// Output / Stdout are the "system"/"hook_response" event's own raw
	// process output. fathom's own hook subcommands print a
	// {"hookSpecificOutput":{"additionalContext":"…"}} envelope
	// (internal/harness/codec.go) to stdout; the clean-room smoke test
	// (docs/bench/scenarios.md) found the field housing it ambiguous
	// between these two names depending on harness version, so
	// hookResponseAdditionalContext tries both.
	Output  string `json:"output"`
	Stdout  string `json:"stdout"`
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
	// `claude -p --output-format json`'s envelope. Subtype is shared with
	// the "system" event this parser also reads ("init" carries
	// MCPServers/Tools above); a result event's own Subtype is otherwise
	// "success" or an error subtype like "error_max_turns".
	IsError      bool    `json:"is_error"`
	Subtype      string  `json:"subtype"`
	NumTurns     int     `json:"num_turns"`
	DurationMS   int64   `json:"duration_ms"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	// ModelUsage is the result event's own per-model cost/token breakdown
	// (keyed by model id, e.g. "claude-sonnet-5") — the SAME field
	// wave N19's rescore-cost fix reads TotalCostUSD's own fallback from
	// (sumModelUsageCost, below): a session whose terminal event carries
	// no top-level total_cost_usd at all (TotalCostUSD's own zero value,
	// indistinguishable from a genuinely free/unmetered session) still
	// prices out from here, exactly as the live path would have priced it
	// had it needed to.
	ModelUsage map[string]struct {
		CostUSD float64 `json:"costUSD"`
	} `json:"modelUsage"`
	// Result is the result event's own final-answer text (SB-32's
	// answer-only oracle).
	Result string `json:"result"`
	Usage  struct {
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
	// mcpServers/mcpTools come from the "system"/"init" event(s); nil map
	// when no such event was seen (the defect this closes: a wired
	// session that never gets one still looked identical to a control
	// before this parser could tell the difference).
	mcpServers map[string]string
	mcpTools   int
	// hooksFired mirrors mcpServers' nil-means-unseen convention: nil when
	// no "system"/"hook_response" event was seen at all, else event name
	// to count (handleHookResponse). Also carries the synthetic
	// "obligations_injected" key — see SessionResult.HooksFired's doc
	// comment.
	hooksFired map[string]int
	// hooksFiredByName mirrors hooksFired at the finer hook_name grain
	// (wave n13) — see SessionResult.HooksFiredByName's own doc comment.
	hooksFiredByName map[string]int
	// sessionStartContext is the FIRST SessionStart hook_response's own
	// rendered text (handleHookResponse) — SessionResult.SessionStartContext.
	sessionStartContext string
	// lastAssistantText is the most recent non-empty assistant "text"
	// content block seen so far — O45 item 3's error_max_turns fallback
	// source for FinalAnswer.
	lastAssistantText string
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
		case "system":
			switch ev.Subtype {
			case "init":
				ps.handleInit(ev)
			case "hook_response":
				ps.handleHookResponse(ev)
			}
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
		if c.Type == "text" {
			if t := strings.TrimSpace(c.Text); t != "" {
				ps.lastAssistantText = t
			}
		}
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
			path := firstLine(toolInputString(c.Input, "file_path"))
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

// handleInit reads one "system"/"init" event: every named MCP server's
// connection status (merged into mcpServers — a later init event's status
// for the same name wins, since only the last is still current) and the
// init event's own tool list, counted for MCP-namespaced entries
// (mcpTools; a later init event's count replaces rather than adds, for
// the same reason). A session with no init event at all leaves both zero,
// which toSessionResult reports as a nil MCPServers, never a false "no
// servers connected".
func (ps *parsedStream) handleInit(ev streamEvent) {
	if ps.mcpServers == nil {
		ps.mcpServers = map[string]string{}
	}
	for _, s := range ev.MCPServers {
		ps.mcpServers[s.Name] = s.Status
	}
	tools := 0
	for _, name := range ev.Tools {
		if strings.HasPrefix(name, "mcp__") {
			tools++
		}
	}
	ps.mcpTools = tools
}

// handleHookResponse reads one "system"/"hook_response" event and counts
// it under its own hook_event name, regardless of the hook's outcome — a
// hook that fired and errored still proves the harness's --settings file
// wired something (the instrument-truth question runOneSession asks),
// unlike one that never fired at all. An event with no hook_event at all
// (should not happen; defensive) is skipped rather than counted under "".
// It also counts under the finer-grained hook_name (wave n13, vs-pilot-5's
// second reading, "hooks_not_fired") — "Event:matcher", e.g.
// "PreToolUse:Grep" — when the event carries one, so a reading can tell
// WHICH tool class a hook actually fired for, not just that its bare
// event name fired at all.
func (ps *parsedStream) handleHookResponse(ev streamEvent) {
	if ev.HookEvent == "" {
		return
	}
	if ps.hooksFired == nil {
		ps.hooksFired = map[string]int{}
	}
	ps.hooksFired[ev.HookEvent]++

	if ev.HookName != "" {
		if ps.hooksFiredByName == nil {
			ps.hooksFiredByName = map[string]int{}
		}
		ps.hooksFiredByName[ev.HookName]++
	}

	ctx := hookResponseAdditionalContext(ev)
	if ev.HookEvent == "SessionStart" && ps.sessionStartContext == "" {
		ps.sessionStartContext = ctx
	}
	if ev.HookEvent == "PostToolUse" && obligationsMarkerIn(ctx) {
		ps.hooksFired["obligations_injected"]++
	}
	if ev.HookEvent == "PreToolUse" && denyMarkerIn(ctx) {
		ps.hooksFired["enforced_refusals"]++
	}
}

// denyMarkerIn reports whether ctx carries fathom's PreToolUse deny shape
// (ADR-038, wave G1: hookSpecificOutput.permissionDecision:"deny") rather
// than an ordinary injected guard/obligation payload.
// hookResponseAdditionalContext's own fallback (raw text when the
// additionalContext envelope shape does not match) is what makes this
// reachable at all: a deny payload never carries additionalContext, so ctx
// here is the raw JSON text of the deny envelope itself.
func denyMarkerIn(ctx string) bool {
	return strings.Contains(ctx, `"permissionDecision":"deny"`) || strings.Contains(ctx, `"permissionDecision": "deny"`)
}

// obligationsMarkerIn reports whether ctx carries either of the two
// obligations-injection marker texts (O45 item 8) — the SessionStart
// stanza's own wording and the PostToolUse render's own header (see
// ruleserved_before_edit.go's obligationsHookMarkers for the full set this
// mirrors, minus the PreToolUse guard marker, which is not an obligations
// injection).
func obligationsMarkerIn(ctx string) bool {
	return strings.Contains(ctx, "Obligations — governing rules") || strings.Contains(ctx, "⚖️ fathom obligations —")
}

// hookResponseAdditionalContext extracts one "system"/"hook_response"
// event's own rendered text: fathom's hook subcommands print a
// {"hookSpecificOutput":{"additionalContext":"…"}} envelope
// (internal/harness/codec.go) to stdout, echoed back by Claude Code on one
// of "output"/"stdout" (the real field is ambiguous between harness
// versions per docs/bench/scenarios.md's clean-room smoke test) — tried in
// that order, first non-empty wins. When the field parses as that
// envelope, its own additionalContext string is returned; otherwise the
// raw field text itself, trimmed, so a hook that printed plain text (a
// non-fathom hook, or a fixture) still yields something to match markers
// against.
func hookResponseAdditionalContext(ev streamEvent) string {
	raw := ev.Output
	if strings.TrimSpace(raw) == "" {
		raw = ev.Stdout
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var envelope struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err == nil && envelope.HookSpecificOutput.AdditionalContext != "" {
		return envelope.HookSpecificOutput.AdditionalContext
	}
	return raw
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
			bc := BoxCheck{Turn: turn, File: path, Text: trimmed}
			bc.RowID, bc.Symbol, bc.Package, bc.Oracle, bc.Path, bc.Pattern = parseBoxCheckAnnotation(trimmed)
			ps.boxChecks = append(ps.boxChecks, bc)
		}
	}
}

// parseBoxCheckAnnotation reads a plan-grammar-v2-shaped {id: ..., symbol:
// ..., package: ...} annotation off a checklist line, if present — or, for
// a non-code row (docs/plans/README.md, ADR-030), {id: ..., oracle:
// lexical, path: ..., pattern: ...} (O57 finding 6: FalseDoneRate must
// adjudicate this shape too, not fold it into "no symbol").
func parseBoxCheckAnnotation(line string) (rowID, symbol, pkg, oracle, path, pattern string) {
	m := boxCheckRE.FindStringSubmatch(line)
	if m == nil {
		return "", "", "", "", "", ""
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
		case "oracle":
			oracle = val
		case "path":
			path = val
		case "pattern":
			pattern = val
		}
	}
	return rowID, symbol, pkg, oracle, path, pattern
}

// sumModelUsageCost sums a result event's own per-model modelUsage.costUSD
// entries — TotalCostUSD's own fallback (toSessionResult) for a session
// whose terminal event carries no top-level total_cost_usd. nil/empty in
// is 0 out, never a guess.
func sumModelUsageCost(usage map[string]struct {
	CostUSD float64 `json:"costUSD"`
}) float64 {
	var sum float64
	for _, u := range usage {
		sum += u.CostUSD
	}
	return sum
}

func (ps *parsedStream) toSessionResult() SessionResult {
	var r SessionResult
	r.Turns = ps.turns
	r.ToolSearchTurns = ps.toolSearchTurns
	r.FirstEditTurn = ps.firstEditTurn
	r.TokensToFirstEdit = ps.tokensToFirstEdit
	r.FilesRead = sortedBoolKeys(ps.filesRead)
	r.FilesEdited = sortedBoolKeys(ps.filesEdited)
	r.BytesRead = ps.bytesRead
	r.BoxChecks = ps.boxChecks
	r.ToolCalls = ps.toolCalls
	r.MCPServers = ps.mcpServers
	r.MCPTools = ps.mcpTools
	r.HooksFired = ps.hooksFired
	r.HooksFiredByName = ps.hooksFiredByName
	r.SessionStartContext = ps.sessionStartContext
	r.PayloadBytes = ps.payloadBytes
	if ps.sawResult {
		u := ps.result.Usage
		r.InputTokens = u.InputTokens
		r.OutputTokens = u.OutputTokens
		r.CacheReadTokens = u.CacheReadInputTokens
		r.CacheCreation = u.CacheCreationInputTokens
		r.TotalTokens = u.InputTokens + u.OutputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
		r.CostUSD = ps.result.TotalCostUSD
		if r.CostUSD == 0 {
			// wave N19 defect 2: a rescored cell's own raw stream carries
			// this same result event RunSession's live path already reads
			// TotalCostUSD from — the live and rescore paths share
			// ParseSessionStream, so this fallback benefits both, never
			// just one. sumModelUsageCost prices the session from its own
			// per-model breakdown when the top-level total is absent or
			// legitimately zero for every model that actually ran (a
			// wholly-cached session, say) leaves it at zero rather than
			// guessing a number the transcript never reported.
			r.CostUSD = sumModelUsageCost(ps.result.ModelUsage)
		}
		r.WallTime = time.Duration(ps.result.DurationMS) * time.Millisecond
		r.FinalAnswer = ps.result.Result
		if r.FinalAnswer != "" {
			r.AnswerFrom = "result"
		}
		switch {
		case ps.result.IsError && ps.result.Subtype == "error_max_turns":
			// The turn budget ended the session: that is the designed
			// hand-off (vision 27 §2), not an error. The tree the agent
			// left is the reading; the next session starts cold from it.
			r.BudgetHit = true
			// O45 item 3: a session that stated its answer and THEN ran
			// out of turns (e.g. writing a plan it was never going to
			// need) must not score as a blank FAIL — the result event's
			// own "result" text is empty on error_max_turns, so fall back
			// to the last assistant text block the transcript actually
			// carries.
			if r.FinalAnswer == "" && ps.lastAssistantText != "" {
				r.FinalAnswer = ps.lastAssistantText
				r.AnswerFrom = "last_text"
			}
		case ps.result.IsError:
			r.Error = "claude reported is_error for this session (" + ps.result.Subtype + ")"
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
		return firstLine(toolInputString(raw, "file"))
	default:
		return firstLine(toolInputString(raw, "file_path"))
	}
}

// firstLine sanitizes a value this parser treats as a file path to its
// first line: a malformed tool_use whose "file_path"/"file" carries an
// entire file body (a Write call's content mistakenly duplicated into its
// path field, observed in the public pilot, 2026-09-13) must not turn
// FilesEdited/FilesRead into a one-entry set holding megabytes of text. A
// trailing "\r" (a "\r\n" line ending inside the malformed body) is
// trimmed too. Only ever applied to a path field — never to box-check
// text (new_string/content), which legitimately spans many lines.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSuffix(s, "\r")
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

func sortedBoolKeys(m map[string]bool) []string {
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
