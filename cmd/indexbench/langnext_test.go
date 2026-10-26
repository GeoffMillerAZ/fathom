package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestCmdLangNext: the driver's facts (§15) under D6 — open queue items in
// order, each eligible or with the rows still blocking it.
func TestCmdLangNext(t *testing.T) {
	var out, errb bytes.Buffer
	if code := cmdLangNext([]string{"-root", "../..", "-all"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("lang-next -all lists no open item:\n%s", out.String())
	}
	for _, l := range lines {
		if !strings.Contains(l, "\teligible") && !strings.Contains(l, "\twaits: ") && !strings.Contains(l, "\tblocked: ") && !strings.Contains(l, "\tneeds re-planning: ") {
			t.Fatalf("line %q is neither eligible, blocked nor needing re-planning", l)
		}
	}
	// A refused item (§14.2: its row, or its -rest sibling while its row stays
	// open, as Swift's) is never offered: the tracker says re-plan, so must this.
	in, err := langbench.TrackerInputFromRepo("../..")
	if err != nil {
		t.Fatal(err)
	}
	for id, why := range in.Refused {
		row, _, _ := strings.Cut(why, ":")
		for _, l := range lines {
			if strings.HasPrefix(l, id+"\t") && l != id+"\tneeds re-planning: "+row {
				t.Errorf("refused %s prints %q, want %q", id, l, id+"\tneeds re-planning: "+row)
			}
		}
	}
}

// lnPlan parses a fixture plan for the lang-next line tests.
func lnPlan(md string) verify.Plan { return verify.ParseMarkdownPlan("s", []byte(md)) }

// lnLevels is a level function over a fixed map.
func lnLevels(m map[string]string) func(string) (string, bool) {
	return func(id string) (string, bool) { l, ok := m[id]; return l, ok }
}

// TestLangNextLinesDeferredIsNotDone is G2-6 in lang-next's output: an item
// held by a deferred row prints blocked, one whose dependency is refused
// prints "needs re-planning" (both with -all only, as plans next's
// --eligible-only leaves them out), and an item whose own row is [x] or [-]
// is not listed at all.
func TestLangNextLinesDeferredIsNotDone(t *testing.T) {
	q := []langbench.QueueEntry{{ID: "a", PlanRow: "LB-Q-a"}, {ID: "b", PlanRow: "LB-Q-b"}, {ID: "c", PlanRow: "LB-Q-c"}, {ID: "d", PlanRow: "LB-Q-d"}, {ID: "e", PlanRow: "LB-Q-e"}}
	plan := lnPlan("- [x] e {id: LB-Q-e}\n")
	blocks := map[string]langbench.QueueBlock{
		"a": {},
		"b": {Blocked: []string{"LB-F3"}},
		"c": {Replan: []string{"LB-F4"}},
		"d": {Blocked: []string{"LB-F3"}, Replan: []string{"LB-Q-r"}},
	}
	none := lnLevels(nil)
	if got, want := langNextLines(q, plan, none, blocks, false), []string{"a\teligible"}; !reflect.DeepEqual(got, want) {
		t.Errorf("without -all = %q, want %q", got, want)
	}
	want := []string{"a\teligible", "b\tblocked: LB-F3", "c\tneeds re-planning: LB-F4", "d\tneeds re-planning: LB-Q-r; blocked: LB-F3"}
	if got := langNextLines(q, plan, none, blocks, true); !reflect.DeepEqual(got, want) {
		t.Errorf("-all =\n%q\nwant\n%q", got, want)
	}
}

// TestLangNextSeparatesStartableFromWaiting: an item a deferred -rest row
// holds waits (printed with -all only) whatever its reading: partial, none,
// absent or at its target (O1394: sass, cmake, svelte and sentinel printed
// eligible with no reading or level none); an unstarted item with no -rest
// row is eligible; a partial item with no -rest row is eligible and says
// where it stands; order is queue order.
func TestLangNextSeparatesStartableFromWaiting(t *testing.T) {
	q := []langbench.QueueEntry{
		{ID: "go", PlanRow: "LB-Q-go", TargetLevel: "L4"},
		{ID: "fresh", PlanRow: "LB-Q-fresh", TargetLevel: "L3"},
		{ID: "part", PlanRow: "LB-Q-part", TargetLevel: "L4"},
		{ID: "zero", PlanRow: "LB-Q-zero", TargetLevel: "L3"},
		{ID: "held", PlanRow: "LB-Q-held", TargetLevel: "L3"},
		{ID: "unread", PlanRow: "LB-Q-unread", TargetLevel: "L3"},
		{ID: "atgoal", PlanRow: "LB-Q-atgoal", TargetLevel: "L2"},
	}
	long := strings.Repeat("é", 200)
	plan := lnPlan("- [ ] go {id: LB-Q-go}\n" +
		"- [-] go rest {id: LB-Q-go-rest, disposition: deferred, reason: \"r\", trigger: \"a type-resolving oracle lands\"}\n" +
		"- [ ] fresh {id: LB-Q-fresh}\n" +
		"- [ ] part {id: LB-Q-part}\n" +
		"- [ ] zero {id: LB-Q-zero}\n" +
		"- [-] zero rest {id: LB-Q-zero-rest, disposition: deferred, reason: \"r\", trigger: \"t\"}\n" +
		"- [ ] held {id: LB-Q-held}\n" +
		"- [-] held rest {id: LB-Q-held-rest, disposition: deferred, reason: \"r\", trigger: \"" + long + "\"}\n" +
		"- [ ] unread {id: LB-Q-unread}\n" +
		"- [-] unread rest {id: LB-Q-unread-rest, disposition: deferred, reason: \"r\", trigger: \"a grammar is pinned\"}\n" +
		"- [ ] atgoal {id: LB-Q-atgoal}\n" +
		"- [-] atgoal rest {id: LB-Q-atgoal-rest, disposition: deferred, reason: \"r\", trigger: \"the other files\"}\n")
	level := lnLevels(map[string]string{"go": "L3", "part": "L2", "zero": "none", "held": "L1", "atgoal": "L2"})
	blocks := map[string]langbench.QueueBlock{"go": {}, "fresh": {}, "part": {}, "zero": {}, "held": {}, "unread": {}, "atgoal": {}}
	wantEligible := []string{"fresh\teligible", "part\teligible (at L2 of L4)"}
	if got := langNextLines(q, plan, level, blocks, false); !reflect.DeepEqual(got, wantEligible) {
		t.Errorf("without -all = %q, want %q", got, wantEligible)
	}
	wantAll := []string{
		"go\twaits: LB-Q-go-rest (at L3 of L4): a type-resolving oracle lands",
		"fresh\teligible",
		"part\teligible (at L2 of L4)",
		"zero\twaits: LB-Q-zero-rest (at none of L3): t",
		"held\twaits: LB-Q-held-rest (at L1 of L3): " + strings.Repeat("é", 120),
		"unread\twaits: LB-Q-unread-rest (at none of L3): a grammar is pinned",
		"atgoal\twaits: LB-Q-atgoal-rest (at L2 of L2): the other files",
	}
	got := langNextLines(q, plan, level, blocks, true)
	if !reflect.DeepEqual(got, wantAll) {
		t.Errorf("-all =\n%q\nwant\n%q", got, wantAll)
	}
	if again := langNextLines(q, plan, level, blocks, true); !reflect.DeepEqual(again, got) {
		t.Error("lang-next is not deterministic")
	}
}
