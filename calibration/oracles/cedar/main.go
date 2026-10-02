// Command cedar-oracle is the language-breadth harness's tier-A oracle for
// Cedar (design §1.3: the language's own parser, the reference
// implementation's CLI, cedar-policy-cli). It writes indexbench lang-oracle's
// tool-oracle JSON (-tool-oracle, design §10):
//
//	go run . -subject <checkout> -cedar <cedar> [-files <list>] > cedar-<subject>.json
//
// Every .cedar file of the subject (or of -files, one subject-relative path
// per line) is checked by `cedar check-parse --policies` and translated by
// `cedar translate-policy --direction cedar-to-json`; every .cedarschema file
// by `cedar check-parse --schema` and `cedar translate-schema --direction
// cedar-to-json-with-resolved-types`. A file the CLI rejects is named on
// stderr and contributes nothing.
//
// What it writes:
//
//   - definitions: per policy file, each static policy (kind policy) and each
//     template (kind template), named by the id the CLI gives it: its @id
//     annotation, else policy<i> by its place in the file; per schema file,
//     each namespace (kind namespace), entity type (entity), action (action,
//     named by its id) and common type (type), entity and common types
//     qualified by their namespace (PhotoApp::User).
//   - calls, a schema's own references (L2): an entity type to each parent
//     type and each entity or common type its shape and tags name; a common
//     type to each type it names; an action to each principal and resource
//     type it applies to, each type its context names, and each parent action
//     (by id). Types are as the CLI resolves them; primitives and extension
//     types are not written.
//   - cross_references, the governed scope: a policy's principal and
//     resource (`is T`, `== T::"x"`, `in T::"x"`) and action (`==
//     Action::"a"`, `in [...]`) to each schema file of the subject that
//     declares T, or the action a.
//
// Neither JSON form carries a source position, so the writer locates lines in
// the source with a statement scan of its own (comments and strings
// skipped): the i-th policy statement, from its first annotation to its `;`,
// is the CLI's policy of that id (checked: the ids the scan derives must be
// the CLI's, else the file is named and skipped); a schema declaration
// (`namespace X {…}`, `entity A, B …;`, `action "a", b …;`, `type T = …;`)
// gives each name it declares its lines (checked the same way). Names, kinds
// and references are the CLI's; lines are the writer's, and a reading says
// so.
//
// Bench-only; never a product dependency.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	End  int    `json:"end"`
	Kind string `json:"kind"`
}

type pair struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

type xref struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
	To     string `json:"to,omitempty"`
}

type output struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	Calls           []pair `json:"calls"`
	CrossReferences []xref `json:"cross_references"`
}

// scope is one policy's governed scope: the entity types and the actions
// (namespace::id) it names.
type scope struct {
	path, from string
	types      []string
	actions    []string
}

func main() {
	subject := flag.String("subject", "", "subject checkout (required)")
	cedar := flag.String("cedar", "cedar", "cedar binary (cedar-policy-cli)")
	list := flag.String("files", "", "file of subject-relative .cedar and .cedarschema paths (default: every such file)")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: cedar-oracle -subject <checkout> [-cedar <bin>] [-files <list>]")
		os.Exit(2)
	}
	root, err := filepath.Abs(*subject)
	if err != nil {
		fail(err)
	}
	files, err := cedarFiles(root, *list)
	if err != nil {
		fail(err)
	}
	vout, err := exec.Command(*cedar, "--version").Output()
	if err != nil {
		fail(err)
	}
	version := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(vout)), "cedar-policy-cli"))

	res := output{Tool: "cedar-policy-cli", Version: version, Tier: "A", Definitions: []def{}, Calls: []pair{}, CrossReferences: []xref{}}
	var scopes []scope
	entityFiles := map[string][]string{} // qualified entity type -> schema files
	actionFiles := map[string][]string{} // namespace::id -> schema files
	skipped := 0
	for _, f := range files {
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			fail(err)
		}
		if strings.HasSuffix(f, ".cedarschema") {
			if err := run(*cedar, root, nil, "check-parse", "--schema", f); err != nil {
				skipped++
				fmt.Fprintf(os.Stderr, "cedar rejected %s: %v\n", f, err)
				continue
			}
			var out bytes.Buffer
			if err := run(*cedar, root, &out, "translate-schema", "--direction", "cedar-to-json-with-resolved-types", "-s", f); err != nil {
				fail(fmt.Errorf("%s: %w", f, err))
			}
			defs, calls, ents, acts, err := schemaFacts(f, src, out.Bytes())
			if err != nil {
				skipped++
				fmt.Fprintf(os.Stderr, "skipped %s: %v\n", f, err)
				continue
			}
			res.Definitions = append(res.Definitions, defs...)
			res.Calls = append(res.Calls, calls...)
			for _, e := range ents {
				entityFiles[e] = append(entityFiles[e], f)
			}
			for _, a := range acts {
				actionFiles[a] = append(actionFiles[a], f)
			}
			continue
		}
		if err := run(*cedar, root, nil, "check-parse", "--policies", f); err != nil {
			skipped++
			fmt.Fprintf(os.Stderr, "cedar rejected %s: %v\n", f, err)
			continue
		}
		var out bytes.Buffer
		if err := run(*cedar, root, &out, "translate-policy", "--direction", "cedar-to-json", "-p", f); err != nil {
			fail(fmt.Errorf("%s: %w", f, err))
		}
		pols, err := policyFacts(f, src, out.Bytes())
		if err != nil {
			skipped++
			fmt.Fprintf(os.Stderr, "skipped %s: %v\n", f, err)
			continue
		}
		for _, p := range pols {
			res.Definitions = append(res.Definitions, p.def)
			scopes = append(scopes, scope{path: f, from: p.def.Name, types: p.types, actions: p.actions})
		}
	}
	for _, s := range scopes {
		for _, t := range s.types {
			for _, to := range entityFiles[t] {
				res.CrossReferences = append(res.CrossReferences, xref{Path: s.path, From: s.from, ToPath: to, To: t})
			}
		}
		for _, a := range s.actions {
			for _, to := range actionFiles[a] {
				res.CrossReferences = append(res.CrossReferences, xref{Path: s.path, From: s.from, ToPath: to, To: a[strings.LastIndex(a, "::")+2:]})
			}
		}
	}
	fmt.Fprintf(os.Stderr, "cedar %s: %d files, %d read, %d rejected or skipped\n", version, len(files), len(files)-skipped, skipped)
	sortAll(&res)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(res); err != nil {
		fail(err)
	}
}

