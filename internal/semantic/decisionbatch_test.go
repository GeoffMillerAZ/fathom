package semantic

// decisionbatch_test.go — DP-26 (O110): decisionBatchPlan/decisionBatchCaps
// pinned directly, independent of either call site (PrefilterSections and
// decisionrank_test.go's own tests exercise the same rule end to end).

import (
	"context"
	"fmt"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

// planItems builds n decisionPlanItem values, each stateBytes long, IDs
// "q0".."qN-1".
func planItems(n, stateBytes int) []decisionPlanItem {
	items := make([]decisionPlanItem, n)
	for i := range items {
		items[i] = decisionPlanItem{QuestionID: fmt.Sprintf("q%d", i), StateBytes: stateBytes}
	}
	return items
}

func TestDecisionBatchCaps(t *testing.T) {
	t.Run("llm: 20 questions, default byte budget", func(t *testing.T) {
		q, b := decisionBatchCaps(decision.NewMockDecider("llm"))
		if q != prefilterChunkSize {
			t.Errorf("maxQuestions = %d, want %d", q, prefilterChunkSize)
		}
		if b != defaultBatchStateBytes {
			t.Errorf("maxStateBytes = %d, want %d", b, defaultBatchStateBytes)
		}
	})

	t.Run("typesafe (and any unrecognized name): default caps on both", func(t *testing.T) {
		q, b := decisionBatchCaps(decision.NewMockDecider("typesafe"))
		if q != defaultBatchQuestions {
			t.Errorf("maxQuestions = %d, want %d", q, defaultBatchQuestions)
		}
		if b != defaultBatchStateBytes {
			t.Errorf("maxStateBytes = %d, want %d", b, defaultBatchStateBytes)
		}
	})

	t.Run("MaxQuestions() opt-in overrides only the count cap", func(t *testing.T) {
		md := &fixedBatchDecider{name: "llm-variant", max: 5}
		q, b := decisionBatchCaps(md)
		if q != 5 {
			t.Errorf("maxQuestions = %d, want 5 (the opt-in)", q)
		}
		if b != defaultBatchStateBytes {
			t.Errorf("maxStateBytes = %d, want the untouched default %d", b, defaultBatchStateBytes)
		}
	})

	t.Run("MaxStateBytes() opt-in overrides only the byte cap", func(t *testing.T) {
		md := &fixedByteBudgetDecider{name: "typesafe-variant", maxBytes: 1024}
		q, b := decisionBatchCaps(md)
		if q != defaultBatchQuestions {
			t.Errorf("maxQuestions = %d, want the untouched default %d", q, defaultBatchQuestions)
		}
		if b != 1024 {
			t.Errorf("maxStateBytes = %d, want 1024 (the opt-in)", b)
		}
	})

	t.Run("both opt-ins present: each overrides its own dimension", func(t *testing.T) {
		md := &fixedBothCapsDecider{name: "full-opt-in", maxQ: 3, maxB: 500}
		q, b := decisionBatchCaps(md)
		if q != 3 || b != 500 {
			t.Errorf("caps = (%d, %d), want (3, 500)", q, b)
		}
	})
}

func TestDecisionBatchPlan_CountBound(t *testing.T) {
	items := planItems(45, 10) // tiny per-item bytes; count cap (20) binds for "llm"
	plan := decisionBatchPlan(decision.NewMockDecider("llm"), items)
	if len(plan.Oversized) != 0 {
		t.Fatalf("Oversized = %v, want none", plan.Oversized)
	}
	if len(plan.Chunks) != 3 {
		t.Fatalf("Chunks = %d, want 3 (45 at cap 20: 20/20/5)", len(plan.Chunks))
	}
	wantSizes := []int{20, 20, 5}
	total := 0
	for i, c := range plan.Chunks {
		if len(c) != wantSizes[i] {
			t.Errorf("chunk %d size = %d, want %d", i, len(c), wantSizes[i])
		}
		total += len(c)
	}
	if total != 45 {
		t.Errorf("total items across chunks = %d, want 45", total)
	}
}

func TestDecisionBatchPlan_ByteBound(t *testing.T) {
	// 72 items of 2048 bytes each under the default 96KiB budget: no chunk
	// may exceed 96*1024 bytes summed, and none may exceed 64 items either
	// (both caps checked; the byte budget binds first here since
	// 96*1024/2048 = 48 < 64).
	items := planItems(72, 2048)
	plan := decisionBatchPlan(decision.NewMockDecider("typesafe"), items)
	if len(plan.Oversized) != 0 {
		t.Fatalf("Oversized = %v, want none (2048 bytes is far under the budget alone)", plan.Oversized)
	}
	total := 0
	for i, c := range plan.Chunks {
		if len(c) > defaultBatchQuestions {
			t.Errorf("chunk %d has %d items, want <= %d", i, len(c), defaultBatchQuestions)
		}
		sum := len(c) * 2048
		if sum > defaultBatchStateBytes {
			t.Errorf("chunk %d sums to %d bytes, want <= %d", i, sum, defaultBatchStateBytes)
		}
		total += len(c)
	}
	if total != 72 {
		t.Errorf("total items across chunks = %d, want 72", total)
	}
	if len(plan.Chunks) < 2 {
		t.Errorf("Chunks = %d, want > 1 (144KiB total must not fit one 96KiB-budget chunk)", len(plan.Chunks))
	}
}

func TestDecisionBatchPlan_OversizedItemGoesAloneAndIsNamed(t *testing.T) {
	items := []decisionPlanItem{
		{QuestionID: "small-1", StateBytes: 100},
		{QuestionID: "giant", StateBytes: 200 * 1024},
		{QuestionID: "small-2", StateBytes: 100},
	}
	plan := decisionBatchPlan(decision.NewMockDecider("typesafe"), items)

	if len(plan.Oversized) != 1 || plan.Oversized[0].Index != 1 {
		t.Fatalf("Oversized = %+v, want exactly index 1 (\"giant\")", plan.Oversized)
	}
	if plan.Oversized[0].Reason == "" {
		t.Error("Oversized[0].Reason is empty, want a human-readable reason")
	}

	// Every item still appears in Chunks exactly once (a plan is a total
	// partition), the oversized one alone in its own chunk.
	seen := map[int]int{}
	for _, c := range plan.Chunks {
		for _, idx := range c {
			seen[idx]++
		}
	}
	if len(seen) != 3 || seen[0] != 1 || seen[1] != 1 || seen[2] != 1 {
		t.Errorf("each item must appear exactly once across Chunks, got %v", seen)
	}
	for _, c := range plan.Chunks {
		if len(c) == 1 && c[0] == 1 {
			continue // the oversized item's own singleton chunk
		}
		for _, idx := range c {
			if idx == 1 {
				t.Error("the oversized item shares a chunk with another item")
			}
		}
	}
}

func TestDecisionBatchPlan_UnboundedBothCapsIsOneChunk(t *testing.T) {
	md := &fixedBothCapsDecider{name: "unbounded", maxQ: 0, maxB: 0}
	items := planItems(500, 4096)
	plan := decisionBatchPlan(md, items)
	if len(plan.Oversized) != 0 {
		t.Fatalf("Oversized = %v, want none (both caps unbounded)", plan.Oversized)
	}
	if len(plan.Chunks) != 1 || len(plan.Chunks[0]) != 500 {
		t.Fatalf("Chunks = %v, want one chunk of 500 items", plan.Chunks)
	}
}

// fixedByteBudgetDecider is a minimal decision.Decider + maxStateByteser
// test double (fixedBatchDecider's byte-budget sibling).
type fixedByteBudgetDecider struct {
	name     string
	maxBytes int
}

func (f *fixedByteBudgetDecider) Name() string       { return f.name }
func (f *fixedByteBudgetDecider) MaxStateBytes() int { return f.maxBytes }
func (f *fixedByteBudgetDecider) Decide(_ context.Context, req decision.Request) (decision.Response, error) {
	return decision.Response{Provider: f.name}, nil
}

// fixedBothCapsDecider implements both maxQuestionser and maxStateByteser.
type fixedBothCapsDecider struct {
	name string
	maxQ int
	maxB int
}

func (f *fixedBothCapsDecider) Name() string       { return f.name }
func (f *fixedBothCapsDecider) MaxQuestions() int  { return f.maxQ }
func (f *fixedBothCapsDecider) MaxStateBytes() int { return f.maxB }
func (f *fixedBothCapsDecider) Decide(_ context.Context, req decision.Request) (decision.Response, error) {
	return decision.Response{Provider: f.name}, nil
}
