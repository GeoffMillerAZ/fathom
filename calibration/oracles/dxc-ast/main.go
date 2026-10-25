// Command dxc-ast is the language-breadth harness's tier-B L1 and L2 oracle for
// HLSL (LB-Q-hlsl, design §1.2, §10). It writes indexbench lang-oracle's
// neutral oracle file (-tool-oracle): definitions and call pairs.
//
//	go run . -subject <checkout> -dxc <dxc> -files <list> [-I <dir>]... [-target lib_6_3] [-scope <out>] > dxc.json
//
// The tool is the compiler's own front end, DXC's text AST: `dxc -T <target>
// -HV 2021 -ast-dump [-I <dir>]... <file>` from the subject root, one process
// per measured file, with run-chosen flags and no build database (tier B). A
// unit whose run writes an `error:` is not read. A file is in scope when its
// own unit parses without error, or when it is a header (a file `dxc -M` lists)
// of a unit that does: -scope writes that list, one subject-relative path per
// line, and the oracle file's `scope` names it, so lang-oracle grades the
// definitions and calls on those files alone: a file the front end cannot
// read is neither a recall miss nor a precision miss, while the run itself
// covers every measured file.
//
// Definitions, each at its name's line with its range's last line: a function
// with a body (a template's pattern included, a struct's methods not), a
// struct or class with a body, a cbuffer or tbuffer, a typedef and a variable
// at file scope (a cbuffer's constants are the buffer's, not the file's). Call
// pairs: (the innermost enclosing definition, the callee's name) for every
// CallExpr naming a function (an intrinsic included) and every member call
// (`tex.Sample(...)`, the member's bare name). DXC places a node at its
// spelling location, so a definition or call a macro body writes sits on the
// #define line: such definitions and calls are not written, and are counted
// in the file's disclose lines. Declarations are read from every unit that
// includes their file and deduplicated. Bench-only; never a product
// dependency.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Def is one definition: its subject-relative file, name, oracle kind and
// first (name) and last line.
type Def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Line int    `json:"line"`
	End  int    `json:"end"`
}

// Pair is one call pair in a file.
type Pair struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

// Unit is what one AST dump gives.
type Unit struct {
	Defs       []Def
	Calls      []Pair
	MacroDefs  int // definitions spelled on a #define line, not written
	MacroCalls int // calls spelled on a #define line, not written
}

type oracleFile struct {
	Tool        string   `json:"tool"`
	Version     string   `json:"version"`
	Tier        string   `json:"tier"`
	Definitions []Def    `json:"definitions"`
	Calls       []Pair   `json:"calls"`
	Disclose    []string `json:"disclose,omitempty"`
	// Scope is the measured files this oracle reads (lang-oracle grades its
	// definitions and calls on them alone, while the run covers every file).
	Scope []string `json:"scope,omitempty"`
}

type pos struct {
	file string
	line int
}

// frame is one open node of the dump's tree.
type frame struct {
	depth int
	kind  string
	def   *Def // the definition this node is, once confirmed
	fn    *Def // a function decl waiting for its body
	call  bool // a call expression waiting for its callee
	path  string
}

var (
	reFileLoc  = regexp.MustCompile(`^(.+?\.[A-Za-z0-9_]+):(\d+):(\d+)`)
	reLineLoc  = regexp.MustCompile(`^line:(\d+):(\d+)`)
	reColLoc   = regexp.MustCompile(`^col:(\d+)`)
	reFuncRef  = regexp.MustCompile(`\b(?:Function|CXXMethod) 0x[0-9a-f]+ '([A-Za-z_][A-Za-z0-9_]*)'`)
	reMember   = regexp.MustCompile(`'?\s(?:\.|->)([A-Za-z_][A-Za-z0-9_]*) 0x[0-9a-f]+`)
	reDeclName = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*) '`)
)

// fileScope are the node kinds a file-scope declaration may sit in.
var fileScope = map[string]bool{"TranslationUnitDecl": true, "NamespaceDecl": true, "LinkageSpecDecl": true}

