package contextbench_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

// ── PB-8: precision-aware scoring (H2-2) ──────────────────────────────

func TestScoreWithPrecision_PassesATightFencedAnswer(t *testing.T) {
	task := contextbench.Task{ID: "T", GroundTruth: []string{"cmd/a.go", "cmd/b.go"}, ScoringRule: contextbench.AllOf}
	out := "Let me think.\n```\ncmd/a.go\ncmd/b.go\n```\n"
	ps := contextbench.ScoreWithPrecision(task, out)
	if !ps.Pass || !ps.Fenced || ps.OverBudget {
		t.Fatalf("want pass, got %+v", ps)
	}
	if ps.Precision != 1 || ps.Recall != 1 || ps.Predicted != 2 || ps.MaxPredicted != 5 {
		t.Errorf("precision/recall/predicted/max = %v/%v/%d/%d", ps.Precision, ps.Recall, ps.Predicted, ps.MaxPredicted)
	}
}

func TestScoreWithPrecision_DumpingTheTreeFailsOnBudget(t *testing.T) {
	// The whole point: every gold token present, but the answer is a tree
	// dump — all-of substring scoring would pass it; H2-2 must not.
	task := contextbench.Task{ID: "T", GroundTruth: []string{"cmd/a.go"}, ScoringRule: contextbench.ExactMatch}
	out := "```\ncmd/a.go\ncmd/b.go\ncmd/c.go\ncmd/d.go\ncmd/e.go\ncmd/f.go\n```"
	ps := contextbench.ScoreWithPrecision(task, out)
	if ps.Pass || !ps.OverBudget {
		t.Fatalf("six predicted lines against a cap of five must fail on budget: %+v", ps)
	}
	if ps.Recall != 1 || ps.Precision < 0.16 || ps.Precision > 0.17 {
		t.Errorf("recall must still be 1 and precision 1/6, got %v/%v", ps.Recall, ps.Precision)
	}
	if plain := contextbench.Score(task, out); !plain.Pass {
		t.Fatal("precondition: H2-1 substring scoring passes the same dump — which is why H2-2 exists")
	}
}

func TestScoreWithPrecision_NoFenceIsAFailNotAFallback(t *testing.T) {
	task := contextbench.Task{ID: "T", GroundTruth: []string{"cmd/a.go"}, ScoringRule: contextbench.ExactMatch}
	ps := contextbench.ScoreWithPrecision(task, "The file is cmd/a.go.")
	if ps.Pass || ps.Fenced {
		t.Fatalf("prose without a fenced block must not pass under H2-2: %+v", ps)
	}
	if !reflect.DeepEqual(ps.Missing, []string{"cmd/a.go"}) {
		t.Errorf("missing = %v", ps.Missing)
	}
}

func TestScoreWithPrecision_LastFenceWinsAndTestFilesLeaveGold(t *testing.T) {
	task := contextbench.Task{ID: "T", GroundTruth: []string{"cmd/a.go", "cmd/a_test.go"}, ScoringRule: contextbench.AllOf}
	out := "```go\nscratch\n```\nfinal:\n```\ncmd/a.go\n```\n"
	ps := contextbench.ScoreWithPrecision(task, out)
	if !ps.Pass {
		t.Fatalf("the test file is excluded from gold, so cmd/a.go alone is a full answer: %+v", ps)
	}
	if ps.Predicted != 1 {
		t.Errorf("only the last block is the answer, predicted = %d", ps.Predicted)
	}
	if got := contextbench.GoldWithoutTestFiles([]string{"x_test.go", "y.go"}); !reflect.DeepEqual(got, []string{"y.go"}) {
		t.Errorf("GoldWithoutTestFiles = %v", got)
	}
}

func TestLastFencedBlock(t *testing.T) {
	cases := map[string]struct {
		in   string
		want string
		ok   bool
	}{
		"none":         {"no fences", "", false},
		"one":          {"```\na\n```", "a\n", true},
		"info string":  {"```text\na\n```", "a\n", true},
		"unterminated": {"```\na\nb", "a\nb", true},
		"last of two":  {"```\nfirst\n```\n```\nsecond\n```", "second\n", true},
	}
	for name, c := range cases {
		got, ok := contextbench.LastFencedBlock(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: got (%q,%v), want (%q,%v)", name, got, ok, c.want, c.ok)
		}
	}
}

// ── PB-9: the memorisation floor ─────────────────────────────────────

func TestLeakedTasks_TwoOfThreeInTheNoneArmIsLeaked(t *testing.T) {
	in := []contextbench.ArmResult{
		res("T1", "none", 1, true, 1), res("T1", "none", 2, true, 1), res("T1", "none", 3, false, 1),
		res("T2", "none", 1, true, 1), res("T2", "none", 2, false, 1), res("T2", "none", 3, false, 1),
		res("T3", "fathom", 1, true, 1), res("T3", "fathom", 2, true, 1), res("T3", "fathom", 3, true, 1),
		res("T0", "none", 1, true, 1), res("T0", "none", 2, true, 1), res("T0", "none", 3, true, 1),
	}
	rule := contextbench.LeakRule{Arm: "none", MinPasses: 2, OfTrials: 3}
	got := contextbench.LeakedTasks(in, rule)
	if !reflect.DeepEqual(got, []string{"T0", "T1"}) {
		t.Fatalf("leaked = %v, want [T0 T1] (sorted; T2 passed once, T3 has no none cell)", got)
	}
	if contextbench.LeakedTasks(in, contextbench.LeakRule{}) != nil {
		t.Error("no rule marks nothing")
	}
	rest := contextbench.WithoutTasks(in, got)
	for _, r := range rest {
		if r.TaskID == "T0" || r.TaskID == "T1" {
			t.Fatalf("leaked task survived exclusion: %+v", r)
		}
	}
	if len(rest) != 6 {
		t.Errorf("want 6 rows after exclusion, got %d", len(rest))
	}
}

