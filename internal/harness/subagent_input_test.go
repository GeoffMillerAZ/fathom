package harness

import "testing"

// ADR-080 AD-11/AD-12 append exactly these rows. All earlier table entries
// remain pinned by the original golden literals.
var adviceClaudeHookAdditions = []struct {
	event, matcher, marker, sub string
	timeout                     int
}{
	{"PreToolUse", "^(Agent|Task)$", "fathom edit-hook --pre", "edit-hook --pre", 5},
	{"PostToolUse", `mcp__fathom__(symbol_context|find_symbol|read_source)|mcp__plugin_[^_]*_fathom__(symbol_context|find_symbol|read_source)`, "fathom edit-hook", "edit-hook", 5},
}

func TestAdviceHookTableAdditions(t *testing.T) {
	d, _ := Lookup(ClaudeCode)
	base := len(preSeamClaudeHookTable) + len(postSeamClaudeHookAdditions) + len(waveThreeClaudeHookAdditions) + len(promptHookClaudeHookAddition) + len(testRunClaudeHookAddition) + len(readoutClaudeHookAddition) + len(claimCheckClaudeHookAddition) + len(readOutlineHintClaudeHookAddition)
	for i, want := range adviceClaudeHookAdditions {
		got := d.Hooks[base+i]
		if got.Harness != want.event || got.Matcher != want.matcher || got.Marker != want.marker || got.Verb != want.sub || got.TimeoutSec != want.timeout {
			t.Errorf("addition %d = %+v, want %+v", i, got, want)
		}
	}
}

func TestAD12SpawnDescriptor(t *testing.T) {
	d, _ := Lookup(ClaudeCode)
	found := false
	for _, b := range d.Hooks {
		if b.Harness == "PreToolUse" && b.Matcher == "^(Agent|Task)$" {
			found = b.Output["updated_input"] == "hookSpecificOutput.updatedInput" && b.Verb == "edit-hook --pre" && b.TimeoutSec <= 5
		}
	}
	if !found {
		t.Error("Claude Code spawn rewrite descriptor missing")
	}
}
