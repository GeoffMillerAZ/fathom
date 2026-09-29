package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestCmdLangNext: the driver's facts (§15) under D6 — open queue items in
// order, each eligible or with the rows still blocking it.
func TestCmdLangNext(t *testing.T) {
	var out, errb bytes.Buffer
	if code := cmdLangNext([]string{"-root", "../..", "-all"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 || !strings.Contains(out.String(), "cpp") {
		t.Fatalf("lang-next -all lists no cpp line:\n%s", out.String())
	}
	for _, l := range lines {
		if !strings.Contains(l, "\teligible") && !strings.Contains(l, "\tblocked: ") && !strings.Contains(l, "\tneeds re-planning: ") {
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

// TestLangNextLinesDeferredIsNotDone is G2-6 in lang-next's output: an item
// held by a deferred row prints blocked, one whose dependency is refused
// prints "needs re-planning" (both with -all only, as plans next's
// --eligible-only leaves them out), and an item whose own row is [x] or [-]
// is not listed at all.
func TestLangNextLinesDeferredIsNotDone(t *testing.T) {
	q := []langbench.QueueEntry{{ID: "a", PlanRow: "LB-Q-a"}, {ID: "b", PlanRow: "LB-Q-b"}, {ID: "c", PlanRow: "LB-Q-c"}, {ID: "d", PlanRow: "LB-Q-d"}, {ID: "e", PlanRow: "LB-Q-e"}}
	done := map[string]bool{"LB-Q-e": true}
	blocks := map[string]langbench.QueueBlock{
		"a": {},
		"b": {Blocked: []string{"LB-F3"}},
		"c": {Replan: []string{"LB-F4"}},
		"d": {Blocked: []string{"LB-F3"}, Replan: []string{"LB-Q-r"}},
	}
	if got, want := langNextLines(q, done, blocks, false), []string{"a\teligible"}; !reflect.DeepEqual(got, want) {
		t.Errorf("without -all = %q, want %q", got, want)
	}
	want := []string{"a\teligible", "b\tblocked: LB-F3", "c\tneeds re-planning: LB-F4", "d\tneeds re-planning: LB-Q-r; blocked: LB-F3"}
	if got := langNextLines(q, done, blocks, true); !reflect.DeepEqual(got, want) {
		t.Errorf("-all =\n%q\nwant\n%q", got, want)
	}
}