// parseDump reads one unit's AST dump. files is the measured set
// (subject-relative); macro reports whether a file's line belongs to a
// #define directive.
func parseDump(r io.Reader, files map[string]bool, macro func(string, int) bool) Unit {
	var u Unit
	var stack []frame
	cur := pos{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		line := sc.Text()
		i := 0
		for i < len(line) && strings.ContainsRune("|-` ", rune(line[i])) {
			i++
		}
		if i >= len(line) {
			continue
		}
		depth := i / 2
		rest := line[i:]
		kind, rest, _ := strings.Cut(rest, " ")
		if kind == "" || !(kind[0] >= 'A' && kind[0] <= 'Z') {
			continue
		}
		for len(stack) > 0 && stack[len(stack)-1].depth >= depth {
			stack = stack[:len(stack)-1]
		}
		// the address, then the range and the location
		_, rest, _ = strings.Cut(rest, " ")
		var begin, end, at pos
		hasAt := false
		// `parent cbuffer 0x..` precedes a cbuffer constant's range, `prev 0x..` a redeclaration's
		if strings.HasPrefix(rest, "parent ") || strings.HasPrefix(rest, "prev ") {
			if i := strings.Index(rest, " <"); i >= 0 {
				rest = rest[i+1:]
			}
		}
		if strings.HasPrefix(rest, "<") {
			rng, after := matchAngle(rest)
			parts := splitTop(rng)
			if len(parts) > 0 {
				begin = resolve(parts[0], &cur)
				end = begin
			}
			if len(parts) > 1 {
				end = resolve(parts[1], &cur)
			}
			rest = strings.TrimPrefix(after, " ")
			if tok, ok := locToken(rest); ok {
				at = resolve(tok, &cur)
				hasAt = true
				rest = strings.TrimPrefix(rest[len(tok):], " ")
			}
		}
		if !hasAt {
			at = begin
		}
		parent := ""
		if len(stack) > 0 {
			parent = stack[len(stack)-1].kind
		}
		fr := frame{depth: depth, kind: kind}
		rp := rel(at.file)
		implicit := strings.Contains(" "+rest+" ", " implicit ")
		inFiles := files[rp]
		mk := func(name, k string) *Def {
			if !inFiles || name == "" || implicit || at.line == 0 {
				return nil
			}
			if macro(rp, at.line) {
				u.MacroDefs++
				return nil
			}
			e := end.line
			if rel(end.file) != rp || e < at.line {
				e = at.line
			}
			return &Def{Path: rp, Name: name, Kind: k, Line: at.line, End: e}
		}
		switch kind {
		case "FunctionDecl":
			if fileScope[parent] || parent == "FunctionTemplateDecl" {
				fr.fn = mk(declName(rest), "function")
			}
		case "CompoundStmt":
			if len(stack) > 0 && stack[len(stack)-1].fn != nil {
				p := &stack[len(stack)-1]
				u.Defs = append(u.Defs, *p.fn)
				p.def, p.fn = p.fn, nil
			}
		case "CXXRecordDecl":
			if strings.HasSuffix(rest, " definition") && (fileScope[parent] || parent == "ClassTemplateDecl" || parent == "CXXRecordDecl") {
				f := strings.Fields(strings.TrimSuffix(rest, " definition"))
				if len(f) >= 2 && (f[len(f)-2] == "struct" || f[len(f)-2] == "class") {
					if d := mk(f[len(f)-1], "struct"); d != nil {
						u.Defs = append(u.Defs, *d)
						fr.def = d
					}
				}
			}
		case "HLSLBufferDecl":
			f := strings.Fields(rest)
			if len(f) >= 2 && (f[len(f)-2] == "cbuffer" || f[len(f)-2] == "tbuffer") {
				if d := mk(f[len(f)-1], "cbuffer"); d != nil {
					u.Defs = append(u.Defs, *d)
					fr.def = d
				}
			}
		case "TypedefDecl":
			if fileScope[parent] {
				if d := mk(declName(rest), "typedef"); d != nil {
					u.Defs = append(u.Defs, *d)
				}
			}
		case "VarDecl":
			if fileScope[parent] {
				if d := mk(declName(rest), "variable"); d != nil {
					u.Defs = append(u.Defs, *d)
					fr.def = d
				}
			}
		case "CallExpr", "CXXMemberCallExpr":
			fr.call = true
			fr.path = rp
			if macro(rp, at.line) {
				u.MacroCalls++
				fr.call = false
			}
		case "DeclRefExpr", "MemberExpr":
			// the callee: the first such node under a call's first child
			for k := len(stack) - 1; k >= 0; k-- {
				if !stack[k].call {
					if strings.HasSuffix(stack[k].kind, "CastExpr") {
						continue
					}
					break
				}
				var m []string
				if kind == "DeclRefExpr" {
					m = reFuncRef.FindStringSubmatch(rest)
				} else {
					m = reMember.FindStringSubmatch(rest)
				}
				if m != nil {
					if from := enclosing(stack[:k]); from != nil && from.Path == stack[k].path {
						u.Calls = append(u.Calls, Pair{Path: stack[k].path, From: from.Name, To: m[1]})
					}
				}
				stack[k].call = false
				break
			}
		}
		if len(stack) > 0 && stack[len(stack)-1].call && kind != "ImplicitCastExpr" && kind != "DeclRefExpr" && kind != "MemberExpr" && !strings.HasSuffix(kind, "CastExpr") {
			stack[len(stack)-1].call = false // the callee is not a name (a call through an expression)
		}
		stack = append(stack, fr)
	}
	return u
}

