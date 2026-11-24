package view_test

import (
	"bytes"
	"reflect"
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/view"
)

func sym(key string, line, end int) lang.Symbol {
	return lang.Symbol{Key: key, Name: key, Line: line, EndLine: end}
}

func res(syms ...lang.Symbol) lang.Result {
	return lang.Result{Dialect: "x", Symbols: syms}
}

func keys(syms []lang.Symbol) []string {
	out := []string{}
	for _, s := range syms {
		out = append(out, s.Key)
	}
	return out
}

func TestStatusConstants(t *testing.T) {
	if string(view.StatusFound) != "found" || string(view.StatusNone) != "none" || string(view.StatusUnknown) != "unknown" {
		t.Fatalf("closed set is found, none, unknown: %q %q %q", view.StatusFound, view.StatusNone, view.StatusUnknown)
	}
}

// HLP-1: known span rule. A span is known when EndLine >= Line > 0, inclusive.
func TestEnclosingKnownSpan(t *testing.T) {
	r := res(sym("a", 3, 5), sym("one", 8, 8))
	for _, c := range []struct {
		line int
		want string
		st   view.Status
	}{
		{1, "", view.StatusNone},
		{2, "", view.StatusNone},
		{3, "a", view.StatusFound},
		{4, "a", view.StatusFound},
		{5, "a", view.StatusFound},
		{6, "", view.StatusNone},
		{8, "one", view.StatusFound},
		{9, "", view.StatusNone},
		{1000, "", view.StatusNone},
	} {
		got, st := view.NewIndex(r).Enclosing(c.line)
		if st != c.st || got.Key != c.want {
			t.Errorf("line %d: got %q %q, want %q %q", c.line, got.Key, st, c.want, c.st)
		}
	}
}

// HLP-1: a symbol with EndLine < Line, or Line <= 0, has no known span.
func TestKnownSpanBoundaries(t *testing.T) {
	// EndLine 0 (extractor computed none) and EndLine < Line are unknown, so line 5 cannot be answered.
	for _, s := range []lang.Symbol{sym("u", 5, 0), sym("u", 5, 4)} {
		got, st := view.NewIndex(res(s)).Enclosing(5)
		if st != view.StatusUnknown {
			t.Errorf("%+v: status %q, want unknown", s, st)
		}
		if got.Key != "" {
			t.Errorf("%+v: an unknown-span symbol is no candidate, got %q", s, got.Key)
		}
	}
	// EndLine == Line is known.
	if got, st := view.NewIndex(res(sym("k", 5, 5))).Enclosing(5); st != view.StatusFound || got.Key != "k" {
		t.Errorf("one-line span: %q %q", got.Key, st)
	}
}

// HLP-1: innermost = latest Line, then earliest EndLine, then largest Key.
func TestEnclosingInnermostTieBreak(t *testing.T) {
	for _, c := range []struct {
		name string
		syms []lang.Symbol
		line int
		want string
	}{
		{"nested: latest Line wins", []lang.Symbol{sym("outer", 1, 20), sym("mid", 5, 15), sym("inner", 8, 10)}, 9, "inner"},
		{"nested: outside inner falls to mid", []lang.Symbol{sym("outer", 1, 20), sym("mid", 5, 15), sym("inner", 8, 10)}, 12, "mid"},
		{"same Line: earliest EndLine wins", []lang.Symbol{sym("long", 5, 20), sym("short", 5, 9)}, 6, "short"},
		{"same Line: earliest EndLine wins over key order", []lang.Symbol{sym("a", 5, 20), sym("z", 5, 9)}, 6, "z"},
		{"equal spans: largest Key wins", []lang.Symbol{sym("b", 5, 9), sym("a", 5, 9), sym("c", 5, 9)}, 7, "c"},
		{"overlap: later start wins", []lang.Symbol{sym("a", 1, 10), sym("b", 6, 14)}, 8, "b"},
		{"overlap: only the earlier covers", []lang.Symbol{sym("a", 1, 10), sym("b", 6, 14)}, 3, "a"},
		{"later start beats shorter outer", []lang.Symbol{sym("a", 1, 100), sym("b", 2, 99)}, 50, "b"},
	} {
		for _, perm := range [][]int{{0, 1, 2}, {2, 1, 0}, {1, 2, 0}} {
			var syms []lang.Symbol
			for _, i := range perm {
				if i < len(c.syms) {
					syms = append(syms, c.syms[i])
				}
			}
			got, st := view.NewIndex(res(syms...)).Enclosing(c.line)
			if st != view.StatusFound || got.Key != c.want {
				t.Errorf("%s (order %v): got %q %q, want %q found", c.name, perm, got.Key, st, c.want)
			}
		}
	}
}