func run(bin, dir string, stdout *bytes.Buffer, args ...string) error {
	cmd := exec.Command(bin, append([]string{"--error-format", "plain"}, args...)...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if stdout != nil {
		cmd.Stdout = stdout
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, firstLine(stderr.String()))
	}
	return nil
}

// ── the source scan: lines only ──

type tok struct {
	text      string
	str       bool // a string literal (text is its value)
	line, end int
}

func isIdentByte(c byte, first bool) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || !first && c >= '0' && c <= '9'
}

func lex(src []byte) []tok {
	var out []tok
	line := 1
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '"':
			j, start := i+1, line
			for j < len(src) && src[j] != '"' {
				if src[j] == '\\' {
					j++
				} else if src[j] == '\n' {
					line++
				}
				j++
			}
			v, err := strconv.Unquote(string(src[i:min(j+1, len(src))]))
			if err != nil {
				v = string(src[i+1 : min(j, len(src))])
			}
			out = append(out, tok{text: v, str: true, line: start, end: line})
			i = j + 1
		case isIdentByte(c, true):
			j := i + 1
			for j < len(src) && isIdentByte(src[j], false) {
				j++
			}
			out = append(out, tok{text: string(src[i:j]), line: line, end: line})
			i = j
		case c == ':' && i+1 < len(src) && src[i+1] == ':':
			out = append(out, tok{text: "::", line: line, end: line})
			i += 2
		default:
			out = append(out, tok{text: string(c), line: line, end: line})
			i++
		}
	}
	return out
}

func (t tok) is(s string) bool { return !t.str && t.text == s }

func depthOf(t tok, d int) int {
	switch {
	case t.is("{") || t.is("(") || t.is("["):
		return d + 1
	case t.is("}") || t.is(")") || t.is("]"):
		return d - 1
	}
	return d
}

type located struct {
	name      string
	line, end int
}

// policyStatements is each policy statement's id (its @id annotation, else
// policy<i>) and lines, a statement ending at its `;` at depth 0.
func policyStatements(src []byte) []located {
	ts := lex(src)
	var out []located
	start, depth, id := -1, 0, ""
	for k := 0; k < len(ts); k++ {
		t := ts[k]
		if start < 0 {
			start, id = k, ""
		}
		if depth == 0 && t.is("@") && k+3 < len(ts) && ts[k+1].text == "id" && ts[k+2].is("(") && ts[k+3].str {
			id = ts[k+3].text
		}
		depth = depthOf(t, depth)
		if depth == 0 && t.is(";") {
			name := id
			if name == "" {
				name = "policy" + strconv.Itoa(len(out))
			}
			out = append(out, located{name: name, line: ts[start].line, end: t.end})
			start = -1
		}
	}
	return out
}

