package hitl

import (
	"strings"
	"testing"
)

func TestRenderToolCall_OK(t *testing.T) {
	got := RenderToolCall(ToolCall{
		Name: "search_evidence", Status: ToolOK, Summary: "12 results", Duration: "320ms",
		Args: `{"query":"deposit"}`, Result: "12 spans matched",
	})
	for _, want := range []string{
		"search_evidence", "12 results", "320ms",
		"<details", "Arguments", "arguments", "result",
		`{&#34;query&#34;:&#34;deposit&#34;}`, // args escaped
		"12 spans matched",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("tool call missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderToolCall_StatusRoles(t *testing.T) {
	if !strings.Contains(RenderToolCall(ToolCall{Name: "x", Status: ToolError}), cssRole("danger")) {
		t.Error("error status should use danger badge")
	}
	if !strings.Contains(RenderToolCall(ToolCall{Name: "x", Status: ToolRunning}), cssRole("info")) {
		t.Error("running status should use info badge")
	}
	// Default (no status) is pending/neutral and renders a badge.
	if !strings.Contains(RenderToolCall(ToolCall{Name: "x"}), "pending") {
		t.Error("missing status should default to pending")
	}
}

// cssRole is a loose check that the badge references the role's token utility.
func cssRole(role string) string { return role }

func TestRenderToolCall_NoDetailsWhenEmpty(t *testing.T) {
	got := RenderToolCall(ToolCall{Name: "ping", Status: ToolOK})
	if strings.Contains(got, "<details") {
		t.Errorf("no args/result → no details element: %s", got)
	}
	if !strings.Contains(got, "ping") {
		t.Errorf("header should still render: %s", got)
	}
}

func TestRenderToolCall_Escaping(t *testing.T) {
	got := RenderToolCall(ToolCall{Name: `<script>`, Args: `<img onerror=x>`, Result: `</pre><script>`})
	if strings.Contains(got, "<script>") || strings.Contains(got, "<img onerror") {
		t.Errorf("tool call did not escape: %s", got)
	}
}

func TestRenderToolCall_Deterministic(t *testing.T) {
	tc := ToolCall{Name: "t", Status: ToolOK, Args: "a", Result: "r"}
	if RenderToolCall(tc) != RenderToolCall(tc) {
		t.Error("RenderToolCall is non-deterministic")
	}
}
