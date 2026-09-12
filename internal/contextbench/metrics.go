package contextbench

import (
	"bufio"
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

// H2-3's readings (ADR-032 §3; docs/design/bench-pyramid.md §3.5). The
// first replicated run showed total_tokens to be cache_read x turns — a
// turn counter in a token costume, under which the discovery payload
// fathom shrinks is invisible (O26). These four metrics are read from the
// same stream-json transcript the runner already captures, so an existing
// run can be re-scored under H2-3 with no new spend:
//
//   - payload_bytes: the tool-result bytes returned to the model. This is
//     the discovery cost fathom controls.
//   - toolsearch_turns: turns spent loading deferred MCP tool schemas
//     (O24) — the harness tax, reported as its own line so it is never a
//     hidden penalty on the fathom arm nor a hidden excuse for it.
//   - turns_excl_toolsearch: assistant turns minus those.
//   - gold_source_tool: the tool whose result FIRST carried a gold token.
//     A cell passes the task; a tool passes only when it is the source.
//     E1/O23 — obligations served six displaced rules and the arm passed
//     the task by reading the file grep's way — becomes a number here.

// ToolSearchToolName is the harness's deferred-schema loader as it appears
// in a stream transcript. Claude Code 2.1.x defers MCP tool schemas, so a
// fathom-arm cell opens with one or two of these turns and the grep arm
// pays none (O24, run-20260902T211103Z).
const ToolSearchToolName = "ToolSearch"

// GoldSourceAnswer is the gold_source_tool value for a cell where no tool
// result carried a gold token but the final answer did — the arm knew (or
// reasoned to) the answer without a tool having surfaced it.
const GoldSourceAnswer = "answer"

// TranscriptStats is TranscriptMetrics' reading of one cell's transcript.
type TranscriptStats struct {
	// PayloadBytes sums the bytes of every tool_result returned to the
	// model — flattened text, as the model received it.
	PayloadBytes int64 `json:"payload_bytes"`
	// Turns counts assistant turns; ToolSearchTurns counts those that
	// carried a ToolSearch call; TurnsExclToolSearch is the difference.
	Turns               int `json:"turns"`
	ToolSearchTurns     int `json:"toolsearch_turns"`
	TurnsExclToolSearch int `json:"turns_excl_toolsearch"`
	// ToolCalls counts tool_use blocks over the whole transcript.
	ToolCalls int `json:"tool_calls"`
	// GoldSourceTool names the tool whose result first carried a gold
	// token, GoldSourceAnswer when only the final answer did, and "" when
	// no gold token appeared anywhere.
	GoldSourceTool string `json:"gold_source_tool,omitempty"`
	// GoldToken is the token that decided GoldSourceTool.
	GoldToken string `json:"gold_token,omitempty"`
}

// TranscriptMetrics reads one `claude -p --output-format stream-json
// --verbose` transcript (one JSON event per line) for the H2-3 figures. It
// is a pure function of its input; non-JSON lines are skipped, so a
// transcript carrying a stray stderr line is still readable.
func TranscriptMetrics(transcript []byte, gold []string) TranscriptStats {
	var st TranscriptStats
	toolByID := map[string]string{}

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
			st.Turns++
			sawToolSearch := false
			var text strings.Builder
			for _, c := range ev.Message.Content {
				switch c.Type {
				case "tool_use":
					st.ToolCalls++
					if c.ID != "" {
						toolByID[c.ID] = c.Name
					}
					if c.Name == ToolSearchToolName {
						sawToolSearch = true
					}
				case "text":
					text.WriteString(c.Text)
					text.WriteByte('\n')
				}
			}
			if sawToolSearch {
				st.ToolSearchTurns++
			}
			// Only the answer can be the gold source once no tool result
			// has been: a tool result always precedes the turn that quotes
			// it, so first-in-transcript order decides.
			if st.GoldSourceTool == "" {
				if tok := firstToken(text.String(), gold); tok != "" {
					st.GoldSourceTool, st.GoldToken = GoldSourceAnswer, tok
				}
			}
		case "user":
			for _, c := range ev.Message.Content {
				if c.Type != "tool_result" {
					continue
				}
				body := rawText(c.Content)
				st.PayloadBytes += int64(len(body))
				if st.GoldSourceTool != "" {
					continue
				}
				if tok := firstToken(body, gold); tok != "" {
					name := toolByID[c.ToolUseID]
					if name == "" {
						name = "unknown"
					}
					st.GoldSourceTool, st.GoldToken = name, tok
				}
			}
		}
	}
	st.TurnsExclToolSearch = st.Turns - st.ToolSearchTurns
	return st
}

// GoldSourceRate reports, per arm, the share of that arm's cells whose
// gold_source_tool was each tool — the reading that separates "the task
// passed" from "the tool that was supposed to answer it did". Cells with
// no gold source anywhere are counted in the denominator under the empty
// key "" (recorded, never dropped: a run where the arm never touched the
// answer is the finding, not a gap). Output is a map, which marshals in
// sorted key order, so the ledger row is deterministic.
func GoldSourceRate(results []ArmResult) map[string]map[string]float64 {
	counts := map[string]map[string]int{}
	totals := map[string]int{}
	for _, r := range results {
		if counts[r.Arm] == nil {
			counts[r.Arm] = map[string]int{}
		}
		counts[r.Arm][r.GoldSourceTool]++
		totals[r.Arm]++
	}
	if len(counts) == 0 {
		return nil
	}
	out := make(map[string]map[string]float64, len(counts))
	for arm, byTool := range counts {
		rates := make(map[string]float64, len(byTool))
		for tool, n := range byTool {
			rates[tool] = float64(n) / float64(totals[arm])
		}
		out[arm] = rates
	}
	return out
}

// RenderGoldSourceRate renders GoldSourceRate as one deterministic line
// per arm, highest rate first then tool name — for the ledger summary a
// human reads.
func RenderGoldSourceRate(rates map[string]map[string]float64) string {
	arms := make([]string, 0, len(rates))
	for arm := range rates {
		arms = append(arms, arm)
	}
	sort.Strings(arms)
	var b strings.Builder
	for _, arm := range arms {
		tools := make([]string, 0, len(rates[arm]))
		for tool := range rates[arm] {
			tools = append(tools, tool)
		}
		sort.Slice(tools, func(i, j int) bool {
			if rates[arm][tools[i]] != rates[arm][tools[j]] {
				return rates[arm][tools[i]] > rates[arm][tools[j]]
			}
			return tools[i] < tools[j]
		})
		b.WriteString("  gold_source_rate[" + arm + "]:")
		for _, tool := range tools {
			name := tool
			if name == "" {
				name = "(none)"
			}
			b.WriteString(" " + name + "=" + trimFloat(rates[arm][tool]))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func trimFloat(f float64) string {
	s := strings.TrimRight(strings.TrimRight(formatFloat(f), "0"), ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func formatFloat(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}
