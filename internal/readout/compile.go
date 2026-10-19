package readout

// Compile and vet errors grouped by root cause (RO-24, design §17): the
// error lines of a `go build` or `go vet` run, extracted as compile_error
// records and grouped by the identifier or the file they share, the group
// whose root sits in a file this session edited named first. Rich mode's one
// injected line; the output it reads is never changed, so every error line
// stays in the raw output. Pure: the caller answers which files were edited
// and which definitions changed.

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// RecordCompileError is the record kind CompileErrors emits.
const RecordCompileError = "compile_error"

// compileLineRe is one compiler or vet diagnostic: file.go:line[:col]: msg,
// with vet's own `vet: ` prefix on a type error. A continuation (have/want)
// is indented and never matches.
var compileLineRe = regexp.MustCompile(`^(?:vet: )?([^\s:][^:]*\.go):(\d+)(?::(\d+))?: (.+)$`)

// The messages whose root is an identifier, each with how the line names it.
var compileIdentRes = []struct {
	re   *regexp.Regexp
	kind string
}{
	{regexp.MustCompile(`^undefined: (\S+)$`), "undefined"},
	{regexp.MustCompile(`(?:not enough|too many) arguments in call to ([^\s(]+)`), "call"},
	{regexp.MustCompile(`in argument to ([^\s(]+)`), "call"},
	{regexp.MustCompile(`but ([^\s(]+)(?:\(.*\))? returns \d+ values?`), "call"},
	{regexp.MustCompile(`^\S+\.\S+ undefined \(type .* has no field or method (\w+)`), "member"},
	{regexp.MustCompile(`^(\S+) redeclared in this block`), "redeclared"},
}

// compileIdentMax bounds an identifier the line names.
const compileIdentMax = 80

// CompileErrors extracts the diagnostics of a go build or go vet output as
// compile_error records, in output order. norm maps a printed path to the
// key the line names and the edited answer reads (nil keeps the path, less
// a leading "./"). The compiler's own "too many errors" notice is no error.
func CompileErrors(text string, norm func(string) string) []Record {
	var out []Record
	for i, l := range strings.Split(text, "\n") {
		m := compileLineRe.FindStringSubmatch(strings.TrimRight(l, "\r"))
		if m == nil || m[4] == "too many errors" {
			continue
		}
		file := strings.TrimPrefix(m[1], "./")
		if norm != nil {
			file = norm(m[1])
		}
		ident, kind := compileIdent(m[4])
		out = append(out, Record{Kind: RecordCompileError, Line: i + 1, Fields: []Field{
			{"file", file}, {"line", m[2]}, {"col", m[3]}, {"msg", m[4]}, {"ident", ident}, {"kind", kind},
		}})
	}
	return out
}

func compileIdent(msg string) (ident, kind string) {
	for _, c := range compileIdentRes {
		if m := c.re.FindStringSubmatch(msg); m != nil && len(m[1]) <= compileIdentMax {
			return m[1], c.kind
		}
	}
	return "", ""
}

// CompileDef is a definition the caller placed for an identifier root: a
// symbol the working tree changed, by its display name ("money.Round",
// "Ledger.Add"), file and line.
type CompileDef struct {
	Name string
	File string
	Line int
}

// compileGroup is one root: the identifier or the file its errors share.
type compileGroup struct {
	ident  string // "" for a file group
	kind   string
	file   string // the location the line names
	line   string
	msg    string // the first error's message
	n      int
	edited bool
}

func field(r Record, name string) string {
	for _, f := range r.Fields {
		if f.Name == name {
			return f.Value
		}
	}
	return ""
}

