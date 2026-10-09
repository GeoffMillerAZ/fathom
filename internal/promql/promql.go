// Package promql is a pure-Go PromQL selector scanner (LB-F316, docs/design/
// language-breadth.md §4): it reads the vector selectors an expression
// writes — the metric name and the label matchers — and never parses the
// expression into a tree, never type-checks it and never evaluates it.
//
// It is what a document engine needs from a Prometheus rule, a Grafana
// panel target or a PrometheusRule spec: which metrics an expression reads
// and which label values it pins. Functions, aggregations and their
// by/without lists, on/ignoring/group_left/group_right lists, keywords,
// numbers, durations, offset and @ modifiers, subquery ranges, strings
// outside braces and comments are not selectors. Its answer is checked
// against the Prometheus parser's own ExtractSelectors over the calibration
// corpus by a test in a nested, test-only module
// (calibration/oracles/prometheus), so this package keeps the standard
// library as its only dependency (ADR-020, ADR-025).
package promql

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// The matcher operators.
const (
	OpEqual    = "="
	OpNotEqual = "!="
	OpRegex    = "=~"
	OpNotRegex = "!~"
)

// NameLabel is the label a metric name is stored under.
const NameLabel = "__name__"

// NamespaceLabel is the identifying label that scopes the others: a
// selector's namespace is where its job, service or app lives.
const NamespaceLabel = "namespace"

// Matcher is one label matcher as written inside a selector's braces, its
// value unquoted.
type Matcher struct {
	Name  string
	Op    string // OpEqual, OpNotEqual, OpRegex or OpNotRegex
	Value string
}

// Selector is one vector selector: Metric is the name written before the
// braces, a quoted name alone inside them, or else the value of an equality
// `__name__` matcher ("" when none fixes it); Matchers are the braces'
// matchers in written order. Start and End are the byte offsets of the
// selector as written (the name and the braces; a range, offset or @ is
// not part of it).
type Selector struct {
	Metric     string
	Matchers   []Matcher
	Start, End int
}

// Equal is the value of the selector's first equality matcher on label,
// and whether there is one.
func (s Selector) Equal(label string) (string, bool) {
	for _, m := range s.Matchers {
		if m.Name == label && m.Op == OpEqual {
			return m.Value, true
		}
	}
	return "", false
}

//go:embed identity_labels.json
var identityData string

