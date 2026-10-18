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
	fromNote                  // an engine note: a cap's or a hunk body's count and recipe
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
	// collapsed: text ends in the ` ×N` suffix collapse added (N may be 1,
	// the escape for an input line that already ended in ` ×N`).
	collapsed bool
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
	// stamp is the head_stamp join's line, which the receipt carries.
	stamp string
	// persisted is the persisted output's display lines, when the caller
	// read them; counts come from there (design §6.7).
	persisted []string
	// cut: the shown output is not the whole output and persisted is nil,
	// so a count is labelled with label.
	cut   bool
	label string
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
			for _, i := range l.src {
				ctx.records = append(ctx.records, Record{Kind: s.Record, Line: i + 1, Fields: s.fields(m)})
			}
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
				l.collapsed = false
			}
			out = append(out, l)
		}
		return out
	case StageKeep, StageDrop:
		out := make([]line, 0, len(ls))
		for _, l := range ls {
			if l.origin == fromJoin || l.origin == fromNote || s.re.MatchString(l.text) == (s.Kind == StageKeep) {
				out = append(out, l)
			}
		}
		return out
	case StageCap:
		return s.cap(ls, ctx)
	case StageHunks:
		return s.hunks(ls)
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
		if !ok || (s.Join == joinHeadStamp && len(extra) == 0) {
			ctx.missed = append(ctx.missed, s.Join)
			return ls
		}
		if s.Join == joinHeadStamp {
			// HEAD stamps the receipt's recovery, never the body.
			ctx.stamp = extra[0]
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

// rep is a line's text without the suffix collapse added, and the count
// of display lines it stands for.
func (l line) rep() (string, int) {
	if !l.collapsed {
		return l.text, 1
	}
	m := collapsedSuffix.FindStringSubmatchIndex(l.text)
	if m == nil {
		return l.text, 1
	}
	n, err := strconv.Atoi(l.text[m[2]:m[3]])
	if err != nil || n < 1 || n > 1<<20 {
		return l.text, 1
	}
	return l.text[:m[0]], n
}

// collapse folds runs of identical consecutive lines to `line ×N`. A single
// input line that already ends in ` ×N` gains ` ×1`, so expandLines inverts
// it exactly; a line collapse already folded folds again by its count, so
// collapse is idempotent (the base stages run it before a rule's own).
func collapse(ls []line) []line {
	out := make([]line, 0, len(ls))
	for i := 0; i < len(ls); {
		base, n := ls[i].rep()
		j := i + 1
		for j < len(ls) && ls[j].origin == ls[i].origin {
			b, k := ls[j].rep()
			if b != base {
				break
			}
			n += k
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
			l.text = fmt.Sprintf("%s ×%d", base, n)
			l.collapsed = true
		case !l.collapsed && collapsedSuffix.MatchString(l.text):
			l.text += " ×1"
			l.collapsed = true
		}
		out = append(out, l)
		i = j
	}
	return out
}

// expandLines is collapse's inverse: each collapsed line back to its run.
func expandLines(ls []line) []string {
	var out []string
	for _, l := range ls {
		t, n := l.rep()
		for k := 0; k < n; k++ {
			out = append(out, t)
		}
	}
	return out
}

// baseStages run first inside every matched rule (RO-13): escape sequences
// and carriage-return progress overwrites rendered as a terminal shows them
// (display-equivalent), then identical consecutive lines collapsed to
// `line ×N`. They are no rule of their own, and a rule's saving is measured
// from their output, so they are never claimed as a saving (design §17).
func baseStages(ls []line) []line {
	out := make([]line, len(ls))
	for i, l := range ls {
		if !l.collapsed {
			l.text = renderTerminal(l.text)
		}
		out[i] = l
	}
	return collapse(out)
}

// baseRoundTrip checks the base stages against their inverse: expanding
// the collapse gives the terminal text of every input line, in order.
func baseRoundTrip(in, out []line) error {
	want := make([]string, len(in))
	for i, l := range in {
		want[i] = renderTerminal(l.text)
	}
	if strings.Join(expandLines(out), "\n") != strings.Join(want, "\n") {
		return fmt.Errorf("base stages are not display-equivalent")
	}
	return nil
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

// weight is how many input lines l stands for.
func weight(l line) int {
	if l.origin != fromInput {
		return 0
	}
	if len(l.src) == 0 {
		return 1
	}
	return len(l.src)
}

// cap keeps the head and tail lines, or with per_group the first per_group
// lines of each run its key groups, a note after each capped run naming
// how many lines it hid and the run's count. With above, the cap applies
// only when the lines it reads (the key's, else every input line) number
// more than above.
func (s *Stage) cap(ls []line, ctx *runCtx) []line {
	if s.Above > 0 {
		n := 0
		for _, l := range ls {
			if s.key == nil || (l.origin == fromInput && s.prefix(l.text) != "") {
				n += weight(l)
			}
		}
		if n <= s.Above {
			return ls
		}
	}
	if s.PerGroup > 0 {
		out := make([]line, 0, len(ls))
		for i := 0; i < len(ls); {
			p := ""
			if ls[i].origin == fromInput {
				p = s.prefix(ls[i].text)
			}
			if p == "" {
				out = append(out, ls[i])
				i++
				continue
			}
			j := i + 1
			for j < len(ls) && ls[j].origin == fromInput && s.prefix(ls[j].text) == p {
				j++
			}
			if j-i <= s.PerGroup {
				out = append(out, ls[i:j]...)
				i = j
				continue
			}
			out = append(out, ls[i:i+s.PerGroup]...)
			hid, all := 0, 0
			for k := i; k < j; k++ {
				all += weight(ls[k])
				if k >= i+s.PerGroup {
					hid += weight(ls[k])
				}
			}
			out = append(out, line{text: capNote(s, p, hid, all, ctx), origin: fromNote})
			i = j
		}
		return out
	}
	if len(ls) <= s.Head+s.Tail {
		return ls
	}
	out := append([]line(nil), ls[:s.Head]...)
	return append(out, ls[len(ls)-s.Tail:]...)
}

// capNote is the line after a capped run: what it hid, and the run's count,
// taken from the persisted output when the caller read it, else labelled
// when the shown output is cut (design §6.7: a count says what it covers).
func capNote(s *Stage, key string, hid, all int, ctx *runCtx) string {
	name := strings.TrimSuffix(key, ":")
	count := fmt.Sprintf("%d in all", all)
	switch {
	case ctx.persisted != nil:
		n := 0
		for _, t := range ctx.persisted {
			if s.prefix(t) == key {
				n++
			}
		}
		count = fmt.Sprintf("%d in the persisted output", n)
	case ctx.cut:
		count = fmt.Sprintf("%d %s", all, ctx.label)
	}
	return fmt.Sprintf("… +%d more lines of %s (%s)", hid, name, count)
}

// hunks reads unified diff output (git diff and git show): each file's
// header factored to its `diff --git` line (the index, --- and +++ lines
// dropped), every hunk header kept, and a hunk body longer than above
// lines capped to its head and tail lines, a note in their place rendered
// from the template: {n} the lines dropped, {file} the new side's path,
// {from} and {to} the new side's line range they cover, {commit} the shown
// commit (git show), else HEAD.
func (s *Stage) hunks(ls []line) []line {
	out := make([]line, 0, len(ls))
	file, commit, header := "", "HEAD", false
	for i := 0; i < len(ls); {
		l := ls[i]
		t := l.text
		if l.origin != fromInput {
			out = append(out, l)
			i++
			continue
		}
		switch {
		case strings.HasPrefix(t, "commit "):
			if f := strings.Fields(t); len(f) > 1 {
				commit = f[1]
			}
			header = false
		case strings.HasPrefix(t, "diff --git "):
			header, file = true, diffPath(t)
		case header && (strings.HasPrefix(t, "index ") || strings.HasPrefix(t, "--- ") || strings.HasPrefix(t, "+++ ")):
			if p, ok := strings.CutPrefix(t, "+++ b/"); ok {
				file = p
			}
			i++
			continue
		case strings.HasPrefix(t, "@@ "):
			header = false
			out = append(out, l)
			j := i + 1
			for j < len(ls) && ls[j].origin == fromInput && isHunkBody(ls[j].text) {
				j++
			}
			out = append(out, s.capBody(ls[i+1:j], hunkNewStart(t), file, commit)...)
			i = j
			continue
		}
		out = append(out, l)
		i++
	}
	return out
}

func isHunkBody(t string) bool {
	return t != "" && strings.ContainsRune(" +-\\", rune(t[0]))
}

// diffPath is the b/ side of a `diff --git a/x b/y` line.
func diffPath(t string) string {
	rest := strings.TrimPrefix(t, "diff --git ")
	if k := strings.Index(rest, " b/"); k >= 0 {
		return rest[k+3:]
	}
	return rest
}

// hunkNewStart is the new side's first line in `@@ -a,b +c,d @@`.
func hunkNewStart(t string) int {
	k := strings.Index(t, " +")
	if k < 0 {
		return 1
	}
	v := t[k+2:]
	end := strings.IndexAny(v, ", ")
	if end >= 0 {
		v = v[:end]
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 1
	}
	return n
}

func (s *Stage) capBody(body []line, start int, file, commit string) []line {
	total := 0
	for _, l := range body {
		total += weight(l)
	}
	if total <= s.Above || len(body) <= s.Head+s.Tail {
		return body
	}
	out := append([]line(nil), body[:s.Head]...)
	next := start // the new-side line the next body line takes
	from, to, n := 0, 0, 0
	for k, l := range body[:len(body)-s.Tail] {
		w := weight(l)
		newSide := strings.HasPrefix(l.text, " ") || strings.HasPrefix(l.text, "+")
		if k >= s.Head {
			n += w
			if newSide {
				if from == 0 {
					from = next
				}
				to = next + w - 1
			}
		}
		if newSide {
			next += w
		}
	}
	if from == 0 {
		from, to = next, next
	}
	vals := map[string]string{"n": strconv.Itoa(n), "file": file, "from": strconv.Itoa(from), "to": strconv.Itoa(to), "commit": commit}
	out = append(out, line{text: renderTemplate(s.tmpl, vals), origin: fromNote})
	return append(out, body[len(body)-s.Tail:]...)
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
		want = strings.Join(expandLines(in), "\n")
		got = strings.Join(expandLines(out), "\n")
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
