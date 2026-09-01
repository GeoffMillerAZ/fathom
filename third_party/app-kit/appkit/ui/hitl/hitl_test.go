package hitl

import (
	"strings"
	"testing"
)

func TestApprovalCard_Basic(t *testing.T) {
	got := RenderApprovalCard(ApprovalCard{
		Intent: "Send email", Title: "Draft to opposing counsel",
		Summary: "A reply proposing a settlement conference.", Risk: RiskMedium,
		Confidence: 0.8, HasConfidence: true,
		Actions: `<button>Approve</button><button>Reject</button>`,
	})
	for _, want := range []string{
		"Send email", "medium risk", "Draft to opposing counsel",
		"settlement conference", "confidence", "Approve", "Reject", `role="group"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("approval card missing %q: %s", want, got)
		}
	}
}

func TestApprovalCard_ImpactStrip(t *testing.T) {
	got := RenderApprovalCard(ApprovalCard{
		Intent: "Add child", Title: "Add Ollie to Abby (Full Time)",
		Effect:  "+$175/wk once enrolled",
		Changes: []Metric{{Label: "Weekly tuition", From: "$8,304", To: "$8,479"}},
		Actions: `<button>Approve</button>`,
	})
	for _, want := range []string{"+$175/wk once enrolled", "Weekly tuition", "$8,304", "→", "$8,479", "border-t"} {
		if !strings.Contains(got, want) {
			t.Errorf("impact strip missing %q: %s", want, got)
		}
	}
	// No impact fields → no strip.
	bare := RenderApprovalCard(ApprovalCard{Intent: "x", Title: "y"})
	if strings.Contains(bare, "border-t") {
		t.Errorf("expected no impact strip when Effect/Changes empty: %s", bare)
	}
	// Escaping.
	xss := RenderApprovalCard(ApprovalCard{Effect: `<script>a</script>`, Changes: []Metric{{Label: "<b>", From: "<i>", To: "<u>"}}})
	if strings.Contains(xss, "<script>") || strings.Contains(xss, "<b>") {
		t.Errorf("impact strip did not escape: %s", xss)
	}
}

func TestApprovalCard_RiskRoles(t *testing.T) {
	cases := map[Risk]string{RiskHigh: "danger", RiskMedium: "caution", RiskLow: "positive"}
	for r, role := range cases {
		got := RenderApprovalCard(ApprovalCard{Intent: "x", Risk: r})
		if !strings.Contains(got, "bg-"+role+"-soft") {
			t.Errorf("risk %s should use %s role: %s", r, role, got)
		}
	}
}

func TestApprovalCard_RichBodyAndEscaping(t *testing.T) {
	rich := RenderApprovalCard(ApprovalCard{Title: "t", SummaryHTML: `see <a href="/x">doc</a>`})
	if !strings.Contains(rich, `<a href="/x">doc</a>`) {
		t.Errorf("SummaryHTML should render verbatim")
	}
	esc := RenderApprovalCard(ApprovalCard{Title: `<script>t</script>`, Summary: `<img src=x>`})
	for _, bad := range []string{"<script>t", "<img src=x>"} {
		if strings.Contains(esc, bad) {
			t.Errorf("approval card did not escape %q", bad)
		}
	}
}

func TestApprovalCard_NoConfidenceNoRating(t *testing.T) {
	got := RenderApprovalCard(ApprovalCard{Intent: "x", Title: "y"})
	if strings.Contains(got, "confidence") {
		t.Errorf("no rating expected when HasConfidence is false")
	}
}

func TestRunTrace_StatesAndStructure(t *testing.T) {
	got := RunTrace([]TraceStep{
		{Label: "Read sources", State: TraceDone, At: "12:01"},
		{Label: "Extract spans", State: TraceDone},
		{Label: "Match elements", State: TraceActive},
		{Label: "Detect conflicts", State: TraceFailed, Detail: "timeout"},
		{Label: "Score coverage", State: TracePending},
	})
	for _, want := range []string{
		"Read sources", "12:01", "bg-positive", "bg-accent",
		"bg-danger", "border-line", "failed", "timeout", `role="list"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("run trace missing %q: %s", want, got)
		}
	}
}

func TestRunTrace_Empty(t *testing.T) {
	got := RunTrace(nil)
	if !strings.Contains(got, "<ol") || strings.Contains(got, "<li") {
		t.Errorf("empty trace should be an empty ol")
	}
}

func TestRunTrace_Escaping(t *testing.T) {
	got := RunTrace([]TraceStep{{Label: `<script>`, Detail: `<b>`, State: TraceDone}})
	for _, bad := range []string{"<script>", "<b>"} {
		if strings.Contains(got, bad) {
			t.Errorf("run trace did not escape %q", bad)
		}
	}
}

func TestHITL_Determinism(t *testing.T) {
	c := ApprovalCard{Intent: "x", Title: "y", Summary: "z", Risk: RiskHigh, Confidence: 0.5, HasConfidence: true}
	if RenderApprovalCard(c) != RenderApprovalCard(c) {
		t.Errorf("approval card non-deterministic")
	}
	s := []TraceStep{{Label: "a", State: TraceDone}, {Label: "b", State: TraceActive}}
	if RunTrace(s) != RunTrace(s) {
		t.Errorf("run trace non-deterministic")
	}
}
