// Package logtemplate is TL-4's template reader (F67; docs/design/
// telemetry-links.md "Log templates (TL-4)"): a logger call in Python,
// Java, JavaScript or TypeScript, or PHP becomes a template fact — its
// file and line, the logger API, the message's template text and the
// spans of its placeholders — so a pasted log line can reach the call
// that wrote it. The logger API list is embedded data with universal
// defaults (apis.json, K15). Facts are read from code only: no log store,
// no fetch, no index of rendered lines.
package logtemplate

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Fact is one logger call's template.
type Fact struct {
	File string
	// Line is the line the call's receiver is on (1-based).
	Line int
	// API names the logger API (apis.json), e.g. "python.logging".
	API string
	// Text is the template: literal text as the log line renders it (an
	// escape such as %% or \{} already rendered), each placeholder spelled
	// as the source writes it (%s, {}, {0}, {name}, ${id}, {expr} for a
	// concatenated operand).
	Text string
	// Holes are the placeholders' byte spans in Text, in order.
	Holes []Hole
	// Keys are a structlog event's keyword keys, rendered beside it.
	Keys []string
	// Enclosing and Package name the innermost symbol around the call
	// (Enclose); empty when none was supplied or none encloses it.
	Enclosing string
	Package   string
}

// Hole is one placeholder's span in a Fact's Text; Int marks a placeholder
// that renders an integer (%d in Python, Java and PHP; %i in Node).
type Hole struct {
	Start, End int
	Int        bool
}

// maxText bounds a template's text: a longer message is a document, not a
// line template.
const maxText = 512

// minLetters is the least literal text a template must carry: a message
// that is all placeholders ("%s", f"{x}") matches every line and is no
// template.
const minLetters = 3

// part is one rendered part of a message: literal text, or a hole spelled
// as written.
type part struct {
	text string
	hole bool
	int  bool
}

// piece is one operand of a message expression.
type piece struct {
	segs      []seg
	hole      string
	isHole    bool
	style     string // a formatter's or operator's style; "" = the API's
	styleArgs bool
}