// lastPart is a dotted name's last component ("money.Round" → "Round").
func lastPart(s string) string {
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// compileDefFor is the one changed definition an identifier names: an exact
// display name, else the one whose last component matches. Two candidates
// place nothing (never a guessed root).
func compileDefFor(ident string, defs []CompileDef) (CompileDef, bool) {
	var hit []CompileDef
	for _, d := range defs {
		if d.Name == ident {
			return d, true
		}
		if lastPart(d.Name) == lastPart(ident) {
			hit = append(hit, d)
		}
	}
	if len(hit) == 1 {
		return hit[0], true
	}
	return CompileDef{}, false
}

// groupCompileErrors groups compile_error records by the identifier they
// share (an identifier root), else by their file. A group leads when its
// root's definition or one of its errors sits in a file edited reports;
// then the larger group, then the first in the output. Deterministic for the
// same records, definitions and answers.
func groupCompileErrors(recs []Record, defs []CompileDef, edited func(file string) bool) []compileGroup {
	seen := map[string]bool{}
	isEdited := func(f string) bool {
		if f == "" || edited == nil {
			return false
		}
		v, ok := seen[f]
		if !ok {
			v = edited(f)
			seen[f] = v
		}
		return v
	}
	var groups []*compileGroup
	byKey := map[string]*compileGroup{}
	for _, r := range recs {
		if r.Kind != RecordCompileError {
			continue
		}
		ident, file := field(r, "ident"), field(r, "file")
		key := "file\x00" + file
		if ident != "" {
			key = "ident\x00" + ident
		}
		g := byKey[key]
		if g == nil {
			g = &compileGroup{ident: ident, kind: field(r, "kind"), file: file, line: field(r, "line"), msg: field(r, "msg")}
			if ident != "" {
				if d, ok := compileDefFor(ident, defs); ok {
					g.file, g.line = d.File, strconv.Itoa(d.Line)
					g.edited = isEdited(d.File)
				}
			}
			byKey[key] = g
			groups = append(groups, g)
		}
		g.n++
		if isEdited(file) {
			g.edited = true
		}
	}
	out := make([]compileGroup, len(groups))
	for i, g := range groups {
		out[i] = *g
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.edited != b.edited {
			return a.edited
		}
		return a.n > b.n
	})
	return out
}

// root is how the line names the group's root.
func (g compileGroup) root() string {
	switch {
	case g.ident == "":
		return g.file
	case g.kind == "call":
		return "the signature of " + g.ident
	case g.kind == "undefined":
		return "undefined: " + g.ident
	case g.kind == "member":
		return g.ident + " (no field or method)"
	case g.kind == "redeclared":
		return g.ident + " redeclared"
	}
	return g.ident
}

func (g compileGroup) render() string {
	var b strings.Builder
	if g.ident == "" {
		b.WriteString(g.file + ":" + g.line)
		if g.n == 1 {
			b.WriteString(": " + g.msg)
		}
	} else {
		b.WriteString(g.root() + ", " + g.file + ":" + g.line)
	}
	if g.edited {
		b.WriteString(", edited this session")
	}
	if g.n > 1 || g.ident != "" {
		b.WriteString(" (" + plural(g.n, "error") + ")")
	}
	return b.String()
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// CompileErrorLine is rich mode's one line for a go build or go vet run
// (tool, "go build"), "" when recs hold no compile_error: the error and root
// counts, then each root in groupCompileErrors' order while the line stays
// inside budget bytes, the rest counted ("and 2 more roots"), never cut
// mid-root.
func CompileErrorLine(tool string, recs []Record, defs []CompileDef, edited func(file string) bool, budget int) string {
	groups := groupCompileErrors(recs, defs, edited)
	if len(groups) == 0 {
		return ""
	}
	total := 0
	for _, g := range groups {
		total += g.n
	}
	more := func(left int) string {
		if left == 0 {
			return ""
		}
		return "; and " + plural(left, "more root")
	}
	line := tool + ": " + plural(total, "error") + ", " + plural(len(groups), "root") + ": "
	for i, g := range groups {
		part := g.render()
		if i == 0 {
			// The leading root always shows; one over the budget is cut,
			// and the cut says so (never a silent truncation).
			line += cutTo(part, budget-len(line)-len(more(len(groups)-1)))
			continue
		}
		if len(line)+len("; ")+len(part)+len(more(len(groups)-i-1)) > budget {
			return line + more(len(groups)-i)
		}
		line += "; " + part
	}
	return line
}

// compileCut discloses a cut root.
const compileCut = " …[cut at the line's byte cap]"

// cutTo is s inside n bytes, cut on a rune boundary with compileCut.
func cutTo(s string, n int) string {
	if len(s) <= n {
		return s
	}
	end := n - len(compileCut)
	if end < 0 {
		end = 0
	}
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end] + compileCut
}
