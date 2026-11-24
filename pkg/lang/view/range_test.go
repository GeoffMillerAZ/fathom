package view_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/view"
)

// refEnclosingRange is the brute-force reference of EnclosingRange (ESCALATE-26 Q2): the innermost known span
// holding the whole range, with START in place of the line in the unknown-span rules. It shares no code with
// the implementation.
func refEnclosingRange(r lang.Result, start, end int) (lang.Symbol, view.Status) {
	if start < 1 || end < start || r.Skipped != "" {
		return lang.Symbol{}, view.StatusUnknown
	}
	var cand lang.Symbol
	have := false
	for _, s := range r.Symbols {
		if !refKnown(s) || start < s.Line || end > s.EndLine {
			continue
		}
		if !have || s.Line > cand.Line ||
			(s.Line == cand.Line && s.EndLine < cand.EndLine) ||
			(s.Line == cand.Line && s.EndLine == cand.EndLine && s.Key > cand.Key) {
			cand, have = s, true
		}
	}
	hidden := false
	for _, s := range r.Symbols {
		if refKnown(s) {
			continue
		}
		if (have && (s.Line < 1 || s.Line >= cand.Line) && s.Line <= start) || (!have && s.Line <= start) {
			hidden = true
		}
	}
	switch {
	case r.Incomplete || hidden:
		return cand, view.StatusUnknown
	case !have:
		return lang.Symbol{}, view.StatusNone
	}
	return cand, view.StatusFound
}

// HLP-1 (ESCALATE-26 Q2): EnclosingRange table, one case per clause.
func TestEnclosingRange(t *testing.T) {
	r := res(sym("outer", 1, 20), sym("inner", 5, 10), sym("later", 12, 14), sym("u", 7, 0))
	inc := r
	inc.Incomplete = true
	for _, c := range []struct {
		name       string
		r          lang.Result
		start, end int
		want       string
		st         view.Status
	}{
		{"range inside the innermost span, unknown starts after it", res(sym("outer", 1, 20), sym("inner", 5, 10)), 6, 9, "inner", view.StatusFound},
		{"range leaves the inner span: the outer holds it", res(sym("outer", 1, 20), sym("inner", 5, 10)), 6, 11, "outer", view.StatusFound},
		{"range edges inclusive", res(sym("outer", 1, 20), sym("inner", 5, 10)), 5, 10, "inner", view.StatusFound},
		{"no span holds the whole range", res(sym("a", 1, 3), sym("b", 4, 6)), 3, 4, "", view.StatusNone},
		{"unknown starts strictly inside the range: cannot hold it, found", r, 6, 8, "inner", view.StatusFound},
		{"unknown starts after END: found", r, 5, 6, "inner", view.StatusFound},
		{"unknown starts at START after the candidate: unknown", r, 7, 8, "inner", view.StatusUnknown},
		{"unknown starts before START after the candidate: unknown", r, 8, 9, "inner", view.StatusUnknown},
		{"unknown starts at the candidate's Line: could be nested in it, unknown", res(sym("c", 7, 9), sym("u", 7, 0)), 8, 8, "c", view.StatusUnknown},
		{"unknown starts before the candidate: cannot be inner, found", res(sym("c", 7, 9), sym("u", 3, 0)), 8, 8, "c", view.StatusFound},
		{"unknown at Line 0 beside a candidate: unknown", res(sym("c", 1, 9), sym("u", 0, 0)), 8, 8, "c", view.StatusUnknown},
		{"no candidate, unknown at or before START: unknown", res(sym("a", 1, 3), sym("u", 6, 0)), 8, 9, "", view.StatusUnknown},
		{"no candidate, unknown only after END: none", res(sym("a", 1, 3), sym("u", 12, 0)), 8, 9, "", view.StatusNone},
		{"no candidate, unknown strictly inside the range: none", res(sym("a", 1, 3), sym("u", 9, 0)), 8, 10, "", view.StatusNone},
		{"incomplete read: hint, never found", inc, 12, 13, "later", view.StatusUnknown},
		{"incomplete read, no candidate: never none", lang.Result{Incomplete: true, Symbols: []lang.Symbol{sym("a", 1, 2)}}, 5, 6, "", view.StatusUnknown},
		{"start below 1", r, 0, 3, "", view.StatusUnknown},
		{"end below start", r, 6, 5, "", view.StatusUnknown},
	} {
		got, st := view.NewIndex(c.r).EnclosingRange(c.start, c.end)
		if st != c.st || got.Key != c.want {
			t.Errorf("%s: EnclosingRange(%d,%d) = %q %q, want %q %q", c.name, c.start, c.end, got.Key, st, c.want, c.st)
		}
	}
	if got, st := view.NewIndex(lang.Result{Skipped: lang.SkipBinary}).EnclosingRange(1, 2); st != view.StatusUnknown || got.Key != "" {
		t.Errorf("skipped: %q %q", got.Key, st)
	}
	var zero view.Index
	if _, st := zero.EnclosingRange(1, 2); st != view.StatusUnknown {
		t.Errorf("zero Index: %q", st)
	}
	view.NewIndex(r).EnclosingRange(1, 1<<40)
}

// Enclosing(line) is EnclosingRange(line, line) for every line, over generated sets.
func TestEnclosingIsDegenerateRange(t *testing.T) {
	rng := rand.New(rand.NewSource(20261123))
	for i := 0; i < 200; i++ {
		ix := view.NewIndex(genResult(rng))
		for line := -1; line <= 45; line++ {
			s1, st1 := ix.Enclosing(line)
			s2, st2 := ix.EnclosingRange(line, line)
			if s1 != s2 || st1 != st2 {
				t.Fatalf("gen#%d line %d: Enclosing %q %q, EnclosingRange %q %q", i, line, s1.Key, st1, s2.Key, st2)
			}
		}
	}
}

// EnclosingRange equals the brute-force reference over every range of generated sets (fixed seed), also
// shuffled, and over the pinned golden Results.
func TestPropertyEnclosingRangeMatchesReference(t *testing.T) {
	checkRange := func(name string, r lang.Result) {
		t.Helper()
		ix := view.NewIndex(r)
		top := 3
		for _, s := range r.Symbols {
			top = max(top, s.Line+2, s.EndLine+2)
		}
		for start := -1; start <= top; start++ {
			for end := start - 1; end <= min(top, start+30); end++ {
				ws, wst := refEnclosingRange(r, start, end)
				gs, gst := ix.EnclosingRange(start, end)
				if gst != wst || (start >= 1 && end >= start && gs != ws) {
					t.Fatalf("%s: EnclosingRange(%d,%d) = %q %q, reference %q %q", name, start, end, gs.Key, gst, ws.Key, wst)
				}
			}
		}
	}
	rng := rand.New(rand.NewSource(20261123))
	for i := 0; i < 300; i++ {
		r := genResult(rng)
		checkRange(fmt.Sprintf("gen#%d", i), r)
		rng.Shuffle(len(r.Symbols), func(a, b int) { r.Symbols[a], r.Symbols[b] = r.Symbols[b], r.Symbols[a] })
		checkRange(fmt.Sprintf("gen#%d shuffled", i), r)
	}
	for n, r := range pinnedResults(t) {
		checkRange(n, r)
	}
}
