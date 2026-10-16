// Command helm-oracle is the language-breadth harness's oracle for the helm
// tags+doc dialect (LB-Q-helm, design §1.3): a from-scratch walk of a
// subject's charts that never touches fathom's engines, written in indexbench
// lang-oracle's tool-oracle shape.
//
//   - Templates (*.tpl, templates/**/*.yaml|yml, templates/NOTES.txt) are read
//     by Go's own text/template/parse — the parser Helm renders them with —
//     so a named template, an `include`/`template` call (a macro_ref
//     definition wherever it is, and a reference when a named template makes
//     it) and a `.Values.a.b` accessor are the parse tree's, not a pattern's. A file Go rejects
//     (Helm would fail to load it) contributes nothing.
//   - Chart.yaml and values.yaml are read with gopkg.in/yaml.v3 (aliases
//     followed and merge keys applied, as Helm's loader does). With -helm, each
//     chart is also loaded by the helm binary (`helm show chart`) and its name
//     and dependency names are compared with the walk; the counts go to stderr.
//   - cross_references: a chart's dependencies[].name resolved to the one
//     other chart of that name in the subject (a chart is the dependency of the
//     chart that names it, whether the repository is a file:// path, a URL or
//     a registry — the monorepo convention), file granularity is not used: the
//     target entity is the chart.
//
// The reading records tier C (an independent parser library): the rules are
// this file's, not a tool's evaluation. With -tier-a (wave ox, LB-F154,
// tiera.go) Helm decides instead, for the charts it renders offline: stdout
// carries tier-A definitions and references, -xrefs-out this walk's tier-C
// cross_references over every chart, -files-out the files of the charts
// measured. With -file-deps-out (wave lf8, LB-F458, filedeps.go) each file://
// dependency is graded by `helm dependency list` instead, tier A, into its own
// file; -xrefs-only writes the walk's cross_references alone to stdout. A walk
// whose edges include an oci:// dependency discloses the written L3 ceiling
// (ociCeiling). Bench-only; never a product dependency.
//
//	go run . -subject <checkout> [-helm <helm>] > <out.json>
//	go run . -subject <checkout> -xrefs-only > <xrefs.json>
//	go run . -subject <checkout> -helm <helm> -scratch <dir> -file-deps-out <file-deps.json> -xrefs-only > /dev/null
//	go run . -subject <checkout> -helm <helm> -tier-a -scratch <dir> -xrefs-out <xrefs.json> -files-out <files.txt> > <defs.json>
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template/parse"

	"gopkg.in/yaml.v3"
)

type def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	Kind string `json:"kind"`
}

type ref struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

type xref struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
	To     string `json:"to"`
}

type out struct {
	Tool            string   `json:"tool"`
	Version         string   `json:"version"`
	Tier            string   `json:"tier"`
	Definitions     []def    `json:"definitions"`
	References      []ref    `json:"references"`
	CrossReferences []xref   `json:"cross_references"`
	Disclose        []string `json:"disclose,omitempty"`
}

// ociCeiling is the written L3 ceiling (LB-F458, wave lf8), disclosed by a
// walk whose cross_references include an oci:// dependency: n edges, m of
// them oci://, f file://.
func ociCeiling(n, m, f int) string {
	return fmt.Sprintf("L3 ceiling: %d of this subject's %d dependency edges name oci:// registry charts, which Helm resolves only by fetching them (helm dependency build), so no offline evaluation attests them and they are graded by this walk's unique-chart-name join, tier C, which caps L3 at good (§1.3); its %d file:// edges are attested by helm dependency list (tier A) where the subjects that hold them are graded by -file-deps-out (LB-F458)", m, n, f)
}

// maxFileBytes mirrors ingest's 2 MiB cap.
const maxFileBytes = 2 << 20

// valuesMaxDepth mirrors the incumbent's cap (helmValuesMaxDepth).
const valuesMaxDepth = 8

// skipDirs are the directories the connector never descends into.
var skipDirs = map[string]bool{"vendor": true, ".git": true, "testdata": true, "generated": true, "node_modules": true, ".idea": true, ".vscode": true, ".claude": true}

