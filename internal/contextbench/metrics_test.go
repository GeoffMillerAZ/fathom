package contextbench

import (
	"encoding/json"
	"strings"
	"testing"
)

// streamLines builds a stream-json transcript from typed events, in the
// shape `claude -p --output-format stream-json --verbose` emits.
func streamLines(events ...map[string]any) []byte {
	var b strings.Builder
	for _, ev := range events {
		raw, err := json.Marshal(ev)
		if err != nil {
			panic(err)
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

func assistantTurn(blocks ...map[string]any) map[string]any {
	return map[string]any{"type": "assistant", "message": map[string]any{"content": blocks}}
}

func toolUse(id, name string) map[string]any {
	return map[string]any{"type": "tool_use", "id": id, "name": name}
}

func textBlock(s string) map[string]any {
	return map[string]any{"type": "text", "text": s}
}

func toolResult(useID, body string) map[string]any {
	return map[string]any{"type": "user", "message": map[string]any{"content": []map[string]any{
		{"type": "tool_result", "tool_use_id": useID, "content": body},
	}}}
}

// TestTranscriptMetrics is BP-9's contract on the four H2-3 readings.
func TestTranscriptMetrics(t *testing.T) {
	gold := []string{"fusion.New", "golden-test"}
	tr := streamLines(
		assistantTurn(toolUse("s1", ToolSearchToolName)),
		toolResult("s1", "loaded 3 tool schemas"),
		assistantTurn(toolUse("o1", "mcp__fathom__obligations")),
		toolResult("o1", "a rule about extractors that does not carry the answer"),
		assistantTurn(toolUse("r1", "mcp__fathom__read_source")),
		toolResult("r1", "register by extension in fusion.New(), golden-test it"),
		assistantTurn(textBlock("The answer is fusion.New and a golden-test.")),
	)

	st := TranscriptMetrics(tr, gold)
	wantPayload := int64(len("loaded 3 tool schemas") + len("a rule about extractors that does not carry the answer") + len("register by extension in fusion.New(), golden-test it"))
	if st.PayloadBytes != wantPayload {
		t.Errorf("payload_bytes = %d, want %d", st.PayloadBytes, wantPayload)
	}
	if st.Turns != 4 || st.ToolSearchTurns != 1 || st.TurnsExclToolSearch != 3 {
		t.Errorf("turns=%d toolsearch=%d excl=%d, want 4/1/3", st.Turns, st.ToolSearchTurns, st.TurnsExclToolSearch)
	}
	if st.ToolCalls != 3 {
		t.Errorf("tool_calls = %d, want 3", st.ToolCalls)
	}
	if st.GoldSourceTool != "mcp__fathom__read_source" {
		t.Errorf("gold_source_tool = %q — the tool whose RESULT first carried a gold token, not the one that was asked", st.GoldSourceTool)
	}
	if st.GoldToken != "fusion.New" {
		t.Errorf("gold_token = %q", st.GoldToken)
	}

	// Only the final answer carries a gold token: the arm answered without
	// a tool having surfaced it.
	only := TranscriptMetrics(streamLines(
		assistantTurn(toolUse("b1", "Bash")),
		toolResult("b1", "no matches"),
		assistantTurn(textBlock("I believe it is fusion.New.")),
	), gold)
	if only.GoldSourceTool != GoldSourceAnswer {
		t.Errorf("gold_source_tool = %q, want %q", only.GoldSourceTool, GoldSourceAnswer)
	}

	// Nothing anywhere: recorded as empty, never as a tool.
	miss := TranscriptMetrics(streamLines(assistantTurn(textBlock("I do not know."))), gold)
	if miss.GoldSourceTool != "" {
		t.Errorf("a transcript with no gold token has no gold source, got %q", miss.GoldSourceTool)
	}

	// A stray stderr line must not make the transcript unreadable.
	noisy := append([]byte("warning: something on stderr\n"), tr...)
	if TranscriptMetrics(noisy, gold) != st {
		t.Error("a non-JSON line changed the reading")
	}
}

// TestGoldSourceRate is VB-5: the flat, single-level computation every
// other grouping (GoldSourceRateByArm, GoldSourceByShape) must go through
// — no duplicate counting loop anywhere else.
func TestGoldSourceRate(t *testing.T) {
	results := []ArmResult{
		{TaskID: "T1", GoldSourceTool: "mcp__fathom__read_source"},
		{TaskID: "T1", GoldSourceTool: "mcp__fathom__obligations"},
		{TaskID: "T2", GoldSourceTool: "mcp__fathom__read_source"},
	}
	rates := GoldSourceRate(results)
	if got := rates["mcp__fathom__read_source"]; got != 2.0/3.0 {
		t.Errorf("read_source rate = %v, want 2/3", got)
	}
	if got := rates["mcp__fathom__obligations"]; got != 1.0/3.0 {
		t.Errorf("obligations rate = %v, want 1/3", got)
	}
	if GoldSourceRate(nil) != nil {
		t.Error("no results, no rates")
	}
}

func TestGoldSourceRateByArm(t *testing.T) {
	results := []ArmResult{
		{TaskID: "T1", Arm: "fathom", GoldSourceTool: "mcp__fathom__read_source"},
		{TaskID: "T1", Arm: "fathom", GoldSourceTool: "mcp__fathom__obligations"},
		{TaskID: "T2", Arm: "fathom", GoldSourceTool: "mcp__fathom__read_source"},
		{TaskID: "T1", Arm: "grep", GoldSourceTool: "Bash"},
		{TaskID: "T2", Arm: "grep", GoldSourceTool: ""},
	}
	rates := GoldSourceRateByArm(results)
	if got := rates["fathom"]["mcp__fathom__read_source"]; got != 2.0/3.0 {
		t.Errorf("fathom read_source rate = %v, want 2/3", got)
	}
	if got := rates["fathom"]["mcp__fathom__obligations"]; got != 1.0/3.0 {
		t.Errorf("fathom obligations rate = %v, want 1/3", got)
	}
	if got := rates["grep"][""]; got != 0.5 {
		t.Errorf("a cell with no gold source stays in the denominator: %v", got)
	}
	line := RenderGoldSourceRate(rates)
	if !strings.Contains(line, "gold_source_rate[fathom]: mcp__fathom__read_source=") {
		t.Errorf("render puts the highest rate first:\n%s", line)
	}
	if GoldSourceRateByArm(nil) != nil {
		t.Error("no results, no rates")
	}
}

// TestRenderGoldSourceRate_ListsEveryToolIncludingZero is VB-5: a tool
// that never won for a given arm must still print, at 0 — not be silently
// absent from that arm's line.
func TestRenderGoldSourceRate_ListsEveryToolIncludingZero(t *testing.T) {
	rates := GoldSourceRateByArm([]ArmResult{
		{TaskID: "T1", Arm: "fathom", GoldSourceTool: "mcp__fathom__obligations"},
		{TaskID: "T1", Arm: "grep", GoldSourceTool: "Bash"},
	})
	line := RenderGoldSourceRate(rates)
	if !strings.Contains(line, "gold_source_rate[fathom]:") || !strings.Contains(line, " Bash=0") {
		t.Errorf("fathom's own line must still name Bash at 0 (it won for grep, never for fathom):\n%s", line)
	}
	if !strings.Contains(line, "gold_source_rate[grep]:") || !strings.Contains(line, " mcp__fathom__obligations=0") {
		t.Errorf("grep's own line must still name obligations at 0:\n%s", line)
	}
}

// TestGoldSourceByShape is VB-5: the same reading, sliced by each task's
// own Category.
func TestGoldSourceByShape(t *testing.T) {
	tasks := []Task{
		{ID: "T1", Category: "what_governs"},
		{ID: "T2", Category: "what_governs"},
		{ID: "T3", Category: "concern_scoped_search"},
	}
	results := []ArmResult{
		{TaskID: "T1", Arm: "fathom", GoldSourceTool: "mcp__fathom__obligations"},
		{TaskID: "T2", Arm: "fathom", GoldSourceTool: "mcp__fathom__obligations"},
		{TaskID: "T3", Arm: "fathom", GoldSourceTool: "Grep"},
		// TaskID absent from tasks: groups under the empty shape, not dropped.
		{TaskID: "T-unknown", Arm: "fathom", GoldSourceTool: "Bash"},
	}
	byShape := GoldSourceByShape(results, tasks)
	if got := byShape["what_governs"]["fathom"]["mcp__fathom__obligations"]; got != 1.0 {
		t.Errorf("what_governs/fathom/obligations = %v, want 1.0", got)
	}
	if got := byShape["concern_scoped_search"]["fathom"]["Grep"]; got != 1.0 {
		t.Errorf("concern_scoped_search/fathom/Grep = %v, want 1.0", got)
	}
	if got := byShape[""]["fathom"]["Bash"]; got != 1.0 {
		t.Errorf("a task absent from tasks must group under the empty shape, not drop: got %v", got)
	}
	if GoldSourceByShape(nil, tasks) != nil {
		t.Error("no results, no breakdown")
	}
}

// TestCriterionH23Pinned is BP-10's anchor: the values H2-3 was registered
// with are pinned here, so the bar cannot move after data arrives (law 12).
func TestCriterionH23Pinned(t *testing.T) {
	c, err := LoadCriterionByID(homeCorpusPath, "H2-3")
	if err != nil {
		t.Fatalf("LoadCriterionByID(H2-3): %v", err)
	}
	if !c.RequireStrictlyMoreSuccesses {
		t.Error("H2-3 keeps H2-1's success bar: strictly more successes")
	}
	if c.MinTrials != 3 {
		t.Errorf("min_trials = %d, want 3", c.MinTrials)
	}
	if c.MaxPayloadRatio != 1.0 {
		t.Errorf("max_payload_ratio = %v, want 1.0", c.MaxPayloadRatio)
	}
	if c.MaxCostRatio != 1.0 {
		t.Errorf("max_cost_ratio = %v, want 1.0", c.MaxCostRatio)
	}
	if c.MaxTokenRatio != 0 {
		t.Errorf("H2-3 does not gate total_tokens (O26): max_token_ratio = %v", c.MaxTokenRatio)
	}
	if c.Scoring != ScoringSubstring {
		t.Errorf("scoring = %q, want %q (H2-2 is the precision criterion)", c.Scoring, ScoringSubstring)
	}
	if c.LeakRule == nil || c.LeakRule.Arm != "none" || c.LeakRule.MinPasses != 2 || c.LeakRule.OfTrials != 3 {
		t.Errorf("H2-3 keeps the committed leak rule: %+v", c.LeakRule)
	}

	// H2-1 is history and keeps its own bar.
	h21, err := LoadCriterionByID(homeCorpusPath, "H2-1")
	if err != nil {
		t.Fatalf("LoadCriterionByID(H2-1): %v", err)
	}
	if h21.MaxTokenRatio != 1.0 || h21.MaxPayloadRatio != 0 {
		t.Errorf("H2-1 must be untouched by H2-3's registration: %+v", h21)
	}
}

// TestEvaluateH23CostHalf pins that the payload and cost caps gate, and
// that a run which recorded no payload is indeterminate rather than a
// pass — the instrument did not look.
func TestEvaluateH23CostHalf(t *testing.T) {
	c, err := LoadCriterionByID(homeCorpusPath, "H2-3")
	if err != nil {
		t.Fatal(err)
	}
	base := PairedSummary{
		Tasks: 3, MinTrials: 3, TreatmentSuccesses: 8, ControlSuccesses: 5,
		TreatmentTrials: 9, ControlTrials: 9,
		AggregateTokenRatio: 1.4, // recorded, never gated
		ControlPayloadBytes: 90000, TreatmentPayloadBytes: 45000, AggregatePayloadRatio: 0.5,
		ControlCostUSD: 2.46, TreatmentCostUSD: 2.43, AggregateCostRatio: 0.988,
	}
	if ev := c.Evaluate(base); ev.Verdict != VerdictPass {
		t.Errorf("a cheaper-payload win must pass even at 1.4× total_tokens: %s — %s", ev.Verdict, ev.Reason)
	}
	if ev := c.Evaluate(base); !strings.Contains(ev.Reason, "toolsearch turns") {
		t.Errorf("the harness tax must be a named line in the reason: %s", ev.Reason)
	}

	over := base
	over.AggregatePayloadRatio = 1.2
	if ev := c.Evaluate(over); ev.Verdict != VerdictFail || !strings.Contains(ev.Reason, "payload_bytes") {
		t.Errorf("payload over the cap is a measured refutation: %s — %s", ev.Verdict, ev.Reason)
	}

	costly := base
	costly.AggregateCostRatio = 1.5
	if ev := c.Evaluate(costly); ev.Verdict != VerdictFail || !strings.Contains(ev.Reason, "cost_usd") {
		t.Errorf("cost over the cap is a measured refutation: %s — %s", ev.Verdict, ev.Reason)
	}

	blind := base
	blind.ControlPayloadBytes, blind.TreatmentPayloadBytes, blind.AggregatePayloadRatio = 0, 0, 0
	ev := c.Evaluate(blind)
	if ev.Verdict != VerdictIndeterminate || !strings.Contains(ev.Reason, "recorded none") {
		t.Errorf("a run with no payload recorded may not pass H2-3: %s — %s", ev.Verdict, ev.Reason)
	}
}
