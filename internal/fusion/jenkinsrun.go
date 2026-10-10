package fusion

// jenkinsrun.go — LB-F374..LB-F376: what a Jenkins pipeline runs and calls.
// The jenkinsfile dialect (internal/extractor/dialects/jenkinsfile) reads a
// Jenkinsfile's declarative sections and a shared library's vars/<name>.groovy
// as definitions with their calls; this pass, hooked into the doc-xref pass
// (docxref.go), turns three of their steps into cross-file references:
//
//   - a `sh`, `bat`, `powershell` or `pwsh` step whose script is a literal: the
//     first command of each line names a Makefile target, a Taskfile task, a
//     package.json script or a script file's function through the SAME token
//     rule a GitHub Actions run step reads (ghaRunRefs, gharun.go), from the
//     repository root (the workspace a checkout gives) under each enclosing
//     `dir('x') { }`. The origin is the definition that encloses the step (a
//     stage's `steps`, a scripted stage, a helper function). A GString keeps
//     its `${...}` as written, so the word holding it is refused exactly as
//     plainWord refuses a shell variable, and a `dir("${X}")` resolves
//     nothing beneath it. A shared library's own steps run in the calling
//     job's workspace, never the library's repository, so a vars/ file's
//     shell steps join nothing (tier C: nothing evaluates Groovy or a shell,
//     which caps the cross-reference family at good).
//   - a step call whose name is a vars/<name>.groovy of the indexed repository
//     (a call the dialect attached to a definition): that variable's `call`.
//   - `build job: 'x'` (also `build(job: ...)` and `build 'x'`): recorded and
//     counted unresolved by its reason, since no dialect declares an
//     in-repository job to join it to (LB-Q-jcasc is deferred: JCasC declares
//     jobs inside Job DSL script strings).
//
// The scanner below is a small Groovy lexer (strings, GStrings, comments,
// braces), not a parser: it reads the step forms by their tokens, so it reads
// a scripted pipeline the grammar leaves with an ERROR node as well as a
// declarative one.

