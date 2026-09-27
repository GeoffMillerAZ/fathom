package references_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/references"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// RT-87 (§4.5): reference text search skips a binary file (a NUL in its
// first 8 KiB) whatever its extension, reads lines without splitting the
// whole file, keeps a line longer than any scanner token, and stops at
// Options.Max — reporting that it stopped.
func TestFind_SkipsBinariesStreamsLines(t *testing.T) {
	long := strings.Repeat("x", 200<<10) + " Foo"
	files := []core.SourceFile{
		{Path: "a.bin.go", Content: []byte("Foo\x00Foo\nFoo\n")},
		{Path: "b.go", Content: []byte("Foo\n" + long + "\nbar\r\nFoo\r\nFoo")},
		{Path: "c.go", Content: []byte("Foo\nFoo\n")},
	}
	occs, err := references.Find("Foo", files, references.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var locs []string
	for _, o := range occs {
		locs = append(locs, o.File+":"+strconv.Itoa(o.Line))
		if strings.HasSuffix(o.Text, "\r") || len(o.Text) > 200 {
			t.Fatalf("occurrence text not trimmed/capped: %q", o.Text[:min(len(o.Text), 40)])
		}
	}
	want := "b.go:1 b.go:2 b.go:4 b.go:5 c.go:1 c.go:2"
	if got := strings.Join(locs, " "); got != want {
		t.Fatalf("occurrences = %s, want %s (the binary skipped, every line of b.go read)", got, want)
	}

	m, err := references.NewMatcher("Foo", references.Options{})
	if err != nil {
		t.Fatal(err)
	}
	got, stopped := m.FindInFile("b.go", files[1].Content, 2)
	if len(got) != 2 || !stopped {
		t.Fatalf("FindInFile limit 2 = %d occurrences, stopped=%v; want 2, true", len(got), stopped)
	}
	occs, _ = references.Find("Foo", files, references.Options{Max: 3})
	if len(occs) != 3 {
		t.Fatalf("Find Max 3 = %d occurrences, want 3", len(occs))
	}
}
