package main

// targets.go — the policy-target half of the kyverno and gator oracles (the
// same file in calibration/oracles/kyverno and calibration/oracles/gator):
// an independent gopkg.in/yaml.v3 walk of the subject's Kubernetes objects
// and the documented policy_ref join (docs/design/language-breadth.md §4,
// "Policy references"), written from the contract, never from fathom's code.
// It is tier C: a rule-based reading of what an admission engine would
// match, cross-checked (never scored) by the engine where it runs offline.

import (
	"bytes"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type policyRef struct {
	Path     string `json:"path"`
	From     string `json:"from"`
	ToPath   string `json:"to_path"`
	To       string `json:"to"`
	Raw      string `json:"raw"`
	Strategy string `json:"strategy"`
}

type policyUnresolved struct {
	Path     string `json:"path"`
	From     string `json:"from"`
	Raw      string `json:"raw"`
	Reason   string `json:"reason"`
	Strategy string `json:"strategy"`
}

type crossCheck struct {
	Tool     string   `json:"tool"`
	Tier     string   `json:"tier"`
	Checked  int      `json:"checked"`
	Agree    int      `json:"agree"`
	Disagree []string `json:"disagree"`
	Note     string   `json:"note,omitempty"`
}

// object is one Kubernetes object a policy can govern.
type object struct {
	path, name   string
	kind, group  string
	ns           string
	labels       map[string]string
	hasAPIVerion bool
}

// target is one policy target as the policy writes it: from an entity, a
// kind (Target "Kind" or "group/Kind", "*" every kind, "" unmapped) or none,
// a namespace scope, a label selector.
type target struct {
	path, from string
	raw        string
	kind       string // "" for a selector alone
	unmapped   bool
	scope      []string
	labels     map[string]string
	line       int
}

const fanOutCap = 256

// skipDirs mirrors the directories fathom's discovery never walks.
var skipDirs = map[string]bool{"vendor": true, ".git": true, "testdata": true, "generated": true, "node_modules": true, ".idea": true, ".vscode": true, ".claude": true}

// yamlFile is one .yaml/.yml file and its decoded documents (up to the
// first one that fails to decode).
type yamlFile struct {
	rel  string
	docs []*yaml.Node
	raw  []byte
}

func walkYAML(subject string) ([]yamlFile, error) {
	var out []yamlFile
	err := filepath.WalkDir(subject, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] && p != subject {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
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
		rel, _ := filepath.Rel(subject, p)
		f := yamlFile{rel: filepath.ToSlash(rel), raw: b}
		dec := yaml.NewDecoder(bytes.NewReader(b))
		for {
			var doc yaml.Node
			if err := dec.Decode(&doc); err != nil {
				break
			}
			if len(doc.Content) == 1 {
				f.docs = append(f.docs, doc.Content[0])
			}
		}
		out = append(out, f)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return out, err
}

func get(n *yaml.Node, keys ...string) *yaml.Node {
	for _, key := range keys {
		if n == nil || n.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == key {
				next = n.Content[i+1]
			}
		}
		n = next
	}
	return n
}

func scalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

func items(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

func scalars(n *yaml.Node) []*yaml.Node {
	var out []*yaml.Node
	for _, it := range items(n) {
		if it.Kind == yaml.ScalarNode && it.Value != "" {
			out = append(out, it)
		}
	}
	return out
}

func labelMap(n *yaml.Node) map[string]string {
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

func canon(m map[string]string) string {
	var parts []string
	for k, v := range m {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func qualify(kind, ns, name string) string {
	if ns != "" {
		return kind + "/" + ns + "/" + name
	}
	return kind + "/" + name
}

var (
	topAPIVersion = regexp.MustCompile(`(?m)^apiVersion:\s*\S`)
	topKind       = regexp.MustCompile(`(?m)^kind:\s*\S`)
)

// notManifest are basenames a Kubernetes classifier reads as another tool.
var notManifest = regexp.MustCompile(`^(Chart\.ya?ml|values.*\.ya?ml|kustomization\.ya?ml|(docker-)?compose.*\.ya?ml|\.gitlab-ci\.ya?ml)$`)

// objectsOf reads a file's Kubernetes objects: a file with a top-level
// apiVersion and kind, not a Helm chart's template, workflow, compose or
// kustomization file, gives one object per document with a kind and a
// metadata.name.
func objectsOf(f yamlFile, chartDirs map[string]bool) []object {
	if !topAPIVersion.Match(f.raw) || !topKind.Match(f.raw) || notManifest.MatchString(path.Base(f.rel)) ||
		strings.Contains(f.rel, ".github/workflows/") {
		return nil
	}
	for d := path.Dir(f.rel); d != "." && d != "/"; d = path.Dir(d) {
		if path.Base(d) == "templates" && chartDirs[path.Dir(d)] {
			return nil
		}
	}
	var out []object
	for _, root := range f.docs {
		kind, name := scalar(get(root, "kind")), scalar(get(root, "metadata", "name"))
		if kind == "" || name == "" {
			continue
		}
		ns := scalar(get(root, "metadata", "namespace"))
		av := scalar(get(root, "apiVersion"))
		o := object{path: f.rel, name: qualify(kind, ns, name), kind: kind, ns: ns, labels: labelMap(get(root, "metadata", "labels")), hasAPIVerion: av != ""}
		if o.ns == "" {
			o.ns = "default"
		}
		if g, _, ok := strings.Cut(av, "/"); ok {
			o.group = g
		}
		out = append(out, o)
	}
	return out
}

// chartDirsOf is every directory holding a Chart.yaml.
func chartDirsOf(files []yamlFile) map[string]bool {
	out := map[string]bool{}
	for _, f := range files {
		if b := path.Base(f.rel); b == "Chart.yaml" || b == "Chart.yml" {
			out[path.Dir(f.rel)] = true
		}
	}
	return out
}

func inScope(ns string, scope []string) bool {
	if len(scope) == 0 {
		return true
	}
	for _, p := range scope {
		if p == ns || p == "*" {
			return true
		}
		if ok, _ := path.Match(p, ns); ok {
			return true
		}
	}
	return false
}

func includes(have, sel map[string]string) bool {
	if len(sel) == 0 {
		return false
	}
	for k, v := range sel {
		if have[k] != v {
			return false
		}
	}
	return true
}

func variable(s string) bool {
	return strings.Contains(s, "{{") || strings.Contains(s, "${")
}

// resolveTargets joins every target to the objects it names, nearest the
// policy's file first (its own file, else its directory's subtree, else the
// repository), refusing more than fanOutCap.
func resolveTargets(ts []target, objs []object) ([]policyRef, []policyUnresolved) {
	var refs []policyRef
	var un []policyUnresolved
	seen := map[policyRef]bool{}
	for _, t := range ts {
		strategy := "resource-kind"
		if t.kind == "" {
			strategy = "label-selector"
		}
		miss := func(reason string) {
			un = append(un, policyUnresolved{Path: t.path, From: t.from, Raw: t.raw, Reason: reason, Strategy: strategy})
		}
		bad := variable(t.raw)
		for _, s := range t.scope {
			bad = bad || variable(s)
		}
		for k, v := range t.labels {
			bad = bad || variable(k) || variable(v)
		}
		switch {
		case bad:
			miss("variable")
			continue
		case t.unmapped:
			miss("unmapped")
			continue
		case t.kind == "*", t.kind == "" && len(t.labels) == 0:
			miss("wildcard")
			continue
		}
		group, kind, grouped := "", t.kind, false
		if i := strings.LastIndex(t.kind, "/"); i >= 0 {
			group, kind, grouped = t.kind[:i], t.kind[i+1:], true
		}
		var cands []object
		for _, o := range objs {
			if kind != "" && o.kind != kind {
				continue
			}
			if grouped && o.hasAPIVerion && o.group != group {
				continue
			}
			if !inScope(o.ns, t.scope) {
				continue
			}
			if len(t.labels) > 0 && !includes(o.labels, t.labels) {
				continue
			}
			cands = append(cands, o)
		}
		if len(cands) == 0 {
			miss("undeclared")
			continue
		}
		near := nearest(t.path, cands)
		if len(near) > fanOutCap {
			miss("fan-out")
			continue
		}
		for _, o := range near {
			r := policyRef{Path: t.path, From: t.from, ToPath: o.path, To: o.name, Raw: t.raw, Strategy: strategy}
			if !seen[r] {
				seen[r] = true
				refs = append(refs, r)
			}
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		a, b := refs[i], refs[j]
		return strings.Join([]string{a.Path, a.From, a.ToPath, a.To, a.Raw}, "\x00") < strings.Join([]string{b.Path, b.From, b.ToPath, b.To, b.Raw}, "\x00")
	})
	sort.Slice(un, func(i, j int) bool {
		a, b := un[i], un[j]
		return strings.Join([]string{a.Path, a.From, a.Raw, a.Reason}, "\x00") < strings.Join([]string{b.Path, b.From, b.Raw, b.Reason}, "\x00")
	})
	return refs, un
}

func nearest(file string, cands []object) []object {
	dir := path.Dir(file)
	var same, sub []object
	for _, o := range cands {
		if o.path == file {
			same = append(same, o)
		}
		if dir == "." || strings.HasPrefix(o.path, dir+"/") {
			sub = append(sub, o)
		}
	}
	switch {
	case len(same) > 0:
		return same
	case len(sub) > 0:
		return sub
	}
	return cands
}