type chart struct {
	path, name string
	deps       []string
	// repos[i] is deps[i]'s `repository` as written.
	repos []string
}

func main() {
	subject := flag.String("subject", "", "subject checkout")
	helm := flag.String("helm", "", "helm binary: cross-check each chart's name and dependencies with `helm show chart`")
	tierAFlag := flag.Bool("tier-a", false, "with -helm: tier-A definitions and references from Helm itself (tiera.go)")
	scratch := flag.String("scratch", "", "with -tier-a: scratch directory for the ablation copies and Helm's caches")
	xrefsOut := flag.String("xrefs-out", "", "with -tier-a: where the walk's tier-C cross_references go")
	filesOut := flag.String("files-out", "", "with -tier-a: the files of the charts measured, one per line")
	workers := flag.Int("workers", 8, "with -tier-a: charts measured in parallel")
	fileDepsOutPath := flag.String("file-deps-out", "", "with -helm and -scratch: each file:// dependency graded by helm dependency list, tier A, as cross_references to this file (filedeps.go)")
	xrefsOnly := flag.Bool("xrefs-only", false, "write the walk's cross_references alone (no definitions or references) to stdout")
	flag.Parse()
	if *subject == "" || (*tierAFlag && (*helm == "" || *scratch == "" || *xrefsOut == "")) || (*fileDepsOutPath != "" && (*helm == "" || *scratch == "")) || (*xrefsOnly && *tierAFlag) {
		fmt.Fprintln(os.Stderr, "usage: helm-oracle -subject <dir> [-helm <helm> [-tier-a -scratch <dir> -xrefs-out <json> [-files-out <txt>]] [-scratch <dir> -file-deps-out <json>]] [-xrefs-only]")
		os.Exit(2)
	}
	o := out{
		Tool:        "helm-oracle (Go text/template/parse for templates, gopkg.in/yaml.v3 for Chart.yaml and values.yaml)",
		Version:     "text/template/parse (go1.26); gopkg.in/yaml.v3 v3.0.1",
		Tier:        "C",
		Definitions: []def{}, References: []ref{}, CrossReferences: []xref{},
	}
	var charts []chart
	var st stats
	err := filepath.WalkDir(*subject, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(*subject, p)
		rel = filepath.ToSlash(rel)
		kind := classify(rel)
		if kind == "" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil || len(b) > maxFileBytes || bytes.IndexByte(b, 0) >= 0 {
			return nil
		}
		switch kind {
		case "chart":
			if c, ok := walkChart(&o, rel, b); ok {
				charts = append(charts, c)
				if *helm != "" {
					crossCheck(*helm, *subject, c, &st)
				}
			}
		case "values":
			walkValues(&o, rel, b)
		case "template":
			walkTemplate(&o, rel, string(b), &st)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "helm-oracle:", err)
		os.Exit(1)
	}
	byName := map[string][]chart{}
	for _, c := range charts {
		byName[c.name] = append(byName[c.name], c)
	}
	seen := map[xref]bool{}
	shapes := map[string]int{}
	for _, c := range charts {
		for i, dep := range c.deps {
			var others []chart
			for _, t := range byName[dep] {
				if t.path != c.path {
					others = append(others, t)
				}
			}
			if len(others) != 1 {
				continue
			}
			x := xref{c.path, c.name, others[0].path, others[0].name}
			if !seen[x] {
				seen[x] = true
				o.CrossReferences = append(o.CrossReferences, x)
				shapes[repoShape(c.repos[i])]++
			}
		}
	}
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
	sort.Slice(o.References, func(i, j int) bool {
		a, b := o.References[i], o.References[j]
		return a.Path+"\x00"+a.From+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.To
	})
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To
	})
	fmt.Fprintf(os.Stderr, "helm-oracle: %d charts, %d templates parsed, %d rejected by go's parser, helm cross-check: %d checked, %d mismatched, %d not loadable; include/template calls with a literal name: %d inside a define, %d outside any\n",
		len(charts), st.templates, st.rejected, st.checked, st.mismatched, st.notLoadable, st.callsInDefines, st.callsOutside)
	fmt.Fprintf(os.Stderr, "helm-oracle: %d cross-references by the dependency's repository: %v\n", len(o.CrossReferences), shapes)
	if shapes["oci"] > 0 {
		o.Disclose = []string{ociCeiling(len(o.CrossReferences), shapes["oci"], shapes["file"])}
	}
	if *fileDepsOutPath != "" {
		if err := writeFileDeps(*subject, *helm, *scratch, *fileDepsOutPath, charts); err != nil {
			fmt.Fprintln(os.Stderr, "helm-oracle:", err)
			os.Exit(1)
		}
	}
	if *xrefsOnly {
		o.Definitions, o.References = []def{}, []ref{}
	}
	if *tierAFlag {
		if err := writeTierA(o, *subject, *helm, *scratch, *xrefsOut, *filesOut, *workers); err != nil {
			fmt.Fprintln(os.Stderr, "helm-oracle:", err)
			os.Exit(1)
		}
		return
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(o); err != nil {
		os.Exit(1)
	}
}

