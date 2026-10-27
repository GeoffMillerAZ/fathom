package main

// Python steps (GDEV-06): a Python script, module or shebang file a wrapper
// runs is read, never run. Only a closed set of spawn calls counts, and only
// with argv a literal, or a name bound once to one, can settle anything;
// every other shape is named uncertain (I38), never guessed.
import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/pysyntax"
)

var doctorPyInterpreter = regexp.MustCompile(`^python(3(\.[0-9]+)?)?$`)

// doctorPyQuietFlags change nothing about which program runs.
var doctorPyQuietFlags = []string{"-B", "-E", "-I", "-O", "-OO", "-P", "-q", "-s", "-S", "-u", "-v"}

// doctorPyInert is the closed list of modules a read file may import and
// still settle: none of them runs a project file on import.
var doctorPyInert = []string{"__future__", "argparse", "asyncio", "collections", "contextlib", "dataclasses", "datetime", "enum", "fnmatch", "functools", "glob", "hashlib", "io", "itertools", "json", "logging", "os", "os.path", "pathlib", "pty", "re", "shlex", "shutil", "string", "subprocess", "sys", "tempfile", "textwrap", "time", "tomllib", "typing"}

// doctorPySpawnModules are the modules whose calls can start a process.
var doctorPySpawnModules = []string{"asyncio", "os", "pty", "subprocess"}

type doctorPySpawn uint8

const (
	doctorPyNoSpawn    doctorPySpawn = iota
	doctorPyRead                     // argv is resolved and judged
	doctorPyRecognized               // a spawn, never read: always uncertain
)

func doctorPySpawnKind(module, attr string) doctorPySpawn {
	switch module {
	case "subprocess":
		if slices.Contains([]string{"run", "call", "check_call", "check_output", "Popen", "getoutput", "getstatusoutput"}, attr) {
			return doctorPyRead
		}
	case "os":
		if slices.Contains([]string{"system", "popen", "execv", "execve", "execvp", "execvpe", "execl", "execle", "execlp", "execlpe"}, attr) {
			return doctorPyRead
		}
		if strings.HasPrefix(attr, "spawn") || attr == "posix_spawn" || attr == "posix_spawnp" {
			return doctorPyRecognized
		}
	case "asyncio":
		if attr == "create_subprocess_exec" || attr == "create_subprocess_shell" {
			return doctorPyRecognized
		}
	case "pty":
		if attr == "spawn" {
			return doctorPyRecognized
		}
	}
	return doctorPyNoSpawn
}

// doctorPyStep is one unit classified as a Python step: the script or module
// it runs, or the reason it is uncertain before any file is read.
type doctorPyStep struct {
	script, uncertain string
}

// pythonUnit classifies a unit (P1). ok is false for anything that is not a
// Python step, which keeps today's handling.
func (r *doctorWrapperReader) pythonUnit(words []string) (doctorPyStep, bool) {
	bin := filepath.Base(words[0])
	switch {
	case doctorPyInterpreter.MatchString(bin):
		for i := 1; i < len(words); i++ {
			switch w := words[i]; {
			case w == "-m":
				return doctorPyStep{}, false // module wrappers: today's handling
			case w == "-c":
				return doctorPyStep{uncertain: "python -c runs an inline program doctor does not read"}, true
			case w == "-":
				return doctorPyStep{uncertain: "python - reads its program from stdin"}, true
			case w == "-W" || w == "-X":
				i++
			case slices.Contains(doctorPyQuietFlags, w):
			case strings.HasPrefix(w, "-"):
				return doctorPyStep{uncertain: "interpreter option " + w + " is not one doctor reads past"}, true
			default:
				return doctorPyStep{script: w}, true
			}
		}
		return doctorPyStep{}, false
	case filepath.Ext(bin) == ".py":
		return doctorPyStep{script: words[0]}, true
	}
	return doctorPyStep{}, false
}

// doctorProjectDirPath resolves the documented project-dir aliases to the
// root doctor is diagnosing, never an ambient environment value.
func doctorProjectDirPath(p string) string {
	for _, name := range []string{"CLAUDE_PROJECT_DIR", "GEMINI_PROJECT_DIR", "CURSOR_PROJECT_DIR"} {
		p = strings.ReplaceAll(p, "${"+name+"}", ".")
		if p == "$"+name || strings.HasPrefix(p, "$"+name+"/") {
			p = "." + strings.TrimPrefix(p, "$"+name)
		}
	}
	return p
}

// pythonUncertainUnit records a cause found before any file was read. It
// can hide only a step whose own words mention it: verify for the gate's
// probes, fathom for hooks and stdio entries.
func (r *doctorWrapperReader) pythonUncertainUnit(words []string, cause string, acc *doctorWiringEvidence) {
	want := "fathom"
	if r.matches != nil {
		want = "verify"
	}
	if !strings.Contains(strings.Join(words, " "), want) {
		return
	}
	acc.unknown = true
	if acc.cause == "" {
		acc.cause = cause
	}
}

