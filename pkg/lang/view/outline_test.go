package view_test

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/view"
)

func osym(key, name, kind, canon, sig string, line, end int) lang.Symbol {
	return lang.Symbol{Key: key, Name: name, Kind: kind, Canonical: canon, Signature: sig, Line: line, EndLine: end}
}

func fixtureResult() lang.Result {
	return lang.Result{
		Dialect: "go", Language: "go", Level: "L3",
		Symbols: []lang.Symbol{
			osym("p.A", "A", "Struct", "type", "", 1, 10),
			osym("p.A.B", "B", "Method", "method", "func (a A) B()", 2, 5),
			osym("p.A.B.C", "C", "Var", "", "", 3, 4),
			osym("p.A.D", "D", "Method", "method", "", 6, 9),
			osym("p.E", "E", "Func", "func", "func E(x int) int", 11, 12),
			osym("p.F", "F", "Func", "func", "", 13, 0),
		},
		References: make([]lang.Reference, 2),
		Unresolved: make([]lang.Unresolved, 1),
		Facts:      make([]lang.Fact, 3),
		Notes:      []string{"n1"},
	}
}

// HLP-2: the text form, exactly.
func TestOutlineTextForm(t *testing.T) {
	want := "# a.go dialect=go level=L3\n" +
		"1-10 type A\n" +
		"  2-5 method B  func (a A) B()\n" +
		"    3-4 Var C\n" +
		"  6-9 method D\n" +
		"11-12 func E  func E(x int) int\n" +
		"13-? func F\n" +
		"# symbols=6 references=2 unresolved=1 facts=3 notes=1\n"
	if got := view.Outline("a.go", fixtureResult(), view.OutlineOptions{}).Text(); got != want {
		t.Errorf("text form:\n--- want\n%s--- got\n%s", want, got)
	}
}

// Incomplete and Skipped reads still produce a document, with the flags in the header.
func TestOutlineIncompleteAndSkipped(t *testing.T) {
	r := lang.Result{Dialect: "go", Level: "L2", Incomplete: true, Symbols: []lang.Symbol{osym("k", "n", "K", "", "", 1, 2)}}
	d := view.Outline("x.go", r, view.OutlineOptions{})
	if !d.Incomplete || len(d.Nodes) != 1 {
		t.Fatalf("%+v", d)
	}
	if got := strings.SplitN(d.Text(), "\n", 2)[0]; got != "# x.go dialect=go level=L2 incomplete" {
		t.Errorf("header %q", got)
	}
	s := view.Outline("x.bin", lang.Result{Language: "binary", Skipped: "binary"}, view.OutlineOptions{})
	want := "# x.bin dialect= level= skipped=binary\n# symbols=0 references=0 unresolved=0 facts=0 notes=0\n"
	if s.Text() != want || s.Skipped != "binary" {
		t.Errorf("skipped text %q", s.Text())
	}
	both := view.Outline("y", lang.Result{Dialect: "d", Level: "L1", Incomplete: true, Skipped: "why"}, view.OutlineOptions{}).Text()
	if !strings.HasPrefix(both, "# y dialect=d level=L1 incomplete skipped=why\n") {
		t.Errorf("both flags %q", both)
	}
}

type flat struct {
	key   string
	depth int
	known bool
}

func flatten(ns []view.OutlineNode, depth int, out *[]flat) {
	for _, n := range ns {
		*out = append(*out, flat{n.Key, depth, n.SpanKnown})
		flatten(n.Children, depth+1, out)
	}
}

func shape(d view.OutlineDoc) []flat {
	var out []flat
	flatten(d.Nodes, 0, &out)
	return out
}

