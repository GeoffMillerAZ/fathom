// Command crossplane-oracle is the language-breadth harness's tier-C oracle for
// the crossplane doc-engine dialect (design §1.3, owner decision D4): a from-scratch
// gopkg.in/yaml.v3 walk of a subject's Crossplane definitions, compositions and
// packages, never fathom's doc engine, written in indexbench lang-oracle's
// tool-oracle shape. `crossplane beta validate` (tier A) needs provider and
// function schemas pulled from a registry, which this oracle never fetches, so the
// oracle re-implements the format's rules over an independent parser and every
// metric is capped at good.
//
//	go run . -subject <checkout> > <out.json>
//
// A file is a Crossplane file when every non-empty document of it is an object of
// apiVersion apiextensions.crossplane.io/*, pkg.crossplane.io/* or
// meta.pkg.crossplane.io/* (a composite resource or claim of a user-defined API
// group, an EnvironmentConfig's neighbours of another group, a CustomResourceDefinition
// stay Kubernetes): the file-level rule fathom's dispatch (fusion.crDispatch)
// applies, written here again from the same statement. A file with such documents
// and others (an XRD beside a composite resource) is mixed: it stays Kubernetes and
// is counted on stderr. A Helm chart's templates and an undecodable file
// contribute nothing.
//
//   - definitions: `resource` per object (`Kind/<namespace>/<name>`, the Kubernetes
//     dialect's naming), `service` per composite kind (an XRD's spec.names.kind) and
//     claim kind (spec.claimNames.kind), `section` per XRD version, `element` per
//     composed resource (spec.resources[] and a pipeline step's input.resources[]),
//     `step` per pipeline step.
//   - references: the names an object writes: spec.compositeTypeRef.kind,
//     a step's functionRef.name, spec.dependsOn[] (provider, function, configuration),
//     spec.defaultCompositionRef.name and spec.enforcedCompositionRef.name.
//   - cross_references: a Composition's compositeTypeRef to the XRD in ANOTHER file
//     whose spec.group and spec.names.kind it names (the target is the XRD's
//     composite-kind entity); a step's functionRef to the Function of that name,
//     and an XRD's default or enforced composition to the Composition of that name,
//     in another file. A target declared in two files is ambiguous and in no truth.
//     A composite resource or claim naming its XRD (a user-defined group) stays
//     Kubernetes and is not read.
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

func isCrossplane(r *yaml.Node) bool {
	av := scalar(get(r, "apiVersion"))
	return strings.HasPrefix(av, "apiextensions.crossplane.io/") || strings.HasPrefix(av, "pkg.crossplane.io/") || strings.HasPrefix(av, "meta.pkg.crossplane.io/")
}

// object is one decoded Crossplane object.
type object struct {
	path, name, kind, ns, rawName string
	line                          int
	node                          *yaml.Node
}

