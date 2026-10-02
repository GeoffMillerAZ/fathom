// Command flux-oracle is the language-breadth harness's tier-C oracle for the
// flux doc-engine dialect (design §1.3, owner decision D4): a from-scratch
// gopkg.in/yaml.v3 walk of a subject's Flux objects, never fathom's doc engine,
// written in indexbench lang-oracle's tool-oracle shape. The Flux CLI is not
// installed here and no release binary was fetched, so `flux build kustomization
// --dry-run` (tier A) did not run: the oracle re-implements the format's rules
// over an independent parser, which caps every metric at good.
//
//	go run . -subject <checkout> > <out.json>
//
// A file is a Flux file when every non-empty document of it is an object of a
// Flux API group (kustomize, helm, source, notification, image .toolkit.fluxcd.io):
// the file-level rule fathom's dispatch (fusion.crDispatch) applies, written here
// again from the same statement. A file with Flux documents and other documents
// (a Namespace beside a Kustomization) is mixed: it stays Kubernetes and is
// counted on stderr, never read. A Helm chart's templates and an undecodable
// file contribute nothing.
//
//   - definitions: one `resource` per object, named `Kind/<namespace>/<name>`
//     (`Kind/<name>` with no namespace), the Kubernetes dialect's naming.
//   - references: the names an object writes: spec.sourceRef.name,
//     spec.chart.spec.sourceRef.name, spec.chartRef.name, spec.dependsOn[].name,
//     spec.valuesFrom[].name.
//   - cross_references: an object to the object it names in ANOTHER file of the
//     subject, by the Flux rules: a sourceRef, a chart's sourceRef or a chartRef
//     names `kind/namespace/name` (the namespace is the ref's own, else the
//     referring object's); a dependsOn names a Kustomization or HelmRelease (the
//     referring kind) likewise; a valuesFrom names a ConfigMap or Secret in the
//     HelmRelease's namespace. A target declared in two files is ambiguous (the
//     staging and production overlays of one repository) and is in no truth.
//     Kustomization spec.path and a local chart's path name directories, whose
//     target is the directory's kustomization or Chart file (the kustomize and
//     helm dialects' files, outside what the flux dialect reads): lang-oracle
//     drops those as out of scope, and they are counted on stderr.
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
	"regexp"
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

var groupRe = regexp.MustCompile(`^(kustomize|helm|source|notification|image)\.toolkit\.fluxcd\.io/`)

// object is one decoded Flux object.
type object struct {
	path, name, kind, ns string
	line                 int
	node                 *yaml.Node
}

func (o object) key() string {
	return o.kind + "|" + o.ns + "|" + scalar(at(o.node, "metadata", "name"))
}