// HLP-1: the returned Symbol is the full symbol, not just a key.
func TestEnclosingReturnsWholeSymbol(t *testing.T) {
	want := lang.Symbol{Key: "p.F", Name: "F", Package: "p", Kind: "GoFunc", Canonical: "function", Line: 3, EndLine: 6, Signature: "func F()", Tier: "verified"}
	got, st := view.NewIndex(res(want)).Enclosing(4)
	if st != view.StatusFound || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v %q", got, st)
	}
}

// HLP-1: found degrades to unknown way 1: the read is Incomplete. The candidate is still the hint.
func TestEnclosingIncompleteDegrades(t *testing.T) {
	r := res(sym("a", 3, 5))
	r.Incomplete = true
	got, st := view.NewIndex(r).Enclosing(4)
	if st != view.StatusUnknown || got.Key != "a" {
		t.Errorf("containing line: got %q %q, want hint a, unknown", got.Key, st)
	}
	got, st = view.NewIndex(r).Enclosing(9)
	if st != view.StatusUnknown || got.Key != "" {
		t.Errorf("no candidate: got %q %q, want zero hint, unknown (never none on an incomplete read)", got.Key, st)
	}
}

// HLP-1: found degrades to unknown way 2: an unknown-span symbol could be the innermost: its Line is below 1, or at or after the candidate's Line and at or before line.
func TestEnclosingUnknownAfterCandidateDegrades(t *testing.T) {
	// An unknown span that shares the candidate's Line, and one at Line 0 beside a known span.
	for _, r := range []lang.Result{
		res(sym("a", 5, 20), sym("u", 5, 0)),
		res(sym("a", 1, 20), sym("u", 0, 0)),
		res(sym("a", 5, 20), sym("u", 5, 3)),
	} {
		if _, st := view.NewIndex(r).Enclosing(7); st != view.StatusUnknown {
			t.Errorf("%v: Enclosing(7) = %q, want unknown", r.Symbols, st)
		}
	}
	outer := sym("outer", 1, 20)
	for _, c := range []struct {
		name string
		u    lang.Symbol
		line int
		st   view.Status
	}{
		{"starts inside the candidate, before line", sym("u", 5, 0), 10, view.StatusUnknown},
		{"starts on line itself", sym("u", 10, 0), 10, view.StatusUnknown},
		{"starts after line", sym("u", 11, 0), 10, view.StatusFound},
		{"starts on the candidate's Line: could be nested in it", sym("u", 1, 0), 10, view.StatusUnknown},
		{"Line 0: position unknown", sym("u", 0, 0), 10, view.StatusUnknown},
		{"negative Line: position unknown", sym("u", -3, 0), 10, view.StatusUnknown},
		{"inverted span at the candidate's Line", sym("u", 1, 0), 1, view.StatusUnknown},
		{"EndLine below Line is unknown too", sym("u", 6, 5), 10, view.StatusUnknown},
	} {
		got, st := view.NewIndex(res(outer, c.u)).Enclosing(c.line)
		if st != c.st || got.Key != "outer" {
			t.Errorf("%s: got %q %q, want outer %q", c.name, got.Key, st, c.st)
		}
	}
}

