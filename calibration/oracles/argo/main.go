// Command argo-oracle is the language-breadth harness's tier-C oracle for the
// argo doc-engine dialect (design §1.3, owner decision D4): a from-scratch
// gopkg.in/yaml.v3 walk of a subject's Argo CD and Argo Workflows objects, never
// fathom's doc engine, written in indexbench lang-oracle's tool-oracle shape.
//
//	go run . -subject <checkout> [-argo <argo binary>] > <out.json>
//
// `argo lint --offline` (tier A, argo v4.1.4) is the tool's own judgement of a
// Workflow, WorkflowTemplate, ClusterWorkflowTemplate or CronWorkflow; it prints
// no entity and no line, so it cannot be the definitions oracle. With -argo each
// such file is linted (the whole file, `--offline`, no cluster) and the number the
// tool accepts, rejects and reads as not Argo is counted on stderr: the
// acceptance cross-check, never scored. An Application, ApplicationSet or
// AppProject has no offline tool, so those objects are tier C by the format's
// rules alone, and every metric is capped at good.
//
// A file is an Argo file when every non-empty document of it is an object of
// apiVersion argoproj.io/* whose kind is Application, ApplicationSet, AppProject,
// Workflow, WorkflowTemplate, ClusterWorkflowTemplate or CronWorkflow (an Argo
// Rollout or Sensor stays Kubernetes): the file-level rule fathom's dispatch
// (fusion.crDispatch) applies, written here again from the same statement. A file
// with Argo documents and other documents is mixed: it stays Kubernetes and is
// counted on stderr. A Helm chart's templates and an undecodable file contribute
// nothing.
//
//   - definitions: `resource` per object (`Kind/<namespace>/<name>`, the
//     Kubernetes dialect's naming; an object with only generateName is named by
//     it), `task` per template (spec.templates[] and a CronWorkflow's
//     spec.workflowSpec.templates[]), `step` per steps item, `job` per dag task,
//     `data_source` per container image (container, script, containerSet
//     containers, sidecars, initContainers, a workflow's executor plugin sidecar),
//     each at the line its node starts.
//     A step's or task's inline template is walked as a template with no name
//     (its images, steps and tasks count). An object with no name at all
//     (no name and no generateName) has no `resource`; its templates still count.
//   - references: a step's or task's `template` and a task's `dependencies[]` to
//     the template or task of the same document that has that name, and an
//     object's entrypoint and onExit to a template of the document.
//   - cross_references: a step's or task's templateRef, and a Workflow's or
//     CronWorkflow's workflowTemplateRef, to the WorkflowTemplate (or, with
//     clusterScope, the ClusterWorkflowTemplate) of that name declared in ANOTHER
//     file, in the referring object's namespace or with none written; an
//     Application's or ApplicationSet template's source.path to the one Argo file
//     directly in that directory of the subject, at that file's first object. A
//     target declared in two files is ambiguous and in no truth.
//
// Bench-only; never a product dependency.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

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
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	References      []ref  `json:"references"`
	CrossReferences []xref `json:"cross_references"`
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

// keyLine is the line of the key a map value is written under.
func keyLine(n *yaml.Node, key string) int {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i].Line
		}
	}
	return 0
}

func at(n *yaml.Node, keys ...string) *yaml.Node {
	for _, k := range keys {
		n = get(n, k)
	}
	return n
}

func scalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		return ""
	}
	return n.Value
}