// pythonTarget resolves a step's script to a repository-relative
// file. ok is false (and cause empty) for a file that is not there or is
// foreign, which hides nothing, as for any named script today.
func (r *doctorWrapperReader) pythonTarget(step doctorPyStep) (rel, cause string, ok bool) {
	p := doctorProjectDirPath(step.script)
	if doctorForeignProgram(r.root, p) {
		return "", "", false
	}
	if strings.ContainsAny(p, "$`{}") {
		return "", "script path " + step.script + " is dynamic", false
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.root, p)
	}
	rel, err := filepath.Rel(r.root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		if _, err := os.Lstat(p); err == nil {
			return "", step.script + " is outside the repository", false
		}
		return "", "", false
	}
	return filepath.ToSlash(rel), "", true
}

// pythonStep judges one Python step (P1, P2, P7). A file counts as one
// wrapper level: its contents are judged at depth+1.
func (r *doctorWrapperReader) pythonStep(step doctorPyStep, words []string, sub string, depth int, acc *doctorWiringEvidence) doctorWiringEvidence {
	if step.uncertain != "" {
		r.pythonUncertainUnit(words, step.uncertain, acc)
		return doctorWiringEvidence{}
	}
	rel, cause, ok := r.pythonTarget(step)
	if cause != "" {
		r.pythonUncertainUnit(words, cause, acc)
		return doctorWiringEvidence{}
	}
	if !ok {
		return doctorWiringEvidence{}
	}
	p := filepath.Join(r.root, rel)
	if depth >= doctorWrapperLevels {
		// Never read past the limit, never charge its bytes. Only a step
		// whose words mention it can hide the step a consumer looks for.
		line := strings.Join(words, " ")
		switch {
		case r.matches != nil && strings.Contains(line, "verify") && r.regularFile(p):
			acc.unknown = true
			r.depthCuts = append(r.depthCuts, doctorWrapperDepthCut{target: rel})
		case r.matches == nil && strings.Contains(line, "fathom"):
			acc.unknown = true
			if acc.cause == "" {
				acc.cause = rel + " is beyond the depth limit"
			}
		}
		return doctorWiringEvidence{}
	}
	b, err := r.read(p)
	if err != nil {
		if os.IsNotExist(err) {
			return doctorWiringEvidence{}
		}
		// I38: a Python file that exists and cannot be read is uncertain for
		// every consumer, named with its cause.
		why := doctorErrCause(err)
		if r.cut {
			why = "the 64 KiB read budget ran out"
		}
		acc.unknown = true
		if acc.cause == "" {
			acc.cause = rel + " could not be read: " + why
		}
		return doctorWiringEvidence{}
	}
	// A file that was read is settled: a later depth cut at it hides nothing.
	r.markWalked(rel)
	firstCut := len(r.depthCuts)
	v := r.pythonFile(rel, b, sub, depth+1)
	r.labelDepthCuts(firstCut, rel)
	if v.found.found {
		ev := v.found
		ev.indirect = true
		switch ev.file {
		case "", rel:
			ev.file = rel
		default:
			ev.file = rel + ", " + ev.file
		}
		return ev
	}
	if acc.missingScript == "" {
		acc.missingScript = v.missingScript
	}
	if acc.opaque == "" {
		acc.opaque = v.opaque
	}
	if v.cause == "" {
		// R4: a definite not-found. A FAIL that rests on it names the file.
		if acc.settled == "" {
			acc.settled = rel
		}
		return doctorWiringEvidence{}
	}
	// R3 is uncertain only where it can hide what the consumer looks for.
	mention := bytes.Contains(bytes.ToLower(b), []byte("fathom"))
	if r.matches != nil {
		mention = mention || strings.Contains(strings.Join(words, " "), "verify")
	}
	if mention {
		acc.unknown = true
		if acc.cause == "" {
			acc.cause = v.cause
		}
	}
	return doctorWiringEvidence{}
}

// doctorPyVerdict is one Python file's verdict (P6): found (R1), the first
// uncertain cause by line (R3), or neither (R4, settled).
type doctorPyVerdict struct {
	found                 doctorWiringEvidence
	cause                 string
	missingScript, opaque string
}

type doctorPyBinding struct {
	plain    bool             // NAME = value, the whole statement
	importOf string           // the module or module.attr an import binds
	value    []pysyntax.Token // a plain binding's value
	line     int
}

// doctorPyFile is the state of reading one file.
type doctorPyFile struct {
	r        *doctorWrapperReader
	rel, sub string
	depth    int
	toks     []pysyntax.Token
	binds    map[string][]doctorPyBinding
	mutated  map[string]bool
	mods     map[string]string // local name -> spawn-relevant module
	funcs    map[string]string // local name -> module.attr
	inImport map[int]bool      // token indexes inside an import statement
	causes   []doctorPyCause
	v        doctorPyVerdict
}