// repoShape names how a dependency says where its chart lives: no repository
// (the chart is under charts/), a file:// path, a registry (oci://) or a URL.
func repoShape(r string) string {
	switch {
	case r == "":
		return "none"
	case strings.HasPrefix(r, "file://"):
		return "file"
	case strings.HasPrefix(r, "oci://"):
		return "oci"
	}
	return "url"
}

type stats struct {
	templates, rejected, checked, mismatched, notLoadable int
	// callsInDefines and callsOutside count the include/template calls a
	// reading can attach to a named template and those in a manifest body,
	// which belong to no symbol (disclosed, never scored).
	callsInDefines, callsOutside int
}

// classify says which reader a repo-relative path belongs to, by the helm
// dialect manifest's match block (basenames, .tpl, templates/ globs).
func classify(rel string) string {
	base := path.Base(rel)
	switch {
	case base == "Chart.yaml":
		return "chart"
	case base == "values.yaml":
		return "values"
	case strings.EqualFold(path.Ext(base), ".tpl"):
		return "template"
	}
	segs := strings.Split(rel, "/")
	for i, s := range segs[:len(segs)-1] {
		if s != "templates" {
			continue
		}
		ext := strings.ToLower(path.Ext(base))
		if ext == ".yaml" || ext == ".yml" {
			return "template"
		}
		if base == "NOTES.txt" && i == len(segs)-2 {
			return "template"
		}
	}
	return ""
}

func get(n *yaml.Node, key string) *yaml.Node {
	n = resolve(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func resolve(n *yaml.Node) *yaml.Node {
	for depth := 0; n != nil && n.Kind == yaml.AliasNode && depth < 64; depth++ {
		n = n.Alias
	}
	return n
}

func scalar(n *yaml.Node) string {
	n = resolve(n)
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		return ""
	}
	return n.Value
}

// walkChart reads a Chart.yaml: the chart, and one dependency per entry.
func walkChart(o *out, rel string, b []byte) (chart, bool) {
	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(b)).Decode(&doc); err != nil || len(doc.Content) == 0 {
		return chart{}, false
	}
	root := resolve(doc.Content[0])
	name := scalar(get(root, "name"))
	if root.Kind != yaml.MappingNode || name == "" {
		return chart{}, false
	}
	c := chart{path: rel, name: name}
	o.Definitions = append(o.Definitions, def{rel, name, root.Line, "package"})
	if deps := resolve(get(root, "dependencies")); deps != nil && deps.Kind == yaml.SequenceNode {
		for _, d := range deps.Content {
			d = resolve(d)
			if dn := scalar(get(d, "name")); dn != "" && d.Kind == yaml.MappingNode {
				o.Definitions = append(o.Definitions, def{rel, dn, d.Line, "data_source"})
				c.deps = append(c.deps, dn)
				c.repos = append(c.repos, scalar(get(d, "repository")))
			}
		}
	}
	return c, true
}

