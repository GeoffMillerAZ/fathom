package harness

import (
	"regexp"
	"testing"
)

func TestSpawnMatcherIsExact(t *testing.T) {
	d, _ := Lookup(ClaudeCode)
	for _, b := range d.Hooks {
		if b.Harness != "PreToolUse" || b.Output["updated_input"] == "" {
			continue
		}
		r := regexp.MustCompile(b.Matcher)
		for _, tool := range []string{"Agent", "Task", "mcp__service__AgentInfo", "TaskOutput", "OtherAgent"} {
			if got, want := r.MatchString(tool), tool == "Agent" || tool == "Task"; got != want {
				t.Errorf("spawn matcher %q matched %q = %v, want %v", b.Matcher, tool, got, want)
			}
		}
		return
	}
	t.Fatal("spawn rewrite descriptor missing")
}