var identityLabels = func() []string {
	var d struct {
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal([]byte(identityData), &d); err != nil || len(d.Labels) == 0 {
		panic(fmt.Sprintf("promql: identity_labels.json: %v", err))
	}
	return d.Labels
}()

// IdentityLabels is the universal default identifying label set (K15: a
// convention is data, identity_labels.json): job, service, app,
// destination_service, destination_workload, service_name and namespace.
// The slice is the caller's.
func IdentityLabels() []string { return append([]string(nil), identityLabels...) }

// keywords are PromQL's reserved words (lower-cased; the lexer folds case):
// binary operators, aggregation operators, modifiers and the number words.
// An identifier among them is never a metric name.
var keywords = map[string]bool{
	"and": true, "or": true, "unless": true, "atan2": true,
	"sum": true, "avg": true, "count": true, "min": true, "max": true, "group": true,
	"stddev": true, "stdvar": true, "topk": true, "bottomk": true, "count_values": true,
	"quantile": true, "limitk": true, "limit_ratio": true,
	"offset": true, "by": true, "without": true, "on": true, "ignoring": true,
	"group_left": true, "group_right": true, "bool": true,
	"inf": true, "nan": true,
}

// labelListKeywords take a parenthesized list of label names, never an
// expression.
var labelListKeywords = map[string]bool{
	"by": true, "without": true, "on": true, "ignoring": true, "group_left": true, "group_right": true,
}

type tokKind int

const (
	tokEOF tokKind = iota
	tokIdent
	tokString
	tokNumber
	tokPunct
)

type token struct {
	kind       tokKind
	text       string // the identifier, the punctuation, or a string's unquoted value
	start, end int
}

// scanner walks an expression token by token.
type scanner struct {
	src string
	pos int
}

func (s *scanner) errorf(at int, format string, args ...any) error {
	return fmt.Errorf("promql: offset %d: %s", at, fmt.Sprintf(format, args...))
}

// skipSpace skips whitespace and `#` line comments.
func (s *scanner) skipSpace() {
	for s.pos < len(s.src) {
		switch c := s.src[s.pos]; {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			s.pos++
		case c == '#':
			for s.pos < len(s.src) && s.src[s.pos] != '\n' {
				s.pos++
			}
		default:
			return
		}
	}
}

func isAlpha(c byte) bool { return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// next reads one token outside braces and brackets. Identifiers may carry
// colons (recording-rule names).
func (s *scanner) next() (token, error) {
	s.skipSpace()
	start := s.pos
	if s.pos >= len(s.src) {
		return token{kind: tokEOF, start: start, end: start}, nil
	}
	c := s.src[s.pos]
	switch {
	case isAlpha(c) || c == ':':
		for s.pos < len(s.src) && (isAlpha(s.src[s.pos]) || isDigit(s.src[s.pos]) || s.src[s.pos] == ':') {
			s.pos++
		}
		return token{tokIdent, s.src[start:s.pos], start, s.pos}, nil
	case isDigit(c) || (c == '.' && s.pos+1 < len(s.src) && isDigit(s.src[s.pos+1])):
		s.scanNumber()
		return token{tokNumber, s.src[start:s.pos], start, s.pos}, nil
	case c == '"' || c == '\'' || c == '`':
		v, err := s.scanString()
		if err != nil {
			return token{}, err
		}
		return token{tokString, v, start, s.pos}, nil
	}
	// punctuation: the two-byte operators first
	if s.pos+1 < len(s.src) {
		switch two := s.src[s.pos : s.pos+2]; two {
		case "==", "!=", ">=", "<=", "=~", "!~":
			s.pos += 2
			return token{tokPunct, two, start, s.pos}, nil
		}
	}
	s.pos++
	return token{tokPunct, s.src[start:s.pos], start, s.pos}, nil
}

// scanNumber reads a number or a duration: digits, letters, dots and
// underscores (1e3, 0x1f, 1h30m, 1_000), and an exponent's sign.
func (s *scanner) scanNumber() {
	start := s.pos
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case isAlpha(c) || isDigit(c) || c == '.':
			s.pos++
		case (c == '+' || c == '-') && s.pos > start && (s.src[s.pos-1] == 'e' || s.src[s.pos-1] == 'E') &&
			!strings.HasPrefix(strings.ToLower(s.src[start:s.pos]), "0x") && s.pos+1 < len(s.src) && isDigit(s.src[s.pos+1]):
			s.pos++
		default:
			return
		}
	}
}

// scanString reads a quoted string at s.pos and returns its value: a
// backquoted string is raw; a double- or single-quoted one takes Go's
// escapes, its own quote escaped, and never a newline.
func (s *scanner) scanString() (string, error) {
	start := s.pos
	q := s.src[s.pos]
	s.pos++
	if q == '`' {
		end := strings.IndexByte(s.src[s.pos:], '`')
		if end < 0 {
			return "", s.errorf(start, "unterminated raw string")
		}
		v := s.src[s.pos : s.pos+end]
		s.pos += end + 1
		return v, nil
	}
	var b strings.Builder
	rest := s.src[s.pos:]
	for {
		if rest == "" || rest[0] == '\n' {
			return "", s.errorf(start, "unterminated quoted string")
		}
		if rest[0] == q {
			s.pos = len(s.src) - len(rest) + 1
			return b.String(), nil
		}
		r, _, tail, err := strconv.UnquoteChar(rest, q)
		if err != nil {
			return "", s.errorf(len(s.src)-len(rest), "bad escape in quoted string")
		}
		b.WriteRune(r)
		rest = tail
	}
}

// Selectors returns every vector selector expr writes, in written order. An
// error is a lexical defect (an unterminated string, brace or bracket, a
// malformed matcher or label list): the expression is then unread, never
// guessed.
func Selectors(expr string) ([]Selector, error) {
	s := &scanner{src: expr}
	var out []Selector
	for {
		t, err := s.next()
		if err != nil {
			return nil, err
		}
		switch t.kind {
		case tokEOF:
			return out, nil
		case tokIdent:
			lower := strings.ToLower(t.text)
			if keywords[lower] {
				if labelListKeywords[lower] {
					if err := s.labelList(); err != nil {
						return nil, err
					}
				}
				continue
			}
			if s.peekByte() == '(' {
				continue // a function call: its arguments are read as the loop goes on
			}
			sel := Selector{Metric: t.text, Start: t.start, End: t.end}
			if s.peekByte() == '{' {
				s.skipSpace()
				if err := s.braces(&sel); err != nil {
					return nil, err
				}
			}
			out = append(out, sel)
		case tokPunct:
			switch t.text {
			case "{":
				s.pos = t.start
				sel := Selector{Start: t.start}
				if err := s.braces(&sel); err != nil {
					return nil, err
				}
				if sel.Metric == "" && len(sel.Matchers) == 0 {
					return nil, s.errorf(t.start, "empty selector")
				}
				out = append(out, sel)
			case "[":
				end := strings.IndexByte(s.src[s.pos:], ']')
				if end < 0 {
					return nil, s.errorf(t.start, "unterminated range")
				}
				s.pos += end + 1
			case "}", "]":
				return nil, s.errorf(t.start, "unbalanced %q", t.text)
			}
		}
	}
}

