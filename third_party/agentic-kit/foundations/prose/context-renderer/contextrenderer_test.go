package contextrenderer_test

import (
	"strings"
	"testing"

	contextrenderer "github.com/geoffmilleraz/agentic-kit/foundations/prose/context-renderer"
)

func TestRenderEmptySegmentsReturnsEmpty(t *testing.T) {
	t.Parallel()
	result := contextrenderer.Render(nil, 100)
	if result != "" {
		t.Errorf("expected empty string for nil segments, got %q", result)
	}
}

func TestRenderZeroBudgetReturnsEmpty(t *testing.T) {
	t.Parallel()
	segs := []contextrenderer.Segment{
		{ID: "s1", Content: "hello", Priority: 5, Tokens: 1},
	}
	result := contextrenderer.Render(segs, 0)
	if result != "" {
		t.Errorf("expected empty string for zero budget, got %q", result)
	}
}

func TestRenderNegativeBudgetReturnsEmpty(t *testing.T) {
	t.Parallel()
	segs := []contextrenderer.Segment{
		{ID: "s1", Content: "hello", Priority: 5, Tokens: 1},
	}
	result := contextrenderer.Render(segs, -1)
	if result != "" {
		t.Errorf("expected empty string for negative budget, got %q", result)
	}
}

func TestRenderAllSegmentsFitInBudget(t *testing.T) {
	t.Parallel()
	segs := []contextrenderer.Segment{
		{ID: "s1", Content: "first", Priority: 10, Tokens: 5},
		{ID: "s2", Content: "second", Priority: 5, Tokens: 5},
	}
	result := contextrenderer.Render(segs, 20)
	if !strings.Contains(result, "first") {
		t.Errorf("expected result to contain 'first', got %q", result)
	}
	if !strings.Contains(result, "second") {
		t.Errorf("expected result to contain 'second', got %q", result)
	}
}

func TestRenderDropsLowestPriorityWhenBudgetTight(t *testing.T) {
	t.Parallel()
	segs := []contextrenderer.Segment{
		{ID: "s-high", Content: "high priority content", Priority: 10, Tokens: 5},
		{ID: "s-low", Content: "low priority content", Priority: 1, Tokens: 5},
	}
	// Budget fits exactly one segment of 5 tokens.
	result := contextrenderer.Render(segs, 5)
	if !strings.Contains(result, "high priority content") {
		t.Errorf("expected high priority content in result, got %q", result)
	}
	if strings.Contains(result, "low priority content") {
		t.Errorf("expected low priority content to be dropped, got %q", result)
	}
}

func TestRenderHighPriorityFirstInOutput(t *testing.T) {
	t.Parallel()
	segs := []contextrenderer.Segment{
		{ID: "s-low", Content: "low", Priority: 1, Tokens: 2},
		{ID: "s-high", Content: "high", Priority: 10, Tokens: 2},
		{ID: "s-mid", Content: "mid", Priority: 5, Tokens: 2},
	}
	result := contextrenderer.Render(segs, 100)
	highIdx := strings.Index(result, "high")
	midIdx := strings.Index(result, "mid")
	lowIdx := strings.Index(result, "low")
	if highIdx > midIdx {
		t.Errorf("expected 'high' before 'mid' in output, got: %q", result)
	}
	if midIdx > lowIdx {
		t.Errorf("expected 'mid' before 'low' in output, got: %q", result)
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	t.Parallel()
	segs := []contextrenderer.Segment{
		{ID: "s1", Content: "alpha", Priority: 3, Tokens: 3},
		{ID: "s2", Content: "beta", Priority: 1, Tokens: 3},
		{ID: "s3", Content: "gamma", Priority: 5, Tokens: 3},
	}
	r1 := contextrenderer.Render(segs, 9)
	r2 := contextrenderer.Render(segs, 9)
	if r1 != r2 {
		t.Errorf("Render is non-deterministic: %q vs %q", r1, r2)
	}
}

func TestRenderTieBrokenByIDOrder(t *testing.T) {
	t.Parallel()
	// Both segments have priority 5; ID ordering should be alphabetical asc.
	segs := []contextrenderer.Segment{
		{ID: "z-seg", Content: "z-content", Priority: 5, Tokens: 3},
		{ID: "a-seg", Content: "a-content", Priority: 5, Tokens: 3},
	}
	result := contextrenderer.Render(segs, 100)
	aIdx := strings.Index(result, "a-content")
	zIdx := strings.Index(result, "z-content")
	if aIdx > zIdx {
		t.Errorf("expected a-seg before z-seg when priority is equal, got: %q", result)
	}
}

func TestRenderSkipsSegmentThatExceedsBudget(t *testing.T) {
	t.Parallel()
	// s2 costs 10 tokens and the remaining budget after s1 is only 5.
	segs := []contextrenderer.Segment{
		{ID: "s1", Content: "fits", Priority: 10, Tokens: 5},
		{ID: "s2", Content: "too-big", Priority: 5, Tokens: 10},
		{ID: "s3", Content: "also-fits", Priority: 1, Tokens: 5},
	}
	result := contextrenderer.Render(segs, 10)
	if !strings.Contains(result, "fits") {
		t.Errorf("expected 'fits' in result, got %q", result)
	}
	if strings.Contains(result, "too-big") {
		t.Errorf("expected 'too-big' to be skipped, got %q", result)
	}
	if !strings.Contains(result, "also-fits") {
		t.Errorf("expected 'also-fits' in result (fits in remaining 5), got %q", result)
	}
}

func TestRenderZeroTokensTreatedAsOne(t *testing.T) {
	t.Parallel()
	segs := []contextrenderer.Segment{
		{ID: "s1", Content: "zero-token-seg", Priority: 5, Tokens: 0},
	}
	// Budget of 1 should accommodate a segment with 0 tokens (treated as 1).
	result := contextrenderer.Render(segs, 1)
	if !strings.Contains(result, "zero-token-seg") {
		t.Errorf("expected segment with zero Tokens to be included (treated as 1), got %q", result)
	}
}
