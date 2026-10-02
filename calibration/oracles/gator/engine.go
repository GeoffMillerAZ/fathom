package main

// engine.go — the policy half at tier A (LB-Q-gatekeeper-rest): which
// subject objects each attested constraint matches is decided by Gatekeeper's
// own match code, used offline as a Go library
// (github.com/open-policy-agent/gatekeeper/v3 pkg/target, the release gator
// v3.23.1 is built from): K8sValidationTarget.ToMatcher(constraint), then the
// matcher's Match over a review of every object, the evaluation audit and
// gator run (kinds × apiGroups, scope, namespaces, excludedNamespaces,
// labelSelector with matchExpressions, namespaceSelector through the target's
// own Namespace cache, name, source). Nothing here reads spec.match itself.
//
// What stays the dialect's, mirrored and said (README): which documents are
// Kubernetes objects (targets.go objectsOf, the kubernetes dialect's object
// set), the target split (each kinds entry's kind × apiGroup is one target, as
// the dialect emits one policy_ref per cell; the cell is given to Gatekeeper
// as a one-entry kinds list, and the union of the cells is exactly the
// constraint's kinds match), the refusals (a "*" kind without a selector, a
// constraint with no kinds and no selector: wildcard; a variable), and the
// join (the nearest matched objects first, more than 256 refused).

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"sort"
	"strings"

	gktarget "github.com/open-policy-agent/gatekeeper/v3/pkg/target"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// constraintDoc is one attested constraint document.
type constraintDoc struct {
	rel, ent string
	d        *yaml.Node
}

// objDoc is one subject object and its decoded form for the engine.
type objDoc struct {
	object
	u *unstructured.Unstructured
}

type engineResult struct {
	refs       []policyRef
	unresolved []policyUnresolved
	// matched is every (constraint file, object file) pair the engine
	// matched, before the nearest rule and the cap.
	matched map[[2]string]bool
	// wildcard is every constraint file the dialect refuses as a wildcard
	// (no kinds and no matchLabels): Gatekeeper still matches, recorded in
	// matched, but the dialect emits no pair.
	wildcard map[string]bool
	// engineErrors counts Match calls that returned an error (no verdict).
	engineErrors int
	// undecoded counts object documents the engine could not take (a
	// non-string map key, say); they are no candidate.
	undecoded int
	// nsDefaulting counts verdicts that would change if a namespaced object
	// that declares no namespace were placed in "default" (fathom's object
	// node does; gator reads the object as written).
	nsDefaulting int
}