func seq(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

// items is a sequence's items, or a map read as the one item it is (what a
// `[*]` step reads; a steps list written flat is a list of items too).
func items(n *yaml.Node) []*yaml.Node {
	switch {
	case n == nil:
		return nil
	case n.Kind == yaml.SequenceNode:
		return n.Content
	case n.Kind == yaml.MappingNode:
		return []*yaml.Node{n}
	}
	return nil
}

var argoKinds = map[string]bool{
	"Application": true, "ApplicationSet": true, "AppProject": true, "Workflow": true,
	"WorkflowTemplate": true, "ClusterWorkflowTemplate": true, "CronWorkflow": true,
}

func isArgo(r *yaml.Node) bool {
	return strings.HasPrefix(scalar(get(r, "apiVersion")), "argoproj.io/") && argoKinds[scalar(get(r, "kind"))]
}

// object is one decoded Argo object.
type object struct {
	path, name, kind, ns, rawName string
	line                          int
	node                          *yaml.Node
	docNames                      map[string]map[string]bool // declared template and task names in the document
}

func main() {
	subject := flag.String("subject", "", "subject checkout")
	argoBin := flag.String("argo", "", "argo binary for the `argo lint --offline` acceptance cross-check (optional)")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: argo-oracle -subject <dir> [-argo <bin>]")
		os.Exit(2)
	}
	chartDirs := map[string]bool{}
	var files []string
	err := filepath.WalkDir(*subject, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(*subject, p)
		rel = filepath.ToSlash(rel)
		if d.Name() == "Chart.yaml" {
			chartDirs[path.Dir(rel)] = true
		}
		if ext := strings.ToLower(path.Ext(rel)); ext == ".yaml" || ext == ".yml" {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "argo-oracle:", err)
		os.Exit(1)
	}
	sort.Strings(files)
	inChart := func(rel string) bool {
		if !strings.Contains("/"+rel, "/templates/") {
			return false
		}
		for d := path.Dir(rel); ; d = path.Dir(d) {
			if chartDirs[d] {
				return true
			}
			if d == "." || d == "/" {
				return false
			}
		}
	}
	o := out{
		Tool: "argo-oracle (independent gopkg.in/yaml.v3 walk)", Version: "gopkg.in/yaml.v3 v3.0.1", Tier: "C",
		Definitions: []def{}, References: []ref{}, CrossReferences: []xref{},
	}
	var objs []object
	argoFiles := map[string][]object{} // path -> objects, in document order
	claimed, mixed, undecodable := 0, 0, 0
	var lintable []string
	for _, rel := range files {
		if inChart(rel) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(*subject, filepath.FromSlash(rel)))
		if err != nil {
			fmt.Fprintln(os.Stderr, "argo-oracle:", err)
			os.Exit(1)
		}
		roots, ok := decode(b)
		if !ok {
			undecodable++
			continue
		}
		n := 0
		for _, r := range roots {
			if isArgo(r) {
				n++
			}
		}
		switch {
		case n == 0:
			continue
		case n != len(roots):
			mixed++
			continue
		}
		claimed++
		wf := false
		for _, r := range roots {
			kind, name := scalar(get(r, "kind")), scalar(at(r, "metadata", "name"))
			if name == "" {
				name = scalar(at(r, "metadata", "generateName"))
			}
			if kind == "Workflow" || kind == "WorkflowTemplate" || kind == "ClusterWorkflowTemplate" || kind == "CronWorkflow" {
				wf = true
			}
			if get(r, "metadata") == nil {
				continue
			}
			ns := scalar(at(r, "metadata", "namespace"))
			ob := object{path: rel, kind: kind, ns: ns, rawName: scalar(at(r, "metadata", "name")), line: r.Line, node: r}
			if name != "" {
				ob.name = qual(kind, ns, name)
				argoFiles[rel] = append(argoFiles[rel], ob)
			}
			objs = append(objs, ob)
		}
		if wf {
			lintable = append(lintable, rel)
		}
	}
	seenRef := map[ref]bool{}
	addRef := func(path, from, to string) {
		r := ref{path, from, to}
		if to != "" && !seenRef[r] {
			seenRef[r] = true
			o.References = append(o.References, r)
		}
	}
	// Per object: definitions and references.
	for _, ob := range objs {
		if ob.name != "" {
			o.Definitions = append(o.Definitions, def{ob.path, ob.name, ob.line, "resource"})
		}
		r := ob.node
		var tmpls []*yaml.Node
		tmpls = append(tmpls, seq(at(r, "spec", "templates"))...)
		tmpls = append(tmpls, seq(at(r, "spec", "workflowSpec", "templates"))...)
		declTemplates, declTasks := map[string]bool{}, map[string]bool{}
		type step struct {
			name, template string
			deps           []string
		}
		var steps, tasks []step
		var walk func(t *yaml.Node, named bool)
		walk = func(t *yaml.Node, named bool) {
			if t.Kind != yaml.MappingNode {
				return
			}
			if named {
				if name := scalar(get(t, "name")); name != "" {
					o.Definitions = append(o.Definitions, def{ob.path, name, t.Line, "task"})
					declTemplates[name] = true
				}
			}
			for _, grp := range seq(get(t, "steps")) {
				for _, s := range items(grp) {
					if s.Kind != yaml.MappingNode {
						continue
					}
					if name := scalar(get(s, "name")); name != "" {
						o.Definitions = append(o.Definitions, def{ob.path, name, s.Line, "step"})
						steps = append(steps, step{name: name, template: scalar(get(s, "template"))})
					}
					if in := get(s, "inline"); in != nil {
						walk(in, false)
					}
				}
			}
			for _, k := range seq(at(t, "dag", "tasks")) {
				if k.Kind != yaml.MappingNode {
					continue
				}
				if name := scalar(get(k, "name")); name != "" {
					o.Definitions = append(o.Definitions, def{ob.path, name, k.Line, "job"})
					declTasks[name] = true
					st := step{name: name, template: scalar(get(k, "template"))}
					for _, d := range seq(get(k, "dependencies")) {
						st.deps = append(st.deps, scalar(d))
					}
					tasks = append(tasks, st)
				}
				if in := get(k, "inline"); in != nil {
					walk(in, false)
				}
			}
			for _, key := range []string{"container", "script"} {
				if c := get(t, key); c != nil && c.Kind == yaml.MappingNode {
					if img := scalar(get(c, "image")); img != "" {
						o.Definitions = append(o.Definitions, def{ob.path, img, keyLine(t, key), "data_source"})
					}
				}
			}
			for _, list := range [][]*yaml.Node{seq(at(t, "containerSet", "containers")), seq(get(t, "sidecars")), seq(get(t, "initContainers"))} {
				for _, c := range list {
					if img := scalar(get(c, "image")); img != "" && c.Kind == yaml.MappingNode {
						o.Definitions = append(o.Definitions, def{ob.path, img, c.Line, "data_source"})
					}
				}
			}
		}
		for _, t := range tmpls {
			if t.Kind == yaml.MappingNode && get(t, "name") != nil {
				walk(t, true)
			}
		}
		// A workflow-level executor plugin's sidecar container carries an image too.
		for _, pl := range seq(at(r, "spec", "executorPlugins")) {
			if sc := at(pl, "spec", "sidecar"); sc != nil {
				if c := get(sc, "container"); c != nil {
					if img := scalar(get(c, "image")); img != "" {
						o.Definitions = append(o.Definitions, def{ob.path, img, keyLine(sc, "container"), "data_source"})
					}
				}
			}
		}
		// A declared_only reference is recorded only when the document declares the name.
		for _, s := range steps {
			if declTemplates[s.template] {
				addRef(ob.path, s.name, s.template)
			}
		}
		for _, k := range tasks {
			if declTemplates[k.template] {
				addRef(ob.path, k.name, k.template)
			}
			for _, d := range k.deps {
				if declTasks[d] {
					addRef(ob.path, k.name, d)
				}
			}
		}
		for _, p := range [][]string{{"spec", "entrypoint"}, {"spec", "workflowSpec", "entrypoint"}, {"spec", "onExit"}} {
			if v := scalar(at(r, p...)); v != "" && declTemplates[v] && ob.name != "" {
				addRef(ob.path, ob.name, v)
			}
		}
	}
	// Cross-file references.
	byKindName := map[string][]object{}
	for _, ob := range objs {
		byKindName[ob.kind+"|"+ob.rawName] = append(byKindName[ob.kind+"|"+ob.rawName], ob)
	}
	seenX := map[xref]bool{}
	unresolved, ambiguous, external := 0, 0, 0
	add := func(from object, fromName string, to object) {
		x := xref{from.path, fromName, to.path, to.name}
		if x.Path != x.ToPath && !seenX[x] {
			seenX[x] = true
			o.CrossReferences = append(o.CrossReferences, x)
		}
	}
	// templateLink: a WorkflowTemplate (cluster scope: ClusterWorkflowTemplate) of that
	// name in another file, in the referring object's namespace or declaring none.
	templateLink := func(ob object, fromName string, rf *yaml.Node) {
		name := scalar(get(rf, "name"))
		if name == "" {
			return
		}
		kind := "WorkflowTemplate"
		if scalar(get(rf, "clusterScope")) == "true" {
			kind = "ClusterWorkflowTemplate"
		}
		var same, none []object
		for _, c := range byKindName[kind+"|"+name] {
			if c.path == ob.path {
				continue
			}
			switch {
			case c.ns == "":
				none = append(none, c)
			case c.ns == ob.ns:
				same = append(same, c)
			}
		}
		hit := false
		for _, cands := range [][]object{same, none} {
			switch {
			case len(cands) == 1:
				add(ob, fromName, cands[0])
				hit = true
			case len(cands) > 1:
				ambiguous++
				hit = true
			}
		}
		if !hit {
			unresolved++
		}
	}
	dirFile := func(dir string) (object, bool) {
		var fs []string
		for p := range argoFiles {
			if path.Dir(p) == dir {
				fs = append(fs, p)
			}
		}
		if len(fs) == 1 {
			return argoFiles[fs[0]][0], true
		}
		return object{}, false
	}
	for _, ob := range objs {
		r := ob.node
		var tmpls []*yaml.Node
		tmpls = append(tmpls, seq(at(r, "spec", "templates"))...)
		tmpls = append(tmpls, seq(at(r, "spec", "workflowSpec", "templates"))...)
		for _, t := range tmpls {
			for _, grp := range seq(get(t, "steps")) {
				for _, s := range items(grp) {
					if rf := get(s, "templateRef"); rf != nil && scalar(get(s, "name")) != "" {
						templateLink(ob, scalar(get(s, "name")), rf)
					}
				}
			}
			for _, k := range seq(at(t, "dag", "tasks")) {
				if rf := get(k, "templateRef"); rf != nil && scalar(get(k, "name")) != "" {
					templateLink(ob, scalar(get(k, "name")), rf)
				}
			}
		}
		for _, p := range [][]string{{"spec", "workflowTemplateRef"}, {"spec", "workflowSpec", "workflowTemplateRef"}} {
			if rf := at(r, p...); rf != nil {
				templateLink(ob, ob.name, rf)
			}
		}
		var sources []*yaml.Node
		switch ob.kind {
		case "Application":
			sources = append(sources, at(r, "spec", "source"))
			sources = append(sources, seq(at(r, "spec", "sources"))...)
		case "ApplicationSet":
			sources = append(sources, at(r, "spec", "template", "spec", "source"))
			sources = append(sources, seq(at(r, "spec", "template", "spec", "sources"))...)
		}
		for _, s := range sources {
			p := scalar(get(s, "path"))
			if p == "" {
				continue
			}
			dir := path.Clean(strings.TrimPrefix(p, "./"))
			if strings.Contains(p, "{{") || strings.HasPrefix(dir, "../") || dir == ".." {
				external++
				continue
			}
			if t, ok := dirFile(dir); ok {
				add(ob, ob.name, t)
			} else {
				unresolved++
			}
		}
	}
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To
	})
	fmt.Fprintf(os.Stderr, "argo-oracle: %d argo files, %d mixed (stay Kubernetes), %d undecodable; %d objects; %d cross-file edges, %d unresolved or not in the subject, %d ambiguous (in no truth), %d templated or climbing paths\n",
		claimed, mixed, undecodable, len(objs), len(o.CrossReferences), unresolved, ambiguous, external)
	if *argoBin != "" {
		lint(*argoBin, *subject, lintable)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(o); err != nil {
		os.Exit(1)
	}
}

