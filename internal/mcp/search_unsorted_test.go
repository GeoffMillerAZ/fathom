package mcp

// search_unsorted_test.go pins O1447's deployed half: ripgrep runs without
// --sort path (which makes it read one file at a time, about five times
// slower on a cold page cache), and searchRipgrep orders its hits itself,
// so every caller still sees them by path, then line.

import (
	"context"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// TestSearchRipgrepOrdersHitsWithoutSortingTheRun pins the order: rg's own
// --sort path compares path components ("a/x.txt" before "a-b.txt"), a
// parallel run any order; the hits come back in byte order of the path,
// then line, the order every search caller renders.
func TestSearchRipgrepOrdersHitsWithoutSortingTheRun(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not on PATH")
	}
	root := t.TempDir()
	body := "needle one\nfiller\nneedle two\n"
	for _, rel := range []string{"b.txt", "a/x.txt", "a-b.txt", "a/y/z.txt"} {
		writeFlagFixture(t, root, rel, body)
	}
	hits, capped, err := searchRipgrep(context.Background(), root, searchOpts{pattern: "needle", caseSensitive: true, contextN: 1})
	if err != nil || capped != "" {
		t.Fatalf("searchRipgrep: err %v, capped %q", err, capped)
	}
	var got []string
	for _, h := range hits {
		got = append(got, h.File+":"+strconv.Itoa(h.Line))
	}
	want := []string{"a-b.txt:1", "a-b.txt:3", "a/x.txt:1", "a/x.txt:3", "a/y/z.txt:1", "a/y/z.txt:3", "b.txt:1", "b.txt:3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("hits = %v, want %v (path bytes, then line)", got, want)
	}
	if len(hits[0].After) != 1 || hits[0].After[0] != "filler" {
		t.Errorf("context of a-b.txt:1 = %+v, want the filler line after it", hits[0].After)
	}
}

// TestSearchRipgrepCappedOutputStaysDeterministic pins the one case a
// parallel run cannot order: output past the 1 MiB cap stops rg, and which
// files a parallel run reached first varies. A capped search is run again
// in path order, so the same files are served on every call.
func TestSearchRipgrepCappedOutputStaysDeterministic(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not on PATH")
	}
	root := t.TempDir()
	line := "needle padding padding padding padding padding\n"
	for _, rel := range []string{"d.txt", "c.txt", "b.txt", "a.txt", "e.txt", "f.txt"} {
		writeFlagFixture(t, root, rel, strings.Repeat(line, 6000)) // ~290 KB of matches each
	}
	var first []string
	for run := 0; run < 3; run++ {
		hits, capped, err := searchRipgrep(context.Background(), root, searchOpts{pattern: "needle", caseSensitive: true})
		if err != nil || capped == "" {
			t.Fatalf("run %d: err %v, capped %q; want the output cap named", run, err, capped)
		}
		var files []string
		for _, h := range hits {
			if len(files) == 0 || files[len(files)-1] != h.File {
				files = append(files, h.File)
			}
		}
		if files[0] != "a.txt" {
			t.Fatalf("run %d: capped hits start at %s, want a.txt (path order)", run, files[0])
		}
		if run == 0 {
			first = files
		} else if !reflect.DeepEqual(files, first) {
			t.Fatalf("run %d served files %v, run 0 %v: a capped search must not depend on read order", run, files, first)
		}
	}
}
