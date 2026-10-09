package fusion

// cguards.go — LB-Q-c-rest (O468): an #include is followed only where the
// build's preprocessor can keep it.
//
// A C or C++ header includes another platform's setup under a guard (curl's
// curl_setup.h: `#ifdef __VMS` / `#include "setup-vms.h"`; libuv's uv.h:
// `#if defined(_WIN32)` / `#include "uv/win.h"`). Following every #include
// put those headers' definitions in scope of every file, and a call to
// getenv or bind bound the VMS or OS/400 macro of that name.
//
// When a compile database is read, the #if, #ifdef, #ifndef, #elif and #else
// groups around each #include are evaluated lexically, in three values (true,
// false, unknown), against what the build is known to define:
//   - a translation unit's own -D and -U flags (a header takes those every
//     unit of the database agrees on);
//   - the target's predefined platform and compiler macros (cHostProfiles,
//     the target read from -target/--target, else -mmacosx-version-min,
//     -isysroot or -arch, else the machine resolving): defined ones with
//     their value, and the other platforms' identification macros as
//     undefined;
//   - a #define or #undef earlier in the same file on lines known to be kept.
// Everything else is unknown, and an unknown condition keeps its include: an
// include is left out only inside a branch known not to be taken. With no
// compile database nothing is evaluated and every include is followed, as
// before.

import (
	"runtime"
	"strconv"
	"strings"
)

// cMacro is what a #if knows of one macro: defined (with its value as
// written, "1" for a bare -D) or undefined. A macro absent from a cMacroEnv
// is unknown.
type cMacro struct {
	defined bool
	value   string
}

// cMacroEnv is the macros a guard evaluation knows.
type cMacroEnv map[string]cMacro

// cPlatformMacros are the platform and compiler identification macros a
// compiler predefines and source never does: for a known target, those its
// profile does not define are known undefined.
var cPlatformMacros = []string{
	"_WIN32", "_WIN64", "_WIN32_WCE", "__CYGWIN__", "__CYGWIN32__", "__MINGW32__", "__MINGW64__",
	"__VMS", "__OS400__", "__ANDROID__", "__linux__", "__linux", "__gnu_linux__", "__APPLE__",
	"__MACH__", "__FreeBSD__", "__NetBSD__", "__OpenBSD__", "__DragonFly__", "__sun", "__SVR4",
	"_AIX", "__hpux", "__HAIKU__", "__QNX__", "__QNXNTO__", "__amigaos__", "__AMIGA__",
	"__DJGPP__", "__MSDOS__", "__riscos__", "__EMSCRIPTEN__", "__wasi__", "__MVS__", "__TANDEM",
	"__OS2__", "__BEOS__", "__SYMBIAN32__", "macintosh", "__unix__", "__unix",
	"_MSC_VER", "__INTEL_COMPILER", "__BORLANDC__", "__WATCOMC__", "__SUNPRO_C", "__IBMC__",
	"__xlc__", "__HP_cc", "__DECC",
}

// cHostProfiles are the macros each target predefines (the platform's and
// its usual compiler's); every other cPlatformMacros name is undefined there.
var cHostProfiles = map[string]map[string]string{
	"darwin":  {"__APPLE__": "1", "__MACH__": "1", "__clang__": "1", "__GNUC__": "4", "__STDC__": "1"},
	"linux":   {"__linux__": "1", "__linux": "1", "__gnu_linux__": "1", "__unix__": "1", "__unix": "1", "__ELF__": "1", "__STDC__": "1"},
	"windows": {"_WIN32": "1"},
}

// cHostGOOS is the machine's platform, the target when no compile command
// names one (a compile database is written on the machine that builds).
var cHostGOOS = runtime.GOOS

// cTargetOf reads the target platform from one compile command's arguments:
// "" when they name none.
func cTargetOf(args []string) string {
	for i, a := range args {
		var triple string
		switch {
		case a == "-target" && i+1 < len(args):
			triple = args[i+1]
		case strings.HasPrefix(a, "--target="):
			triple = strings.TrimPrefix(a, "--target=")
		case strings.HasPrefix(a, "-mmacosx-version-min"), a == "-arch",
			a == "-isysroot" && i+1 < len(args) && strings.Contains(args[i+1], "MacOSX"):
			return "darwin"
		}
		switch {
		case triple == "":
		case strings.Contains(triple, "apple") || strings.Contains(triple, "darwin") || strings.Contains(triple, "macos"):
			return "darwin"
		case strings.Contains(triple, "linux"):
			return "linux"
		case strings.Contains(triple, "windows") || strings.Contains(triple, "mingw") || strings.Contains(triple, "win32"):
			return "windows"
		default:
			return "other" // a target named but not profiled: no platform macro is known
		}
	}
	return ""
}