// lint runs `argo lint --offline` on each Workflow-kind file (tier A, the tool's own
// acceptance): counted, never scored. A file with several documents is linted whole.
func lint(bin, subject string, files []string) {
	ok, bad := 0, 0
	var rejected []string
	for _, rel := range files {
		cmd := exec.Command(bin, "lint", "--offline", "--output", "simple", filepath.Join(subject, filepath.FromSlash(rel)))
		cmd.Stdin = nil
		if err := cmd.Run(); err != nil {
			bad++
			rejected = append(rejected, rel)
		} else {
			ok++
		}
	}
	fmt.Fprintf(os.Stderr, "argo-oracle: `argo lint --offline` accepts %d of %d workflow files, rejects %d\n", ok, len(files), bad)
	for i, r := range rejected {
		if i >= 10 {
			fmt.Fprintf(os.Stderr, "argo-oracle:   ... and %d more\n", len(rejected)-i)
			break
		}
		fmt.Fprintf(os.Stderr, "argo-oracle:   rejected: %s\n", r)
	}
}

func qual(kind, ns, name string) string {
	if ns != "" {
		return kind + "/" + ns + "/" + name
	}
	return kind + "/" + name
}

// decode returns every non-empty document's root, as the engine's decode does:
// the documents before a malformed one are kept, a file whose first document is
// malformed has none (ok false).
func decode(b []byte) (roots []*yaml.Node, ok bool) {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return roots, len(roots) > 0
		}
		if len(doc.Content) > 0 {
			roots = append(roots, doc.Content[0])
		}
	}
	return roots, len(roots) > 0
}