// enclosing is the innermost definition among the open frames.
func enclosing(stack []frame) *Def {
	for k := len(stack) - 1; k >= 0; k-- {
		if stack[k].def != nil {
			return stack[k].def
		}
	}
	return nil
}

// declName is a declaration line's name: the word before its quoted type.
func declName(rest string) string {
	m := reDeclName.FindStringSubmatch(rest)
	if m == nil {
		return ""
	}
	return m[1]
}

// matchAngle splits s, which starts with `<`, at its matching `>`.
func matchAngle(s string) (inner, after string) {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				return s[1:i], s[i+1:]
			}
		}
	}
	return s[1:], ""
}

// splitTop splits a range at its top-level ", ".
func splitTop(s string) []string {
	var out []string
	depth, last := 0, 0
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '<':
			depth++
		case s[i] == '>':
			depth--
		case depth == 0 && s[i] == ',' && i+1 < len(s) && s[i+1] == ' ':
			out = append(out, s[last:i])
			last = i + 2
		}
	}
	return append(out, s[last:])
}

// locToken is the location token rest starts with, if any.
func locToken(rest string) (string, bool) {
	for _, re := range []*regexp.Regexp{reLineLoc, reColLoc, reFileLoc} {
		if m := re.FindString(rest); m != "" {
			return m, true
		}
	}
	return "", false
}

// resolve reads one location (`line:L:C`, `col:C`, `path:L:C`, with any
// `<Spelling=...>` suffix) against the dump's compression state.
func resolve(tok string, cur *pos) pos {
	tok = strings.TrimSpace(tok)
	if i := strings.Index(tok, " <Spelling="); i >= 0 {
		tok = tok[:i]
	}
	if strings.HasPrefix(tok, "<") || tok == "" {
		return pos{}
	}
	if m := reLineLoc.FindStringSubmatch(tok); m != nil {
		cur.line, _ = strconv.Atoi(m[1])
		return *cur
	}
	if reColLoc.MatchString(tok) {
		return *cur
	}
	if m := reFileLoc.FindStringSubmatch(tok); m != nil {
		cur.file = m[1]
		cur.line, _ = strconv.Atoi(m[2])
		return *cur
	}
	return pos{}
}

// rel is a dump path relative to the subject root (the run's working
// directory).
func rel(p string) string {
	if p == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(p))
}