func main() {
	subject := flag.String("subject", "", "subject checkout")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: crossplane-oracle -subject <dir>")
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
		fmt.Fprintln(os.Stderr, "crossplane-oracle:", err)
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
		Tool: "crossplane-oracle (independent gopkg.in/yaml.v3 walk)", Version: "gopkg.in/yaml.v3 v3.0.1", Tier: "C",
		Definitions: []def{}, References: []ref{}, CrossReferences: []xref{},
	}
	var objs []object
	claimed, mixed, undecodable := 0, 0, 0
	for _, rel := range files {
		if inChart(rel) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(*subject, filepath.FromSlash(rel)))
		if err != nil {
			fmt.Fprintln(os.Stderr, "crossplane-oracle:", err)
			os.Exit(1)
		}
		roots, ok := decode(b)
		if !ok {
			undecodable++
			continue
		}
		n := 0
		for _, r := range roots {
			if isCrossplane(r) {
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
		for _, r := range roots {
			kind, name := scalar(get(r, "kind")), scalar(at(r, "metadata", "name"))
			if kind == "" || get(r, "metadata") == nil {
				continue
			}
			ns := scalar(at(r, "metadata", "namespace"))
			ob := object{path: rel, kind: kind, ns: ns, rawName: name, line: r.Line, node: r}
			if name != "" {
				ob.name = qual(kind, ns, name)
			}
			objs = append(objs, ob)
		}
	}
	seenRef := map[ref]bool{}
	addRef := func(path, from, to string) {
		r := ref{path, from, to}
		if to != "" && from != "" && !seenRef[r] {
			seenRef[r] = true
			o.References = append(o.References, r)
		}
	}
	type kindEnt struct {
		path, group string
	}
	xrKinds := map[string][]kindEnt{}   // composite kind -> XRDs declaring it
	byKindName := map[string][]object{} // "Kind|name" -> objects
	for _, ob := range objs {
		if ob.name != "" {
			byKindName[ob.kind+"|"+ob.rawName] = append(byKindName[ob.kind+"|"+ob.rawName], ob)
		}
		if k := scalar(at(ob.node, "spec", "names", "kind")); k != "" {
			xrKinds[k] = append(xrKinds[k], kindEnt{ob.path, scalar(at(ob.node, "spec", "group"))})
		}
	}
	for _, ob := range objs {
		r := ob.node
		if ob.name != "" {
			o.Definitions = append(o.Definitions, def{ob.path, ob.name, ob.line, "resource"})
		}
		if names := at(r, "spec", "names"); names != nil && names.Kind == yaml.MappingNode {
			if k := scalar(get(names, "kind")); k != "" {
				o.Definitions = append(o.Definitions, def{ob.path, k, keyLine(at(r, "spec"), "names"), "service"})
			}
		}
		if names := at(r, "spec", "claimNames"); names != nil && names.Kind == yaml.MappingNode {
			if k := scalar(get(names, "kind")); k != "" {
				o.Definitions = append(o.Definitions, def{ob.path, k, keyLine(at(r, "spec"), "claimNames"), "service"})
			}
		}
		for _, v := range seq(at(r, "spec", "versions")) {
			if v.Kind == yaml.MappingNode {
				if n := scalar(get(v, "name")); n != "" {
					o.Definitions = append(o.Definitions, def{ob.path, n, v.Line, "section"})
				}
			}
		}
		composed := func(list []*yaml.Node) {
			for _, c := range list {
				if c.Kind == yaml.MappingNode {
					if n := scalar(get(c, "name")); n != "" {
						o.Definitions = append(o.Definitions, def{ob.path, n, c.Line, "element"})
					}
				}
			}
		}
		composed(seq(at(r, "spec", "resources")))
		for _, st := range seq(at(r, "spec", "pipeline")) {
			if st.Kind != yaml.MappingNode {
				continue
			}
			composed(seq(at(st, "input", "resources")))
			if n := scalar(get(st, "step")); n != "" {
				o.Definitions = append(o.Definitions, def{ob.path, n, st.Line, "step"})
				addRef(ob.path, n, scalar(at(st, "functionRef", "name")))
			}
		}
		addRef(ob.path, ob.name, scalar(at(r, "spec", "compositeTypeRef", "kind")))
		for _, d := range seq(at(r, "spec", "dependsOn")) {
			for _, k := range []string{"provider", "function", "configuration"} {
				addRef(ob.path, ob.name, scalar(get(d, k)))
			}
		}
		addRef(ob.path, ob.name, scalar(at(r, "spec", "defaultCompositionRef", "name")))
		addRef(ob.path, ob.name, scalar(at(r, "spec", "enforcedCompositionRef", "name")))
	}
	seenX := map[xref]bool{}
	unresolved, ambiguous := 0, 0
	add := func(from, fromName, toPath, to string) {
		x := xref{from, fromName, toPath, to}
		if from != toPath && !seenX[x] {
			seenX[x] = true
			o.CrossReferences = append(o.CrossReferences, x)
		}
	}
	objLink := func(ob object, fromName, kind, name string) {
		if name == "" {
			return
		}
		var other []object
		for _, c := range byKindName[kind+"|"+name] {
			if c.path != ob.path && c.ns == "" {
				other = append(other, c)
			}
		}
		switch {
		case len(other) == 1:
			add(ob.path, fromName, other[0].path, other[0].name)
		case len(other) > 1:
			ambiguous++
		default:
			unresolved++
		}
	}
	for _, ob := range objs {
		r := ob.node
		if ob.name == "" {
			continue
		}
		if ob.kind == "Composition" {
			kind := scalar(at(r, "spec", "compositeTypeRef", "kind"))
			group := strings.SplitN(scalar(at(r, "spec", "compositeTypeRef", "apiVersion")), "/", 2)[0]
			var other []kindEnt
			for _, c := range xrKinds[kind] {
				if c.path != ob.path && (group == "" || c.group == group) {
					other = append(other, c)
				}
			}
			switch {
			case kind == "":
			case len(other) == 1:
				add(ob.path, ob.name, other[0].path, kind)
			case len(other) > 1:
				ambiguous++
			default:
				unresolved++
			}
		}
		for _, st := range seq(at(r, "spec", "pipeline")) {
			if n := scalar(get(st, "step")); n != "" {
				objLink(object{path: ob.path, ns: ob.ns}, n, "Function", scalar(at(st, "functionRef", "name")))
			}
		}
		objLink(ob, ob.name, "Composition", scalar(at(r, "spec", "defaultCompositionRef", "name")))
		objLink(ob, ob.name, "Composition", scalar(at(r, "spec", "enforcedCompositionRef", "name")))
	}
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To
	})
	fmt.Fprintf(os.Stderr, "crossplane-oracle: %d crossplane files, %d mixed (stay Kubernetes), %d undecodable; %d objects; %d cross-file edges, %d unresolved, %d ambiguous (in no truth)\n",
		claimed, mixed, undecodable, len(objs), len(o.CrossReferences), unresolved, ambiguous)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(o); err != nil {
		os.Exit(1)
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