func TestLeakRuleIsCommittedInTheHomeCorpus(t *testing.T) {
	c, err := contextbench.LoadCriterion("../../testdata/contextbench/tasks-home.json")
	if err != nil {
		t.Fatal(err)
	}
	if c.LeakRule == nil || c.LeakRule.Arm != "none" || c.LeakRule.MinPasses != 2 {
		t.Fatalf("the default criterion must carry the registered leak rule (none arm, >=2 of 3), got %+v", c.LeakRule)
	}
}

func TestH2_2IsRegisteredBesideH2_1(t *testing.T) {
	h21, err := contextbench.LoadCriterionByID("../../testdata/contextbench/tasks-home.json", "H2-1")
	if err != nil {
		t.Fatal(err)
	}
	h22, err := contextbench.LoadCriterionByID("../../testdata/contextbench/tasks-home.json", "H2-2")
	if err != nil {
		t.Fatalf("H2-2 must be registered in the corpus: %v", err)
	}
	if h22.Scoring != contextbench.ScoringPrecision {
		t.Errorf("H2-2 is the precision-aware criterion, got scoring %q", h22.Scoring)
	}
	// H2-1's bar is untouched by the registration: same strictness, same
	// cap, same floor — H2-2 adds a success definition, not a softer gate.
	if h22.RequireStrictlyMoreSuccesses != h21.RequireStrictlyMoreSuccesses || h22.MaxTokenRatio != h21.MaxTokenRatio || h22.MinTrials != h21.MinTrials {
		t.Errorf("H2-2 must keep H2-1's bar: %+v vs %+v", h22, h21)
	}
	if h21.Scoring != "" && h21.Scoring != contextbench.ScoringSubstring {
		t.Errorf("H2-1 stays substring-scored; its verdicts stand as history")
	}
	if _, err := contextbench.LoadCriterionByID("../../testdata/contextbench/tasks-home.json", "H9-9"); err == nil {
		t.Error("an unregistered criterion id must error, never default")
	}
	if byID, err := contextbench.LoadCriterionByID("../../testdata/contextbench/tasks-home.json", "H2-1"); err != nil || byID.ID != h21.ID {
		t.Errorf("LoadCriterionByID(H2-1) must return the machine_criterion block: %v", err)
	}
}

// ── PB-11: transcript parsing ────────────────────────────────────────

const streamTranscript = `{"type":"system","subtype":"init","model":"claude-sonnet-4-6"}
{"type":"assistant","message":{"content":[{"type":"text","text":"Looking."},{"type":"tool_use","name":"mcp__fathom__find_symbol","input":{}}],"usage":{"input_tokens":10,"cache_creation_input_tokens":100,"cache_read_input_tokens":0,"output_tokens":5}}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"nothing here"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__fathom__references","input":{}}],"usage":{"input_tokens":20,"cache_creation_input_tokens":0,"cache_read_input_tokens":100,"output_tokens":5}}}
{"type":"user","message":{"content":[{"type":"tool_result","content":[{"type":"text","text":"cmd/fathom/servehttp.go:105"}]}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"The file is cmd/fathom/servehttp.go."},{"type":"tool_use","name":"x","input":{}}],"usage":{"input_tokens":30,"output_tokens":5}}}
{"type":"result","subtype":"success","total_cost_usd":0.1}
`

func TestFirstGoldHit_CountsTokensAndToolCallsToTheFirstHit(t *testing.T) {
	got := contextbench.FirstGoldHit([]byte(streamTranscript), []string{"cmd/fathom/servehttp.go"})
	if !got.Found || got.FirstHitToken != "cmd/fathom/servehttp.go" {
		t.Fatalf("want a hit on the second tool result, got %+v", got)
	}
	// Two assistant turns before the tool result that carried the token:
	// (10+100+0+5) + (20+0+100+5) = 240; two tool calls to that point;
	// three over the whole transcript.
	if got.TokensToFirstHit != 240 || got.ToolCallsToFirstHit != 2 || got.ToolCalls != 3 {
		t.Errorf("tokens/calls-to-hit/calls = %d/%d/%d, want 240/2/3", got.TokensToFirstHit, got.ToolCallsToFirstHit, got.ToolCalls)
	}
	again := contextbench.FirstGoldHit([]byte(streamTranscript), []string{"cmd/fathom/servehttp.go"})
	if !reflect.DeepEqual(got, again) {
		t.Error("parsing must be deterministic")
	}
}

func TestFirstGoldHit_NoHitCoversTheWholeTranscript(t *testing.T) {
	got := contextbench.FirstGoldHit([]byte(streamTranscript), []string{"never-said"})
	if got.Found {
		t.Fatal("no gold token appears")
	}
	if got.TokensToFirstHit != 275 || got.ToolCalls != 3 {
		t.Errorf("without a hit the counts cover the whole transcript: %+v", got)
	}
}
