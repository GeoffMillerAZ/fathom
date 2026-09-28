package tsengine

import (
	"fmt"
	"regexp"
	"strings"
)

// Query predicates (LB-F2, docs/design/language-breadth.md §3.2).
//
// tree-sitter's C core parses a predicate — `(#eq? @name "x")` inside a
// pattern — into steps but never evaluates it: the calling application does.
// This file is that application side. It reads each top-level pattern's
// predicates from the query SOURCE at compile time (a top-level pattern in
// the text is one pattern index, in order — the same numbering ts_query_new
// assigns), and evaluates the text predicates over each match's captured
// text, dropping matches that fail. Reading the source rather than calling
// ts_query_predicates_for_pattern keeps the committed grammar blobs, which do
// not export it, usable unchanged.
//
// Supported: #eq? #not-eq? #match? #not-match? #any-of? #not-any-of?.
// Accepted and ignored (documentation association, which fathom does not
// extract): #set! (kept per pattern, see Query.PatternSetting), #strip!,
// #select-adjacent!, #offset!. Anything else fails compilation, naming the
// predicate — a query must be rewritten structurally, not the engine
// extended per dialect.

// predArg is one predicate argument: a capture reference or a literal.
type predArg struct {
	capture string // "" for a literal
	literal string
}

// textPredicate is one evaluable predicate of one pattern.
type textPredicate struct {
	op      string // "eq", "match", "any-of"
	negate  bool
	capture string
	other   predArg         // eq: the literal or capture compared against
	re      *regexp.Regexp  // match
	set     map[string]bool // any-of
}

// patternMeta is what the host keeps per pattern index.
type patternMeta struct {
	preds    []textPredicate
	settings map[string]string // #set! key -> value
}

// qtok is one token of the query language.
type qtok struct {
	kind byte // '(' ')' '[' ']' 's' (string) '@' (capture) '#' (predicate name) 'w' (word/other)
	text string
	pos  int
}

func tokenizeQuery(src string) ([]qtok, error) {
	var toks []qtok
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == ';':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '(' || c == ')' || c == '[' || c == ']':
			toks = append(toks, qtok{kind: c, text: string(c), pos: i})
			i++
		case c == '"':
			start := i
			i++
			var b strings.Builder
			for i < len(src) && src[i] != '"' {
				if src[i] == '\\' && i+1 < len(src) {
					i++
					switch src[i] {
					case 'n':
						b.WriteByte('\n')
					case 't':
						b.WriteByte('\t')
					case 'r':
						b.WriteByte('\r')
					case '0':
						b.WriteByte(0)
					default:
						b.WriteByte(src[i])
					}
					i++
					continue
				}
				b.WriteByte(src[i])
				i++
			}
			if i >= len(src) {
				return nil, fmt.Errorf("unterminated string at byte %d", start)
			}
			i++
			toks = append(toks, qtok{kind: 's', text: b.String(), pos: start})
		default:
			start := i
			for i < len(src) && !strings.ContainsRune(" \t\n\r()[]\";", rune(src[i])) {
				i++
			}
			word := src[start:i]
			kind := byte('w')
			switch word[0] {
			case '@':
				kind = '@'
				word = word[1:]
			case '#':
				kind = '#'
			}
			toks = append(toks, qtok{kind: kind, text: word, pos: start})
		}
	}
	return toks, nil
}

// parseQueryPredicates returns one patternMeta per top-level pattern of src,
// in order. A top-level pattern is a parenthesized or bracketed group, a
// string (anonymous node) or a wildcard; captures and quantifiers that
// follow it at depth 0 belong to it.
func parseQueryPredicates(src string) ([]patternMeta, error) {
	toks, err := tokenizeQuery(src)
	if err != nil {
		return nil, err
	}
	var metas []patternMeta
	depth := 0
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if depth == 0 {
			switch t.kind {
			case '(', '[':
				metas = append(metas, patternMeta{})
			case 's':
				metas = append(metas, patternMeta{})
				continue
			case 'w':
				if t.text == "_" {
					metas = append(metas, patternMeta{})
				}
				continue
			default:
				continue
			}
		}
		switch t.kind {
		case '(':
			if i+1 < len(toks) && toks[i+1].kind == '#' {
				end, err := parsePredicate(toks, i, &metas[len(metas)-1], len(metas)-1)
				if err != nil {
					return nil, err
				}
				i = end
				continue
			}
			depth++
		case '[':
			depth++
		case ')', ']':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced %q at byte %d", t.text, t.pos)
			}
		}
	}
	return metas, nil
}