// walkValues emits one key definition per key path down valuesMaxDepth,
// descending only into non-empty mappings, after Helm's own reading of the
// file: an alias is followed and a `<<` merge key contributes the keys it
// merges (an explicit key of the same name wins).
func walkValues(o *out, rel string, b []byte) {
	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(b)).Decode(&doc); err != nil || len(doc.Content) == 0 {
		return
	}
	root := resolve(doc.Content[0])
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}
	var walk func(m *yaml.Node, prefix string, depth int)
	walk = func(m *yaml.Node, prefix string, depth int) {
		if depth > valuesMaxDepth {
			return
		}
		type kv struct {
			key  *yaml.Node
			val  *yaml.Node
			line int
		}
		var pairs []kv
		own := map[string]bool{}
		var merged []kv
		for i := 0; i+1 < len(m.Content); i += 2 {
			k, v := m.Content[i], m.Content[i+1]
			if k.Tag == "!!merge" || k.Value == "<<" {
				for _, src := range mergeSources(v) {
					for j := 0; j+1 < len(src.Content); j += 2 {
						merged = append(merged, kv{src.Content[j], src.Content[j+1], src.Content[j].Line})
					}
				}
				continue
			}
			own[k.Value] = true
			pairs = append(pairs, kv{k, v, k.Line})
		}
		for _, p := range merged {
			if !own[p.key.Value] {
				own[p.key.Value] = true
				pairs = append(pairs, p)
			}
		}
		for _, p := range pairs {
			if p.key.Value == "" {
				continue
			}
			name := p.key.Value
			if prefix != "" {
				name = prefix + "." + name
			}
			o.Definitions = append(o.Definitions, def{rel, name, p.line, "key"})
			if v := resolve(p.val); v != nil && v.Kind == yaml.MappingNode && len(v.Content) > 0 {
				walk(v, name, depth+1)
			}
		}
	}
	walk(root, "", 1)
}

func mergeSources(v *yaml.Node) []*yaml.Node {
	v = resolve(v)
	switch {
	case v == nil:
		return nil
	case v.Kind == yaml.MappingNode:
		return []*yaml.Node{v}
	case v.Kind == yaml.SequenceNode:
		var out []*yaml.Node
		for _, it := range v.Content {
			if it = resolve(it); it != nil && it.Kind == yaml.MappingNode {
				out = append(out, it)
			}
		}
		return out
	}
	return nil
}

// walkTemplate parses one template file with Go's own parser and records its
// named templates, `include`/`template` calls made inside them, and the
// .Values accessors anywhere.
func walkTemplate(o *out, rel, text string, st *stats) {
	trees := map[string]*parse.Tree{}
	t := parse.New(rel)
	t.Mode = parse.SkipFuncCheck
	if _, err := t.Parse(text, "{{", "}}", trees); err != nil {
		st.rejected++
		return
	}
	st.templates++
	lineOf := func(off int) int { return 1 + strings.Count(text[:off], "\n") }
	names := make([]string, 0, len(trees))
	for n := range trees {
		names = append(names, n)
	}
	sort.Strings(names)
	callSeen := map[ref]bool{}
	for _, name := range names {
		tr := trees[name]
		isDefine := name != rel
		if isDefine {
			re := regexp.MustCompile("\\{\\{-?\\s*(?:define|block)\\s+(?:\"" + regexp.QuoteMeta(name) + "\"|`" + regexp.QuoteMeta(name) + "`)")
			if loc := re.FindStringIndex(text); loc != nil {
				// the line of the name literal, as a definition's line is where its name is written
				nameOff := loc[0] + strings.LastIndexAny(text[loc[0]:loc[1]], "\"`")
				o.Definitions = append(o.Definitions, def{rel, name, lineOf(nameOff), "macro"})
			}
		}
		if tr.Root == nil {
			continue
		}
		walkNodes(tr.Root, func(n parse.Node) {
			switch n := n.(type) {
			case *parse.FieldNode:
				if len(n.Ident) >= 2 && n.Ident[0] == "Values" {
					o.Definitions = append(o.Definitions, def{rel, strings.Join(n.Ident[1:], "."), lineOf(int(n.Pos)), "key"})
				}
			case *parse.VariableNode:
				if len(n.Ident) >= 3 && n.Ident[0] == "$" && n.Ident[1] == "Values" {
					o.Definitions = append(o.Definitions, def{rel, strings.Join(n.Ident[2:], "."), lineOf(int(n.Pos)), "key"})
				}
			case *parse.TemplateNode:
				// a `block` is its own definition and call, which no
				// enclosing template can own: only `template` counts
				if !isBlock(text, int(n.Pos)) {
					o.Definitions = append(o.Definitions, def{rel, n.Name, lineOf(int(n.Pos)), "macro_ref"})
					if isDefine {
						st.callsInDefines++
						addCall(o, callSeen, rel, name, n.Name)
					} else {
						st.callsOutside++
					}
				}
			case *parse.CommandNode:
				if len(n.Args) >= 2 {
					if id, ok := n.Args[0].(*parse.IdentifierNode); ok && id.Ident == "include" {
						if s, ok := n.Args[1].(*parse.StringNode); ok {
							o.Definitions = append(o.Definitions, def{rel, s.Text, lineOf(int(id.Pos)), "macro_ref"})
							if isDefine {
								st.callsInDefines++
								addCall(o, callSeen, rel, name, s.Text)
							} else {
								st.callsOutside++
							}
						}
					}
				}
			}
		})
	}
}