// cHostEnv is target's profile: its predefined macros defined, the other
// platforms' identification macros undefined (nil for an unprofiled target).
func cHostEnv(target string) cMacroEnv {
	prof, ok := cHostProfiles[target]
	if !ok {
		return nil
	}
	env := cMacroEnv{}
	for _, m := range cPlatformMacros {
		env[m] = cMacro{}
	}
	for m, v := range prof {
		env[m] = cMacro{defined: true, value: v}
	}
	return env
}

// defineFlags reads the -D and -U flags of one compile command, in order
// (a later flag for the same name wins, as for the compiler).
func defineFlags(args []string) cMacroEnv {
	env := cMacroEnv{}
	for i := 0; i < len(args); i++ {
		a := strings.Trim(args[i], `"'`)
		var flag, val string
		switch {
		case a == "-D" || a == "-U":
			if i+1 >= len(args) {
				continue
			}
			i++
			flag, val = a, strings.Trim(args[i], `"'`)
		case strings.HasPrefix(a, "-D") || strings.HasPrefix(a, "-U"):
			flag, val = a[:2], a[2:]
		default:
			continue
		}
		name, v, hasV := strings.Cut(val, "=")
		if name == "" || strings.ContainsAny(name, "( ") {
			continue // a function-like -D is no value a guard reads here
		}
		if flag == "-U" {
			env[name] = cMacro{}
			continue
		}
		if !hasV {
			v = "1"
		}
		env[name] = cMacro{defined: true, value: v}
	}
	return env
}

// guardEnv is what an #if in file is known to see: the target profile, then
// the file's own compile commands' -D/-U flags (a file no entry compiles takes
// those every entry agrees on). nil when no compile database is read.
func (e *resolveEnv) guardEnv(file string) cMacroEnv {
	db := e.compDatabase()
	if len(db.entries) == 0 {
		return nil
	}
	if env, ok := db.guardMemo[file]; ok {
		return env
	}
	env := cMacroEnv{}
	for m, v := range db.host {
		env[m] = v
	}
	idxs := db.byFile[file]
	if len(idxs) == 0 {
		for m, v := range db.commonDefs {
			env[m] = v
		}
	} else {
		for m, v := range agreedDefs(db, idxs) {
			env[m] = v
		}
	}
	if db.guardMemo == nil {
		db.guardMemo = map[string]cMacroEnv{}
	}
	db.guardMemo[file] = env
	return env
}

// agreedDefs is the -D/-U flags every one of the entries idxs gives with the
// same meaning; a name some entry leaves alone is unknown.
func agreedDefs(db *compDB, idxs []int) cMacroEnv {
	out := cMacroEnv{}
	for k, i := range idxs {
		d := db.entries[i].defs
		if k == 0 {
			for m, v := range d {
				out[m] = v
			}
			continue
		}
		for m, v := range out {
			if w, ok := d[m]; !ok || w != v {
				delete(out, m)
			}
		}
	}
	return out
}

// tri is a three-valued truth: known true, known false or unknown.
type tri int8

const (
	triUnknown tri = iota
	triTrue
	triFalse
)

func triOf(b bool) tri {
	if b {
		return triTrue
	}
	return triFalse
}

func (t tri) not() tri {
	switch t {
	case triTrue:
		return triFalse
	case triFalse:
		return triTrue
	}
	return triUnknown
}

func triAnd(a, b tri) tri {
	switch {
	case a == triFalse || b == triFalse:
		return triFalse
	case a == triTrue && b == triTrue:
		return triTrue
	}
	return triUnknown
}

func triOr(a, b tri) tri {
	switch {
	case a == triTrue || b == triTrue:
		return triTrue
	case a == triFalse && b == triFalse:
		return triFalse
	}
	return triUnknown
}

