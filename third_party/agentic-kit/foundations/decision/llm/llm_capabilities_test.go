package llm_test

import (
	"strconv"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/llm"
)

// TestLLMDeclaresNoScoreLevelLimit: LLM renders any Levels into its prompt, so
// it declares no level limit and the generic check never refuses a score for
// it. Its golden prompt and schema stay byte-identical.
func TestLLMDeclaresNoScoreLevelLimit(t *testing.T) {
	caps := decision.CapabilitiesOf(&llm.LLM{})
	if caps.MinScoreLevels != 0 || caps.MaxScoreLevels != 0 {
		t.Fatalf("LLM declares score levels %d..%d, want no limit (0..0)", caps.MinScoreLevels, caps.MaxScoreLevels)
	}
	levels := make([]string, 40)
	for i := range levels {
		levels[i] = strconv.Itoa(i)
	}
	q := decision.Question{ID: "q", Kind: decision.KindScore, Levels: levels}
	if reason, outside := decision.OutsideDeclaredLimits("llm", caps, q); outside {
		t.Errorf("a 40-level score is outside LLM's declared limits (%q), want inside", reason)
	}
}
