// Command opa-parse is the language-breadth harness's tier-A oracle for Rego
// (design §1.3: the language's reference parser). It writes indexbench
// lang-oracle's tool-oracle JSON (-tool-oracle, design §10):
//
//	go run . -subject <checkout> [-opa <opa>] [-files <list>] > rego-<subject>.json
//
// Every .rego file of the subject (or of -files, one subject-relative path per
// line: the file list the harness measured) is parsed on its own by `opa parse
// --format json --json-include locations,-comments <file>` (Rego v1 syntax, OPA's
// default), and the AST OPA prints is decoded. A file OPA rejects is counted and
// named on stderr and contributes nothing: it is not Rego the reference parser
// accepts.
//
// What it writes, every line from OPA's own locations (row; a rule's last line is
// its first plus the lines of the source text OPA records for it):
//
//   - definitions: the file's `package` (named by its path without `data.`, at
//     the package clause), and one definition per rule OPA parsed — a `function`
//     when its head has arguments, else a `rule` (complete, partial set, partial
//     object, default and test rules alike). A rule is named by its head's name,
//     or, for a ref head OPA gives no name (`a.b.c := 1`), by the head's static
//     path joined with `.`. Several bodies sharing a name are several rules in
//     OPA's AST, each its own definition at its own line; an `else` chain belongs
//     to the rule it follows and extends its range.
//   - calls, attributed to the rule whose head, body or else chain holds them:
//     every call OPA parsed whose operator is written as its own name (`count(x)`,
//     `object.get(o, k, d)`, `utils.f(x)`, `data.lib.f(x)`), named by the last
//     segment of that name; an infix operator (`==`, `:=`, `+`, `in`) is a call
//     in OPA's AST but is not written as one and is not a call here. Also every
//     reference rooted at `data` that is not a call's operator (a rule reference
//     spelled in full, `data.lib.roles.admins`), named by its last static segment.
//     A bare name that refers to a rule of the same package, or a reference
//     through an import alias, is not a call: telling it from a local variable or
//     an input path needs OPA's compiler, not its parser.
//   - imports: every `import data.x.y [as z]`, at its line, to each file of the
//     subject that declares `package x.y`, or else declares `package x` and a rule
//     named y (the files the import's path names); `import input…`, `import
//     rego.v1` and `import future…` name no file and are not written.
//
// Bench-only; never a product dependency.
package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

type imp struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	ToPath string `json:"to_path"`
	Module string `json:"module"`
}

type output struct {
	Tool        string `json:"tool"`
	Version     string `json:"version"`
	Tier        string `json:"tier"`
	Definitions []def  `json:"definitions"`
	Calls       []pair `json:"calls"`
	Imports     []imp  `json:"imports"`
}

type location struct {
	Row  int    `json:"row"`
	Text string `json:"text"`
}

type term struct {
	Location *location       `json:"location"`
	Type     string          `json:"type"`
	Value    json.RawMessage `json:"value"`
}

type module struct {
	Package struct {
		Location *location `json:"location"`
		Path     []term    `json:"path"`
	} `json:"package"`
	Imports []struct {
		Location *location `json:"location"`
		Path     term      `json:"path"`
		Alias    string    `json:"alias"`
	} `json:"imports"`
	Rules []json.RawMessage `json:"rules"`
}

type rule struct {
	Location *location `json:"location"`
	Head     struct {
		Name string          `json:"name"`
		Ref  []term          `json:"ref"`
		Args json.RawMessage `json:"args"`
	} `json:"head"`
}

type parsed struct {
	path  string
	pkg   string
	mod   module
	rules map[string]bool // rule names the file defines
}

