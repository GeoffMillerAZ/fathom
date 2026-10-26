package main

import (
	"reflect"
	"strings"
	"testing"
)

// TestKeptLines: line markers reset the source line, blank output lines (a
// skipped region's placeholder) are not kept, a header's lines are its own,
// and a file outside the subject is not recorded.
func TestKeptLines(t *testing.T) {
	out := `# 1 "/r/a.c"
# 1 "<built-in>" 1
#define __X 1
# 1 "/r/a.c" 2
int first;
# 1 "/r/b.h" 1
int b;


# 2 "/r/a.c" 2
int a;

#define KEPT 1
# 1 "/usr/include/stdio.h" 1 3
int outside;
# 5 "/r/a.c" 2
# 1 "/r/skipped.h" 1


# 6 "/r/a.c" 2
`
	active := map[string]map[int]bool{}
	imports := map[importEntry]bool{}
	if err := keptLines(strings.NewReader(out), "/r", "/r", active, imports); err != nil {
		t.Fatal(err)
	}
	got := map[string][][2]int{}
	for p, l := range active {
		got[p] = ranges(l)
	}
	// `int first;` is line 1; after `# 2 "/r/a.c" 2`, `int a;` is line 2, the
	// blank line 3 is not kept, and `#define KEPT 1` (printed by -dD) is line 4.
	if r := got["a.c"]; !reflect.DeepEqual(r, [][2]int{{1, 2}, {4, 4}}) {
		t.Errorf("a.c kept lines %v, want [[1 2] [4 4]]", r)
	}
	if r := got["b.h"]; !reflect.DeepEqual(r, [][2]int{{1, 1}}) {
		t.Errorf("b.h kept lines %v, want [[1 1]]", r)
	}
	// a.c includes b.h at line 1 (the marker resuming a.c says line 2) and
	// skipped.h at line 5; the include of the system header is outside the
	// subject and is no import.
	if len(imports) != 2 || !imports[importEntry{Path: "a.c", Line: 1, ToPath: "b.h"}] || !imports[importEntry{Path: "a.c", Line: 5, ToPath: "skipped.h"}] {
		t.Errorf("imports %v, want a.c:1 -> b.h and a.c:5 -> skipped.h only", imports)
	}
	// skipped.h was entered and keeps nothing: listed, with no range.
	if r, ok := got["skipped.h"]; !ok || len(r) != 0 {
		t.Errorf("skipped.h kept lines %v (listed %v), want listed with none", r, ok)
	}
	if len(got) != 3 {
		t.Errorf("files %v, want a.c, b.h and skipped.h only", got)
	}
}

func TestSplitCommandAndPreprocessArgs(t *testing.T) {
	args := splitCommand(`/usr/bin/cc -DNAME="a b" -I/x -o out.o -c /x/a.c`)
	want := []string{"/usr/bin/cc", "-DNAME=a b", "-I/x", "-o", "out.o", "-c", "/x/a.c"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("splitCommand = %q, want %q", args, want)
	}
	got := preprocessArgs(args)
	wantPP := []string{"/usr/bin/cc", "-DNAME=a b", "-I/x", "/x/a.c", "-E", "-dD", "-w"}
	if !reflect.DeepEqual(got, wantPP) {
		t.Fatalf("preprocessArgs = %q, want %q", got, wantPP)
	}
}

// TestTierFlag (ruling C-R6): the output's tier is the -tier flag's, never a
// hand edit of the JSON; an unknown tier is refused.
func TestTierFlag(t *testing.T) {
	for _, s := range []string{"A", "B"} {
		if got, err := tierFlag(s); err != nil || got != s {
			t.Errorf("tierFlag(%q) = %q, %v", s, got, err)
		}
	}
	if _, err := tierFlag("tier-a"); err == nil {
		t.Error("tierFlag(tier-a): no error")
	}
}