// parsePredicate reads `( #name args... )` starting at toks[open] into meta
// and returns the index of its closing paren.
func parsePredicate(toks []qtok, open int, meta *patternMeta, pattern int) (int, error) {
	name := toks[open+1].text
	var args []predArg
	i := open + 2
	for ; i < len(toks) && toks[i].kind != ')'; i++ {
		switch toks[i].kind {
		case '@':
			args = append(args, predArg{capture: toks[i].text})
		case 's', 'w':
			args = append(args, predArg{literal: toks[i].text})
		default:
			return 0, fmt.Errorf("pattern %d: predicate %s: unexpected %q", pattern, name, toks[i].text)
		}
	}
	if i >= len(toks) {
		return 0, fmt.Errorf("pattern %d: predicate %s: unterminated", pattern, name)
	}
	bad := func(why string) (int, error) {
		return 0, fmt.Errorf("pattern %d: predicate %s: %s", pattern, name, why)
	}
	firstCapture := len(args) > 0 && args[0].capture != ""
	switch name {
	case "#eq?", "#not-eq?":
		if len(args) != 2 || !firstCapture {
			return bad("wants a capture and a capture or string")
		}
		meta.preds = append(meta.preds, textPredicate{op: "eq", negate: name == "#not-eq?", capture: args[0].capture, other: args[1]})
	case "#match?", "#not-match?":
		if len(args) != 2 || !firstCapture || args[1].capture != "" {
			return bad("wants a capture and a regex string")
		}
		re, err := regexp.Compile(args[1].literal)
		if err != nil {
			return bad(fmt.Sprintf("regex: %v", err))
		}
		meta.preds = append(meta.preds, textPredicate{op: "match", negate: name == "#not-match?", capture: args[0].capture, re: re})
	case "#any-of?", "#not-any-of?":
		if len(args) < 2 || !firstCapture {
			return bad("wants a capture and at least one string")
		}
		set := map[string]bool{}
		for _, a := range args[1:] {
			if a.capture != "" {
				return bad("wants strings after the capture")
			}
			set[a.literal] = true
		}
		meta.preds = append(meta.preds, textPredicate{op: "any-of", negate: name == "#not-any-of?", capture: args[0].capture, set: set})
	case "#set!":
		lits := args
		if len(lits) > 0 && lits[0].capture != "" {
			lits = lits[1:]
		}
		if len(lits) < 1 || len(lits) > 2 {
			return bad("wants a key and an optional value")
		}
		if meta.settings == nil {
			meta.settings = map[string]string{}
		}
		v := ""
		if len(lits) == 2 {
			v = lits[1].literal
		}
		meta.settings[lits[0].literal] = v
	case "#strip!", "#select-adjacent!", "#offset!":
		// documentation association: accepted, ignored
	default:
		return bad("unsupported (supported: #eq? #not-eq? #match? #not-match? #any-of? #not-any-of?; ignored: #set! #strip! #select-adjacent! #offset!)")
	}
	return i, nil
}

// evalTextPredicates reports whether match m satisfies every predicate. A
// predicate over a capture the match lacks (an optional capture) holds; a
// capture quantified to several nodes must satisfy it on every node.
func evalTextPredicates(preds []textPredicate, m Match, src []byte) bool {
	for i := range preds {
		p := &preds[i]
		var nodes []Node
		for _, c := range m.Captures {
			if c.Name == p.capture {
				nodes = append(nodes, c.Node)
			}
		}
		switch p.op {
		case "eq":
			var others []string
			if p.other.capture != "" {
				for _, c := range m.Captures {
					if c.Name == p.other.capture {
						others = append(others, c.Node.Text(src))
					}
				}
			}
			for k, n := range nodes {
				want := p.other.literal
				if p.other.capture != "" {
					if k >= len(others) {
						break
					}
					want = others[k]
				}
				if (n.Text(src) == want) == p.negate {
					return false
				}
			}
		case "match":
			for _, n := range nodes {
				if p.re.MatchString(n.Text(src)) == p.negate {
					return false
				}
			}
		case "any-of":
			for _, n := range nodes {
				if p.set[n.Text(src)] == p.negate {
					return false
				}
			}
		}
	}
	return true
}
