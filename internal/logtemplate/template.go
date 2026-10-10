package logtemplate

// template.go — matching a log line against templates. A template matches
// when its literal text appears in the line in order, each placeholder a
// wildcard (an integer placeholder digits only). Of the templates a line
// matches, the one covering the longest stretch of it wins (placeholders
// counted as covered), its literal length breaking ties: deterministic,
// no model, no budget.

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

// Template is a Fact compiled for matching.
type Template struct {
	Fact
	elems   []elem
	literal int
	longest string
	event   bool
	re      *regexp.Regexp
	reDone  bool
}

type elem struct {
	lit  string
	hole bool
	int  bool
}

// Match is a template's match on one line.
type Match struct {
	// Coverage is the length of the line the template covers, its
	// placeholders' values included.
	Coverage int
	// Literal is the template's literal length: its specificity.
	Literal int
}

// Compile prepares f for matching. ok is false for a fact with no literal
// text to anchor a match.
func Compile(f Fact) (*Template, bool) {
	t := &Template{Fact: f}
	at := 0
	for _, h := range f.Holes {
		if h.Start < at || h.End > len(f.Text) || h.Start > h.End {
			return nil, false
		}
		if h.Start > at {
			t.elems = append(t.elems, elem{lit: f.Text[at:h.Start]})
		}
		t.elems = append(t.elems, elem{hole: true, int: h.Int})
		at = h.End
	}
	if at < len(f.Text) {
		t.elems = append(t.elems, elem{lit: f.Text[at:]})
	}
	for _, e := range t.elems {
		if !e.hole {
			t.literal += len(e.lit)
			if len(e.lit) > len(t.longest) {
				t.longest = e.lit
			}
		}
	}
	if t.literal == 0 {
		return nil, false
	}
	t.event = f.API == "python.structlog" && len(f.Holes) == 0
	if t.event {
		for _, k := range f.Keys {
			t.literal += len(k) + 1
		}
	}
	return t, true
}

func isWord(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// regexp builds the ordered matcher once: a leading or trailing untyped
// placeholder bounds nothing and is dropped.
func (t *Template) regexp() *regexp.Regexp {
	if t.reDone {
		return t.re
	}
	t.reDone = true
	es := t.elems
	for len(es) > 0 && es[0].hole && !es[0].int {
		es = es[1:]
	}
	for len(es) > 0 && es[len(es)-1].hole && !es[len(es)-1].int {
		es = es[:len(es)-1]
	}
	if len(es) == 0 {
		return nil
	}
	var b strings.Builder
	if !es[0].hole && isWord(es[0].lit[0]) {
		b.WriteString(`\b`)
	}
	for _, e := range es {
		switch {
		case !e.hole:
			b.WriteString(regexp.QuoteMeta(e.lit))
		case e.int:
			b.WriteString(`\s*-?\d+`)
		default:
			b.WriteString(`(?s:.*?)`)
		}
	}
	if last := es[len(es)-1]; !last.hole && isWord(last.lit[len(last.lit)-1]) {
		b.WriteString(`\b`)
	}
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil
	}
	t.re = re
	return re
}

// Match reports whether t matches text, and how much of it it covers.
func (t *Template) Match(text string) (Match, bool) {
	if !strings.Contains(text, t.longest) {
		return Match{}, false
	}
	if t.event {
		return t.matchEvent(text)
	}
	re := t.regexp()
	if re == nil {
		return Match{}, false
	}
	loc := re.FindStringIndex(text)
	if loc == nil {
		return Match{}, false
	}
	return Match{Coverage: loc[1] - loc[0], Literal: t.literal}, true
}

// matchEvent matches a structlog event: the event name as a whole token
// and every keyword key rendered beside it (key=, "key": or 'key':), in
// any order, since each renderer orders keys its own way.
func (t *Template) matchEvent(text string) (Match, bool) {
	pat := regexp.QuoteMeta(t.Text)
	if isWord(t.Text[0]) {
		pat = `\b` + pat
	}
	if isWord(t.Text[len(t.Text)-1]) {
		pat += `\b`
	}
	if !regexp.MustCompile(pat).MatchString(text) {
		return Match{}, false
	}
	for _, k := range t.Keys {
		if !regexp.MustCompile(`(?:^|[^\w])` + regexp.QuoteMeta(k) + `(?:=|"\s*:|'\s*:)`).MatchString(text) {
			return Match{}, false
		}
	}
	return Match{Coverage: t.literal, Literal: t.literal}, true
}

// candidateTexts is what a pasted line is matched as: the line, and, for a
// JSON object line (pino, a JSON renderer), each of its string values
// decoded. The line is read here and discarded.
func candidateTexts(line string) []string {
	out := []string{line}
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "{") {
		return out
	}
	var obj map[string]any
	if json.Unmarshal([]byte(trimmed), &obj) != nil {
		return out
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if s, ok := obj[k].(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Best returns the templates that match line best: the longest coverage,
// then the longest literal; every template tied on both, ordered by file,
// line and text. Empty when none matches.
func Best(line string, ts []*Template) ([]*Template, Match) {
	texts := candidateTexts(line)
	var best []*Template
	var bm Match
	for _, t := range ts {
		var m Match
		ok := false
		for _, s := range texts {
			if x, hit := t.Match(s); hit && (!ok || x.Coverage > m.Coverage) {
				m, ok = x, true
			}
		}
		if !ok {
			continue
		}
		switch {
		case len(best) == 0 || m.Coverage > bm.Coverage || m.Coverage == bm.Coverage && m.Literal > bm.Literal:
			best, bm = []*Template{t}, m
		case m.Coverage == bm.Coverage && m.Literal == bm.Literal:
			best = append(best, t)
		}
	}
	sort.SliceStable(best, func(i, j int) bool {
		a, b := best[i], best[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Text < b.Text
	})
	return best, bm
}