// peekByte is the next byte after whitespace and comments (0 at the end),
// without consuming it.
func (s *scanner) peekByte() byte {
	save := s.pos
	s.skipSpace()
	var c byte
	if s.pos < len(s.src) {
		c = s.src[s.pos]
	}
	s.pos = save
	return c
}

// labelList skips a by/without/on/ignoring/group_left/group_right label
// list: `(` names `)` with optional commas, the names identifiers or quoted
// strings. A group_left or group_right with no list is not one.
func (s *scanner) labelList() error {
	if s.peekByte() != '(' {
		return nil
	}
	open, _ := s.next()
	want := true // a label is expected (or the close)
	for {
		t, err := s.next()
		if err != nil {
			return err
		}
		switch {
		case t.kind == tokPunct && t.text == ")":
			return nil
		case (t.kind == tokIdent || t.kind == tokString) && want:
			want = false
		case t.kind == tokPunct && t.text == "," && !want:
			want = true
		case t.kind == tokEOF:
			return s.errorf(open.start, "unterminated label list")
		default:
			return s.errorf(t.start, "unexpected %q in a label list", s.src[t.start:t.end])
		}
	}
}

// braces reads a selector's `{…}` at s.pos into sel: matchers `name op
// "value"` (the name an identifier or a quoted string) and at most one
// quoted metric name alone, comma-separated, a trailing comma allowed.
func (s *scanner) braces(sel *Selector) error {
	open := s.pos
	s.pos++ // {
	quotedName := false
	for {
		s.skipSpace()
		if s.pos >= len(s.src) {
			return s.errorf(open, "unterminated selector braces")
		}
		if s.src[s.pos] == '}' {
			s.pos++
			sel.End = s.pos
			break
		}
		var name string
		switch c := s.src[s.pos]; {
		case isAlpha(c):
			st := s.pos
			for s.pos < len(s.src) && (isAlpha(s.src[s.pos]) || isDigit(s.src[s.pos])) {
				s.pos++
			}
			name = s.src[st:s.pos]
		case c == '"' || c == '\'' || c == '`':
			v, err := s.scanString()
			if err != nil {
				return err
			}
			name = v
			s.skipSpace()
			if s.pos < len(s.src) && (s.src[s.pos] == ',' || s.src[s.pos] == '}') {
				// a quoted metric name alone (UTF-8 names)
				if quotedName || sel.Metric != "" {
					return s.errorf(open, "metric name set twice")
				}
				sel.Metric, quotedName = name, true
				if s.src[s.pos] == ',' {
					s.pos++
				}
				continue
			}
		default:
			return s.errorf(s.pos, "unexpected %q in selector braces", string(c))
		}
		s.skipSpace()
		op := ""
		for _, o := range []string{OpNotEqual, OpRegex, OpNotRegex, OpEqual} {
			if strings.HasPrefix(s.src[s.pos:], o) {
				op = o
				break
			}
		}
		if op == "" {
			return s.errorf(s.pos, "label %q has no matcher operator", name)
		}
		s.pos += len(op)
		s.skipSpace()
		if s.pos >= len(s.src) || (s.src[s.pos] != '"' && s.src[s.pos] != '\'' && s.src[s.pos] != '`') {
			return s.errorf(s.pos, "label %q matcher has no quoted value", name)
		}
		v, err := s.scanString()
		if err != nil {
			return err
		}
		sel.Matchers = append(sel.Matchers, Matcher{Name: name, Op: op, Value: v})
		s.skipSpace()
		if s.pos < len(s.src) && s.src[s.pos] == ',' {
			s.pos++
			continue
		}
		if s.pos < len(s.src) && s.src[s.pos] != '}' {
			return s.errorf(s.pos, "expected , or } after a matcher")
		}
	}
	if sel.Metric == "" {
		if v, ok := sel.Equal(NameLabel); ok {
			sel.Metric = v
		}
	}
	return nil
}