// cGroup is one open #if group: whether its current branch is taken, and
// whether some earlier branch of it was.
type cGroup struct {
	branch, anyTaken tri
}

// cExcludedLines is the 1-based lines of src inside a conditional branch
// known not to be taken under env (nil when env is nil or none is).
func cExcludedLines(src []byte, env cMacroEnv) map[int]bool {
	if env == nil {
		return nil
	}
	local := cMacroEnv{}
	for m, v := range env {
		local[m] = v
	}
	var (
		stack []cGroup
		out   map[int]bool
	)
	state := func(groups []cGroup) tri { // the known state of a line inside groups
		s := triTrue
		for _, g := range groups {
			s = triAnd(s, g.branch)
		}
		return s
	}
	lines := strings.Split(string(src), "\n")
	inComment := false
	for i := 0; i < len(lines); i++ {
		text := lines[i]
		first := i
		for strings.HasSuffix(strings.TrimRight(text, "\r"), "\\") && i+1 < len(lines) {
			i++
			text = strings.TrimSuffix(strings.TrimRight(text, "\r"), "\\") + " " + lines[i]
		}
		code := stripCComments(text, &inComment)
		trim := strings.TrimSpace(code)
		cur := state(stack)
		if !strings.HasPrefix(trim, "#") {
			if cur == triFalse {
				for l := first + 1; l <= i+1; l++ {
					if out == nil {
						out = map[int]bool{}
					}
					out[l] = true
				}
			}
			continue
		}
		dir := strings.TrimSpace(trim[1:])
		kw := dir
		if j := strings.IndexFunc(dir, func(r rune) bool { return !(r == '_' || r >= 'a' && r <= 'z') }); j >= 0 {
			kw = dir[:j]
		}
		rest := strings.TrimSpace(dir[len(kw):])
		switch kw {
		case "elif", "elifdef", "elifndef", "else", "endif":
			// a group's own directive line belongs to the level around it
			if len(stack) > 0 {
				cur = state(stack[:len(stack)-1])
			}
		}
		switch kw {
		case "if":
			c := evalCCondition(rest, local)
			stack = append(stack, cGroup{branch: c, anyTaken: c})
		case "ifdef", "ifndef":
			c := cDefinedState(firstIdent(rest), local)
			if kw == "ifndef" {
				c = c.not()
			}
			stack = append(stack, cGroup{branch: c, anyTaken: c})
		case "elif", "elifdef", "elifndef":
			if len(stack) == 0 {
				continue
			}
			g := &stack[len(stack)-1]
			var c tri
			switch kw {
			case "elif":
				c = evalCCondition(rest, local)
			case "elifdef":
				c = cDefinedState(firstIdent(rest), local)
			default:
				c = cDefinedState(firstIdent(rest), local).not()
			}
			g.branch = triAnd(g.anyTaken.not(), c)
			g.anyTaken = triOr(g.anyTaken, c)
		case "else":
			if len(stack) == 0 {
				continue
			}
			g := &stack[len(stack)-1]
			g.branch = g.anyTaken.not()
			g.anyTaken = triTrue
		case "endif":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case "define", "undef":
			name := firstIdent(rest)
			if name == "" || cur == triFalse {
				break
			}
			if cur == triUnknown {
				delete(local, name) // defined or not depending on an unknown branch
				break
			}
			if kw == "undef" {
				local[name] = cMacro{}
				break
			}
			body := strings.TrimSpace(rest[len(name):])
			if strings.HasPrefix(body, "(") {
				local[name] = cMacro{defined: true} // function-like: defined, no value
			} else {
				local[name] = cMacro{defined: true, value: body}
			}
		}
		if cur == triFalse {
			for l := first + 1; l <= i+1; l++ {
				if out == nil {
					out = map[int]bool{}
				}
				out[l] = true
			}
		}
	}
	return out
}