// defineLines are the 1-based lines of src that belong to a #define directive,
// its backslash continuations included.
func defineLines(src []byte) map[int]bool {
	out := map[int]bool{}
	cont := false
	for i, l := range strings.Split(string(src), "\n") {
		t := strings.TrimSpace(strings.TrimSuffix(l, "\r"))
		if cont || isDefine(t) {
			out[i+1] = true
			cont = strings.HasSuffix(t, "\\")
		}
	}
	return out
}

func isDefine(t string) bool {
	if !strings.HasPrefix(t, "#") {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(t[1:]), "define")
}

func sortDefs(d []Def) {
	sort.Slice(d, func(i, j int) bool {
		a, b := d[i], d[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Kind < b.Kind
	})
}

func sortPairs(p []Pair) {
	sort.Slice(p, func(i, j int) bool {
		a, b := p[i], p[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.To < b.To
	})
}

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

func main() {
	subject := flag.String("subject", "", "checkout to measure (required)")
	dxc := flag.String("dxc", "dxc", "dxc binary")
	target := flag.String("target", "lib_6_3", "dxc -T profile")
	retry := flag.String("retry-target", "lib_6_9", "the newest library profile, retried on a unit with an error")
	filesList := flag.String("files", "", "file of subject-relative paths, one per line: the harness's measured set (required)")
	scopeOut := flag.String("scope", "", "write the in-scope files here, one per line")
	var incs multi
	flag.Var(&incs, "I", "include root, subject-relative (repeatable)")
	flag.Parse()
	if *subject == "" || *filesList == "" {
		fmt.Fprintln(os.Stderr, "usage: dxc-ast -subject <dir> -files <list> [-dxc <dxc>] [-I <dir>]... [-target lib_6_3] [-scope <out>]")
		os.Exit(2)
	}
	if err := run(*subject, *dxc, *target, *retry, *filesList, *scopeOut, incs, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(subject, dxc, target, retryTarget, filesList, scopeOut string, incs []string, stdout, stderr io.Writer) error {
	b, err := os.ReadFile(filesList)
	if err != nil {
		return err
	}
	var list []string
	files := map[string]bool{}
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			list = append(list, l)
			files[l] = true
		}
	}
	defines := map[string]map[int]bool{}
	macro := func(p string, line int) bool {
		m, ok := defines[p]
		if !ok {
			src, err := os.ReadFile(filepath.Join(subject, filepath.FromSlash(p)))
			if err == nil {
				m = defineLines(src)
			}
			defines[p] = m
		}
		return m[line]
	}
	version := "unknown"
	if out, err := exec.Command(dxc, "--version").CombinedOutput(); err == nil {
		version = strings.TrimSpace(string(out))
	}
	// The run's setting, then the retries an errored unit gets (H5-R5): the
	// 2018 language version, and the newest library profile.
	settings := []setting{{target, "2021"}, {target, "2018"}, {retryTarget, "2021"}}
	argsFor := func(st setting, mode, f string) []string {
		a := []string{"-T", st.target, "-HV", st.hv}
		for _, inc := range incs {
			a = append(a, "-I", inc)
		}
		return append(a, mode, "./"+f)
	}
	dumpUnit := func(st setting, f string) (*bytes.Buffer, string, bool) {
		cmd := exec.Command(dxc, argsFor(st, "-ast-dump", f)...)
		cmd.Dir = subject
		var dump, diag bytes.Buffer
		cmd.Stdout, cmd.Stderr = &dump, &diag
		runErr := cmd.Run()
		if first := firstError(diag.String()); first != "" || runErr != nil {
			if first == "" {
				first = strings.TrimSpace(diag.String())
			}
			return nil, first, false
		}
		return &dump, "", true
	}
	scope := map[string]bool{}
	seenDef := map[Def]bool{}
	seenCall := map[Pair]bool{}
	var out oracleFile
	errored, macroDefs, macroCalls := 0, 0, 0
	var excluded []unitError             // units no setting parses, with the first error of the run's setting
	admits := make([]int, len(settings)) // errored units each retry setting parses
	var admitted []string                // "<file> (<setting>)": units a retry setting admitted
	for _, f := range list {
		st := settings[0]
		dump, first, ok := dumpUnit(st, f)
		if !ok {
			errored++
			var chosen *bytes.Buffer
			for k, retry := range settings[1:] {
				if d, _, ok := dumpUnit(retry, f); ok {
					admits[k+1]++
					if chosen == nil {
						chosen, st = d, retry
					}
				}
			}
			if chosen == nil {
				excluded = append(excluded, unitError{File: f, Msg: first})
				continue
			}
			dump = chosen
			admitted = append(admitted, fmt.Sprintf("%s (%s)", f, st))
		}
		scope[f] = true
		dep := exec.Command(dxc, argsFor(st, "-M", f)...)
		dep.Dir = subject
		if deps, err := dep.Output(); err == nil {
			for _, d := range depFiles(string(deps), subject) {
				if files[d] {
					scope[d] = true
				}
			}
		}
		u := parseDump(dump, files, macro)
		macroDefs += u.MacroDefs
		macroCalls += u.MacroCalls
		for _, d := range u.Defs {
			if !seenDef[d] {
				seenDef[d] = true
				out.Definitions = append(out.Definitions, d)
			}
		}
		for _, c := range u.Calls {
			if !seenCall[c] {
				seenCall[c] = true
				out.Calls = append(out.Calls, c)
			}
		}
	}
	// a definition read through an includer is kept only when its file is in scope
	defs := out.Definitions[:0]
	for _, d := range out.Definitions {
		if scope[d.Path] {
			defs = append(defs, d)
		}
	}
	out.Definitions = defs
	calls := out.Calls[:0]
	for _, c := range out.Calls {
		if scope[c.Path] {
			calls = append(calls, c)
		}
	}
	out.Calls = calls
	sortDefs(out.Definitions)
	sortPairs(out.Calls)
	out.Tool, out.Version, out.Tier = "dxc-ast", version, "B"
	out.Disclose = []string{
		fmt.Sprintf("dxc-ast (dxc -T %s -HV 2021 -ast-dump, run-chosen flags, no build database): %d of %d measured files in scope (their own unit, or a header of one, parses without error under a setting below); %d units with an error under that setting, %d of them not read under any", target, len(scope), len(list), errored, len(excluded)),
		fmt.Sprintf("dxc-ast: of the %d units with an error, %s admits %d and %s admits %d, each tried on every one; a unit is read under the first that parses it (%s, then %s), chosen per unit: %d admitted%s", errored, settings[1], admits[1], settings[2], admits[2], settings[1], settings[2], len(admitted), listSuffix(admitted)),
		fmt.Sprintf("dxc-ast: %d definitions and %d calls spelled on a #define line (a macro body) not written, macro-generated (counted per reading unit)", macroDefs, macroCalls),
	}
	out.Disclose = append(out.Disclose, errorClasses(excluded)...)
	for k := range scope {
		out.Scope = append(out.Scope, k)
	}
	sort.Strings(out.Scope)
	if scopeOut != "" {
		if err := os.WriteFile(scopeOut, []byte(strings.Join(out.Scope, "\n")+"\n"), 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(stderr, "files %d in_scope %d errored %d excluded %d admitted %d definitions %d calls %d macro_defs %d macro_calls %d\n",
		len(list), len(scope), errored, len(excluded), len(admitted), len(out.Definitions), len(out.Calls), macroDefs, macroCalls)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", " ")
	return enc.Encode(out)
}

// depFiles are the subject-relative files a `dxc -M` listing names (a
// make rule: `target: dep dep \` with escaped spaces).
func depFiles(s, subject string) []string {
	_, deps, ok := strings.Cut(s, ":")
	if !ok {
		return nil
	}
	deps = strings.ReplaceAll(deps, "\\\n", " ")
	deps = strings.ReplaceAll(deps, "\\ ", "\x00")
	var out []string
	for _, d := range strings.Fields(deps) {
		d = strings.ReplaceAll(d, "\x00", " ")
		if filepath.IsAbs(d) {
			r, err := filepath.Rel(subject, d)
			if err != nil {
				continue
			}
			d = r
		}
		out = append(out, rel(d))
	}
	return out
}

// setting is one dxc front-end configuration: a profile and a language
// version.
type setting struct{ target, hv string }

func (s setting) String() string { return fmt.Sprintf("-T %s -HV %s", s.target, s.hv) }

// unitError is a unit no setting parses and the first error its run wrote.
type unitError struct{ File, Msg string }

var (
	reErrorLine = regexp.MustCompile(`(?m)^(.*?):(\d+):\d+: (?:fatal )?error: (.*)$`)
	reQuoted    = regexp.MustCompile(`'[^']*'`)
)

// firstError is the first `error:` diagnostic of a dxc run, as
// `<file>:<line>: <message>`, or "" when it wrote none.
func firstError(diag string) string {
	if m := reErrorLine.FindStringSubmatch(diag); m != nil {
		return fmt.Sprintf("%s:%s: %s", strings.TrimPrefix(m[1], "./"), m[2], strings.TrimSpace(m[3]))
	}
	for _, l := range strings.Split(diag, "\n") {
		if strings.Contains(l, "error:") {
			return strings.TrimSpace(l)
		}
	}
	return ""
}

// errorClass is a first error's class: its message with every quoted name
// replaced by '_' and any trailing [-W...] flag dropped.
func errorClass(first string) string {
	msg := first
	if i := strings.Index(first, ": "); i >= 0 && strings.Contains(first[:i], ":") {
		msg = first[i+2:]
	}
	if i := strings.LastIndex(msg, " [-W"); i >= 0 {
		msg = msg[:i]
	}
	return reQuoted.ReplaceAllString(msg, "'_'")
}

// errorClasses discloses the excluded units by first-error class, the most
// frequent first, with two examples each.
func errorClasses(units []unitError) []string {
	by := map[string][]string{}
	for _, u := range units {
		c := errorClass(u.Msg)
		by[c] = append(by[c], u.Msg)
	}
	classes := make([]string, 0, len(by))
	for c := range by {
		classes = append(classes, c)
	}
	sort.Slice(classes, func(i, j int) bool {
		if len(by[classes[i]]) != len(by[classes[j]]) {
			return len(by[classes[i]]) > len(by[classes[j]])
		}
		return classes[i] < classes[j]
	})
	out := make([]string, 0, len(classes)+1)
	for _, c := range classes {
		ex := slices.Compact(slices.Sorted(slices.Values(by[c]))) // one header's error reached from two units is one example
		if len(ex) > 2 {
			ex = ex[:2]
		}
		out = append(out, fmt.Sprintf("dxc-ast excluded units by first error: %d %q (e.g. %s)", len(by[c]), c, strings.Join(ex, "; ")))
	}
	missing, elsewhere := 0, 0
	for _, u := range units {
		switch c := errorClass(u.Msg); {
		case strings.Contains(c, "file not found"):
			missing++
		case strings.HasPrefix(c, "use of undeclared identifier"), strings.HasPrefix(c, "unknown type name"):
			elsewhere++
		}
	}
	if len(units) > 0 {
		out = append(out, fmt.Sprintf("dxc-ast scope bias: of the %d units excluded, %d stop at an #include this run's -I roots do not resolve (a build passes its own) and %d use a name no file they include declares (a header written to be compiled inside an includer, or a name a build define supplies), so the scope leans to self-contained units and away from code built with its own include roots and defines", len(units), missing, elsewhere))
	}
	return out
}

// listSuffix is ": a, b" for a non-empty list, sorted.
func listSuffix(items []string) string {
	if len(items) == 0 {
		return ""
	}
	sorted := append([]string(nil), items...)
	sort.Strings(sorted)
	return ": " + strings.Join(sorted, ", ")
}