import (
	"context"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// The edges and unresolved reasons this pass writes.
const (
	jenkinsVarEdge     = "library_step"
	jenkinsVarStrategy = "jenkins-vars"

	// jenkinsJobUndeclared: a literal `build job:` name no in-repository
	// declaration provides (none can yet: LB-Q-jcasc is deferred).
	jenkinsJobUndeclared = "job-undeclared"
	// jenkinsJobTemplated: a job name written as a GString with ${...}.
	jenkinsJobTemplated = "job-templated"
	// jenkinsVarAmbiguous: a step call two vars/ directories declare.
	jenkinsVarAmbiguous = "var-ambiguous"
	// jenkinsShellOutside: a shell step no definition encloses (a scripted
	// pipeline's step outside every stage and function).
	jenkinsShellOutside = "shell-outside-definition"
)

// jenkinsShellSteps are the steps whose literal script is a shell's (sh), a
// Windows batch's (bat) or PowerShell's (powershell, pwsh).
var jenkinsShellSteps = map[string]bool{"sh": true, "bat": true, "powershell": true, "pwsh": true}

// isJenkinsXrefInput: a file the jenkinsfile dialect claims (its manifest's
// basename, extension and vars/ glob), whose change re-runs the doc-xref pass.
func isJenkinsXrefInput(p string) bool {
	b := path.Base(p)
	return b == "Jenkinsfile" || strings.EqualFold(path.Ext(b), ".jenkinsfile") || isJenkinsVarsFile(p)
}

// isJenkinsVarsFile: a shared library's global variable, vars/<name>.groovy
// (the manifest glob **/vars/*.groovy).
func isJenkinsVarsFile(p string) bool {
	return path.Ext(p) == ".groovy" && path.Base(path.Dir(p)) == "vars"
}

// jenkinsShell is one shell step with a literal script.
type jenkinsShell struct {
	Line   int
	Step   string // sh, bat, powershell or pwsh
	Dir    string // the working directory, repository-relative; "?" when computed
	Script string // the script, escapes applied, each ${...} kept as written
}

// jenkinsJob is one `build` step's job.
type jenkinsJob struct {
	Line      int
	Name      string
	Templated bool
}

// jenkinsFile is one file the jenkinsfile dialect read.
type jenkinsFile struct {
	Path    string
	Content []byte
	Nodes   []core.SymbolNode
}

// jenkinsResult is the pass's outcome: the references it resolved and the
// ones it recorded without a target, by reason.
type jenkinsResult struct {
	Xrefs      []DocXref
	Shells     int // literal shell steps read in pipelines
	Jobs       int // build steps recorded
	Unresolved map[string]int
}

// resolveJenkins resolves the files' shell steps, vars calls and build jobs.
// files and extract serve the token rule's targets as for a GitHub Actions run
// step (a Taskfile or package.json from the doc engine, a Makefile or script
// read on demand).
func resolveJenkins(jfs []jenkinsFile, files []DocFile, paths map[string]bool, extract func(file string) []core.SymbolNode) jenkinsResult {
	res := jenkinsResult{Unresolved: map[string]int{}}
	run := &ghaRunResolver{paths: paths, nodes: map[string]map[string]core.SymbolNode{}, extract: extract, cache: map[string][]core.SymbolNode{}}
	for _, f := range files {
		m := map[string]core.SymbolNode{}
		for _, n := range f.Result.Nodes {
			if _, dup := m[n.Name]; !dup {
				m[n.Name] = n
			}
		}
		run.nodes[f.Path] = m
	}
	vars := jenkinsVars(jfs)
	seen := map[string]bool{}
	add := func(x DocXref) {
		id := x.FromFile + "\x00" + x.FromKey + "\x00" + x.ToKey
		if x.FromKey == x.ToKey || seen[id] {
			return
		}
		seen[id] = true
		res.Xrefs = append(res.Xrefs, x)
	}
	for _, jf := range jfs {
		if !isJenkinsVarsFile(jf.Path) {
			shells, jobs := jenkinsSteps(jf.Content)
			res.Shells += len(shells)
			res.Jobs += len(jobs)
			for _, s := range shells {
				from, ok := innermostNode(jf.Nodes, s.Line)
				if !ok {
					res.Unresolved[jenkinsShellOutside]++
					continue
				}
				if s.Dir == "?" {
					continue
				}
				for _, ref := range ghaRunRefs(s.Script, s.Dir) {
					file, to, ok := run.resolve(ref)
					if !ok {
						continue
					}
					add(DocXref{FromFile: jf.Path, From: from.Name, FromKey: from.Key(), ToFile: file, To: to.Name, ToKey: to.Key(), Edge: ghaRunEdge, Strategy: ghaRunStrategy})
				}
			}
			for _, j := range jobs {
				if j.Templated {
					res.Unresolved[jenkinsJobTemplated]++
				} else {
					res.Unresolved[jenkinsJobUndeclared]++
				}
			}
		}
		for _, n := range jf.Nodes {
			for _, callee := range n.Calls {
				ts := vars[callee]
				switch {
				case len(ts) == 1:
					add(DocXref{FromFile: jf.Path, From: n.Name, FromKey: n.Key(), ToFile: ts[0].File, To: ts[0].Call.Name, ToKey: ts[0].Call.Key(), Edge: jenkinsVarEdge, Strategy: jenkinsVarStrategy})
				case len(ts) > 1:
					res.Unresolved[jenkinsVarAmbiguous]++
				}
			}
		}
	}
	sort.Slice(res.Xrefs, func(i, j int) bool {
		a, b := res.Xrefs[i], res.Xrefs[j]
		for _, p := range [][2]string{{a.FromFile, b.FromFile}, {a.From, b.From}, {a.ToFile, b.ToFile}, {a.To, b.To}, {a.Edge, b.Edge}} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		return false
	})
	return res
}

// jenkinsVar is a shared library's global variable: the vars/<name>.groovy
// file and its call, the definition a step call `name(...)` runs.
type jenkinsVar struct {
	File string
	Call core.SymbolNode
}

// jenkinsVars indexes the repository's vars/<name>.groovy files by name, each
// with its first `call` (a variable without one is no step); a name two vars/
// directories declare keeps both, and a call to it resolves to neither.
func jenkinsVars(jfs []jenkinsFile) map[string][]jenkinsVar {
	vars := map[string][]jenkinsVar{}
	for _, jf := range jfs {
		if !isJenkinsVarsFile(jf.Path) {
			continue
		}
		var call *core.SymbolNode
		for i := range jf.Nodes {
			n := jf.Nodes[i]
			if n.Name == "call" && string(n.Kind) == "JenkinsFunction" && (call == nil || n.Line < call.Line) {
				call = &jf.Nodes[i]
			}
		}
		if call != nil {
			name := strings.TrimSuffix(path.Base(jf.Path), ".groovy")
			vars[name] = append(vars[name], jenkinsVar{jf.Path, *call})
		}
	}
	return vars
}

// innermostNode is the smallest definition whose lines hold line (ties: the
// later start).
func innermostNode(nodes []core.SymbolNode, line int) (core.SymbolNode, bool) {
	best := -1
	for i, n := range nodes {
		end := n.EndLine
		if end < n.Line {
			end = n.Line
		}
		if line < n.Line || line > end {
			continue
		}
		if best < 0 {
			best = i
			continue
		}
		b := nodes[best]
		bEnd := b.EndLine
		if bEnd < b.Line {
			bEnd = b.Line
		}
		if end-n.Line < bEnd-b.Line || (end-n.Line == bEnd-b.Line && n.Line > b.Line) {
			best = i
		}
	}
	if best < 0 {
		return core.SymbolNode{}, false
	}
	return nodes[best], true
}

// jenkinsXrefs is the doc-xref pass's Jenkins hook: every file the
// jenkinsfile dialect reads is extracted through ingest's gates and dispatch,
// and its shell steps, vars calls and build jobs are resolved (resolveJenkins).
// The unresolved counts are disclosed on the pass's diagnostic line.
func (p *Pipeline) jenkinsXrefs(ctx context.Context, svc core.ServiceRef, files []DocFile, refs []core.SourceRef, idx *siblingIndex) []DocXref {
	byPath := make(map[string]core.SourceRef, len(refs))
	paths := make(map[string]bool, len(refs))
	var cands []core.SourceRef
	for _, r := range refs {
		byPath[r.Path] = r
		paths[r.Path] = true
		if isJenkinsXrefInput(r.Path) {
			cands = append(cands, r)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	read := func(file string) ([]byte, bool) {
		r, ok := byPath[file]
		if !ok {
			return nil, false
		}
		content, err := p.conn.Fetch(ctx, r)
		if err != nil || len(content) > MaxExtractSize || connector.IsBinary(content) || gateDialectFile(file, content).excluded != "" {
			return nil, false
		}
		return content, true
	}
	extract := func(file string) []core.SymbolNode {
		content, ok := read(file)
		if !ok {
			return nil
		}
		e, _, ok, _ := p.resolveExtractor(file, content, idx)
		if !ok {
			return nil
		}
		nodes, err := e.Extract(svc.ID, file, content)
		if err != nil {
			return nil
		}
		return nodes
	}
	var jfs []jenkinsFile
	for _, r := range cands {
		content, ok := read(r.Path)
		if !ok || connector.ClassifyDialect(dialectRules(), r.Path, content).Dialect != "jenkinsfile" {
			continue
		}
		e, _, ok, _ := p.resolveExtractor(r.Path, content, idx)
		if !ok {
			continue
		}
		nodes, err := e.Extract(svc.ID, r.Path, content)
		if err != nil {
			continue
		}
		jfs = append(jfs, jenkinsFile{Path: r.Path, Content: content, Nodes: nodes})
	}
	res := resolveJenkins(jfs, files, paths, extract)
	if len(jfs) > 0 {
		parts := []string{"none"}
		for _, k := range []string{jenkinsShellOutside, jenkinsJobUndeclared, jenkinsJobTemplated, jenkinsVarAmbiguous} {
			if n := res.Unresolved[k]; n > 0 {
				if parts[0] == "none" {
					parts = parts[:0]
				}
				parts = append(parts, k+" "+strconv.Itoa(n))
			}
		}
		p.diagInfo("fusion: doc-xref pass %q: jenkins: %d files, %d shell steps, %d build steps, %d references; unresolved: %s\n",
			svc.ID, len(jfs), res.Shells, res.Jobs, len(res.Xrefs), strings.Join(parts, ", "))
	}
	return res.Xrefs
}

// ── the step scanner ─────────────────────────────────────────────────────

// gtok is one Groovy token: an identifier ('i'), a string ('s'), a number
// ('d'), a punctuation character ('p') or a line break ('n').
type gtok struct {
	kind byte
	text string // an identifier's or punctuation's text
	val  string // a string's value: escapes applied, each interpolation kept as written
	tmpl bool   // the string interpolates (${...} or $name)
	line int
}

// groovyLex splits src into tokens; comments and whitespace are dropped, a
// string is one token whatever it spans.
func groovyLex(src []byte) []gtok {
	var out []gtok
	line, i, n := 1, 0, len(src)
	if strings.HasPrefix(string(src[:min(n, 2)]), "#!") {
		for i < n && src[i] != '\n' {
			i++
		}
	}
	slashyOK := func() bool {
		if len(out) == 0 {
			return true
		}
		t := out[len(out)-1]
		switch t.kind {
		case 'n':
			return true
		case 'p':
			return strings.Contains("([{,=:;!&|?~+-*%<>", t.text)
		case 'i':
			return t.text == "return"
		}
		return false
	}
	for i < n {
		c := src[i]
		switch {
		case c == '\n':
			out = append(out, gtok{kind: 'n', line: line})
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f':
			i++
		case c == '\\' && i+1 < n && src[i+1] == '\n':
			line++
			i += 2
		case c == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			i += 2
			for i < n && !(src[i] == '*' && i+1 < n && src[i+1] == '/') {
				if src[i] == '\n' {
					line++
				}
				i++
			}
			i += 2
		case c == '\'' || c == '"':
			start := line
			var t gtok
			t, i, line = lexString(src, i, line)
			t.line = start
			out = append(out, t)
		case c == '$' && i+1 < n && src[i+1] == '/':
			// a dollar-slashy string, $/ ... /$
			start := line
			i += 2
			for i < n && !(src[i] == '/' && i+1 < n && src[i+1] == '$') {
				if src[i] == '\n' {
					line++
				}
				i++
			}
			i += 2
			out = append(out, gtok{kind: 's', line: start, tmpl: true})
		case c == '/' && slashyOK():
			// a slashy string (a regular expression), / ... /
			start := line
			i++
			for i < n && src[i] != '/' {
				if src[i] == '\\' && i+1 < n {
					i++
				}
				if src[i] == '\n' {
					line++
				}
				i++
			}
			i++
			out = append(out, gtok{kind: 's', line: start, tmpl: true})
		case isGroovyIdentStart(c):
			j := i + 1
			for j < n && isGroovyIdentPart(src[j]) {
				j++
			}
			out = append(out, gtok{kind: 'i', text: string(src[i:j]), line: line})
			i = j
		case c >= '0' && c <= '9':
			j := i + 1
			for j < n && (isGroovyIdentPart(src[j]) || src[j] == '.') {
				j++
			}
			out = append(out, gtok{kind: 'd', text: string(src[i:j]), line: line})
			i = j
		default:
			out = append(out, gtok{kind: 'p', text: string(c), line: line})
			i++
		}
	}
	return out
}

func isGroovyIdentStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isGroovyIdentPart(c byte) bool { return isGroovyIdentStart(c) || (c >= '0' && c <= '9') }

// lexString reads the string starting at src[i] (a quote or a double quote,
// single or tripled): its value with escapes applied and, in a double-quoted
// string, each ${...} or $name kept as written. A single-line string ends at
// a line break it never closed.
func lexString(src []byte, i, line int) (gtok, int, int) {
	n := len(src)
	q := src[i]
	triple := i+2 < n && src[i+1] == q && src[i+2] == q
	gstring := q == '"'
	if triple {
		i += 3
	} else {
		i++
	}
	var b strings.Builder
	t := gtok{kind: 's'}
	for i < n {
		c := src[i]
		switch {
		case triple && c == q && i+2 < n && src[i+1] == q && src[i+2] == q:
			t.val = b.String()
			return t, i + 3, line
		case !triple && c == q:
			t.val = b.String()
			return t, i + 1, line
		case !triple && c == '\n':
			t.val = b.String()
			return t, i, line
		case c == '\\' && i+1 < n:
			e := src[i+1]
			i += 2
			switch e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case '\n':
				line++ // a line continuation
			case 'u':
				if i+4 <= n {
					if r, err := strconv.ParseUint(string(src[i:i+4]), 16, 32); err == nil {
						b.WriteRune(rune(r))
						i += 4
						continue
					}
				}
				b.WriteString("\\u")
			default:
				b.WriteByte(e)
			}
		case gstring && c == '$' && i+1 < n && src[i+1] == '{':
			j, depth := i+2, 1
			for j < n && depth > 0 {
				switch src[j] {
				case '{':
					depth++
				case '}':
					depth--
				case '\n':
					line++
				}
				j++
			}
			b.Write(src[i:j])
			t.tmpl = true
			i = j
		case gstring && c == '$' && i+1 < n && isGroovyIdentStart(src[i+1]) && src[i+1] != '$':
			j := i + 1
			for j < n && (isGroovyIdentPart(src[j]) || (src[j] == '.' && j+1 < n && isGroovyIdentStart(src[j+1]))) && src[j] != '$' {
				j++
			}
			b.Write(src[i:j])
			t.tmpl = true
			i = j
		default:
			if c == '\n' {
				line++
			}
			r, size := utf8.DecodeRune(src[i:])
			b.WriteRune(r)
			i += size
		}
	}
	t.val = b.String()
	return t, i, line
}

// gargs is a call's argument list: positional single-token values and named
// single-token values (an argument that is an expression has no token).
type garg struct {
	key string
	tok *gtok
}

// callArgs reads the arguments of the call whose name is toks[i]: a
// parenthesised list, or a command expression's list on the same line. ok is
// false when toks[i] is not called (a map key, a variable, a receiver's
// member). end is the index after the call's arguments.
func callArgs(toks []gtok, i int) (args []garg, end int, ok bool) {
	if i > 0 && toks[i-1].kind == 'p' && (toks[i-1].text == "." || toks[i-1].text == "&") {
		return nil, i + 1, false // a receiver's member: x.sh, x?.sh, x*.sh
	}
	if i+1 >= len(toks) {
		return nil, i + 1, false
	}
	next := toks[i+1]
	if next.kind == 'p' && next.text == "(" {
		return splitArgs(toks, i+2, ")")
	}
	switch {
	case next.kind == 's':
	case next.kind == 'i' && i+2 < len(toks) && toks[i+2].kind == 'p' && toks[i+2].text == ":":
	default:
		return nil, i + 1, false
	}
	return splitArgs(toks, i+1, "")
}

// splitArgs reads arguments from toks[j] to the closing token (")" for a
// parenthesised list; "" for a command expression, which ends at a line break
// not after a comma, a ';', a '{' or a '}').
func splitArgs(toks []gtok, j int, closer string) ([]garg, int, bool) {
	var args []garg
	depth := 0
	cur := []int{}
	flush := func() {
		var a garg
		k := cur
		if len(k) >= 2 && toks[k[0]].kind == 'i' && toks[k[1]].kind == 'p' && toks[k[1]].text == ":" {
			a.key = toks[k[0]].text
			k = k[2:]
		}
		if len(k) == 1 {
			t := toks[k[0]]
			a.tok = &t
		}
		if len(cur) > 0 {
			args = append(args, a)
		}
		cur = cur[:0]
	}
	for ; j < len(toks); j++ {
		t := toks[j]
		if t.kind == 'p' {
			switch t.text {
			case "(", "[":
				depth++
			case ")", "]":
				if depth == 0 && closer == ")" && t.text == ")" {
					flush()
					return args, j + 1, true
				}
				depth--
			case "{":
				if depth == 0 && closer == "" {
					flush()
					return args, j, true
				}
				depth++
			case "}":
				if depth == 0 {
					flush()
					return args, j, closer == ""
				}
				depth--
			case ",":
				if depth == 0 {
					flush()
					continue
				}
			case ";":
				if depth == 0 && closer == "" {
					flush()
					return args, j, true
				}
			}
		}
		if t.kind == 'n' && depth == 0 && closer == "" {
			if len(cur) == 0 && len(args) > 0 {
				continue // a line break after a comma
			}
			flush()
			return args, j, true
		}
		if t.kind == 'n' {
			continue
		}
		cur = append(cur, j)
	}
	flush()
	return args, j, closer == ""
}

// argString is the string a call passes as key (key "": the first
// positional argument).
func argString(args []garg, key string) (gtok, bool) {
	for _, a := range args {
		if a.key == key && a.tok != nil && a.tok.kind == 's' {
			return *a.tok, true
		}
		if key == "" && a.key == "" {
			break
		}
	}
	return gtok{}, false
}

// jenkinsSteps reads a pipeline's literal shell steps, each under its
// enclosing dir() blocks, and its build steps' jobs.
func jenkinsSteps(src []byte) ([]jenkinsShell, []jenkinsJob) {
	toks := groovyLex(src)
	dirs := []string{"."}
	pending, hasPending := "", false
	var shells []jenkinsShell
	var jobs []jenkinsJob
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch {
		case t.kind == 'p' && t.text == "{":
			d := dirs[len(dirs)-1]
			if hasPending {
				d, hasPending = pending, false
			}
			dirs = append(dirs, d)
		case t.kind == 'p' && t.text == "}":
			if len(dirs) > 1 {
				dirs = dirs[:len(dirs)-1]
			}
		case t.kind == 'i' && (jenkinsShellSteps[t.text] || t.text == "dir" || t.text == "build"):
			args, end, ok := callArgs(toks, i)
			if !ok {
				continue
			}
			cwd := dirs[len(dirs)-1]
			switch t.text {
			case "dir":
				s, ok := argString(args, "")
				if !ok {
					s, ok = argString(args, "path")
				}
				if end < len(toks) && toks[end].kind == 'p' && toks[end].text == "{" {
					pending, hasPending = "?", true
					if ok {
						pending = joinRunDir(cwd, s.val)
					}
				}
			case "build":
				s, ok := argString(args, "job")
				if !ok {
					s, ok = argString(args, "")
				}
				if ok {
					jobs = append(jobs, jenkinsJob{Line: t.line, Name: s.val, Templated: s.tmpl})
				}
			default:
				s, ok := argString(args, "script")
				if !ok {
					s, ok = argString(args, "")
				}
				if ok {
					shells = append(shells, jenkinsShell{Line: t.line, Step: t.text, Dir: cwd, Script: s.val})
				}
			}
			i = end - 1
		}
	}
	return shells, jobs
}