// Nesting per the shared rule: strict containment, equal spans nest first as parent, unknown spans are top level.
func TestOutlineNesting(t *testing.T) {
	r := lang.Result{Symbols: []lang.Symbol{
		osym("z.inner", "", "", "", "", 5, 6),
		osym("a.eq1", "", "", "", "", 3, 8),
		osym("b.eq2", "", "", "", "", 3, 8),
		osym("c.eq3", "", "", "", "", 3, 8),
		osym("outer", "", "", "", "", 1, 20),
		osym("u1", "", "", "", "", 4, 0),
		osym("u2", "", "", "", "", 0, 0),
		osym("sib", "", "", "", "", 9, 12),
		osym("overlap", "", "", "", "", 7, 10), // starts inside eq*, ends outside: not contained
		osym("after", "", "", "", "", 21, 22),
	}}
	got := shape(view.Outline("f", r, view.OutlineOptions{}))
	exp := []flat{
		{"u2", 0, false},
		{"outer", 0, true},
		{"a.eq1", 1, true},
		{"b.eq2", 2, true},
		{"c.eq3", 3, true},
		{"z.inner", 4, true},
		{"overlap", 1, true},
		{"sib", 1, true},
		{"after", 0, true},
	}
	// u1 sorts by Line 4 among the top-level nodes after outer; find and remove it for the structural check.
	var withoutU1 []flat
	var u1 *flat
	for i := range got {
		if got[i].key == "u1" {
			u1 = &got[i]
			continue
		}
		withoutU1 = append(withoutU1, got[i])
	}
	if u1 == nil || u1.depth != 0 || u1.known {
		t.Errorf("u1 = %+v, want top level with SpanKnown false", u1)
	}
	if !reflect.DeepEqual(withoutU1, exp) {
		t.Errorf("nesting:\n got %v\nwant %v", withoutU1, exp)
	}
}

// An independent brute force: the parent of a known-span node is the innermost other known-span symbol that
// holds its whole span and sorts before it in (Line, EndLine desc, Key).
func TestOutlineParentsMatchBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(8))
	for round := 0; round < 300; round++ {
		n := 1 + rng.Intn(14)
		var syms []lang.Symbol
		for i := 0; i < n; i++ {
			line := rng.Intn(12)
			end := line + rng.Intn(8) - 1
			if rng.Intn(6) == 0 {
				end = 0
			}
			syms = append(syms, osym(string(rune('a'+rng.Intn(5)))+string(rune('a'+i)), "n", "K", "", "", line, end))
		}
		doc := view.Outline("f", lang.Result{Symbols: syms}, view.OutlineOptions{})
		gotParent := map[string]string{}
		var walk func(ns []view.OutlineNode, parent string)
		walk = func(ns []view.OutlineNode, parent string) {
			for _, c := range ns {
				gotParent[c.Key] = parent
				walk(c.Children, c.Key)
			}
		}
		walk(doc.Nodes, "")
		if len(gotParent) != n {
			t.Fatalf("round %d: %d nodes, want %d", round, len(gotParent), n)
		}
		known := func(s lang.Symbol) bool { return s.Line > 0 && s.EndLine >= s.Line }
		less := func(a, b lang.Symbol) bool {
			if a.Line != b.Line {
				return a.Line < b.Line
			}
			if a.EndLine != b.EndLine {
				return a.EndLine > b.EndLine
			}
			return a.Key < b.Key
		}
		for _, s := range syms {
			want := ""
			if known(s) {
				var best *lang.Symbol
				for i := range syms {
					p := syms[i]
					if p.Key == s.Key || !known(p) || !(p.Line <= s.Line && s.EndLine <= p.EndLine) || !less(p, s) {
						continue
					}
					// innermost: the latest in listing order
					if best == nil || less(*best, p) {
						c := p
						best = &c
					}
				}
				if best != nil {
					want = best.Key
				}
			}
			if gotParent[s.Key] != want {
				t.Fatalf("round %d: parent of %s = %q, want %q (symbols %v)", round, s.Key, gotParent[s.Key], want, syms)
			}
		}
	}
}

// Byte-equal output for equal input and for symbols supplied in shuffled order.
func TestOutlineDeterministicAndShuffleStable(t *testing.T) {
	r := fixtureResult()
	base := view.Outline("a.go", r, view.OutlineOptions{})
	baseText := base.Text()
	baseJSON, _ := json.Marshal(base)
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 50; i++ {
		s := slices.Clone(r.Symbols)
		rng.Shuffle(len(s), func(a, b int) { s[a], s[b] = s[b], s[a] })
		r2 := r
		r2.Symbols = s
		d := view.Outline("a.go", r2, view.OutlineOptions{})
		j, _ := json.Marshal(d)
		if d.Text() != baseText || string(j) != string(baseJSON) {
			t.Fatalf("shuffle %d changed the outline", i)
		}
	}
	if again := view.Outline("a.go", r, view.OutlineOptions{}).Text(); again != baseText {
		t.Error("equal input gave different bytes")
	}
	// The caller's slice is not reordered.
	if r.Symbols[0].Key != "p.A" || r.Symbols[5].Key != "p.F" {
		t.Error("Outline reordered the caller's symbols")
	}
}

