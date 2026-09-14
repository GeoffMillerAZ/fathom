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

// GoldSourceRate reports the share of results whose gold_source_tool was
// each tool — the reading that separates "the task passed" from "the tool
// that was supposed to answer it did". Cells with no gold source anywhere
// are counted in the denominator under the empty key "" (recorded, never
// dropped: a run where the arm never touched the answer is the finding,
// not a gap). This is the SINGLE per-tool computation every caller that
// needs a rate over some slice of results goes through — one arm's own
// cells (GoldSourceRateByArm), one question shape's own cells
// (GoldSourceByShape), or any later grouping — so how a rate is computed
// never drifts between the summary and the ledger (VB-5). Output is a
// map, which marshals in sorted key order, so a caller's result is
// deterministic. nil for no results.
func GoldSourceRate(results []ArmResult) map[string]float64 {
	if len(results) == 0 {
		return nil
	}
	counts := map[string]int{}
	for _, r := range results {
		counts[r.GoldSourceTool]++
	}
	rates := make(map[string]float64, len(counts))
	for tool, n := range counts {
		rates[tool] = float64(n) / float64(len(results))
	}
	return rates
}

// GoldSourceRateByArm groups results by arm and computes GoldSourceRate
// independently within each group.
func GoldSourceRateByArm(results []ArmResult) map[string]map[string]float64 {
	byArm := map[string][]ArmResult{}
	for _, r := range results {
		byArm[r.Arm] = append(byArm[r.Arm], r)
	}
	if len(byArm) == 0 {
		return nil
	}
	out := make(map[string]map[string]float64, len(byArm))
	for arm, subset := range byArm {
		out[arm] = GoldSourceRate(subset)
	}
	return out
}

// GoldSourceByShape groups results by their task's own Category (tasks,
// keyed by TaskID) and then by arm, computing GoldSourceRate
// independently within each (shape, arm) group (VB-5): does a tool's
// share of the gold source change with the question shape, or is
// "obligations wins on governance, grep wins on lookups" one number
// hiding two different findings? A result whose task carries no Category
// (or whose TaskID is absent from tasks) groups under the empty-string
// shape — never dropped.
func GoldSourceByShape(results []ArmResult, tasks []Task) map[string]map[string]map[string]float64 {
	catByID := make(map[string]string, len(tasks))
	for _, t := range tasks {
		catByID[t.ID] = t.Category
	}
	byShape := map[string][]ArmResult{}
	for _, r := range results {
		shape := catByID[r.TaskID]
		byShape[shape] = append(byShape[shape], r)
	}
	if len(byShape) == 0 {
		return nil
	}
	out := make(map[string]map[string]map[string]float64, len(byShape))
	for shape, subset := range byShape {
		out[shape] = GoldSourceRateByArm(subset)
	}
	return out
}

// RenderGoldSourceRate renders a GoldSourceRateByArm result as one
// deterministic line per arm, highest rate first then tool name — for the
// ledger summary a human reads. Every arm's line lists the UNION of tools
// seen across every arm, defaulting a tool an arm never won with to 0 —
// so a reader sees "obligations was zero of those three" without having
// to notice an absence (VB-5).
func RenderGoldSourceRate(rates map[string]map[string]float64) string {
	arms := make([]string, 0, len(rates))
	toolSet := map[string]bool{}
	for arm := range rates {
		arms = append(arms, arm)
		for tool := range rates[arm] {
			toolSet[tool] = true
		}
	}
	sort.Strings(arms)
	tools := make([]string, 0, len(toolSet))
	for tool := range toolSet {
		tools = append(tools, tool)
	}
	var b strings.Builder
	for _, arm := range arms {
		sort.Slice(tools, func(i, j int) bool {
			ri, rj := rates[arm][tools[i]], rates[arm][tools[j]]
			if ri != rj {
				return ri > rj
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

// RenderGoldSourceByShape renders a GoldSourceByShape result as one
// RenderGoldSourceRate block per question shape, shapes in sorted order,
// each headed by its own name (the empty shape prints as "(uncategorized)")
// — the per-question-shape companion to RenderGoldSourceRate's per-arm
// view (VB-5).
func RenderGoldSourceByShape(byShape map[string]map[string]map[string]float64) string {
	shapes := make([]string, 0, len(byShape))
	for shape := range byShape {
		shapes = append(shapes, shape)
	}
	sort.Strings(shapes)
	var b strings.Builder
	for _, shape := range shapes {
		name := shape
		if name == "" {
			name = "(uncategorized)"
		}
		b.WriteString("  gold_source_by_shape[" + name + "]:\n")
		for _, line := range strings.Split(strings.TrimRight(RenderGoldSourceRate(byShape[shape]), "\n"), "\n") {
			b.WriteString("  " + line + "\n")
		}
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