type doctorPyCause struct {
	line int
	why  string
}

func (f *doctorPyFile) uncertain(line int, why string) {
	f.causes = append(f.causes, doctorPyCause{line, why})
}

// pythonFile reads one Python file's text at its content depth.
func (r *doctorWrapperReader) pythonFile(rel string, b []byte, sub string, depth int) doctorPyVerdict {
	toks, issues := pysyntax.Lex(b)
	if len(issues) > 0 {
		// Tokens past an issue may not be what Python reads, so not even a
		// match counts: a misread string can look like a call.
		return doctorPyVerdict{cause: fmt.Sprintf("%s:%d: %s", rel, issues[0].Line, issues[0].Kind)}
	}
	f := &doctorPyFile{r: r, rel: rel, sub: sub, depth: depth, binds: map[string][]doctorPyBinding{}, mutated: map[string]bool{}, mods: map[string]string{}, funcs: map[string]string{}, inImport: map[int]bool{}}
	for _, t := range toks {
		if t.Kind != pysyntax.Newline && t.Kind != pysyntax.Indent && t.Kind != pysyntax.Dedent && t.Kind != pysyntax.EOF {
			f.toks = append(f.toks, t)
		}
	}
	f.collect(toks)
	f.checkAliases()
	f.spawns()
	if f.v.found.found {
		return f.v
	}
	if len(f.causes) > 0 {
		first := slices.MinFunc(f.causes, func(a, b doctorPyCause) int { return a.line - b.line })
		f.v.cause = fmt.Sprintf("%s:%d: %s", rel, first.line, first.why)
	}
	return f.v
}

// collect records every binding of every name, statement by statement, and
// the file's imports. Over-counting a binding only makes a name dynamic.
func (f *doctorPyFile) collect(all []pysyntax.Token) {
	var stmt []pysyntax.Token
	start := 0 // index in f.toks of stmt[0]
	n := 0
	flush := func() {
		if len(stmt) > 0 {
			f.statement(stmt, start)
		}
		stmt = nil
	}
	for _, t := range all {
		switch t.Kind {
		case pysyntax.Newline, pysyntax.EOF:
			flush()
			continue
		case pysyntax.Indent, pysyntax.Dedent:
			continue
		}
		if t.Kind == pysyntax.Op && t.Text == ";" && doctorPyDepth(stmt) == 0 {
			flush()
			n++
			continue
		}
		if len(stmt) == 0 {
			start = n
		}
		stmt = append(stmt, t)
		n++
	}
	flush()
	// Names a whole-file pass sees bind wherever they appear.
	for i, t := range f.toks {
		if t.Kind != pysyntax.Name || f.after(i, ".") {
			continue
		}
		switch {
		case f.next(i, ":="):
			f.bind(t.Text, doctorPyBinding{line: t.Line})
		case f.next(i, ".") || f.next(i, "["):
			f.mutated[t.Text] = true
		}
		if i > 0 && f.toks[i-1].Kind == pysyntax.Name {
			switch f.toks[i-1].Text {
			case "as", "def", "class", "global", "nonlocal", "del":
				if !f.inImport[i] {
					f.bind(t.Text, doctorPyBinding{line: t.Line})
				}
			}
		}
	}
	for i, t := range f.toks {
		if t.Kind == pysyntax.Name && (t.Text == "for" || t.Text == "lambda") {
			f.targets(i)
		}
	}
}

func doctorPyDepth(toks []pysyntax.Token) int {
	d := 0
	for _, t := range toks {
		if t.Kind == pysyntax.Op {
			switch t.Text {
			case "(", "[", "{":
				d++
			case ")", "]", "}":
				d--
			}
		}
	}
	return d
}

func (f *doctorPyFile) bind(name string, b doctorPyBinding) {
	f.binds[name] = append(f.binds[name], b)
}

func (f *doctorPyFile) next(i int, op string) bool {
	return i+1 < len(f.toks) && f.toks[i+1].Kind == pysyntax.Op && f.toks[i+1].Text == op
}

func (f *doctorPyFile) after(i int, op string) bool {
	return i > 0 && f.toks[i-1].Kind == pysyntax.Op && f.toks[i-1].Text == op
}

// targets binds the names a for or lambda introduces: those up to the
// matching `in` or `:` at the same bracket depth, and a def's parameters.
func (f *doctorPyFile) targets(i int) {
	stop := "in"
	if f.toks[i].Text == "lambda" {
		stop = ":"
	}
	d := 0
	for j := i + 1; j < len(f.toks); j++ {
		t := f.toks[j]
		if t.Kind == pysyntax.Op {
			switch t.Text {
			case "(", "[", "{":
				d++
			case ")", "]", "}":
				d--
			}
			if d < 0 || d == 0 && t.Text == stop {
				return
			}
		}
		if d == 0 && t.Kind == pysyntax.Name && t.Text == stop {
			return
		}
		if t.Kind == pysyntax.Name && !f.after(j, ".") {
			f.bind(t.Text, doctorPyBinding{line: t.Line})
		}
	}
}

