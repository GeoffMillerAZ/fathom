package view

import (
	"slices"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// Status is the answer class of Enclosing and EnclosingRange.
type Status string

// The closed set of Status values.
const (
	// StatusFound: the returned symbol is the innermost one holding the line.
	StatusFound Status = "found"
	// StatusNone: no symbol holds the line, and the Result can say so.
	StatusNone Status = "none"
	// StatusUnknown: the Result cannot decide. The symbol returned, possibly the
	// zero Symbol, is a hint and not an answer.
	StatusUnknown Status = "unknown"
)

// Index answers span lookups over one Result. It is immutable after NewIndex
// and safe for concurrent use. The zero Index holds no symbols and reads as
// incomplete: every answer is unknown.
type Index struct {
	syms       []lang.Symbol // every symbol, in listing order
	unknown    []lang.Symbol // symbols without a known span, in listing order
	incomplete bool          // the read was Incomplete or Skipped
	built      bool
}

// NewIndex builds the lookup once. It keeps copies of what it needs, so a
// later change to r by the caller changes no answer. Keys are expected to be
// unique within r: two symbols with the same Key and span tie, and which one an
// answer returns then follows the input order.
func NewIndex(r lang.Result) *Index {
	ix := &Index{built: true, incomplete: r.Incomplete || r.Skipped != ""}
	if r.Skipped != "" {
		return ix
	}
	ix.syms = slices.Clone(r.Symbols)
	slices.SortStableFunc(ix.syms, func(a, b lang.Symbol) int {
		switch {
		case before(a, b):
			return -1
		case before(b, a):
			return 1
		}
		return 0
	})
	for _, s := range ix.syms {
		if !known(s) {
			ix.unknown = append(ix.unknown, s)
		}
	}
	return ix
}

// Enclosing returns the innermost symbol whose known span holds line. It is
// EnclosingRange(line, line).
func (ix *Index) Enclosing(line int) (lang.Symbol, Status) {
	return ix.EnclosingRange(line, line)
}

// EnclosingRange returns the innermost symbol whose known span holds every
// line of start..end: the latest Line, then the earliest EndLine, then the
// largest Key (the last symbol in listing order).
//
// The status is found only when the read is complete, a known span holds the
// whole range, and no symbol with an unknown span could be the innermost: none
// has a Line below 1, or a Line at or after the candidate's Line and at or
// before start. A symbol with an unknown span that starts before the candidate's
// Line can only be outer and does not block found. It is none only when the
// read is complete, no known span holds the range and no symbol with an unknown
// span starts at or before start, a Line below 1 counting as before start.
// Otherwise it is
// unknown, and the candidate (possibly the zero Symbol) is returned as a hint.
// A Skipped Result, start below 1 and end below start are unknown with the
// zero Symbol.
func (ix *Index) EnclosingRange(start, end int) (lang.Symbol, Status) {
	if ix == nil || !ix.built || start < 1 || end < start {
		return lang.Symbol{}, StatusUnknown
	}
	var cand lang.Symbol
	have := false
	for _, s := range ix.syms {
		if s.Line > start {
			break // listing order: nothing later starts at or before start
		}
		if covers(s, start, end) && (!have || moreInner(s, cand)) {
			cand, have = s, true
		}
	}
	hidden := false
	for _, u := range ix.unknown {
		if u.Line > start {
			break // starting after start, it cannot hold the whole range
		}
		if !have || u.Line < 1 || u.Line >= cand.Line {
			hidden = true
			break
		}
	}
	switch {
	case ix.incomplete || hidden:
		return cand, StatusUnknown
	case !have:
		return lang.Symbol{}, StatusNone
	}
	return cand, StatusFound
}

// Touched returns every symbol whose known span intersects start..end, plus
// every symbol with an unknown span whose Line lies in the range, sorted by
// Line, then EndLine descending, then Key. complete is false when the read is
// Incomplete or Skipped, or a symbol with an unknown span starts before start
// (it may have reached into the range). start below 1 and end below start give
// an empty set and false.
func (ix *Index) Touched(start, end int) (syms []lang.Symbol, complete bool) {
	if ix == nil || !ix.built || start < 1 || end < start {
		return nil, false
	}
	complete = !ix.incomplete
	for _, s := range ix.syms {
		if s.Line > end {
			break
		}
		if known(s) {
			if s.EndLine >= start {
				syms = append(syms, s)
			}
			continue
		}
		if s.Line >= start && s.Line <= end {
			syms = append(syms, s)
		} else if s.Line < start {
			complete = false
		}
	}
	return syms, complete
}

// Slice returns the bytes of lines s.Line to s.EndLine of src, with their line
// endings, and false when the span is unknown or does not lie inside src. It
// reads nothing but src, and the returned slice shares src's memory.
func Slice(src []byte, s lang.Symbol) ([]byte, bool) {
	if !known(s) || len(src) == 0 {
		return nil, false
	}
	line, from := 1, 0
	if s.Line > 1 {
		from = -1
	}
	to := -1
	for i, c := range src {
		if c != '\n' {
			continue
		}
		if line == s.Line-1 {
			from = i + 1
		}
		if line == s.EndLine {
			to = i + 1
			break
		}
		line++
	}
	if from < 0 {
		return nil, false
	}
	if to < 0 { // the last line has no newline
		if line != s.EndLine || src[len(src)-1] == '\n' {
			return nil, false
		}
		to = len(src)
	}
	if from >= len(src) {
		return nil, false
	}
	return src[from:to], true
}