// schemaDecls is each declared name's lines: namespaces, entity and common
// types qualified by their namespace, actions as namespace::id.
type schemaDecls struct {
	ns, ents, types, acts map[string]located
}

func scanSchema(src []byte) schemaDecls {
	ts := lex(src)
	d := schemaDecls{ns: map[string]located{}, ents: map[string]located{}, types: map[string]located{}, acts: map[string]located{}}
	var walk func(lo, hi int, ns string)
	walk = func(lo, hi int, ns string) {
		q := func(n string) string {
			if ns == "" {
				return n
			}
			return ns + "::" + n
		}
		k := lo
		for k < hi {
			for k < hi && ts[k].is("@") { // annotations: @name or @name("…")
				k += 2
				if k < hi && ts[k].is("(") {
					for k < hi && !ts[k].is(")") {
						k++
					}
					k++
				}
			}
			if k >= hi {
				return
			}
			kw := ts[k]
			end, depth := k, 0
			for end < hi {
				depth = depthOf(ts[end], depth)
				if depth == 0 && (ts[end].is(";") || kw.is("namespace") && ts[end].is("}")) {
					break
				}
				end++
			}
			if end >= hi {
				end = hi - 1
			}
			names := func() []string {
				var out []string
				for j := k + 1; j < end; j++ {
					t := ts[j]
					if t.is(",") {
						continue
					}
					if !t.str && (t.text == "in" || t.text == "appliesTo" || t.text == "enum" || t.text == "tags" || t.text == "attributes" || t.text == "=" || t.text == "{") {
						break
					}
					out = append(out, t.text)
				}
				return out
			}
			at := located{line: kw.line, end: ts[end].end}
			switch {
			case kw.is("namespace"):
				var path []string
				j := k + 1
				for ; j < end && !ts[j].is("{"); j++ {
					if !ts[j].is("::") {
						path = append(path, ts[j].text)
					}
				}
				name := strings.Join(path, "::")
				at.name = name
				d.ns[name] = at
				walk(j+1, end, name)
			case kw.is("entity"):
				for _, n := range names() {
					at.name = q(n)
					d.ents[at.name] = at
				}
			case kw.is("type") && k+1 < end:
				at.name = q(ts[k+1].text)
				d.types[at.name] = at
			case kw.is("action"):
				for _, n := range names() {
					at.name = ns + "::" + n
					d.acts[at.name] = at
				}
			}
			k = end + 1
		}
	}
	walk(0, len(ts), "")
	return d
}

// ── the CLI's facts ──

type policy struct {
	def     def
	types   []string
	actions []string
}

func policyFacts(path string, src, js []byte) ([]policy, error) {
	var set struct {
		Static    map[string]map[string]any `json:"staticPolicies"`
		Templates map[string]map[string]any `json:"templates"`
	}
	if err := json.Unmarshal(js, &set); err != nil {
		return nil, err
	}
	stmts := policyStatements(src)
	if len(stmts) != len(set.Static)+len(set.Templates) {
		return nil, fmt.Errorf("%d statements scanned, the CLI parsed %d policies", len(stmts), len(set.Static)+len(set.Templates))
	}
	var out []policy
	for _, s := range stmts {
		p, kind := set.Static[s.name], "policy"
		if p == nil {
			p, kind = set.Templates[s.name], "template"
		}
		if p == nil {
			return nil, fmt.Errorf("the statement at line %d scans as %q, an id the CLI does not give", s.line, s.name)
		}
		pol := policy{def: def{Path: path, Name: s.name, Line: s.line, End: s.end, Kind: kind}}
		for _, part := range []string{"principal", "resource"} {
			c, _ := p[part].(map[string]any)
			if t, ok := c["entity_type"].(string); ok {
				pol.types = append(pol.types, t)
			}
			for _, e := range scopeEntities(c) {
				pol.types = append(pol.types, e[0])
			}
		}
		a, _ := p["action"].(map[string]any)
		for _, e := range scopeEntities(a) {
			ns := strings.TrimSuffix(strings.TrimSuffix(e[0], "Action"), "::")
			pol.actions = append(pol.actions, ns+"::"+e[1])
		}
		out = append(out, pol)
	}
	return out, nil
}

// scopeEntities is a scope constraint's entity literals, as (type, id).
func scopeEntities(c map[string]any) [][2]string {
	var out [][2]string
	add := func(v any) {
		m, _ := v.(map[string]any)
		t, _ := m["type"].(string)
		id, _ := m["id"].(string)
		if t != "" {
			out = append(out, [2]string{t, id})
		}
	}
	add(c["entity"])
	if es, ok := c["entities"].([]any); ok {
		for _, e := range es {
			add(e)
		}
	}
	if in, ok := c["in"].(map[string]any); ok {
		add(in["entity"])
	}
	return out
}

