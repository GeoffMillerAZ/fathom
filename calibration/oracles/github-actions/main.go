// Command github-actions-oracle is the language-breadth harness's tier-C
// oracle for the github-actions doc-engine dialect (design §1.3 "an
// independent parser library", owner decision D4): workflows through
// actionlint's own parser (github.com/rhysd/actionlint), composite/Docker
// action metadata (action.yml, which actionlint does not parse) through a
// plain gopkg.in/yaml.v3 walk — never fathom's doc engine — written in
// indexbench lang-oracle's tool-oracle shape: definitions, references
// (job needs:) and cross_references (a local `uses: ./path`, resolved
// from the repository root, GitHub's rule, to the workflow file or the
// action.yml/action.yaml in that directory; file granularity). Bench-only;
// never a product dependency. -resolve-out writes actionlint's own
// resolution of the same local uses values instead (resolve.go, LB-F457).
//
//	go run . -subject <checkout> [-resolve-out <file>] > <out.json>
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rhysd/actionlint"
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
}

type out struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	References      []ref  `json:"references"`
	CrossReferences []xref `json:"cross_references"`
}

var subject string

// res and resolved are actionlint's own resolution (-resolve-out, resolve.go).
var (
	res          *resolver
	resolved     []xref
	resolvedSeen = map[xref]bool{}
)