// Every string in an outline is a field of the Result it came from (CH-D4): the doc strings and every token of
// the text form besides the fixed header, footer and layout.
func TestOutlineStringsComeFromTheResult(t *testing.T) {
	r := fixtureResult()
	allowed := map[string]bool{"a.go": true, "": true}
	for _, s := range []string{r.Dialect, r.Language, r.Level, r.Skipped} {
		allowed[s] = true
	}
	for _, s := range r.Symbols {
		for _, f := range []string{s.Key, s.Name, s.Kind, s.Canonical, s.Signature} {
			allowed[f] = true
		}
	}
	d := view.Outline("a.go", r, view.OutlineOptions{})
	var walk func(ns []view.OutlineNode)
	walk = func(ns []view.OutlineNode) {
		for _, n := range ns {
			for _, f := range []string{n.Key, n.Name, n.Kind, n.Canonical, n.Signature} {
				if !allowed[f] {
					t.Errorf("node string %q is not a field of the Result", f)
				}
			}
			walk(n.Children)
		}
	}
	walk(d.Nodes)
	for _, f := range []string{d.Path, d.Dialect, d.Language, d.Level, d.Skipped} {
		if !allowed[f] {
			t.Errorf("doc string %q is not from the Result or the path", f)
		}
	}
	// The doc type has no string field besides those: reflect over it so a new one fails here.
	for _, typ := range []reflect.Type{reflect.TypeOf(d), reflect.TypeOf(view.OutlineNode{})} {
		var names []string
		for i := 0; i < typ.NumField(); i++ {
			if typ.Field(i).Type.Kind() == reflect.String {
				names = append(names, typ.Field(i).Name)
			}
		}
		want := map[string][]string{
			"OutlineDoc":  {"Path", "Dialect", "Language", "Level", "Skipped"},
			"OutlineNode": {"Key", "Name", "Kind", "Canonical", "Signature"},
		}[typ.Name()]
		if !reflect.DeepEqual(names, want) {
			t.Errorf("%s string fields = %v, want %v: a new string must be audited against CH-D4", typ.Name(), names, want)
		}
	}
	// A text line of a node carries only its numbers, a canonical or kind, the name and the signature.
	for _, line := range strings.Split(d.Text(), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		body := strings.TrimLeft(line, " ")
		_, after, _ := strings.Cut(body, " ")
		ok := false
		for _, s := range r.Symbols {
			c := s.Canonical
			if c == "" {
				c = s.Kind
			}
			want := c + " " + s.Name
			if s.Signature != "" {
				want += "  " + s.Signature
			}
			if after == want {
				ok = true
			}
		}
		if !ok {
			t.Errorf("node line %q carries text that is not from one symbol", line)
		}
	}
}

func chain() lang.Result {
	return lang.Result{Dialect: "d", Level: "L1", Symbols: []lang.Symbol{
		osym("A", "A", "K", "", "", 1, 10),
		osym("B", "B", "K", "", "", 2, 5),
		osym("C", "C", "K", "", "", 3, 4),
		osym("D", "D", "K", "", "", 6, 9),
		osym("E", "E", "K", "", "", 11, 12),
	}}
}