// HLP-1: found degrades to unknown way 3: no candidate and an unknown-span symbol starts at or before the line.
func TestEnclosingNoCandidateUnknownBefore(t *testing.T) {
	r := res(sym("u", 4, 0), sym("k", 20, 22))
	for _, c := range []struct {
		line int
		st   view.Status
		key  string
	}{
		{3, view.StatusNone, ""},    // before the unknown symbol: nothing could hide here
		{4, view.StatusUnknown, ""}, // at its start
		{10, view.StatusUnknown, ""},
		{21, view.StatusFound, "k"}, // u starts before k's Line, so rule 2 does not fire
	} {
		got, st := view.NewIndex(r).Enclosing(c.line)
		if st != c.st || got.Key != c.key {
			t.Errorf("line %d: got %q %q, want %q %q", c.line, got.Key, st, c.key, c.st)
		}
	}
}

// HLP-1: none only when complete, no known span contains the line, and no unknown span starts at or before it.
func TestEnclosingNone(t *testing.T) {
	if got, st := view.NewIndex(res()).Enclosing(1); st != view.StatusNone || got.Key != "" {
		t.Errorf("empty result: %q %q", got.Key, st)
	}
	if got, st := view.NewIndex(res(sym("a", 5, 7), sym("u", 12, 0))).Enclosing(9); st != view.StatusNone || got.Key != "" {
		t.Errorf("gap before the first unknown start: %q %q", got.Key, st)
	}
}

// HLP-1: a Skipped result has no symbols; Enclosing is unknown, Touched empty and incomplete.
func TestSkipped(t *testing.T) {
	r := lang.Result{Skipped: lang.SkipBinary, Symbols: []lang.Symbol{}}
	ix := view.NewIndex(r)
	for _, line := range []int{1, 5, 100} {
		if got, st := ix.Enclosing(line); st != view.StatusUnknown || got.Key != "" {
			t.Errorf("Enclosing(%d): %q %q", line, got.Key, st)
		}
	}
	syms, complete := ix.Touched(1, 10)
	if len(syms) != 0 || complete {
		t.Errorf("Touched: %v %v, want empty and incomplete", keys(syms), complete)
	}
}

// HLP-1: line < 1 and end < start are unknown / incomplete and never panic.
func TestInvalidArguments(t *testing.T) {
	r := res(sym("a", 1, 5), sym("u", 7, 0))
	ix := view.NewIndex(r)
	for _, line := range []int{0, -1, -1 << 31} {
		if _, st := ix.Enclosing(line); st != view.StatusUnknown {
			t.Errorf("Enclosing(%d) = %q, want unknown", line, st)
		}
	}
	for _, c := range [][2]int{{5, 4}, {0, 3}, {-3, -1}, {3, -1}, {1 << 40, 1}} {
		syms, complete := ix.Touched(c[0], c[1])
		if len(syms) != 0 || complete {
			t.Errorf("Touched(%d,%d) = %v %v, want empty, incomplete", c[0], c[1], keys(syms), complete)
		}
	}
	// A zero value Index must not panic either.
	var zero view.Index
	zero.Enclosing(1)
	zero.Touched(1, 2)
	// Huge lines must not panic or loop.
	ix.Enclosing(1 << 40)
	ix.Touched(1, 1<<40)
}

// HLP-1: Touched is every known span that intersects, plus unknown-span symbols whose Line lies in the range.
func TestTouchedMembership(t *testing.T) {
	r := res(
		sym("a", 1, 3), sym("b", 3, 6), sym("c", 8, 9), sym("d", 10, 10),
		sym("big", 1, 100), sym("u1", 5, 0), sym("u2", 12, 11),
	)
	for _, c := range []struct {
		name       string
		start, end int
		want       []string
		complete   bool
	}{
		{"single line shared by three spans, widest first", 3, 3, []string{"big", "a", "b"}, true},
		{"edge: end touches start of c, unknown u1 starts before", 7, 8, []string{"big", "c"}, false},
		{"gap line holds only big", 7, 7, []string{"big"}, false},
		{"range ending at b's last line, u1 outside the range", 6, 9, []string{"big", "b", "c"}, false},
		{"range holding the unknown start is listed", 4, 5, []string{"big", "b", "u1"}, true},
		{"unknown EndLine below Line is listed by Line", 12, 12, []string{"big", "u2"}, false},
		{"range after every unknown start: incomplete", 20, 30, []string{"big"}, false},
		{"whole file", 1, 200, []string{"big", "a", "b", "u1", "c", "d", "u2"}, true},
	} {
		got, complete := view.NewIndex(r).Touched(c.start, c.end)
		if !slices.Equal(keys(got), c.want) || complete != c.complete {
			t.Errorf("%s: Touched(%d,%d) = %v %v, want %v %v", c.name, c.start, c.end, keys(got), complete, c.want, c.complete)
		}
	}
}

