package labs

import (
	"reflect"
	"testing"
)

// Each rule is read with its near-miss negative: the call one step away
// that the rule must not claim.
func TestClassifyDecisionPointRules(t *testing.T) {
	cases := []struct {
		name   string
		point  Point
		label  Label
		reason Reason
	}{
		{"a bounded Read is a read_source point", Point{Call: Call{Kind: "tool_use", Name: "Read", Shape: "object{number,number,string}"}}, ShouldFire, ReasonBoundedReread},
		{"near miss: a whole-file Read is open discovery", Point{Call: Call{Kind: "tool_use", Name: "Read", Shape: "object{string}"}}, ShouldNot, ReasonOpenDiscovery},
		{"a subagent spawn is a brief point", Point{Call: Call{Kind: "subagent_start", Name: "Task", Shape: "object{string}"}}, ShouldFire, ReasonSubagentSpawn},
		{"near miss: a plain tool call is no spawn", Point{Call: Call{Kind: "tool_use", Name: "TodoWrite", Shape: "object{array[object{string,string}]}"}}, ShouldNot, ReasonNoCompetingSurface},
		{"a Grep needs its query text", Point{Call: Call{Kind: "tool_use", Name: "Grep", Shape: "object{string}"}}, Unclassified, ReasonNeedsArgument},
		{"near miss: a WebFetch competes with no surface", Point{Call: Call{Kind: "tool_use", Name: "WebFetch", Shape: "object{string,string}"}}, ShouldNot, ReasonNoCompetingSurface},
		{"a fathom verb's own rule reads its argument", Point{Call: Call{Kind: "tool_use", Name: "mcp__fathom__find_symbol", Shape: "object{string}"}}, Unclassified, ReasonNeedsArgument},
		{"a blast_radius point needs a definition check", Point{Call: Call{Kind: "tool_use", Name: "mcp__fathom__blast_radius", Shape: "object{string}"}}, Unclassified, ReasonNeedsDefinitionCheck},
		{"a skill point needs the prompt", Point{Call: Call{Kind: "skill", Name: "fathom-plan", Shape: "object{string}"}}, Unclassified, ReasonNoPromptText},
		{"an edit is a hook seam", Point{Call: Call{Kind: "tool_use", Name: "Edit", Shape: "object{string,string,string}"}}, Unclassified, ReasonHookSeam},
		{"near miss: a Bash shell call needs its command", Point{Call: Call{Kind: "tool_use", Name: "Bash", Shape: "object{string}"}}, Unclassified, ReasonNeedsArgument},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			label, reason := ClassifyDecisionPoint(c.point)
			if label != c.label || reason != c.reason {
				t.Fatalf("got (%s, %s), want (%s, %s)", label, reason, c.label, c.reason)
			}
		})
	}
}

func TestSurfaceKeyStripsServerAndPluginPrefixes(t *testing.T) {
	cases := []struct {
		call Call
		want string
	}{
		{Call{Kind: "tool_use", Name: "mcp__fathom__find_symbol"}, "mcp:find_symbol"},
		{Call{Kind: "tool_use", Name: "mcp__other-server__search"}, "mcp:search"},
		{Call{Kind: "skill", Name: "fathom-plan"}, "skill:fathom-plan"},
		{Call{Kind: "skill", Name: "fathom:fathom-verify"}, "skill:fathom-verify"},
		{Call{Kind: "tool_use", Name: "Read"}, ""},
		{Call{Kind: "subagent_start", Name: "Task"}, ""},
	}
	for _, c := range cases {
		if got := SurfaceKey(c.call); got != c.want {
			t.Errorf("SurfaceKey(%q) = %q, want %q", c.call.Name, got, c.want)
		}
	}
}

func TestSubstituteToolFoldsShellCalls(t *testing.T) {
	cases := map[string]string{"Grep": "Grep", "Read": "Read", "Bash": "Bash", "Glob": "Glob", "mcp__fathom__search": ""}
	for name, want := range cases {
		if got := SubstituteTool(Call{Kind: "tool_use", Name: name}); got != want {
			t.Errorf("SubstituteTool(%q) = %q, want %q", name, got, want)
		}
	}
}

// Every classified point is counted, reasons included, and no point's
// branch tree is recoverable from an HMAC (owner ruling R2).
func TestClassifyPointsTallies(t *testing.T) {
	points := []Point{
		{Ordinal: 0, Call: Call{Kind: "tool_use", Name: "Read", Shape: "object{number,number,string}"}},
		{Ordinal: 1, Call: Call{Kind: "tool_use", Name: "Grep", Shape: "object{string}"}},
		{Ordinal: 2, Call: Call{Kind: "tool_use", Name: "Read", Shape: "object{string}"}},
		{Ordinal: 3, Call: Call{Kind: "tool_use", Name: "Grep", Shape: "object{string}"}},
	}
	tally := ClassifyPoints(points)
	if tally.ShouldFire != 1 || tally.ShouldNot != 1 || tally.Unclassified != 2 {
		t.Fatalf("tally: %+v", tally)
	}
	if tally.TreeUnrecovered != len(points) {
		t.Fatalf("tree_unrecovered = %d, want %d", tally.TreeUnrecovered, len(points))
	}
	want := []ReasonCount{
		{Reason: ReasonBoundedReread, Count: 1},
		{Reason: ReasonNeedsArgument, Count: 2},
		{Reason: ReasonOpenDiscovery, Count: 1},
	}
	if !reflect.DeepEqual(tally.Reasons, want) {
		t.Fatalf("reasons: %+v, want %+v", tally.Reasons, want)
	}
}