// stripCComments removes the comments of one logical line, carrying an open
// block comment across lines in *open.
func stripCComments(s string, open *bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if *open {
			if j := strings.Index(s[i:], "*/"); j >= 0 {
				i += j + 1
				*open = false
				b.WriteByte(' ')
				continue
			}
			return b.String()
		}
		switch {
		case s[i] == '/' && i+1 < len(s) && s[i+1] == '*':
			*open = true
			i++
		case s[i] == '/' && i+1 < len(s) && s[i+1] == '/':
			return b.String()
		case s[i] == '"' || s[i] == '\'':
			q := s[i]
			b.WriteByte(q)
			for i++; i < len(s) && s[i] != q; i++ {
				if s[i] == '\\' && i+1 < len(s) {
					b.WriteByte(s[i])
					i++
				}
				b.WriteByte(s[i])
			}
			if i < len(s) {
				b.WriteByte(q)
			}
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func firstIdent(s string) string {
	s = strings.TrimSpace(s)
	j := 0
	for j < len(s) && isIdentByteC(s[j]) {
		j++
	}
	return s[:j]
}

func isIdentByteC(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// cDefinedState is `defined(name)` under env.
func cDefinedState(name string, env cMacroEnv) tri {
	if name == "" {
		return triUnknown
	}
	m, ok := env[name]
	if !ok {
		return triUnknown
	}
	return triOf(m.defined)
}

// cVal is a preprocessor expression's value: known (v) or unknown.
type cVal struct {
	known bool
	v     int64
}

// evalCCondition evaluates an #if or #elif expression under env: true when
// it is known nonzero, false when known zero, unknown otherwise (an unknown
// macro, a function-like macro such as __has_include, or text this
// evaluator does not parse).
func evalCCondition(expr string, env cMacroEnv) tri {
	p := &cExprParser{toks: cExprTokens(expr), env: env}
	v := p.ternary(0)
	if p.bad || p.pos != len(p.toks) || !v.known {
		return triUnknown
	}
	return triOf(v.v != 0)
}

// cExprTokens splits an #if expression into identifiers, numbers and
// operators.
func cExprTokens(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			i++
		case isIdentByteC(c):
			j := i
			for j < len(s) && isIdentByteC(s[j]) {
				j++
			}
			out = append(out, s[i:j])
			i = j
		default:
			two := ""
			if i+1 < len(s) {
				two = s[i : i+2]
			}
			switch two {
			case "&&", "||", "==", "!=", "<=", ">=", "<<", ">>":
				out = append(out, two)
				i += 2
				continue
			}
			out = append(out, string(c))
			i++
		}
	}
	return out
}

type cExprParser struct {
	toks  []string
	pos   int
	env   cMacroEnv
	bad   bool
	depth int
}

func (p *cExprParser) peek() string {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return ""
}

func (p *cExprParser) next() string {
	t := p.peek()
	if p.pos < len(p.toks) {
		p.pos++
	}
	return t
}

// cBinary are the binary operators by precedence, loosest first.
var cBinary = [][]string{{"||"}, {"&&"}, {"|"}, {"^"}, {"&"}, {"==", "!="}, {"<", ">", "<=", ">="}, {"<<", ">>"}, {"+", "-"}, {"*", "/", "%"}}

func (p *cExprParser) ternary(_ int) cVal {
	c := p.binary(0)
	if p.peek() != "?" {
		return c
	}
	p.next()
	a := p.ternary(0)
	if p.next() != ":" {
		p.bad = true
		return cVal{}
	}
	b := p.ternary(0)
	switch {
	case !c.known:
		if a.known && b.known && a.v == b.v {
			return a
		}
		return cVal{}
	case c.v != 0:
		return a
	}
	return b
}

func (p *cExprParser) binary(level int) cVal {
	if level == len(cBinary) {
		return p.unary()
	}
	l := p.binary(level + 1)
	for {
		op := p.peek()
		found := false
		for _, o := range cBinary[level] {
			if op == o {
				found = true
			}
		}
		if !found || p.bad {
			return l
		}
		p.next()
		r := p.binary(level + 1)
		l = cApply(op, l, r)
	}
}

// cApply is one binary operation; && and || are known from one known side
// that settles them.
func cApply(op string, l, r cVal) cVal {
	switch op {
	case "&&":
		if l.known && l.v == 0 || r.known && r.v == 0 {
			return cVal{true, 0}
		}
		if l.known && r.known {
			return cVal{true, 1}
		}
		return cVal{}
	case "||":
		if l.known && l.v != 0 || r.known && r.v != 0 {
			return cVal{true, 1}
		}
		if l.known && r.known {
			return cVal{true, 0}
		}
		return cVal{}
	}
	if !l.known || !r.known {
		return cVal{}
	}
	b := func(x bool) cVal {
		if x {
			return cVal{true, 1}
		}
		return cVal{true, 0}
	}
	switch op {
	case "|":
		return cVal{true, l.v | r.v}
	case "^":
		return cVal{true, l.v ^ r.v}
	case "&":
		return cVal{true, l.v & r.v}
	case "==":
		return b(l.v == r.v)
	case "!=":
		return b(l.v != r.v)
	case "<":
		return b(l.v < r.v)
	case ">":
		return b(l.v > r.v)
	case "<=":
		return b(l.v <= r.v)
	case ">=":
		return b(l.v >= r.v)
	case "<<":
		if r.v < 0 || r.v > 62 {
			return cVal{}
		}
		return cVal{true, l.v << r.v}
	case ">>":
		if r.v < 0 || r.v > 62 {
			return cVal{}
		}
		return cVal{true, l.v >> r.v}
	case "+":
		return cVal{true, l.v + r.v}
	case "-":
		return cVal{true, l.v - r.v}
	case "*":
		return cVal{true, l.v * r.v}
	case "/", "%":
		if r.v == 0 {
			return cVal{}
		}
		if op == "/" {
			return cVal{true, l.v / r.v}
		}
		return cVal{true, l.v % r.v}
	}
	return cVal{}
}

func (p *cExprParser) unary() cVal {
	switch p.peek() {
	case "!":
		p.next()
		v := p.unary()
		if !v.known {
			return v
		}
		if v.v == 0 {
			return cVal{true, 1}
		}
		return cVal{true, 0}
	case "~":
		p.next()
		v := p.unary()
		if v.known {
			v.v = ^v.v
		}
		return v
	case "-":
		p.next()
		v := p.unary()
		if v.known {
			v.v = -v.v
		}
		return v
	case "+":
		p.next()
		return p.unary()
	}
	return p.primary()
}

func (p *cExprParser) primary() cVal {
	t := p.next()
	switch {
	case t == "":
		p.bad = true
		return cVal{}
	case t == "(":
		v := p.ternary(0)
		if p.next() != ")" {
			p.bad = true
		}
		return v
	case t == "defined":
		paren := p.peek() == "("
		if paren {
			p.next()
		}
		name := p.next()
		if paren && p.next() != ")" {
			p.bad = true
		}
		switch cDefinedState(name, p.env) {
		case triTrue:
			return cVal{true, 1}
		case triFalse:
			return cVal{true, 0}
		}
		return cVal{}
	case t[0] >= '0' && t[0] <= '9':
		return cNumber(t)
	case isIdentByteC(t[0]):
		if p.peek() == "(" { // a function-like macro (__has_include, a version test)
			depth := 0
			for p.pos < len(p.toks) {
				switch p.next() {
				case "(":
					depth++
				case ")":
					depth--
				}
				if depth == 0 {
					break
				}
			}
			return cVal{}
		}
		return p.macroValue(t)
	}
	p.bad = true
	return cVal{}
}

// macroValue is an identifier's value in an #if: a known undefined name is
// 0, a known definition its value when that is a number or another macro
// (followed a few levels), anything else unknown.
func (p *cExprParser) macroValue(name string) cVal {
	m, ok := p.env[name]
	if !ok {
		return cVal{}
	}
	if !m.defined {
		return cVal{true, 0}
	}
	v := strings.TrimSpace(m.value)
	if v == "" {
		return cVal{}
	}
	if v[0] >= '0' && v[0] <= '9' {
		return cNumber(v)
	}
	if p.depth < 4 && firstIdent(v) == v {
		p.depth++
		defer func() { p.depth-- }()
		return p.macroValue(v)
	}
	return cVal{}
}

// cNumber reads an integer literal with its suffixes (u, l), decimal, octal
// or hex; anything else is unknown.
func cNumber(t string) cVal {
	t = strings.TrimRight(t, "uUlL")
	base := 10
	switch {
	case strings.HasPrefix(t, "0x") || strings.HasPrefix(t, "0X"):
		base, t = 16, t[2:]
	case len(t) > 1 && t[0] == '0':
		base, t = 8, t[1:]
	}
	v, err := strconv.ParseInt(t, base, 64)
	if err != nil {
		return cVal{}
	}
	return cVal{true, v}
}
