package readout

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// origin is where an output line came from.
type origin int

const (
	fromInput   origin = iota // carries input lines
	fromHeader                // a group header (part of group's encoding)
	fromSummary               // a summarize line
	fromJoin                  // added by a join or an outline
)

// fate is how faithfully an output line carries its source lines.
type fate int

const (
	fateExact   fate = iota // byte- or display-equivalent
	fateColumns             // columns dropped
	fateSummary             // folded into a summary
)

// line is one output line and its provenance: the input lines it carries.
type line struct {
	text   string
	src    []int
	fate   fate
	origin origin
}

// Record is one typed record an extract stage emits.
type Record struct {
	Kind   string
	Line   int // 1-based input line
	Fields []Field
}

// Field is one named capture of a record.
type Field struct {
	Name, Value string
}

// runCtx is what stages read and write besides the lines.
type runCtx struct {
	joins   map[string][]string
	records []Record
	missed  []string
}

func splitLines(s string) ([]string, bool) {
	if s == "" {
		return nil, false
	}
	nl := strings.HasSuffix(s, "\n")
	if nl {
		s = s[:len(s)-1]
	}
	return strings.Split(s, "\n"), nl
}

func joinLines(ls []string, nl bool) string {
	if len(ls) == 0 {
		return ""
	}
	s := strings.Join(ls, "\n")
	if nl {
		s += "\n"
	}
	return s
}

func texts(ls []line) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.text
	}
	return out
}

func inputLines(in []string) []line {
	out := make([]line, len(in))
	for i, t := range in {
		out[i] = line{text: t, src: []int{i}}
	}
	return out
}

func worse(a, b fate) fate {
	if a > b {
		return a
	}
	return b
}

// apply runs one stage over the lines.
func (s *Stage) apply(ls []line, ctx *runCtx) []line {
	switch s.Kind {
	case StageANSI:
		out := make([]line, len(ls))
		for i, l := range ls {
			l.text = renderTerminal(l.text)
			out[i] = l
		}
		return out
	case StageCollapse:
		return collapse(ls)
	case StageGroup:
		return s.group(ls)
	case StageExtract:
		for _, l := range ls {
			if l.origin != fromInput {
				continue
			}
			m := s.re.FindStringSubmatch(l.text)
			if m == nil {
				continue
			}
			ctx.records = append(ctx.records, Record{Kind: s.Record, Line: l.src[0] + 1, Fields: s.fields(m)})
		}
		return ls
	case StageFields:
		out := make([]line, 0, len(ls))
		for _, l := range ls {
			m := s.re.FindStringSubmatch(l.text)
			if m == nil || l.origin != fromInput {
				out = append(out, l)
				continue
			}
			vals := map[string]string{}
			for _, f := range s.fields(m) {
				vals[f.Name] = f.Value
			}
			text := ""
			if s.tmpl != nil {
				text = renderTemplate(s.tmpl, vals)
			} else {
				parts := make([]string, len(s.Keep))
				for i, k := range s.Keep {
					parts[i] = vals[k]
				}
				text = strings.Join(parts, " ")
			}
			if text != l.text {
				l.text = text
				l.fate = worse(l.fate, fateColumns)
			}
			out = append(out, l)
		}
		return out
	case StageKeep, StageDrop:
		out := make([]line, 0, len(ls))
		for _, l := range ls {
			if l.origin == fromJoin || s.re.MatchString(l.text) == (s.Kind == StageKeep) {
				out = append(out, l)
			}
		}
		return out
	case StageCap:
		return s.cap(ls)
	case StageSummarize:
		return s.summarize(ls)
	case StageOutline:
		if len(ls) <= s.Head {
			return ls
		}
		out := append([]line(nil), ls[:s.Head]...)
		extra, ok := ctx.joins["outline"]
		if !ok {
			ctx.missed = append(ctx.missed, "outline")
			extra = []string{"outline: not computed"}
		}
		for _, t := range extra {
			out = append(out, line{text: t, origin: fromJoin})
		}
		return out
	case StageJoin:
		extra, ok := ctx.joins[s.Join]
		if !ok {
			ctx.missed = append(ctx.missed, s.Join)
			return ls
		}
		out := append([]line(nil), ls...)
		for _, t := range extra {
			out = append(out, line{text: t, origin: fromJoin})
		}
		return out
	}
	return ls
}

func (s *Stage) fields(m []string) []Field {
	idx, _ := captureIndex(s.re, s.Captures)
	out := make([]Field, 0, len(s.Captures))
	for _, n := range s.Captures {
		v := ""
		if g := idx[n]; g < len(m) {
			v = m[g]
		}
		out = append(out, Field{Name: n, Value: v})
	}
	return out
}

var collapsedSuffix = regexp.MustCompile(` ×(\d+)$`)

// collapse folds runs of identical consecutive lines to `line ×N`. A single
// line that already ends in ` ×N` gains ` ×1`, so expand inverts it exactly.
func collapse(ls []line) []line {
	out := make([]line, 0, len(ls))
	for i := 0; i < len(ls); {
		j := i + 1
		for j < len(ls) && ls[j].text == ls[i].text && ls[j].origin == ls[i].origin {
			j++
		}
		l := ls[i]
		switch {
		case j-i > 1:
			l.src = nil
			for k := i; k < j; k++ {
				l.src = append(l.src, ls[k].src...)
				l.fate = worse(l.fate, ls[k].fate)
			}
			l.text = fmt.Sprintf("%s ×%d", ls[i].text, j-i)
		case collapsedSuffix.MatchString(l.text):
			l.text += " ×1"
		}
		out = append(out, l)
		i = j
	}
	return out
}