// isBlock reports whether the template call whose name literal starts at pos
// was written `block` (its node's position is the name, not the keyword, so
// the keyword is the word before it).
func isBlock(text string, pos int) bool {
	return strings.HasSuffix(strings.TrimRight(text[:pos], " \t\r\n"), "block")
}

func addCall(o *out, seen map[ref]bool, rel, from, to string) {
	r := ref{rel, from, to}
	if to == "" || seen[r] {
		return
	}
	seen[r] = true
	o.References = append(o.References, r)
}

// walkNodes visits every node under n, depth first.
func walkNodes(n parse.Node, visit func(parse.Node)) {
	if n == nil {
		return
	}
	visit(n)
	switch n := n.(type) {
	case *parse.ListNode:
		if n == nil {
			return
		}
		for _, c := range n.Nodes {
			walkNodes(c, visit)
		}
	case *parse.ActionNode:
		walkNodes(n.Pipe, visit)
	case *parse.PipeNode:
		if n == nil {
			return
		}
		for _, d := range n.Decl {
			walkNodes(d, visit)
		}
		for _, c := range n.Cmds {
			walkNodes(c, visit)
		}
	case *parse.CommandNode:
		for _, a := range n.Args {
			walkNodes(a, visit)
		}
	case *parse.IfNode:
		walkBranch(&n.BranchNode, visit)
	case *parse.RangeNode:
		walkBranch(&n.BranchNode, visit)
	case *parse.WithNode:
		walkBranch(&n.BranchNode, visit)
	case *parse.TemplateNode:
		walkNodes(n.Pipe, visit)
	case *parse.ChainNode:
		walkNodes(n.Node, visit)
	}
}

func walkBranch(b *parse.BranchNode, visit func(parse.Node)) {
	walkNodes(b.Pipe, visit)
	walkNodes(b.List, visit)
	walkNodes(b.ElseList, visit)
}

// crossCheck loads a chart with the helm binary and compares its name and
// dependency names with the walk's.
func crossCheck(helm, subject string, c chart, st *stats) {
	cmd := exec.Command(helm, "show", "chart", filepath.Join(subject, filepath.FromSlash(path.Dir(c.path))))
	b, err := cmd.Output()
	if err != nil {
		st.notLoadable++
		return
	}
	var meta struct {
		Name         string `yaml:"name"`
		Dependencies []struct {
			Name string `yaml:"name"`
		} `yaml:"dependencies"`
	}
	if err := yaml.Unmarshal(b, &meta); err != nil {
		st.notLoadable++
		return
	}
	st.checked++
	var deps []string
	for _, d := range meta.Dependencies {
		deps = append(deps, d.Name)
	}
	if meta.Name != c.name || strings.Join(deps, ",") != strings.Join(c.deps, ",") {
		st.mismatched++
		fmt.Fprintf(os.Stderr, "helm-oracle: %s: helm says %q %v, walk says %q %v\n", c.path, meta.Name, deps, c.name, c.deps)
	}
}