func main() {
	subject := flag.String("subject", "", "subject checkout")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: flux-oracle -subject <dir>")
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
		fmt.Fprintln(os.Stderr, "flux-oracle:", err)
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
	o := out{Tool: "flux-oracle (independent gopkg.in/yaml.v3 walk)", Version: "gopkg.in/yaml.v3 v3.0.1", Tier: "C",
		Definitions: []def{}, References: []ref{}, CrossReferences: []xref{}}
	var objs []object
	claimed, mixed, undecodable := 0, 0, 0
	for _, rel := range files {
		if inChart(rel) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(*subject, filepath.FromSlash(rel)))
		if err != nil {
			fmt.Fprintln(os.Stderr, "flux-oracle:", err)
			os.Exit(1)
		}
		roots, ok := decode(b)
		if !ok {
			undecodable++
			continue
		}
		flux := 0
		for _, r := range roots {
			if groupRe.MatchString(scalar(get(r, "apiVersion"))) {
				flux++
			}
		}
		switch {
		case flux == 0:
			continue
		case flux != len(roots):
			mixed++
			continue
		}
		claimed++
		for _, r := range roots {
			kind, name := scalar(get(r, "kind")), scalar(at(r, "metadata", "name"))
			if kind == "" || get(r, "metadata") == nil || name == "" {
				continue
			}
			ns := scalar(at(r, "metadata", "namespace"))
			objs = append(objs, object{path: rel, name: qual(kind, ns, name), kind: kind, ns: ns, line: r.Line, node: r})
		}
	}
	byKey := map[string][]object{}
	for _, ob := range objs {
		o.Definitions = append(o.Definitions, def{ob.path, ob.name, ob.line, "resource"})
		byKey[ob.key()] = append(byKey[ob.key()], ob)
	}
	seenRef := map[ref]bool{}
	addRef := func(ob object, to string) {
		r := ref{ob.path, ob.name, to}
		if to != "" && !seenRef[r] {
			seenRef[r] = true
			o.References = append(o.References, r)
		}
	}
	seenX := map[xref]bool{}
	unresolved, ambiguous, dirs := 0, 0, 0
	// A reference names `kind/namespace/name`. An object that writes no
	// namespace (a cluster repository whose Kustomizations set targetNamespace)
	// takes the namespace of its deployment, so it also answers a reference that
	// names one: each of the two forms resolves when exactly one other file
	// declares it.
	link := func(ob object, kind, ns, name string) {
		if kind == "" || name == "" {
			return
		}
		hit := false
		for _, key := range []string{kind + "|" + ns + "|" + name, kind + "||" + name} {
			var other []object
			for _, c := range byKey[key] {
				if c.path != ob.path {
					other = append(other, c)
				}
			}
			switch {
			case len(other) == 1:
				hit = true
				x := xref{ob.path, ob.name, other[0].path, other[0].name}
				if !seenX[x] {
					seenX[x] = true
					o.CrossReferences = append(o.CrossReferences, x)
				}
			case len(other) > 1:
				ambiguous++
				hit = true
			}
		}
		if !hit {
			unresolved++
		}
	}
	for _, ob := range objs {
		r := ob.node
		own := ob.ns
		nsOf := func(n *yaml.Node) string {
			if s := scalar(get(n, "namespace")); s != "" {
				return s
			}
			return own
		}
		for _, p := range [][]string{{"spec", "sourceRef"}, {"spec", "chart", "spec", "sourceRef"}, {"spec", "chartRef"}} {
			if n := at(r, p...); n != nil {
				addRef(ob, scalar(get(n, "name")))
				link(ob, scalar(get(n, "kind")), nsOf(n), scalar(get(n, "name")))
			}
		}
		for _, d := range seq(at(r, "spec", "dependsOn")) {
			addRef(ob, scalar(get(d, "name")))
			if ob.kind == "Kustomization" || ob.kind == "HelmRelease" {
				link(ob, ob.kind, nsOf(d), scalar(get(d, "name")))
			}
		}
		for _, v := range seq(at(r, "spec", "valuesFrom")) {
			addRef(ob, scalar(get(v, "name")))
			if ob.kind == "HelmRelease" {
				link(ob, scalar(get(v, "kind")), own, scalar(get(v, "name")))
			}
		}
		if ob.kind == "Kustomization" && scalar(at(r, "spec", "path")) != "" {
			dirs++
		}
		if ob.kind == "HelmRelease" && scalar(at(r, "spec", "chart", "spec", "sourceRef", "kind")) == "GitRepository" {
			dirs++
		}
	}
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To
	})
	fmt.Fprintf(os.Stderr, "flux-oracle: %d flux files, %d mixed (stay Kubernetes), %d undecodable; %d objects; %d cross-file edges, %d unresolved, %d ambiguous (in no truth); %d directory references (path, chart: target outside the dialect)\n",
		claimed, mixed, undecodable, len(objs), len(o.CrossReferences), unresolved, ambiguous, dirs)
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
		if len(doc.Content) > 0 && doc.Content[0].Kind == yaml.MappingNode {
			roots = append(roots, doc.Content[0])
		} else if len(doc.Content) > 0 {
			roots = append(roots, doc.Content[0])
		}
	}
	return roots, len(roots) > 0
}
