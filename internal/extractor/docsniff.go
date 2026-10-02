package extractor

// docsniff.go — LB-F209 (wave lg, LB-M5): a necessary-literal prefilter for detect.content_sniff.
//
// Ingest asks every content-claimed dialect (CloudFormation, OpenAPI, the three cloud-policy
// dialects, ...) whether a .json, .yaml or .yml file is its own, and asks again in each of
// four passes (extract, doc xref, policy ref, route ref). The sniffs are alternations of
// unanchored multi-line patterns, which Go's regexp runs as an NFA over the whole file:
// 0.84 s of the polyglot set's 28 s of CPU, nearly all of it in files that contain none of
// the words the pattern is about (a 1.9 MB data JSON against five sniffs, four times).
//
// sniffLiterals reads the pattern's syntax tree and returns a set of literals one of which
// every match must contain (nil when it cannot promise one: a case-folded literal, an
// alternative of character classes, an optional part). A src that holds none of them cannot
// match, so DetectsContent answers false without the NFA; a src that holds one still runs the
// pattern, so the answer is the pattern's own, never a prefilter's.

import (
	"bytes"
	"regexp/syntax"
)

// sniffLiterals returns the literals of which every match of pattern contains at least one,
// or nil when the pattern has no such promise.
func sniffLiterals(pattern string) []string {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil
	}
	lits, ok := requiredLiterals(re)
	if !ok {
		return nil
	}
	return lits
}

// requiredLiterals returns (set, true) when every string re matches contains at least one
// member of set.
func requiredLiterals(re *syntax.Regexp) ([]string, bool) {
	switch re.Op {
	case syntax.OpLiteral:
		if re.Flags&syntax.FoldCase != 0 || len(re.Rune) == 0 {
			return nil, false
		}
		return []string{string(re.Rune)}, true
	case syntax.OpCapture, syntax.OpPlus:
		return requiredLiterals(re.Sub[0])
	case syntax.OpRepeat:
		if re.Min < 1 {
			return nil, false
		}
		return requiredLiterals(re.Sub[0])
	case syntax.OpConcat:
		// Every child must match, so any child's promise is the concatenation's; keep the
		// sharpest (the longest shortest literal, then the fewest alternatives).
		var best []string
		for _, sub := range re.Sub {
			lits, ok := requiredLiterals(sub)
			if ok && sharper(lits, best) {
				best = lits
			}
		}
		return best, best != nil
	case syntax.OpAlternate:
		// One alternative matches, so the promise is the union of every alternative's, and
		// an alternative with none breaks it.
		var union []string
		for _, sub := range re.Sub {
			lits, ok := requiredLiterals(sub)
			if !ok {
				return nil, false
			}
			union = append(union, lits...)
		}
		return union, union != nil
	}
	return nil, false
}

// sharper reports whether a is a stronger prefilter than b (b nil is the weakest).
func sharper(a, b []string) bool {
	if b == nil {
		return true
	}
	am, bm := minLen(a), minLen(b)
	if am != bm {
		return am > bm
	}
	return len(a) < len(b)
}

func minLen(set []string) int {
	m := -1
	for _, s := range set {
		if m < 0 || len(s) < m {
			m = len(s)
		}
	}
	return m
}

// containsAny reports whether src holds at least one literal.
func containsAny(src []byte, lits []string) bool {
	for _, l := range lits {
		if bytes.Contains(src, []byte(l)) {
			return true
		}
	}
	return false
}