func main() {
	subject := flag.String("subject", "", "subject checkout (required)")
	opa := flag.String("opa", "opa", "opa binary")
	list := flag.String("files", "", "file of subject-relative .rego paths to parse (default: every .rego file)")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: opa-parse -subject <checkout> [-opa <bin>] [-files <list>]")
		os.Exit(2)
	}
	root, err := filepath.Abs(*subject)
	if err != nil {
		fail(err)
	}
	files, err := regoFiles(root, *list)
	if err != nil {
		fail(err)
	}
	vout, err := exec.Command(*opa, "version").Output()
	if err != nil {
		fail(err)
	}
	version := strings.TrimSpace(strings.TrimPrefix(strings.SplitN(string(vout), "\n", 2)[0], "Version:"))

	var mods []parsed
	rejected := 0
	for _, f := range files {
		cmd := exec.Command(*opa, "parse", "--format", "json", "--json-include", "locations,-comments", f)
		cmd.Dir = root
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			rejected++
			fmt.Fprintf(os.Stderr, "opa rejected %s: %s\n", f, firstLine(stderr.String()))
			continue
		}
		var m module
		if err := json.Unmarshal(stdout.Bytes(), &m); err != nil {
			fail(fmt.Errorf("%s: %w", f, err))
		}
		p := parsed{path: f, mod: m, rules: map[string]bool{}}
		var segs []string
		for i, t := range m.Package.Path {
			if s, ok := segment(t); ok && !(i == 0 && s == "data") {
				segs = append(segs, s)
			}
		}
		p.pkg = strings.Join(segs, ".")
		mods = append(mods, p)
	}
	fmt.Fprintf(os.Stderr, "opa %s: %d files, %d parsed, %d rejected\n", version, len(files), len(files)-rejected, rejected)

	res := output{Tool: "opa parse", Version: version, Tier: "A", Definitions: []def{}, Calls: []pair{}, Imports: []imp{}}
	byPkg := map[string][]*parsed{}
	for i := range mods {
		p := &mods[i]
		if p.pkg != "" && p.mod.Package.Location != nil {
			row := p.mod.Package.Location.Row
			res.Definitions = append(res.Definitions, def{Path: p.path, Name: p.pkg, Line: row, End: row, Kind: "package"})
		}
		byPkg[p.pkg] = append(byPkg[p.pkg], p)
		for _, raw := range p.mod.Rules {
			var r rule
			if err := json.Unmarshal(raw, &r); err != nil {
				fail(fmt.Errorf("%s: %w", p.path, err))
			}
			name := r.Head.Name
			if name == "" {
				var segs []string
				for _, t := range r.Head.Ref {
					s, ok := segment(t)
					if !ok {
						break
					}
					segs = append(segs, s)
				}
				name = strings.Join(segs, ".")
			}
			if name == "" || r.Location == nil {
				continue
			}
			p.rules[name] = true
			kind := "rule"
			var args []json.RawMessage
			if json.Unmarshal(r.Head.Args, &args) == nil && len(args) > 0 {
				kind = "function"
			}
			line := r.Location.Row
			end := line + strings.Count(decode(r.Location.Text), "\n")
			res.Definitions = append(res.Definitions, def{Path: p.path, Name: name, Line: line, End: end, Kind: kind})
			from := name[strings.LastIndex(name, ".")+1:]
			var tree any
			if err := json.Unmarshal(raw, &tree); err != nil {
				fail(err)
			}
			for _, callee := range callees(tree) {
				res.Calls = append(res.Calls, pair{Path: p.path, From: from, To: callee})
			}
		}
	}
	for i := range mods {
		p := &mods[i]
		for _, im := range p.mod.Imports {
			var segs []string
			var head []term
			if json.Unmarshal(im.Path.Value, &head) != nil {
				continue
			}
			for _, t := range head {
				s, ok := segment(t)
				if !ok {
					break
				}
				segs = append(segs, s)
			}
			if len(segs) < 2 || segs[0] != "data" || im.Location == nil {
				continue
			}
			path := strings.Join(segs[1:], ".")
			var targets []string
			for _, q := range byPkg[path] {
				targets = append(targets, q.path)
			}
			if len(targets) == 0 && len(segs) > 2 {
				parent, leaf := strings.Join(segs[1:len(segs)-1], "."), segs[len(segs)-1]
				for _, q := range byPkg[parent] {
					if q.rules[leaf] {
						targets = append(targets, q.path)
					}
				}
			}
			for _, t := range targets {
				if t != p.path {
					res.Imports = append(res.Imports, imp{Path: p.path, Line: im.Location.Row, ToPath: t, Module: strings.Join(segs, ".")})
				}
			}
		}
	}
	sortAll(&res)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(res); err != nil {
		fail(err)
	}
}