// toUnstructured converts a YAML document to the engine's object form.
func toUnstructured(n *yaml.Node) (*unstructured.Unstructured, error) {
	var v any
	if err := n.Decode(&v); err != nil {
		return nil, err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	if err := u.UnmarshalJSON(b); err != nil {
		return nil, err
	}
	return u, nil
}

// subjectObjects is the dialect's object set (objectsOf) with each object's
// decoded document.
func subjectObjects(files []yamlFile) ([]objDoc, int) {
	charts := chartDirsOf(files)
	var out []objDoc
	bad := 0
	for _, f := range files {
		if claimed(f) {
			continue
		}
		objs := objectsOf(f, charts)
		i := 0
		for _, root := range f.docs {
			if scalar(get(root, "kind")) == "" || scalar(get(root, "metadata", "name")) == "" {
				continue
			}
			if i >= len(objs) {
				break
			}
			o := objs[i]
			i++
			u, err := toUnstructured(root)
			if err != nil {
				bad++
				continue
			}
			out = append(out, objDoc{object: o, u: u})
		}
	}
	return out, bad
}

// cell is one target of a constraint: the dialect's target fields and the
// one-entry kinds list Gatekeeper evaluates it with (nil: no kinds).
type cell struct {
	target
	kinds []any
}

func constraintCells(c constraintDoc) ([]cell, string) {
	m := get(c.d, "spec", "match")
	var scope []string
	for _, n := range scalars(get(m, "namespaces")) {
		scope = append(scope, n.Value)
	}
	sel := get(m, "labelSelector", "matchLabels")
	labels := labelMap(sel)
	var out []cell
	seen := map[[2]string]bool{}
	for _, ks := range items(get(m, "kinds")) {
		gs := []string{}
		// Every group as written, "" (the core group) included: Gatekeeper
		// tells the core group from any group.
		for _, g := range items(get(ks, "apiGroups")) {
			if g.Kind == yaml.ScalarNode {
				gs = append(gs, g.Value)
			}
		}
		if len(gs) == 0 {
			gs = []string{"*"}
		}
		for _, k := range scalars(get(ks, "kinds")) {
			for _, g := range gs {
				if seen[[2]string{g, k.Value}] {
					continue
				}
				seen[[2]string{g, k.Value}] = true
				kt := k.Value
				if kt != "*" && g != "" && g != "*" {
					kt = g + "/" + kt
				}
				t := target{path: c.rel, from: c.ent, raw: k.Value, kind: kt, scope: scope, labels: labels, line: k.Line}
				if kt == "*" && labels != nil {
					t.kind, t.raw = "", canon(labels)
				}
				out = append(out, cell{target: t, kinds: []any{map[string]any{"apiGroups": []any{g}, "kinds": []any{k.Value}}}})
			}
		}
	}
	if len(out) == 0 {
		if labels != nil {
			return []cell{{target: target{path: c.rel, from: c.ent, raw: canon(labels), labels: labels, scope: scope, line: c.d.Line}}}, ""
		}
		return nil, "*"
	}
	return out, ""
}

// engineRefs evaluates every constraint's cells with Gatekeeper's matcher
// over the subject's objects and applies the dialect's join.
func engineRefs(cs []constraintDoc, files []yamlFile) (engineResult, error) {
	res := engineResult{matched: map[[2]string]bool{}, wildcard: map[string]bool{}}
	objs, bad := subjectObjects(files)
	res.undecoded = bad
	tgt := &gktarget.K8sValidationTarget{}
	// The target's own cache holds the subject's Namespace objects, as the
	// constraint framework's data replication fills it for audit.
	for _, o := range objs {
		if o.kind != "Namespace" || o.group != "" {
			continue
		}
		if ok, key, data, err := tgt.ProcessData(o.u); ok && err == nil {
			_ = tgt.GetCache().Add(key, data)
		}
	}
	type review struct{ asWritten, defaulted any }
	reviews := make([]review, len(objs))
	for i, o := range objs {
		_, r, err := tgt.HandleReview(o.u)
		if err != nil {
			return res, fmt.Errorf("%s %s: %w", o.path, o.name, err)
		}
		reviews[i].asWritten = r
		if o.u.GetNamespace() == "" && o.kind != "Namespace" {
			d := o.u.DeepCopy()
			d.SetNamespace("default")
			_, rd, err := tgt.HandleReview(d)
			if err != nil {
				return res, err
			}
			reviews[i].defaulted = rd
		}
	}
	seen := map[policyRef]bool{}
	for _, c := range cs {
		cu, err := toUnstructured(c.d)
		if err != nil {
			return res, fmt.Errorf("%s: %w", c.rel, err)
		}
		cells, wild := constraintCells(c)
		if wild != "" {
			res.unresolved = append(res.unresolved, policyUnresolved{Path: c.rel, From: c.ent, Raw: "*", Reason: "wildcard", Strategy: "resource-kind"})
			res.wildcard[c.rel] = true
			if matcher, err := tgt.ToMatcher(cu); err == nil {
				for i, o := range objs {
					if ok, err := matcher.Match(reviews[i].asWritten); err == nil && ok {
						res.matched[[2]string{c.rel, o.path}] = true
					}
				}
			}
			continue
		}
		for _, cl := range cells {
			t := cl.target
			strategy := "resource-kind"
			if t.kind == "" {
				strategy = "label-selector"
			}
			miss := func(reason string) {
				res.unresolved = append(res.unresolved, policyUnresolved{Path: t.path, From: t.from, Raw: t.raw, Reason: reason, Strategy: strategy})
			}
			bad := variable(t.raw)
			for _, s := range t.scope {
				bad = bad || variable(s)
			}
			for k, v := range t.labels {
				bad = bad || variable(k) || variable(v)
			}
			if bad {
				miss("variable")
				continue
			}
			if t.kind == "*" {
				miss("wildcard")
				continue
			}
			cellU := cu.DeepCopy()
			if cl.kinds != nil {
				if err := unstructured.SetNestedSlice(cellU.Object, cl.kinds, "spec", "match", "kinds"); err != nil {
					return res, err
				}
			} else {
				unstructured.RemoveNestedField(cellU.Object, "spec", "match", "kinds")
			}
			matcher, err := tgt.ToMatcher(cellU)
			if err != nil {
				return res, fmt.Errorf("%s: %w", c.rel, err)
			}
			var cands []object
			for i, o := range objs {
				ok, err := matcher.Match(reviews[i].asWritten)
				if err != nil {
					res.engineErrors++
					continue
				}
				if reviews[i].defaulted != nil {
					if okd, errd := matcher.Match(reviews[i].defaulted); errd == nil && okd != ok {
						res.nsDefaulting++
					}
				}
				if ok {
					cands = append(cands, o.object)
					res.matched[[2]string{c.rel, o.path}] = true
				}
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
					res.refs = append(res.refs, r)
				}
			}
		}
	}
	sort.Slice(res.refs, func(i, j int) bool {
		a, b := res.refs[i], res.refs[j]
		return strings.Join([]string{a.Path, a.From, a.ToPath, a.To, a.Raw}, "\x00") < strings.Join([]string{b.Path, b.From, b.ToPath, b.To, b.Raw}, "\x00")
	})
	sort.Slice(res.unresolved, func(i, j int) bool {
		a, b := res.unresolved[i], res.unresolved[j]
		return strings.Join([]string{a.Path, a.From, a.Raw, a.Reason}, "\x00") < strings.Join([]string{b.Path, b.From, b.Raw, b.Reason}, "\x00")
	})
	if res.engineErrors > 0 || res.undecoded > 0 {
		fmt.Fprintf(os.Stderr, "gator-oracle: engine: %d Match errors (no verdict), %d object documents not decodable (no candidate)\n", res.engineErrors, res.undecoded)
	}
	return res, nil
}

// gatekeeperModVersion is the gatekeeper/v3 module version this binary was
// built with (the go.mod pin), read from the build info.
func gatekeeperModVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Path == "github.com/open-policy-agent/gatekeeper/v3" {
				return d.Version
			}
		}
	}
	return "(unknown)"
}
