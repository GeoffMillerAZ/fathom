// Command kubernetes-oracle is the language-breadth harness's tier-C
// oracle for the kubernetes doc-engine dialect (design §1.3 "an
// independent parser library", owner decision D4): a from-scratch
// gopkg.in/yaml.v3 walk of a subject's manifests — never fathom's doc
// engine — written in indexbench lang-oracle's tool-oracle shape:
// definitions (resources, container images), references (ServiceAccount,
// ConfigMap and Secret names) and cross_references (a Service's
// spec.selector joined to every workload whose pod-template labels include
// it, in the same namespace), plus route_references and route_unresolved
// (LB-F99: a routing object's target host joined to the Service or compose
// service it names, by the Kubernetes and Istio DNS rules, routes.go) and,
// with -istioctl, a tier-A cross-check of Istio destination hosts and
// gateways from `istioctl analyze --use-kube=false` (IST0101); the side
// pass's pairs (mesh.go, LB-F315) and, with -labeljoins-out, the Stage A
// label-join truth (labeljoins.go). Bench-only; never a product dependency.
//
// With -kubectl (wave ox, LB-F153) the definitions are tier A: kubectl's own
// offline decode names every object and container image (kubectl.go), a file
// it rejects contributes no definition (counted on stderr, left out of
// -files-out; fathom's extraction of it counts against sym_precision, the
// docker-compose rule), and stdout carries the definitions alone; the
// references, cross_references and route pairs, which kubectl evaluates none
// of offline, stay this walk's (tier C, every file as before) in -refs-out.
//
//	go run . -subject <checkout> [-istioctl <bin>] > <out.json>
//	go run . -subject <checkout> -kubectl <bin> -refs-out <refs.json> -files-out <files.txt> > <defs.json>
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
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
	// LB-F99: route pairs and the references that name no provider
	// (routes.go); lang-oracle refuses these fields, lang-routes reads them.
	RouteReferences []routeRef        `json:"route_references"`
	RouteUnresolved []routeUnresolved `json:"route_unresolved"`
	RouteCrossCheck *crossCheck       `json:"route_crosscheck,omitempty"`
}

// get returns a mapping node's value for key.
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

func path(n *yaml.Node, keys ...string) *yaml.Node {
	for _, k := range keys {
		n = get(n, k)
	}
	return n
}

func scalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// labels is a mapping of scalars, nil otherwise.
func labels(n *yaml.Node) map[string]string {
	if n == nil || n.Kind != yaml.MappingNode || len(n.Content) == 0 {
		return nil
	}
	m := map[string]string{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i+1].Kind != yaml.ScalarNode {
			return nil
		}
		m[n.Content[i].Value] = n.Content[i+1].Value
	}
	return m
}

type labelled struct {
	path, name, ns string
	labels         map[string]string
}

type selector struct {
	path, name, ns string
	sel            map[string]string
}