// HLP-1: Touched order is Line, then EndLine descending, then Key.
func TestTouchedSortOrder(t *testing.T) {
	syms := []lang.Symbol{
		sym("z", 5, 9), sym("a", 5, 9), sym("m", 5, 12), sym("first", 2, 3),
		sym("u", 5, 0), sym("late", 8, 8),
	}
	want := []string{"first", "m", "a", "z", "u", "late"}
	for _, perm := range [][]int{{0, 1, 2, 3, 4, 5}, {5, 4, 3, 2, 1, 0}, {2, 4, 0, 5, 1, 3}} {
		var in []lang.Symbol
		for _, i := range perm {
			in = append(in, syms[i])
		}
		got, complete := view.NewIndex(res(in...)).Touched(1, 20)
		if !slices.Equal(keys(got), want) || !complete {
			t.Errorf("input order %v: got %v %v, want %v true", perm, keys(got), complete, want)
		}
	}
}

// HLP-1: complete is false when the read is Incomplete or an unknown-span symbol starts before start.
func TestTouchedCompleteFalse(t *testing.T) {
	// unknown-span symbol starts before start
	r := res(sym("a", 10, 12), sym("u", 3, 0))
	if got, c := view.NewIndex(r).Touched(10, 11); c || !slices.Equal(keys(got), []string{"a"}) {
		t.Errorf("unknown before start: %v %v", keys(got), c)
	}
	// unknown-span symbol starting exactly at start does not make it incomplete: it is listed
	r = res(sym("a", 10, 12), sym("u", 10, 0))
	if got, c := view.NewIndex(r).Touched(10, 11); !c || !slices.Equal(keys(got), []string{"a", "u"}) {
		t.Errorf("unknown at start: %v %v", keys(got), c)
	}
	// unknown-span symbol after the range does not matter
	r = res(sym("a", 10, 12), sym("u", 30, 0))
	if got, c := view.NewIndex(r).Touched(10, 11); !c || !slices.Equal(keys(got), []string{"a"}) {
		t.Errorf("unknown after end: %v %v", keys(got), c)
	}
	// Incomplete read
	r = res(sym("a", 10, 12))
	r.Incomplete = true
	if got, c := view.NewIndex(r).Touched(10, 11); c || !slices.Equal(keys(got), []string{"a"}) {
		t.Errorf("incomplete: %v %v", keys(got), c)
	}
	// nothing touched on a complete read is a complete empty answer
	if got, c := view.NewIndex(res(sym("a", 10, 12))).Touched(1, 5); !c || len(got) != 0 {
		t.Errorf("empty complete: %v %v", keys(got), c)
	}
}