var doctorPyCompound = []string{"if", "elif", "else", "while", "for", "with", "try", "except", "finally", "def", "class", "async"}

// statement records one logical statement's bindings and imports. start is
// the index of s[0] in f.toks.
func (f *doctorPyFile) statement(s []pysyntax.Token, start int) {
	head := s[0]
	if head.Kind == pysyntax.Name && slices.Contains(doctorPyCompound, head.Text) {
		// A compound header binds through its keywords; a body on the same
		// line is a statement of its own.
		d := 0
		for j, t := range s {
			if t.Kind != pysyntax.Op {
				continue
			}
			switch t.Text {
			case "(", "[", "{":
				d++
			case ")", "]", "}":
				d--
			case ":":
				if d == 0 {
					if head.Text == "def" {
						f.params(s[:j], start)
					}
					if j+1 < len(s) {
						f.statement(s[j+1:], start+j+1)
					}
					return
				}
			}
		}
		return
	}
	if head.Kind == pysyntax.Name && (head.Text == "import" || head.Text == "from") {
		for k := range s {
			f.inImport[start+k] = true
		}
		f.imports(s)
		return
	}
	if head.Kind == pysyntax.Name && head.Text == "case" {
		for _, t := range s {
			if t.Kind == pysyntax.Name {
				f.bind(t.Text, doctorPyBinding{line: t.Line})
			}
		}
		return
	}
	// The last depth-0 assignment operator splits targets from the value.
	d, last, plainOps := 0, -1, 0
	for j, t := range s {
		if t.Kind != pysyntax.Op {
			continue
		}
		switch t.Text {
		case "(", "[", "{":
			d++
		case ")", "]", "}":
			d--
		case "=":
			if d == 0 {
				last = j
				plainOps++
			}
		case "+=", "-=", "*=", "/=", "//=", "%=", "**=", ">>=", "<<=", "&=", "|=", "^=", "@=":
			if d == 0 {
				last = j
				plainOps += 2 // never plain
			}
		case ":":
			if d == 0 && last < 0 {
				last = j // an annotation: never plain
				plainOps += 2
			}
		}
	}
	if last < 0 {
		return
	}
	if last == 1 && plainOps == 1 && head.Kind == pysyntax.Name {
		f.bind(head.Text, doctorPyBinding{plain: true, value: s[2:], line: head.Line})
		return
	}
	for j, t := range s[:last] {
		if t.Kind == pysyntax.Name && (j == 0 || s[j-1].Kind != pysyntax.Op || s[j-1].Text != ".") {
			f.bind(t.Text, doctorPyBinding{line: t.Line})
		}
	}
}

// params binds a def's parameters (with their defaults' names: over-
// counting only makes a name dynamic).
func (f *doctorPyFile) params(header []pysyntax.Token, start int) {
	open := slices.IndexFunc(header, func(t pysyntax.Token) bool { return t.Kind == pysyntax.Op && t.Text == "(" })
	if open < 0 {
		return
	}
	for _, t := range header[open:] {
		if t.Kind == pysyntax.Name {
			f.bind(t.Text, doctorPyBinding{line: t.Line})
		}
	}
}

// imports reads an import statement: the names it binds, the spawn-module
// aliases, and any module outside the inert list (R3).
func (f *doctorPyFile) imports(s []pysyntax.Token) {
	line := s[0].Line
	dotted := func(k int) (string, int) {
		var b strings.Builder
		for k < len(s) {
			t := s[k]
			if t.Kind == pysyntax.Name && !(b.Len() > 0 && !strings.HasSuffix(b.String(), ".")) {
				b.WriteString(t.Text)
			} else if t.Kind == pysyntax.Op && (t.Text == "." || t.Text == "...") {
				b.WriteString(t.Text)
			} else {
				break
			}
			k++
		}
		return b.String(), k
	}
	inert := func(mod string) bool {
		if !slices.Contains(doctorPyInert, mod) {
			f.uncertain(line, "import "+mod+" (a module doctor does not follow)")
			return false
		}
		return true
	}
	if s[0].Text == "import" {
		for k := 1; k < len(s); {
			mod, nk := dotted(k)
			if mod == "" {
				f.uncertain(line, "an import doctor cannot read")
				return
			}
			k = nk
			local := strings.Split(mod, ".")[0]
			target := local
			if k+1 < len(s) && s[k].Kind == pysyntax.Name && s[k].Text == "as" {
				local, target = s[k+1].Text, mod
				k += 2
			}
			inert(mod)
			f.bind(local, doctorPyBinding{importOf: target, line: line})
			if slices.Contains(doctorPySpawnModules, target) || target == "shutil" {
				f.mods[local] = target
			}
			if k < len(s) && s[k].Kind == pysyntax.Op && s[k].Text == "," {
				k++
			}
		}
		return
	}
	mod, k := dotted(1)
	if mod == "" || strings.HasPrefix(mod, ".") {
		f.uncertain(line, "from "+mod+" import (a project-local import doctor does not follow)")
		return
	}
	if !inert(mod) {
		return
	}
	if k >= len(s) || s[k].Text != "import" {
		f.uncertain(line, "an import doctor cannot read")
		return
	}
	for k++; k < len(s); k++ {
		t := s[k]
		if t.Kind == pysyntax.Op {
			if t.Text == "*" && slices.Contains(doctorPySpawnModules, mod) {
				f.uncertain(line, "from "+mod+" import * (spawn names doctor cannot see)")
			}
			continue
		}
		if t.Kind != pysyntax.Name {
			continue
		}
		local := t.Text
		if k+2 < len(s) && s[k+1].Kind == pysyntax.Name && s[k+1].Text == "as" {
			local = s[k+2].Text
			k += 2
		}
		f.bind(local, doctorPyBinding{importOf: mod + "." + t.Text, line: line})
		f.funcs[local] = mod + "." + t.Text
	}
}