// Read returns the template facts of path's logger calls, in line order.
// Nothing for a file the registry does not read.
func Read(path string, content []byte) []Fact {
	ls, ok := reg.langFor(path)
	if !ok {
		return nil
	}
	src := string(content)
	if !strings.Contains(src, "(") {
		return nil
	}
	mask := codeMask(ls.lang, src)
	recv := receivers(ls, src, mask)
	var out []Fact
	line, at := 1, 0
	for i := 0; i < len(src); i++ {
		k := strings.IndexByte(src[i:], '(')
		if k < 0 {
			break
		}
		open := i + k
		i = open
		if !mask[open] {
			continue
		}
		rs, re, ms, ok := callBefore(src, open)
		if !ok || !ls.methods[src[ms:open-trailingSpace(src, open)]] {
			continue
		}
		api, ok := receiverAPI(ls, recv, normalizeReceiver(src[rs:re]), src)
		if !ok {
			continue
		}
		a := &ls.APIs[api]
		level, ok := a.method(src[ms : open-trailingSpace(src, open)])
		if !ok {
			continue
		}
		args, _, ok := splitArgs(ls.lang, src, open)
		if !ok {
			continue
		}
		f, ok := readCall(ls, a, src, args, level)
		if !ok {
			continue
		}
		line += strings.Count(src[at:rs], "\n")
		at = rs
		f.File, f.Line, f.API = path, line, a.API
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out
}

// trailingSpace counts the blanks before src[open].
func trailingSpace(src string, open int) int {
	n := 0
	for open-n-1 >= 0 && isSpace(src[open-n-1]) {
		n++
	}
	return n
}

// callBefore reads `<receiver> <sep> <method> (` backwards from the '(' at
// open: the receiver's span [rs, re) and the method's start. The receiver
// is the longest chain of names joined by ., -> or :: (a call inside it,
// logging.getLogger(__name__), kept whole).
func callBefore(src string, open int) (rs, re, ms int, ok bool) {
	j := open - trailingSpace(src, open)
	me := j
	for j > 0 && isIdent(src[j-1]) && src[j-1] != '$' {
		j--
	}
	if j == me || src[j] >= '0' && src[j] <= '9' {
		return 0, 0, 0, false
	}
	ms = j
	for j > 0 && isSpace(src[j-1]) {
		j--
	}
	switch {
	case j >= 1 && src[j-1] == '.':
		j--
	case j >= 2 && (src[j-2:j] == "->" || src[j-2:j] == "::"):
		j -= 2
	default:
		return 0, 0, 0, false
	}
	for j > 0 && isSpace(src[j-1]) {
		j--
	}
	re = j
	for {
		if j > 0 && src[j-1] == ')' {
			depth := 0
			k := j - 1
			for ; k >= 0; k-- {
				if src[k] == ')' {
					depth++
				} else if src[k] == '(' {
					depth--
					if depth == 0 {
						break
					}
				}
			}
			if k < 0 {
				return 0, 0, 0, false
			}
			j = k
		}
		n := j
		for j > 0 && isIdent(src[j-1]) {
			j--
		}
		if j == n {
			return 0, 0, 0, false
		}
		switch {
		case j >= 1 && src[j-1] == '.':
			j--
			continue
		case j >= 2 && (src[j-2:j] == "->" || src[j-2:j] == "::"):
			j -= 2
			continue
		}
		break
	}
	return j, re, ms, j < re
}

// normalizeReceiver drops whitespace inside a receiver ("$this -> logger").
func normalizeReceiver(s string) string {
	return strings.Join(strings.Fields(s), "")
}

// receiverAPI is the API a receiver logs through: a name the file binds or
// the language names, or a logger got and called in one expression.
func receiverAPI(ls *langSpec, recv map[string]int, name, src string) (int, bool) {
	if j, ok := recv[name]; ok {
		return j, true
	}
	for _, re := range ls.receiverCalls {
		if !re.MatchString(name) {
			continue
		}
		best, api := "", -1
		for j, a := range ls.APIs {
			if p := a.ReceiverCallPrefix; p != "" && strings.HasPrefix(name, p) && len(p) > len(best) {
				best, api = p, j
			}
		}
		if api < 0 {
			api = defaultAPI(ls, src)
		}
		return api, true
	}
	return 0, false
}

// defaultAPI is the file's API for an unbound receiver: the first API whose
// import marker the file carries, else the language's default.
func defaultAPI(ls *langSpec, src string) int {
	def := 0
	for j, a := range ls.APIs {
		for _, imp := range a.Imports {
			if strings.Contains(src, imp) {
				return j
			}
		}
		if a.Default {
			def = j
		}
	}
	return def
}

// bindLHS reads the name a bind call's result is assigned to, at the end of
// the text before the call on its line (`private static final Logger log =`,
// `const log: winston.Logger =`, `$this->logger = new`).
var bindLHS = regexp.MustCompile(`([A-Za-z_$][\w$]*(?:(?:\.|->)[A-Za-z_$][\w$]*)?)\s*(?::[^=;]*)?=\s*(?:new\s+)?$`)

// receivers maps each receiver a logger is called through to its API: the
// language's default names, fixed receivers (console, the logging module),
// annotation-declared loggers and names the file binds.
func receivers(ls *langSpec, src string, mask []bool) map[string]int {
	recv := map[string]int{}
	def := defaultAPI(ls, src)
	for _, r := range ls.Receivers {
		recv[r] = def
	}
	for j, a := range ls.APIs {
		for _, r := range a.Receivers {
			recv[r] = j
		}
	}
	for _, an := range ls.annotations {
		if strings.Contains(src, an.marker) && an.re.MatchString(src) {
			recv["log"] = an.api
		}
	}
	for j, a := range ls.APIs {
		for _, re := range a.binds {
			for _, m := range re.FindAllStringIndex(src, -1) {
				if !mask[m[0]] || m[0] > 0 && isWord(src[m[0]]) && (isIdent(src[m[0]-1]) || src[m[0]-1] == '.') {
					continue
				}
				lineStart := strings.LastIndexByte(src[:m[0]], '\n') + 1
				if lhs := bindLHS.FindStringSubmatch(src[lineStart:m[0]]); lhs != nil {
					recv[normalizeReceiver(lhs[1])] = j
				}
			}
		}
	}
	return recv
}

// readCall reads one call's message argument into a fact.
func readCall(ls *langSpec, a *apiSpec, src string, args []argSpan, level bool) (Fact, bool) {
	idx := 0
	if level {
		idx = 1
	}
	if idx >= len(args) {
		return Fact{}, false
	}
	pieces, ok := parseMessage(ls, src[args[idx].start:args[idx].end])
	if !ok && !level && idx+1 < len(args) {
		// pino's merging object, an SLF4J marker, a winston meta first.
		idx++
		pieces, ok = parseMessage(ls, src[args[idx].start:args[idx].end])
	}
	if !ok {
		return Fact{}, false
	}
	positional := 0
	var keys []string
	for _, x := range args[idx+1:] {
		text := src[x.start:x.end]
		if ls.lang == langPython {
			if k, isKw := pythonKeyword(text); isKw {
				if a.Style == styleEvent && k != "exc_info" && k != "stack_info" && k != "stacklevel" {
					keys = append(keys, k)
				}
				continue
			}
			if strings.HasPrefix(text, "**") {
				continue
			}
		}
		positional++
	}
	style := a.Style
	if style == styleEvent && positional > 0 {
		style = stylePercent // structlog's positional formatting
	}
	text, holes := compose(pieces, style, positional > 0, ls.lang)
	if text == "" || len(text) > maxText || letters(text, holes) < minLetters {
		return Fact{}, false
	}
	return Fact{Text: text, Holes: holes, Keys: keys}, true
}

var pyKwRE = regexp.MustCompile(`^([A-Za-z_]\w*)\s*=[^=]`)

func pythonKeyword(arg string) (string, bool) {
	m := pyKwRE.FindStringSubmatch(arg)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// letters counts the letters of a template's literal text.
func letters(text string, holes []Hole) int {
	n, at := 0, 0
	count := func(s string) {
		for _, r := range s {
			if unicode.IsLetter(r) {
				n++
			}
		}
	}
	for _, h := range holes {
		count(text[at:h.Start])
		at = h.End
	}
	count(text[at:])
	return n
}

// compose renders the pieces into a template text and its holes.
func compose(pieces []piece, apiStyle string, apiArgs bool, l lang) (string, []Hole) {
	var b strings.Builder
	var holes []Hole
	add := func(p part) {
		if p.hole {
			start := b.Len()
			b.WriteString(p.text)
			holes = append(holes, Hole{Start: start, End: b.Len(), Int: p.int})
			return
		}
		b.WriteString(p.text)
	}
	for _, pc := range pieces {
		if pc.isHole {
			add(part{text: pc.hole, hole: true})
			continue
		}
		style, args := apiStyle, apiArgs
		if pc.style != "" {
			style, args = pc.style, pc.styleArgs
		}
		for _, s := range pc.segs {
			if s.hole {
				add(part{text: s.text, hole: true})
				continue
			}
			for _, p := range applyStyle(style, s.text, args, l) {
				add(p)
			}
		}
	}
	return b.String(), holes
}

// spell renders a non-literal operand as a hole: {expr}, whitespace
// collapsed, at most 40 bytes.
func spell(expr string) string {
	e := strings.Join(strings.Fields(expr), " ")
	if len(e) > 40 {
		cut := 40
		for cut > 0 && !utfStart(e[cut]) {
			cut--
		}
		e = e[:cut] + "…"
	}
	return "{" + e + "}"
}

func utfStart(c byte) bool { return c&0xC0 != 0x80 }

// parseMessage reads a message expression: string literals joined by the
// language's concatenation, each operand that is not a literal a hole, a
// formatter call (String.format, sprintf, util.format) or Python's % and
// .format rendering its literal in their own style. ok is false when no
// operand is a string literal.
func parseMessage(ls *langSpec, text string) ([]piece, bool) {
	var out []piece
	sawString := false
	for _, op := range splitConcat(ls.lang, text) {
		pc, isStr := parseOperand(ls, op)
		if isStr {
			sawString = true
		}
		out = append(out, pc...)
	}
	return out, sawString
}

// splitConcat splits text at its top-level concatenation operators.
func splitConcat(l lang, text string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(text); {
		if e, ok := commentEnd(l, text, i); ok {
			i = e
			continue
		}
		if _, isStr := stringStart(l, text, i); isStr {
			e, _, ok := scanString(l, text, i)
			if !ok {
				return []string{text}
			}
			i = e
			continue
		}
		c := text[i]
		switch {
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case depth == 0 && isConcat(l, text, i):
			out = append(out, strings.TrimSpace(text[start:i]))
			start = i + 1
		}
		i++
	}
	return append(out, strings.TrimSpace(text[start:]))
}

// isConcat reports whether text[i] is a concatenation operator.
func isConcat(l lang, text string, i int) bool {
	c := text[i]
	next := byte(0)
	if i+1 < len(text) {
		next = text[i+1]
	}
	prev := byte(0)
	if i > 0 {
		prev = text[i-1]
	}
	switch l {
	case langPHP:
		if c != '.' || next == '=' || next == '.' || prev == '.' {
			return false
		}
		return !(prev >= '0' && prev <= '9' && next >= '0' && next <= '9')
	default:
		return c == '+' && next != '+' && next != '=' && prev != '+'
	}
}

// parseOperand reads one operand: a string literal (Python's adjacent
// literals joined; a % or .format suffix), a formatter call, or a hole.
func parseOperand(ls *langSpec, op string) ([]piece, bool) {
	if op == "" {
		return nil, false
	}
	if _, isStr := stringStart(ls.lang, op, 0); isStr {
		var segs []seg
		i := 0
		for {
			e, s, ok := scanString(ls.lang, op, i)
			if !ok {
				return []piece{{isHole: true, hole: spell(op)}}, false
			}
			segs = append(segs, s...)
			i = skipSpace(op, e)
			if ls.lang != langPython {
				break
			}
			if _, more := stringStart(ls.lang, op, i); !more {
				break
			}
		}
		rest := strings.TrimSpace(op[i:])
		switch {
		case rest == "":
			return []piece{{segs: segs}}, true
		case ls.lang == langPython && strings.HasPrefix(rest, "%") && !strings.HasPrefix(rest, "%="):
			return []piece{{segs: segs, style: stylePercent, styleArgs: true}}, true
		case ls.lang == langPython && strings.HasPrefix(rest, ".format("):
			return []piece{{segs: segs, style: "pybrace", styleArgs: true}}, true
		}
		return []piece{{isHole: true, hole: spell(op)}}, false
	}
	for _, f := range ls.Formatters {
		if !strings.HasPrefix(op, f.Call) {
			continue
		}
		j := skipSpace(op, len(f.Call))
		if j >= len(op) || op[j] != '(' {
			continue
		}
		if len(f.Call) < len(op) && isIdent(op[len(f.Call)]) {
			continue
		}
		args, end, ok := splitArgs(ls.lang, op, j)
		if !ok || strings.TrimSpace(op[end:]) != "" || len(args) == 0 {
			continue
		}
		for k := 0; k < len(args) && k < 2; k++ {
			pcs, isStr := parseMessage(ls, op[args[k].start:args[k].end])
			if !isStr {
				continue
			}
			for n := range pcs {
				if !pcs[n].isHole && pcs[n].style == "" {
					pcs[n].style, pcs[n].styleArgs = f.Style, true
				}
			}
			return pcs, true
		}
	}
	return []piece{{isHole: true, hole: spell(op)}}, false
}

func skipSpace(s string, i int) int {
	for i < len(s) && isSpace(s[i]) {
		i++
	}
	return i
}