func TestOutlineMaxNodes(t *testing.T) {
	cases := []struct {
		max     int
		keys    []string
		omitted int
	}{
		{0, []string{"A", "B", "C", "D", "E"}, 0},
		{-1, []string{"A", "B", "C", "D", "E"}, 0},
		{5, []string{"A", "B", "C", "D", "E"}, 0},
		{4, []string{"A", "B", "D", "E"}, 1}, // the deepest level (C) goes first
		{3, []string{"A", "E"}, 3},           // then the next level (B, D), though that leaves fewer than 3
		{2, []string{"A", "E"}, 3},
		{1, []string{"A"}, 4}, // the top level alone exceeds it: keep the first in order
	}
	for _, c := range cases {
		d := view.Outline("f", chain(), view.OutlineOptions{MaxNodes: c.max})
		var ks []string
		for _, f := range shape(d) {
			ks = append(ks, f.key)
		}
		if !reflect.DeepEqual(ks, c.keys) || d.Omitted != c.omitted {
			t.Errorf("MaxNodes %d: keys %v omitted %d, want %v omitted %d", c.max, ks, d.Omitted, c.keys, c.omitted)
		}
		if d.Symbols != 5 {
			t.Errorf("MaxNodes %d: Symbols = %d, the count of the Result, want 5", c.max, d.Symbols)
		}
		lines := strings.Split(strings.TrimSuffix(d.Text(), "\n"), "\n")
		last := lines[len(lines)-1]
		if c.omitted > 0 && !strings.HasSuffix(last, " omitted="+itoa(c.omitted)) {
			t.Errorf("MaxNodes %d: last line %q does not state the omission", c.max, last)
		}
		if c.omitted == 0 && strings.Contains(last, "omitted") {
			t.Errorf("MaxNodes %d: last line %q states an omission that did not happen", c.max, last)
		}
	}
	// Top level over the limit keeps children out too.
	r := lang.Result{Symbols: []lang.Symbol{osym("a", "a", "K", "", "", 1, 3), osym("b", "b", "K", "", "", 2, 2), osym("c", "c", "K", "", "", 4, 5), osym("d", "d", "K", "", "", 6, 7)}}
	d := view.Outline("f", r, view.OutlineOptions{MaxNodes: 2})
	if got := shape(d); !reflect.DeepEqual(got, []flat{{"a", 0, true}, {"c", 0, true}}) || d.Omitted != 2 {
		t.Errorf("top level truncation: %v omitted %d", got, d.Omitted)
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

// The JSON form of a doc: children and nodes are arrays, never null.
func TestOutlineJSONShape(t *testing.T) {
	d := view.Outline("f", lang.Result{Symbols: []lang.Symbol{osym("a", "a", "K", "", "", 1, 2)}}, view.OutlineOptions{})
	b, _ := json.Marshal(d)
	want := `{"path":"f","dialect":"","language":"","level":"","skipped":"","incomplete":false,"nodes":[{"key":"a","name":"a","kind":"K","canonical":"","signature":"","line":1,"end_line":2,"span_known":true,"children":[]}],"symbols":1,"references":0,"unresolved":0,"facts":0,"notes":0,"omitted":0}`
	if string(b) != want {
		t.Errorf("json:\n got %s\nwant %s", b, want)
	}
	e, _ := json.Marshal(view.Outline("f", lang.Result{}, view.OutlineOptions{}))
	if !strings.Contains(string(e), `"nodes":[]`) {
		t.Errorf("empty nodes must be [], got %s", e)
	}
}

// A Go Signature carries raw newlines and tabs. The text form stays line-oriented: a run of whitespace that holds a
// newline or CR becomes one space in Text only, and the node strings (JSON) are unchanged.
func TestOutlineTextIsOneLinePerNode(t *testing.T) {
	multi := "func F(\n\ta int,\r\n\tb int,\n) (\n\tint,\n\terror,\n)"
	r := lang.Result{Dialect: "go", Level: "L3", Symbols: []lang.Symbol{
		osym("p.F", "F", "Func", "function", multi, 1, 9),
		osym("p.G", "G", "Func", "function", "func G() {\n\n\n}\n", 10, 11), // a stripped comment leaves an empty line
		osym("p.H", "H", "Func", "function", "func H(a  int,\tb int)", 12, 13),
		osym("p.N", "N", "Func", "function", "\n", 14, 15),
	}}
	d := view.Outline("a.go", r, view.OutlineOptions{})
	want := "# a.go dialect=go level=L3\n" +
		"1-9 function F  func F( a int, b int, ) ( int, error, )\n" +
		"10-11 function G  func G() { }\n" +
		"12-13 function H  func H(a  int,\tb int)\n" +
		"14-15 function N\n" +
		"# symbols=4 references=0 unresolved=0 facts=0 notes=0\n"
	if got := d.Text(); got != want {
		t.Errorf("text:\n--- want\n%q\n--- got\n%q", want, d.Text())
	}
	if n := strings.Count(d.Text(), "\n"); n != 4+2 {
		t.Errorf("%d lines, want nodes+2", n)
	}
	if d.Nodes[0].Signature != multi {
		t.Errorf("node signature changed: %q", d.Nodes[0].Signature)
	}
	if strings.ContainsAny(d.Text(), "\r") {
		t.Error("CR in text")
	}
}

// Keys are not unique in real reads: one-line TypeScript overloads give two o.f symbols on the same span. The
// outline is byte-equal under every input order.
func TestOutlineShuffleStableOnEqualKeyAndSpan(t *testing.T) {
	mk := func(sig, kind string) lang.Symbol {
		return lang.Symbol{Key: "o.f", Name: "f", Package: "o", Kind: kind, Canonical: "method", Signature: sig, Line: 1, EndLine: 1, Tier: "verified"}
	}
	r := lang.Result{Dialect: "typescript", Level: "L3", Symbols: []lang.Symbol{
		osym("o", "o", "Class", "class", "", 1, 1),
		mk("f(a: string): void", "Method"),
		mk("f(a: number): void", "Method"),
		mk("f(a: number): void", "Overload"),
		mk("f(a: number): void", "Method"),
	}}
	r.Symbols[4].Tier = "assumed"
	base := view.Outline("a.ts", r, view.OutlineOptions{})
	baseJSON, _ := json.Marshal(base)
	rng := rand.New(rand.NewSource(11))
	for i := 0; i < 200; i++ {
		s := slices.Clone(r.Symbols)
		rng.Shuffle(len(s), func(a, b int) { s[a], s[b] = s[b], s[a] })
		r2 := r
		r2.Symbols = s
		d := view.Outline("a.ts", r2, view.OutlineOptions{})
		j, _ := json.Marshal(d)
		if d.Text() != base.Text() || string(j) != string(baseJSON) {
			t.Fatalf("shuffle %d changed the outline:\n%s\nvs\n%s", i, d.Text(), base.Text())
		}
		ix := view.NewIndex(r2)
		a, _ := ix.Enclosing(1)
		b, _ := view.NewIndex(r).Enclosing(1)
		if a != b {
			t.Fatalf("shuffle %d changed Enclosing: %+v vs %+v", i, a, b)
		}
	}
}

// The one containment rule: Outline's parent of each node is the Index's innermost symbol over the symbols listed
// before it, so Outline and EnclosingRange cannot drift apart.
func TestOutlineParentEqualsIndexEnclosingRange(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	for round := 0; round < 300; round++ {
		n := 1 + rng.Intn(12)
		r := lang.Result{Dialect: "go", Level: "L3"}
		for i := 0; i < n; i++ {
			line := 1 + rng.Intn(8)
			end := line + rng.Intn(6)
			if rng.Intn(6) == 0 {
				end = 0
			}
			r.Symbols = append(r.Symbols, lang.Symbol{Key: fmt.Sprintf("k%d", rng.Intn(4)), Name: fmt.Sprintf("n%d", i), Line: line, EndLine: end})
		}
		doc := view.Outline("x", r, view.OutlineOptions{})
		var flat []view.OutlineNode
		parent := map[string]string{}
		var walk func(ns []view.OutlineNode, p string)
		walk = func(ns []view.OutlineNode, p string) {
			for _, c := range ns {
				flat = append(flat, c)
				parent[c.Name] = p
				walk(c.Children, c.Name)
			}
		}
		walk(doc.Nodes, "")
		if len(flat) != n {
			t.Fatalf("round %d: %d nodes, want %d", round, len(flat), n)
		}
		// Symbols before s in listing order, as a Result of their own.
		sorted := slices.Clone(r.Symbols)
		slices.SortStableFunc(sorted, func(a, b lang.Symbol) int {
			return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(b.EndLine, a.EndLine), cmp.Compare(a.Key, b.Key), cmp.Compare(a.Name, b.Name))
		})
		for i, s := range sorted {
			if s.EndLine < s.Line {
				if parent[s.Name] != "" {
					t.Fatalf("round %d: unknown span %s has parent", round, s.Name)
				}
				continue
			}
			prior := lang.Result{Dialect: "go", Level: "L3", Symbols: sorted[:i]}
			got, st := view.NewIndex(prior).EnclosingRange(s.Line, s.EndLine)
			want := ""
			if st != view.StatusNone {
				want = got.Name
			}
			if parent[s.Name] != want {
				t.Fatalf("round %d: %s parent %q, Index says %q (status %v)\n%+v", round, s.Name, parent[s.Name], want, st, r.Symbols)
			}
		}
	}
}
