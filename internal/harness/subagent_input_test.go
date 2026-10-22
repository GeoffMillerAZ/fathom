package harness

import "testing"

func TestAD12SpawnDescriptor(t *testing.T) {
	d, _ := Lookup(ClaudeCode)
	found := false
	for _, b := range d.Hooks {
		if b.Harness == "PreToolUse" && b.Matcher == "Agent|Task" {
			found = b.Output["updated_input"] == "hookSpecificOutput.updatedInput" && b.Verb == "edit-hook --pre" && b.TimeoutSec <= 5
		}
	}
	if !found {
		t.Error("Claude Code spawn rewrite descriptor missing")
	}
}