func main() {
	subject := flag.String("subject", "", "subject checkout")
	istioctl := flag.String("istioctl", "", "istioctl binary for the Istio destination-host cross-check (optional)")
	kubectl := flag.String("kubectl", "", "kubectl binary: definitions from its offline decode, tier A (requires -refs-out)")
	refsOut := flag.String("refs-out", "", "with -kubectl: where the tier-C references, cross_references and route pairs go")
	filesOut := flag.String("files-out", "", "with -kubectl: the subject files kubectl accepted, one per line")
	flag.BoolVar(&ruleRoutes, "rule-routes", false, "write PrometheusRule identifying matchers as route targets (mesh.go; off: the Stage A kill withheld the join)")
	labelJoinsOut := flag.String("labeljoins-out", "", "the Stage A label-join truth: every PrometheusRule expression and the Services its job and service matchers read (labeljoins.go)")
	flag.Parse()
	if *subject == "" || (*kubectl != "" && *refsOut == "") {
		fmt.Fprintln(os.Stderr, "usage: kubernetes-oracle -subject <dir> [-kubectl <bin> -refs-out <json> [-files-out <txt>]]")
		os.Exit(2)
	}
	var kdefs []def
	var kst locateStats
	var accepted []string
	rejected := 0
	o := out{Tool: "kubernetes-oracle (independent gopkg.in/yaml.v3 walk)", Version: "gopkg.in/yaml.v3 v3.0.1", Tier: "C",
		Definitions: []def{}, References: []ref{}, CrossReferences: []xref{},
		RouteReferences: []routeRef{}, RouteUnresolved: []routeUnresolved{}}
	rt := &routeWalk{}
	var workloads []labelled
	var services []selector
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
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(*subject, p)
		rel = filepath.ToSlash(rel)
		if *kubectl != "" {
			objs, kerr := kubectlDecode(*kubectl, p)
			if kerr != nil {
				rejected++
				fmt.Fprintf(os.Stderr, "kubernetes-oracle: kubectl rejects %s: %v\n", rel, kerr)
			} else {
				accepted = append(accepted, rel)
				ds, st := locate(rel, b, objs)
				kdefs = append(kdefs, ds...)
				kst.add(st)
			}
		}
		dec := yaml.NewDecoder(bytes.NewReader(b))
		for {
			var doc yaml.Node
			if err := dec.Decode(&doc); err != nil {
				// io.EOF, or a malformed stream (a Helm template, say):
				// the rest of the file is unreadable to any YAML parser.
				break
			}
			if len(doc.Content) == 0 {
				continue
			}
			root := doc.Content[0]
			rt.document(rel, root)
			kind, name := scalar(get(root, "kind")), scalar(path(root, "metadata", "name"))
			if kind == "" || name == "" {
				continue
			}
			ns := scalar(path(root, "metadata", "namespace"))
			qual := kind + "/" + name
			if ns != "" {
				qual = kind + "/" + ns + "/" + name
			}
			o.Definitions = append(o.Definitions, def{rel, qual, root.Line, "resource"})
			scope := ns
			if scope == "" {
				scope = "default"
			}
			for _, pod := range []*yaml.Node{get(root, "spec"), path(root, "spec", "template", "spec")} {
				for _, list := range []string{"containers", "initContainers"} {
					cs := get(pod, list)
					if cs == nil || cs.Kind != yaml.SequenceNode {
						continue
					}
					for _, c := range cs.Content {
						if img := scalar(get(c, "image")); img != "" {
							o.Definitions = append(o.Definitions, def{rel, img, c.Line, "data_source"})
						}
						for _, e := range seq(get(c, "envFrom")) {
							addRef(&o, rel, qual, scalar(path(e, "configMapRef", "name")))
							addRef(&o, rel, qual, scalar(path(e, "secretRef", "name")))
						}
						for _, e := range seq(get(c, "env")) {
							addRef(&o, rel, qual, scalar(path(e, "valueFrom", "configMapKeyRef", "name")))
							addRef(&o, rel, qual, scalar(path(e, "valueFrom", "secretKeyRef", "name")))
						}
					}
				}
				addRef(&o, rel, qual, scalar(get(pod, "serviceAccountName")))
				for _, v := range seq(get(pod, "volumes")) {
					addRef(&o, rel, qual, scalar(path(v, "configMap", "name")))
					addRef(&o, rel, qual, scalar(path(v, "secret", "secretName")))
				}
			}
			for _, l := range []map[string]string{labels(path(root, "spec", "template", "metadata", "labels")),
				labels(path(root, "spec", "jobTemplate", "spec", "template", "metadata", "labels"))} {
				if l != nil {
					workloads = append(workloads, labelled{rel, qual, scope, l})
				}
			}
			if kind == "Pod" {
				if l := labels(path(root, "metadata", "labels")); l != nil {
					workloads = append(workloads, labelled{rel, qual, scope, l})
				}
			}
			if kind == "Service" {
				if s := labels(path(root, "spec", "selector")); s != nil {
					services = append(services, selector{rel, qual, scope, s})
				}
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "kubernetes-oracle:", err)
		os.Exit(1)
	}
	seen := map[xref]bool{}
	for _, s := range services {
		for _, w := range workloads {
			if w.ns != s.ns || (w.path == s.path && w.name == s.name) {
				continue
			}
			ok := true
			for k, v := range s.sel {
				if w.labels[k] != v {
					ok = false
				}
			}
			x := xref{s.path, s.name, w.path, w.name}
			if ok && !seen[x] {
				seen[x] = true
				o.CrossReferences = append(o.CrossReferences, x)
			}
		}
	}
	o.RouteReferences, o.RouteUnresolved = rt.resolve()
	if *labelJoinsOut != "" {
		if err := rt.lj.write(*labelJoinsOut); err != nil {
			fmt.Fprintln(os.Stderr, "kubernetes-oracle: label joins:", err)
			os.Exit(1)
		}
	}
	if *istioctl != "" {
		c, err := istioCrossCheck(*istioctl, *subject, rt, o.RouteReferences, o.RouteUnresolved)
		if err != nil {
			fmt.Fprintln(os.Stderr, "kubernetes-oracle: istioctl cross-check:", err)
			os.Exit(1)
		}
		o.RouteCrossCheck = c
		fmt.Fprintf(os.Stderr, "kubernetes-oracle: %s: %d of %d Istio references agree\n", c.Tool, c.Agree, c.Checked)
	}
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To
	})
	if *kubectl != "" {
		if err := writeKubectl(o, kdefs, kst, accepted, rejected, *kubectl, *refsOut, *filesOut); err != nil {
			fmt.Fprintln(os.Stderr, "kubernetes-oracle:", err)
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

func seq(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

var refSeen = map[ref]bool{}

func addRef(o *out, path, from, to string) {
	r := ref{path, from, to}
	if to == "" || refSeen[r] {
		return
	}
	refSeen[r] = true
	o.References = append(o.References, r)
}