func addResolved(x xref) {
	if res == nil || x.ToPath == "" || x.From == "" || resolvedSeen[x] {
		return
	}
	resolvedSeen[x] = true
	resolved = append(resolved, x)
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// localTarget resolves a local `uses:` value from the repository root:
// the file itself, else the action metadata file in that directory. ""
// when it is not local or names nothing on disk.
func localTarget(uses string) string {
	if uses != "." && uses != "./" && !strings.HasPrefix(uses, "./") {
		return ""
	}
	p := path.Clean(uses)
	if strings.HasPrefix(p, "..") {
		return ""
	}
	for _, c := range []string{p, path.Join(p, "action.yml"), path.Join(p, "action.yaml")} {
		if st, err := os.Stat(filepath.Join(subject, filepath.FromSlash(c))); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func str(s *actionlint.String) string {
	if s == nil {
		return ""
	}
	return s.Value
}

func line(s *actionlint.String, p *actionlint.Pos) int {
	if s != nil && s.Pos != nil {
		return s.Pos.Line
	}
	if p != nil {
		return p.Line
	}
	return 0
}

func main() {
	flag.StringVar(&subject, "subject", "", "subject checkout")
	resolveOut := flag.String("resolve-out", "", "write actionlint's own resolution of each local uses (LocalActionsCache, LocalReusableWorkflowCache) as cross_references to this file")
	flag.Parse()
	if subject == "" {
		fmt.Fprintln(os.Stderr, "usage: github-actions-oracle -subject <dir> [-resolve-out <file>]")
		os.Exit(2)
	}
	if *resolveOut != "" {
		r, err := newResolver(subject)
		if err != nil {
			fmt.Fprintln(os.Stderr, "github-actions-oracle:", err)
			os.Exit(1)
		}
		res = r
	}
	o := out{Tool: "github-actions-oracle (actionlint AST + yaml.v3 for action metadata)", Version: "github.com/rhysd/actionlint v1.7.12", Tier: "C",
		Definitions: []def{}, References: []ref{}, CrossReferences: []xref{}}
	err := filepath.WalkDir(subject, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(subject, p)
		rel = filepath.ToSlash(rel)
		base := path.Base(rel)
		switch {
		case (path.Dir(rel) == ".github/workflows") && (strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".yaml")):
			return workflow(&o, p, rel)
		case base == "action.yml" || base == "action.yaml":
			return action(&o, p, rel)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "github-actions-oracle:", err)
		os.Exit(1)
	}
	sortXrefs(o.CrossReferences)
	if res != nil {
		sortXrefs(resolved)
		ro := struct {
			Tool            string       `json:"tool"`
			Version         string       `json:"version"`
			Tier            string       `json:"tier"`
			Definitions     []def        `json:"definitions"`
			CrossReferences []xref       `json:"cross_references"`
			Stats           resolveStats `json:"-"`
		}{"actionlint LocalActionsCache + LocalReusableWorkflowCache", "github.com/rhysd/actionlint v1.7.12", "C", []def{}, append([]xref{}, resolved...), res.stats}
		b, err := json.MarshalIndent(ro, "", " ")
		if err != nil || os.WriteFile(*resolveOut, append(b, '\n'), 0o644) != nil {
			fmt.Fprintln(os.Stderr, "github-actions-oracle: -resolve-out:", err)
			os.Exit(1)
		}
		sb, _ := json.Marshal(res.stats)
		fmt.Fprintf(os.Stderr, "github-actions-oracle: actionlint resolution %s, %d edges\n", sb, len(resolved))
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(o); err != nil {
		os.Exit(1)
	}
}

func sortXrefs(xs []xref) {
	sort.Slice(xs, func(i, j int) bool {
		a, b := xs[i], xs[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath < b.Path+"\x00"+b.From+"\x00"+b.ToPath
	})
}

var xrefSeen = map[xref]bool{}

func addXref(o *out, x xref) {
	if x.ToPath == "" || x.From == "" || xrefSeen[x] {
		return
	}
	xrefSeen[x] = true
	o.CrossReferences = append(o.CrossReferences, x)
}

// rootLine is the first line of a YAML document's root mapping.
func rootLine(b []byte) (int, *yaml.Node) {
	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(b)).Decode(&doc); err != nil || len(doc.Content) == 0 {
		return 0, nil
	}
	return doc.Content[0].Line, doc.Content[0]
}

func workflow(o *out, p, rel string) error {
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	w, _ := actionlint.Parse(b)
	if w == nil {
		return nil
	}
	if name := str(w.Name); name != "" {
		l, _ := rootLine(b)
		o.Definitions = append(o.Definitions, def{rel, name, l, "target"})
	}
	ids := make([]string, 0, len(w.Jobs))
	for k := range w.Jobs {
		ids = append(ids, k)
	}
	sort.Strings(ids)
	for _, k := range ids {
		j := w.Jobs[k]
		id := str(j.ID)
		o.Definitions = append(o.Definitions, def{rel, id, line(j.ID, j.Pos), "job"})
		for _, n := range j.Needs {
			o.References = append(o.References, ref{rel, id, str(n)})
		}
		if j.WorkflowCall != nil {
			addXref(o, xref{rel, id, localTarget(str(j.WorkflowCall.Uses))})
			if res != nil {
				addResolved(xref{rel, id, res.job(str(j.WorkflowCall.Uses))})
			}
		}
		for _, s := range j.Steps {
			if sid := str(s.ID); sid != "" {
				o.Definitions = append(o.Definitions, def{rel, sid, line(nil, s.Pos), "step"})
			}
			if a, ok := s.Exec.(*actionlint.ExecAction); ok && str(a.Uses) != "" {
				o.Definitions = append(o.Definitions, def{rel, str(a.Uses), line(nil, s.Pos), "resource"})
				addXref(o, xref{rel, id, localTarget(str(a.Uses))})
				if res != nil {
					addResolved(xref{rel, id, res.step(str(a.Uses))})
				}
			}
		}
	}
	return nil
}

func get(n *yaml.Node, key string) *yaml.Node {
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

func scalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

func action(o *out, p, rel string) error {
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	l, root := rootLine(b)
	runs := get(root, "runs")
	if runs == nil {
		return nil
	}
	name := scalar(get(root, "name"))
	if name != "" {
		o.Definitions = append(o.Definitions, def{rel, name, l, "target"})
	}
	if steps := get(runs, "steps"); steps != nil && steps.Kind == yaml.SequenceNode {
		for _, s := range steps.Content {
			if id := scalar(get(s, "id")); id != "" {
				o.Definitions = append(o.Definitions, def{rel, id, s.Line, "step"})
			}
			if u := scalar(get(s, "uses")); u != "" {
				o.Definitions = append(o.Definitions, def{rel, u, s.Line, "resource"})
				addXref(o, xref{rel, name, localTarget(u)})
				if res != nil {
					addResolved(xref{rel, name, res.step(u)})
				}
			}
		}
	}
	for key, kind := range map[string]string{"inputs": "variable_decl", "outputs": "output"} {
		m := get(root, key)
		if m == nil || m.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(m.Content); i += 2 {
			if m.Content[i+1].Kind == yaml.MappingNode {
				o.Definitions = append(o.Definitions, def{rel, m.Content[i].Value, m.Content[i+1].Line, kind})
			}
		}
	}
	return nil
}