// expand is collapse's inverse.
func expand(ls []string) []string {
	var out []string
	for _, t := range ls {
		m := collapsedSuffix.FindStringSubmatchIndex(t)
		if m == nil {
			out = append(out, t)
			continue
		}
		n, err := strconv.Atoi(t[m[2]:m[3]])
		if err != nil || n > 1<<20 {
			out = append(out, t)
			continue
		}
		for k := 0; k < n; k++ {
			out = append(out, t[:m[0]])
		}
	}
	return out
}

const groupIndent = "  "

// prefix is the key's match at the start of t, or "".
func (s *Stage) prefix(t string) string {
	m := s.key.FindStringIndex(t)
	if m == nil || m[0] != 0 || m[1] == 0 {
		return ""
	}
	return t[:m[1]]
}

// group factors a key repeated on consecutive lines into one header line,
// the lines indented under it. It keeps the result only when ungroup gives
// the input back byte for byte, so group never loses a line's identity.
func (s *Stage) group(ls []line) []line {
	out := make([]line, 0, len(ls))
	for i := 0; i < len(ls); {
		p := ""
		if ls[i].origin == fromInput {
			p = s.prefix(ls[i].text)
		}
		j := i + 1
		for p != "" && j < len(ls) && ls[j].origin == fromInput && s.prefix(ls[j].text) == p {
			j++
		}
		if p == "" || j-i < 2 {
			out = append(out, ls[i])
			i++
			continue
		}
		out = append(out, line{text: p, origin: fromHeader})
		for k := i; k < j; k++ {
			l := ls[k]
			l.text = groupIndent + l.text[len(p):]
			out = append(out, l)
		}
		i = j
	}
	if strings.Join(s.ungroup(texts(out)), "\n") != strings.Join(texts(ls), "\n") {
		return ls
	}
	return out
}

// ungroup is group's inverse when group grouped (an output with no header
// is its own inverse): a line the key matches whole, followed by indented
// lines, is a header whose key is restored onto each of them.
func (s *Stage) ungroup(ls []string) []string {
	var out []string
	for i := 0; i < len(ls); i++ {
		t := ls[i]
		if s.prefix(t) == t && t != "" && i+1 < len(ls) && strings.HasPrefix(ls[i+1], groupIndent) {
			for i+1 < len(ls) && strings.HasPrefix(ls[i+1], groupIndent) {
				out = append(out, t+ls[i+1][len(groupIndent):])
				i++
			}
			continue
		}
		out = append(out, t)
	}
	return out
}

func (s *Stage) cap(ls []line) []line {
	if s.PerGroup > 0 {
		out := make([]line, 0, len(ls))
		run, last := 0, ""
		for _, l := range ls {
			p := ""
			if l.origin == fromInput {
				p = s.prefix(l.text)
			}
			if p == "" {
				run, last = 0, ""
				out = append(out, l)
				continue
			}
			if p != last {
				run, last = 0, p
			}
			run++
			if run <= s.PerGroup {
				out = append(out, l)
			}
		}
		return out
	}
	if len(ls) <= s.Head+s.Tail {
		return ls
	}
	out := append([]line(nil), ls[:s.Head]...)
	return append(out, ls[len(ls)-s.Tail:]...)
}

func (s *Stage) summarize(ls []line) []line {
	out := make([]line, 0, len(ls))
	at := -1
	var sum line
	count := 0
	var first []string
	for _, l := range ls {
		if l.origin != fromInput || !s.re.MatchString(l.text) {
			out = append(out, l)
			continue
		}
		if at < 0 {
			at = len(out)
			first = s.re.FindStringSubmatch(l.text)
			out = append(out, line{})
			sum = line{origin: fromSummary, fate: fateSummary}
		}
		sum.src = append(sum.src, l.src...)
		count++
	}
	if at < 0 {
		return ls
	}
	vals := map[string]string{"count": strconv.Itoa(count)}
	for _, f := range s.fields(first) {
		vals[f.Name] = f.Value
	}
	sum.text = renderTemplate(s.tmpl, vals)
	out[at] = sum
	return out
}

// roundTrip checks a lossless stage against its inverse on the lines it was
// given and the lines it gave: the inverse must restore the input (design
// §6.3: lossiness is tested, not declared).
func (s *Stage) roundTrip(in, out []line) error {
	want := strings.Join(texts(in), "\n")
	var got string
	switch s.Kind {
	case StageANSI:
		r := make([]string, len(in))
		for i, l := range in {
			r[i] = renderTerminal(l.text)
		}
		want = strings.Join(r, "\n")
		r = make([]string, len(out))
		for i, l := range out {
			r[i] = renderTerminal(l.text)
		}
		got = strings.Join(r, "\n")
	case StageCollapse:
		got = strings.Join(expand(texts(out)), "\n")
	case StageGroup:
		got = strings.Join(texts(out), "\n")
		for _, l := range out {
			if l.origin == fromHeader {
				got = strings.Join(s.ungroup(texts(out)), "\n")
				break
			}
		}
	case StageExtract:
		got = strings.Join(texts(out), "\n")
	case StageJoin:
		var kept []string
		for i, l := range out {
			if i >= len(in) && l.origin == fromJoin {
				continue
			}
			kept = append(kept, l.text)
		}
		got = strings.Join(kept, "\n")
	default:
		return fmt.Errorf("%s stage is lossy: it has no inverse", s.Kind)
	}
	if got != want {
		return fmt.Errorf("%s stage does not round-trip through its inverse", s.Kind)
	}
	return nil
}