type schemaJSON map[string]struct {
	EntityTypes map[string]map[string]any `json:"entityTypes"`
	Actions     map[string]map[string]any `json:"actions"`
	CommonTypes map[string]any            `json:"commonTypes"`
}

func schemaFacts(path string, src, js []byte) (defs []def, calls []pair, entities, actions []string, err error) {
	var schema schemaJSON
	if err := json.Unmarshal(js, &schema); err != nil {
		return nil, nil, nil, nil, err
	}
	d := scanSchema(src)
	q := func(ns, n string) string {
		if ns == "" {
			return n
		}
		return ns + "::" + n
	}
	call := func(from string, tos ...string) {
		for _, to := range tos {
			calls = append(calls, pair{Path: path, From: from, To: to})
		}
	}
	for ns, body := range schema {
		if ns != "" {
			l, ok := d.ns[ns]
			if !ok {
				return nil, nil, nil, nil, fmt.Errorf("namespace %s not found by the scan", ns)
			}
			defs = append(defs, def{Path: path, Name: ns, Line: l.line, End: l.end, Kind: "namespace"})
		}
		for n, e := range body.EntityTypes {
			name := q(ns, n)
			l, ok := d.ents[name]
			if !ok {
				return nil, nil, nil, nil, fmt.Errorf("entity %s not found by the scan", name)
			}
			defs = append(defs, def{Path: path, Name: name, Line: l.line, End: l.end, Kind: "entity"})
			entities = append(entities, name)
			call(name, strs(e["memberOfTypes"])...)
			call(name, typeNames(e["shape"])...)
			call(name, typeNames(e["tags"])...)
		}
		for n, t := range body.CommonTypes {
			name := q(ns, n)
			l, ok := d.types[name]
			if !ok {
				return nil, nil, nil, nil, fmt.Errorf("type %s not found by the scan", name)
			}
			defs = append(defs, def{Path: path, Name: name, Line: l.line, End: l.end, Kind: "type"})
			call(name, typeNames(t)...)
		}
		for n, a := range body.Actions {
			l, ok := d.acts[ns+"::"+n]
			if !ok {
				return nil, nil, nil, nil, fmt.Errorf("action %s::%s not found by the scan", ns, n)
			}
			defs = append(defs, def{Path: path, Name: n, Line: l.line, End: l.end, Kind: "action"})
			actions = append(actions, ns+"::"+n)
			if ap, ok := a["appliesTo"].(map[string]any); ok {
				call(n, strs(ap["principalTypes"])...)
				call(n, strs(ap["resourceTypes"])...)
				call(n, typeNames(ap["context"])...)
			}
			if ms, ok := a["memberOf"].([]any); ok {
				for _, m := range ms {
					mm, _ := m.(map[string]any)
					if id, ok := mm["id"].(string); ok {
						call(n, id)
					}
				}
			}
		}
	}
	return defs, calls, entities, actions, nil
}

func strs(v any) []string {
	var out []string
	if xs, ok := v.([]any); ok {
		for _, x := range xs {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// typeNames is every entity or common type a resolved type names.
func typeNames(v any) []string {
	var out []string
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if t, _ := x["type"].(string); t == "Entity" || t == "EntityOrCommon" || t == "CommonType" {
				if n, ok := x["name"].(string); ok {
					out = append(out, n)
				}
			}
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(x[k])
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(v)
	return out
}

func cedarFiles(root, list string) ([]string, error) {
	var out []string
	if list != "" {
		f, err := os.Open(list)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if l := strings.TrimSpace(sc.Text()); l != "" {
				out = append(out, l)
			}
		}
		sort.Strings(out)
		return out, sc.Err()
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() && (strings.HasSuffix(d.Name(), ".cedar") || strings.HasSuffix(d.Name(), ".cedarschema")) {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func sortAll(o *output) {
	sort.Slice(o.Definitions, func(i, j int) bool {
		a, b := o.Definitions[i], o.Definitions[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	sort.Slice(o.Calls, func(i, j int) bool {
		a, b := o.Calls[i], o.Calls[j]
		return a.Path+"\x00"+a.From+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.To
	})
	calls := o.Calls[:0]
	for i, c := range o.Calls {
		if i == 0 || c != o.Calls[i-1] {
			calls = append(calls, c)
		}
	}
	o.Calls = calls
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To
	})
	xs := o.CrossReferences[:0]
	for i, x := range o.CrossReferences {
		if i == 0 || x != o.CrossReferences[i-1] {
			xs = append(xs, x)
		}
	}
	o.CrossReferences = xs
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "cedar-oracle:", err)
	os.Exit(1)
}