func TestSlice(t *testing.T) {
	src := []byte("l1\nl2\r\nl3\nl4")
	for _, c := range []struct {
		name string
		s    lang.Symbol
		want string
		ok   bool
	}{
		{"first line keeps LF", sym("s", 1, 1), "l1\n", true},
		{"CRLF line keeps CRLF", sym("s", 2, 2), "l2\r\n", true},
		{"span of LF and CRLF lines", sym("s", 1, 3), "l1\nl2\r\nl3\n", true},
		{"last line without newline", sym("s", 4, 4), "l4", true},
		{"through the last line", sym("s", 3, 4), "l3\nl4", true},
		{"whole file", sym("s", 1, 4), string(src), true},
		{"EndLine past the file", sym("s", 3, 5), "", false},
		{"Line past the file", sym("s", 9, 9), "", false},
		{"unknown span: EndLine 0", sym("s", 2, 0), "", false},
		{"unknown span: EndLine < Line", sym("s", 3, 2), "", false},
		{"Line 0", sym("s", 0, 2), "", false},
		{"negative", sym("s", -2, 2), "", false},
	} {
		got, ok := view.Slice(src, c.s)
		if ok != c.ok || (c.ok && string(got) != c.want) || (!c.ok && len(got) != 0) {
			t.Errorf("%s: got %q %v, want %q %v", c.name, got, ok, c.want, c.ok)
		}
	}
	// A trailing newline ends the last line: EndLine one past it is outside the file.
	if got, ok := view.Slice([]byte("a\nb\n"), sym("s", 2, 3)); ok || len(got) != 0 {
		t.Errorf("EndLine past a newline-terminated file: %q %v", got, ok)
	}
	if got, ok := view.Slice(nil, sym("s", 1, 1)); ok || len(got) != 0 {
		t.Errorf("nil src: %q %v", got, ok)
	}
	// a file that ends with a newline: the last line is the one before the final newline
	if got, ok := view.Slice([]byte("a\nb\n"), sym("s", 2, 2)); !ok || string(got) != "b\n" {
		t.Errorf("trailing newline: %q %v", got, ok)
	}
	// Slice reads src and nothing else, and does not change it.
	keep := bytes.Clone(src)
	view.Slice(src, sym("s", 1, 4))
	if !bytes.Equal(src, keep) {
		t.Error("Slice changed src")
	}
}

// HLP-1: NewIndex copies what it keeps, so later mutation of the caller's slices changes no answer.
func TestNewIndexCopies(t *testing.T) {
	syms := []lang.Symbol{sym("a", 1, 10), sym("b", 3, 5), sym("u", 20, 0)}
	r := res(syms...)
	ix := view.NewIndex(r)
	wantE, wantS := ix.Enclosing(4)
	wantT, wantC := ix.Touched(1, 30)
	wantT = slices.Clone(wantT)

	// mutate every way a caller could
	syms[0].Line, syms[0].EndLine, syms[0].Key = 50, 60, "zz"
	syms[1] = sym("evil", 1, 100)
	syms[2].EndLine = 25
	r.Symbols = append(r.Symbols[:0], sym("other", 1, 1))
	r.Incomplete = true
	r.Skipped = lang.SkipBinary

	gotE, gotS := ix.Enclosing(4)
	gotT, gotC := ix.Touched(1, 30)
	if gotE != wantE || gotS != wantS || gotC != wantC || !reflect.DeepEqual(gotT, wantT) {
		t.Errorf("answers moved after caller mutation: %v %v / %v %v", gotE.Key, gotS, keys(gotT), gotC)
	}

	// mutating a returned slice must not change later answers either
	for i := range gotT {
		gotT[i].Key = "mut"
	}
	again, _ := ix.Touched(1, 30)
	if !reflect.DeepEqual(again, wantT) {
		t.Errorf("returned slice aliases the index: %v", keys(again))
	}
}

// Equal input, equal answer, whatever the symbol order of the input.
func TestInputOrderIrrelevant(t *testing.T) {
	a := []lang.Symbol{sym("a", 1, 10), sym("b", 1, 10), sym("c", 4, 6), sym("u", 5, 0), sym("d", 7, 7)}
	b := []lang.Symbol{a[4], a[2], a[0], a[3], a[1]}
	x, y := view.NewIndex(res(a...)), view.NewIndex(res(b...))
	for line := 0; line < 14; line++ {
		g1, s1 := x.Enclosing(line)
		g2, s2 := y.Enclosing(line)
		if g1 != g2 || s1 != s2 {
			t.Errorf("Enclosing(%d) differs by input order", line)
		}
		for end := line; end < 14; end++ {
			t1, c1 := x.Touched(line, end)
			t2, c2 := y.Touched(line, end)
			if !reflect.DeepEqual(t1, t2) || c1 != c2 {
				t.Errorf("Touched(%d,%d) differs by input order", line, end)
			}
		}
	}
}