// checkAliases makes a spawn-module or spawn-function alias that is bound
// any other way uncertain, and stops reading its calls as the module's.
func (f *doctorPyFile) checkAliases() {
	for _, name := range doctorSortedKeys(f.mods, f.funcs) {
		target := ""
		for _, b := range f.binds[name] {
			why := ""
			switch {
			case b.importOf == "":
				why = name + " is rebound, so its calls may not be the module's"
			case target != "" && b.importOf != target:
				why = name + " is imported as two different things"
			default:
				target = b.importOf
				continue
			}
			f.uncertain(b.line, why)
			delete(f.mods, name)
			delete(f.funcs, name)
			break
		}
	}
}

func doctorSortedKeys(ms ...map[string]string) []string {
	var keys []string
	for _, m := range ms {
		for k := range m {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}

// spawns finds every spawn site and judges each (P4).
func (f *doctorPyFile) spawns() {
	for i := 0; i < len(f.toks); i++ {
		t := f.toks[i]
		if t.Kind == pysyntax.Op && t.Text == "." && i+2 < len(f.toks) && f.toks[i+1].Kind == pysyntax.Name &&
			(f.toks[i+1].Text == "subprocess_exec" || f.toks[i+1].Text == "subprocess_shell") && f.next(i+1, "(") {
			f.uncertain(t.Line, f.toks[i+1].Text+": spawn form not read")
			continue
		}
		if t.Kind != pysyntax.Name || f.after(i, ".") || f.inImport[i] {
			continue
		}
		switch {
		case slices.Contains([]string{"exec", "eval", "compile", "__import__"}, t.Text) && f.next(i, "("):
			f.uncertain(t.Line, t.Text+"( runs code doctor cannot read")
		case t.Text == "getattr" && f.next(i, "(") && i+2 < len(f.toks) && f.mods[f.toks[i+2].Text] != "":
			f.uncertain(t.Line, "getattr on "+f.mods[f.toks[i+2].Text]+" reaches a spawn by name")
		case f.mods[t.Text] != "" && f.next(i, ".") && i+2 < len(f.toks) && f.toks[i+2].Kind == pysyntax.Name:
			f.site(f.mods[t.Text], f.toks[i+2].Text, i+2)
		case f.funcs[t.Text] != "":
			mod, attr, _ := strings.Cut(f.funcs[t.Text], ".")
			f.site(mod, attr, i)
		}
	}
}

// site judges one use of module.attr whose name token is at i.
func (f *doctorPyFile) site(module, attr string, i int) {
	kind := doctorPySpawnKind(module, attr)
	if kind == doctorPyNoSpawn {
		return
	}
	line := f.toks[i].Line
	name := module + "." + attr
	if !f.next(i, "(") {
		f.uncertain(line, name+" is used without being called (passed or assigned)")
		return
	}
	if kind == doctorPyRecognized {
		f.uncertain(line, name+": spawn form not read")
		return
	}
	args, ok := f.args(i + 1)
	if !ok {
		f.uncertain(line, name+": its arguments do not parse")
		return
	}
	var pos [][]pysyntax.Token
	kw := map[string][]pysyntax.Token{}
	for _, a := range args {
		switch {
		case a.star:
			f.uncertain(line, name+": *args or **kwargs can carry any argument")
			return
		case a.kw != "":
			kw[a.kw] = a.toks
		default:
			pos = append(pos, a.toks)
		}
	}
	if _, ok := kw["cwd"]; ok {
		f.uncertain(line, name+": cwd= runs it in another directory")
		return
	}
	if _, ok := kw["executable"]; ok {
		f.uncertain(line, name+": executable= replaces the program")
		return
	}
	switch attr {
	case "run", "call", "check_call", "check_output", "Popen":
		argv := kw["args"]
		if len(pos) > 0 {
			argv = pos[0]
		}
		if argv == nil {
			f.uncertain(line, name+" has no argv doctor can read")
			return
		}
		shell := false
		if v, ok := kw["shell"]; ok {
			switch {
			case len(v) == 1 && v[0].Kind == pysyntax.Name && v[0].Text == "True":
				shell = true
			case len(v) == 1 && v[0].Kind == pysyntax.Name && v[0].Text == "False":
			default:
				f.uncertain(line, name+": shell= is not a literal")
				return
			}
		}
		v := f.eval(argv, 0)
		switch {
		case shell && v.kind == doctorPyStr:
			f.shellString(line, name, v.s)
		case shell:
			f.uncertain(line, name+": shell=True with argv that is not one literal string")
		case v.kind == doctorPyStr:
			f.judge(line, name, []doctorPyElem{{lit: v.s}}, false)
		case v.kind == doctorPyList:
			f.judge(line, name, v.elems, v.tail)
		default:
			f.uncertain(line, name+": argv is not a literal list ("+v.why+")")
		}
	case "getoutput", "getstatusoutput", "system", "popen":
		if len(pos) == 0 {
			f.uncertain(line, name+" has no command doctor can read")
			return
		}
		if v := f.eval(pos[0], 0); v.kind == doctorPyStr {
			f.shellString(line, name, v.s)
		} else {
			f.uncertain(line, name+": the shell command is not a literal ("+v.why+")")
		}
	case "execv", "execve", "execvp", "execvpe":
		if len(pos) < 2 {
			f.uncertain(line, name+" has no argv doctor can read")
			return
		}
		prog, rest := f.elem(pos[0]), f.eval(pos[1], 0)
		if rest.kind != doctorPyList {
			f.uncertain(line, name+": argv is not a literal list ("+rest.why+")")
			return
		}
		elems := []doctorPyElem{prog}
		if len(rest.elems) > 1 {
			elems = append(elems, rest.elems[1:]...)
		}
		f.judge(line, name, elems, rest.tail || len(rest.elems) == 0 && rest.tail)
	default: // execl, execle, execlp, execlpe
		if strings.HasSuffix(attr, "e") && len(pos) > 0 {
			pos = pos[:len(pos)-1] // the environment
		}
		if len(pos) == 0 {
			f.uncertain(line, name+" has no program doctor can read")
			return
		}
		elems := []doctorPyElem{f.elem(pos[0])}
		if len(pos) > 2 {
			for _, p := range pos[2:] {
				elems = append(elems, f.elem(p))
			}
		}
		f.judge(line, name, elems, false)
	}
}

type doctorPyArg struct {
	kw   string
	star bool
	toks []pysyntax.Token
}

// args splits the call whose "(" is at i into its depth-0 arguments.
func (f *doctorPyFile) args(i int) ([]doctorPyArg, bool) {
	var out []doctorPyArg
	var cur []pysyntax.Token
	d := 0
	push := func() {
		if len(cur) == 0 {
			return
		}
		a := doctorPyArg{toks: cur}
		switch {
		case cur[0].Kind == pysyntax.Op && (cur[0].Text == "*" || cur[0].Text == "**"):
			a.star = true
		case len(cur) > 2 && cur[0].Kind == pysyntax.Name && cur[1].Kind == pysyntax.Op && cur[1].Text == "=":
			a.kw, a.toks = cur[0].Text, cur[2:]
		}
		out = append(out, a)
		cur = nil
	}
	for j := i + 1; j < len(f.toks); j++ {
		t := f.toks[j]
		if t.Kind == pysyntax.Op {
			switch t.Text {
			case "(", "[", "{":
				d++
			case ")", "]", "}":
				if d == 0 {
					push()
					return out, true
				}
				d--
			case ",":
				if d == 0 {
					push()
					continue
				}
			}
		}
		cur = append(cur, t)
	}
	return nil, false
}

type doctorPyKind uint8

const (
	doctorPyDyn doctorPyKind = iota
	doctorPyStr
	doctorPyList
	doctorPyFathom // a value that resolves to the fathom program
)

type doctorPyValue struct {
	kind  doctorPyKind
	s     string
	elems []doctorPyElem
	tail  bool   // a list with a dynamic tail
	why   string // why a value is dynamic
}

// doctorPyElem is one argv element: a literal, the fathom program, or
// dynamic (neither).
type doctorPyElem struct {
	lit    string
	fathom bool
	dyn    bool
}

// eval resolves an expression (P5). hop counts name hops: at most one.
func (f *doctorPyFile) eval(e []pysyntax.Token, hop int) doctorPyValue {
	dyn := func(why string) doctorPyValue { return doctorPyValue{why: why} }
	if len(e) == 0 {
		return dyn("an empty expression")
	}
	if alts := f.split(e, "or"); len(alts) > 1 {
		for _, alt := range alts {
			if !f.eval(alt, hop).doctorPyIsFathom() {
				return dyn("an or whose alternatives are not all fathom")
			}
		}
		return doctorPyValue{kind: doctorPyFathom}
	}
	if parts := f.split(e, "+"); len(parts) > 1 {
		left := f.eval(parts[0], hop)
		if left.kind != doctorPyList {
			return dyn("a concatenation that does not start with a literal list")
		}
		left.tail = true
		return left
	}
	if f.fathomCall(e, hop) {
		return doctorPyValue{kind: doctorPyFathom}
	}
	switch {
	case e[0].Kind == pysyntax.String:
		var b strings.Builder
		for _, t := range e {
			if t.Kind != pysyntax.String {
				return dyn("an expression around a string")
			}
			for _, p := range t.Parts {
				if p.Dyn {
					return dyn("an f-string with a replacement field")
				}
				b.WriteString(p.Lit)
			}
		}
		if strings.Contains(b.String(), "\\") {
			return dyn("a string with an escape doctor does not decode")
		}
		return doctorPyValue{kind: doctorPyStr, s: b.String()}
	case len(e) == 1 && e[0].Kind == pysyntax.Name:
		name := e[0].Text
		if hop > 0 {
			return dyn(name + " is more than one binding hop away")
		}
		binds := f.binds[name]
		if len(binds) != 1 || !binds[0].plain || f.mutated[name] {
			return dyn(name + " is not bound exactly once by a plain assignment")
		}
		return f.eval(binds[0].value, hop+1)
	case e[0].Kind == pysyntax.Op && (e[0].Text == "[" || e[0].Text == "(") && f.closes(e):
		inner := e[1 : len(e)-1]
		items := f.split(inner, ",")
		if e[0].Text == "(" && len(items) == 1 && !(len(inner) > 0 && inner[len(inner)-1].Text == ",") {
			return f.eval(inner, hop) // parentheses, not a tuple
		}
		v := doctorPyValue{kind: doctorPyList}
		for _, item := range items {
			if len(item) == 0 {
				continue
			}
			if item[0].Kind == pysyntax.Op && item[0].Text == "*" {
				v.tail = true
				continue
			}
			v.elems = append(v.elems, f.elemAt(item, hop))
		}
		return v
	}
	return dyn("a value doctor does not resolve")
}

// elem resolves one argv element from scratch.
func (f *doctorPyFile) elem(e []pysyntax.Token) doctorPyElem { return f.elemAt(e, 0) }

func (f *doctorPyFile) elemAt(e []pysyntax.Token, hop int) doctorPyElem {
	switch v := f.eval(e, hop); v.kind {
	case doctorPyStr:
		return doctorPyElem{lit: v.s}
	case doctorPyFathom:
		return doctorPyElem{fathom: true}
	}
	return doctorPyElem{dyn: true}
}

// fathomCall reports a call that yields the fathom program, mirroring the
// shell reader's $FATHOM: shutil.which("fathom"), os.getenv("FATHOM", d),
// os.environ.get("FATHOM", d) and os.environ["FATHOM"], where a default d
// must itself be fathom.
func (f *doctorPyFile) fathomCall(e []pysyntax.Token, hop int) bool {
	if len(e) < 4 || e[0].Kind != pysyntax.Name || e[1].Text != "." {
		return false
	}
	mod := f.mods[e[0].Text]
	isLit := func(toks []pysyntax.Token, want string) bool {
		v := f.eval(toks, hop)
		return v.kind == doctorPyStr && v.s == want
	}
	callArgs := func(open int) ([]doctorPyArg, bool) {
		if open >= len(e) || e[open].Text != "(" || !f.closesAt(e, open) {
			return nil, false
		}
		var out []doctorPyArg
		var cur []pysyntax.Token
		d := 0
		for _, t := range e[open+1 : len(e)-1] {
			if t.Kind == pysyntax.Op {
				switch t.Text {
				case "(", "[", "{":
					d++
				case ")", "]", "}":
					d--
				case ",":
					if d == 0 {
						out = append(out, doctorPyArg{toks: cur})
						cur = nil
						continue
					}
				}
			}
			cur = append(cur, t)
		}
		if len(cur) > 0 {
			out = append(out, doctorPyArg{toks: cur})
		}
		return out, true
	}
	switch {
	case mod == "shutil" && e[2].Text == "which":
		a, ok := callArgs(3)
		return ok && len(a) == 1 && isLit(a[0].toks, "fathom")
	case mod == "os" && e[2].Text == "getenv":
		a, ok := callArgs(3)
		return ok && len(a) == 2 && isLit(a[0].toks, "FATHOM") && f.eval(a[1].toks, hop).doctorPyIsFathom()
	case mod == "os" && e[2].Text == "environ" && len(e) > 5 && e[3].Text == "." && e[4].Text == "get":
		a, ok := callArgs(5)
		return ok && len(a) == 2 && isLit(a[0].toks, "FATHOM") && f.eval(a[1].toks, hop).doctorPyIsFathom()
	case mod == "os" && e[2].Text == "environ" && e[3].Text == "[" && f.closesAt(e, 3):
		return isLit(e[4:len(e)-1], "FATHOM")
	}
	return false
}

func (v doctorPyValue) doctorPyIsFathom() bool {
	return v.kind == doctorPyFathom || v.kind == doctorPyStr && filepath.Base(v.s) == "fathom"
}

// closes reports whether e[0]'s bracket closes at e's last token.
func (f *doctorPyFile) closes(e []pysyntax.Token) bool { return f.closesAt(e, 0) }

func (f *doctorPyFile) closesAt(e []pysyntax.Token, open int) bool {
	d := 0
	for j := open; j < len(e); j++ {
		if e[j].Kind != pysyntax.Op {
			continue
		}
		switch e[j].Text {
		case "(", "[", "{":
			d++
		case ")", "]", "}":
			d--
			if d == 0 {
				return j == len(e)-1
			}
		}
	}
	return false
}

// split cuts e at each depth-0 sep (an operator or the keyword "or").
func (f *doctorPyFile) split(e []pysyntax.Token, sep string) [][]pysyntax.Token {
	var out [][]pysyntax.Token
	d, from := 0, 0
	for j, t := range e {
		if t.Kind == pysyntax.Op {
			switch t.Text {
			case "(", "[", "{":
				d++
				continue
			case ")", "]", "}":
				d--
				continue
			}
		}
		if d == 0 && t.Text == sep && (t.Kind == pysyntax.Op || sep == "or" && t.Kind == pysyntax.Name) {
			out = append(out, e[from:j])
			from = j + 1
		}
	}
	return append(out, e[from:])
}

// judge settles one resolved argv (P5's decisive positions, then P8).
func (f *doctorPyFile) judge(line int, name string, elems []doctorPyElem, tail bool) {
	if len(elems) == 0 {
		if tail {
			f.uncertain(line, name+": the program is not a literal")
		}
		return
	}
	prog := elems[0]
	if prog.dyn {
		f.uncertain(line, name+": the program is not a literal")
		return
	}
	if prog.fathom || filepath.Base(prog.lit) == "fathom" {
		decisive := 1
		if f.r.matches == nil {
			decisive = max(1, len(strings.Fields(f.sub)))
		}
		for j := 1; j <= decisive; j++ {
			if j >= len(elems) {
				if tail {
					f.uncertain(line, name+": the fathom subcommand is dynamic")
					return
				}
				break
			}
			if elems[j].dyn {
				f.uncertain(line, name+": the fathom subcommand is dynamic")
				return
			}
		}
		if f.r.matches != nil && len(elems) > 1 && elems[1].lit == "verify" && !slices.ContainsFunc(elems[2:], func(e doctorPyElem) bool { return e.lit == "--strict" }) &&
			(tail || slices.ContainsFunc(elems[2:], func(e doctorPyElem) bool { return e.dyn })) {
			f.uncertain(line, name+": whether fathom verify gets --strict is dynamic")
			return
		}
	}
	words := make([]string, 0, len(elems)+1)
	for _, e := range elems {
		switch {
		case e.fathom:
			words = append(words, "fathom")
		case e.dyn:
			words = append(words, "$DYN")
		default:
			words = append(words, e.lit)
		}
	}
	if tail {
		words = append(words, "$DYN")
	}
	f.result(line, name, f.r.argv(words, f.sub, f.depth))
}

// shellString judges a literal shell command like a line of a script (R2).
func (f *doctorPyFile) shellString(line int, name, s string) {
	if strings.Contains(s, "<<") || strings.Contains(s, "$(") || strings.Contains(s, "()") || strings.Contains(s, "`") {
		f.uncertain(line, name+": the shell command uses <<, $(, () or a backtick")
		return
	}
	f.result(line, name, f.r.command(s, f.sub, f.depth))
}

func (f *doctorPyFile) result(line int, name string, ev doctorWiringEvidence) {
	if ev.found {
		if !f.v.found.found {
			f.v.found = ev
		}
		return
	}
	if ev.unknown {
		why := name + ": the command it runs could not be settled"
		if ev.cause != "" {
			why += " (" + ev.cause + ")"
		}
		f.uncertain(line, why)
	}
	if f.v.missingScript == "" {
		f.v.missingScript = ev.missingScript
	}
	if f.v.opaque == "" {
		f.v.opaque = ev.opaque
	}
}