// callees walks one rule's AST (head, body, else chain, nested
// comprehensions and every/some bodies) for the calls and data references
// the package comment defines, in walk order, deduplicated by the caller.
func callees(n any) []string {
	var out []string
	operators := map[any]bool{}
	var walk func(n any)
	walk = func(n any) {
		switch v := n.(type) {
		case []any:
			for _, x := range v {
				walk(x)
			}
		case map[string]any:
			// An expression whose terms are a list is a call: its first term
			// is the operator. A term of type "call" is a call written inside
			// another term.
			var op any
			if ts, ok := v["terms"].([]any); ok && len(ts) > 0 {
				op = ts[0]
			}
			if v["type"] == "call" {
				if vs, ok := v["value"].([]any); ok && len(vs) > 0 {
					op = vs[0]
				}
			}
			if om, ok := op.(map[string]any); ok {
				operators[fmt.Sprintf("%p", om)] = true
				if name, ok := writtenName(om); ok {
					out = append(out, name)
				}
			}
			if v["type"] == "ref" && !operators[fmt.Sprintf("%p", v)] {
				if name, ok := dataRef(v); ok {
					out = append(out, name)
				}
			}
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if k != "location" {
					walk(v[k])
				}
			}
		}
	}
	walk(n)
	return out
}

// writtenName is an operator's callee when the source spells the operator as
// its own dotted name (the reference OPA parsed, rendered), else ok is false.
func writtenName(op map[string]any) (string, bool) {
	if op["type"] != "ref" {
		return "", false
	}
	segs, ok := staticSegments(op)
	if !ok || len(segs) == 0 {
		return "", false
	}
	loc, _ := op["location"].(map[string]any)
	text, _ := loc["text"].(string)
	if strings.Join(strings.Fields(decode(text)), "") != strings.Join(segs, ".") {
		return "", false
	}
	return segs[len(segs)-1], true
}

// dataRef is a reference rooted at `data`, named by its last static segment.
func dataRef(ref map[string]any) (string, bool) {
	vs, ok := ref["value"].([]any)
	if !ok || len(vs) < 2 {
		return "", false
	}
	head, _ := vs[0].(map[string]any)
	if head["type"] != "var" || head["value"] != "data" {
		return "", false
	}
	last := ""
	for _, x := range vs[1:] {
		t, _ := x.(map[string]any)
		s, ok := t["value"].(string)
		if t["type"] != "string" || !ok {
			break
		}
		last = s
	}
	return last, last != ""
}

// staticSegments renders a reference whose every segment is a var or a
// string; ok is false for one with a computed segment.
func staticSegments(ref map[string]any) ([]string, bool) {
	vs, ok := ref["value"].([]any)
	if !ok {
		return nil, false
	}
	var out []string
	for _, x := range vs {
		t, _ := x.(map[string]any)
		s, ok := t["value"].(string)
		if !ok || (t["type"] != "var" && t["type"] != "string") {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// segment is a package or head path term's name.
func segment(t term) (string, bool) {
	if t.Type != "var" && t.Type != "string" {
		return "", false
	}
	var s string
	if json.Unmarshal(t.Value, &s) != nil {
		return "", false
	}
	return s, true
}

func decode(s string) string {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	return string(b)
}

// regoFiles is list's paths, or every .rego file under root (sorted; .git
// is not walked).
func regoFiles(root, list string) ([]string, error) {
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
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".rego") {
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
	dedup := func(ps []pair) []pair {
		sort.Slice(ps, func(i, j int) bool {
			a, b := ps[i], ps[j]
			return a.Path+"\x00"+a.From+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.To
		})
		out := ps[:0]
		for i, p := range ps {
			if i == 0 || p != ps[i-1] {
				out = append(out, p)
			}
		}
		return out
	}
	o.Calls = dedup(o.Calls)
	sort.Slice(o.Imports, func(i, j int) bool {
		a, b := o.Imports[i], o.Imports[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.ToPath < b.ToPath
	})
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "opa-parse:", err)
	os.Exit(1)
}
