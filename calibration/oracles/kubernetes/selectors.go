package main

// selectors.go — LB-F456 (wave lf8, design §1.8's Kubernetes label-selector
// rule, K19): a Service's selector joins graded by Kubernetes' own matcher,
// k8s.io/apimachinery v0.37.0, tier A for the one decision it attests:
// whether a selector selects a label set.
//
// Each document is read by apimachinery's own decode (utilyaml.NewYAMLReader
// splits the stream, an unstructured.Unstructured holds the object). A core
// v1 Service's spec.selector becomes labels.ValidatedSelectorFromSet (the
// API server's label validation applied); a Service with no selector or an
// empty one selects nothing (the Service API: "If empty or not present, the
// service is assumed to have an external process managing its endpoints"),
// and one the validation refuses selects nothing, counted. The pod labels
// are the manifest's own: a Pod's metadata.labels and any object's pod
// template labels (spec.template.metadata.labels, a CronJob's
// spec.jobTemplate.spec.template.metadata.labels), each a string map
// (unstructured.NestedStringMap: a non-string value is no label set,
// counted). The namespace rule: a Service selects only objects of its own
// namespace, compared as written, an absent metadata.namespace read as
// `default` (kubectl's namespace for an unnamespaced object under an empty
// kubeconfig, the oracle's own environment). An object is named as the
// walk names it, `Kind/[namespace/]name` with the namespace as written.
//
// The pairs are written at tier A as cross_references (-selectors-out), and
// the walk's own selector pairs leave its tier-C -refs-out file, so no pair
// is scored twice; the walk's agreement is disclosed, never scored. The
// route pairs and the name references stay the walk's, tier C, and the
// written L3 ceiling says why (ceilingNote).

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	klabels "k8s.io/apimachinery/pkg/labels"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// apimachineryVersion is the pinned module (go.mod) the matcher comes from.
const apimachineryVersion = "k8s.io/apimachinery v0.37.0"

// ceilingNote is the written L3 ceiling (LB-F456, design §1.8): disclosed
// for every subject whose route pairs or name references are scored.
const ceilingNote = "L3 ceiling: Kubernetes L3 grades good, at tier C, because the xref family also holds the route pairs " +
	"(LB-F99, LB-F315: Ingress and Gateway API backends, Istio destination hosts, subsets and gateways, ServiceEntry hosts, Probe targets, " +
	"and the side pass's ServiceMonitor, PodMonitor and AuthorizationPolicy selectors joined in the nearest scope), which join by DNS and " +
	"object-name rules no Kubernetes library evaluates offline (istioctl analyze cross-checks Istio's, unscored); only the Service selector " +
	"joins are tier A (k8s.io/apimachinery labels.ValidatedSelectorFromSet and Matches). L2 stays good, at tier C, for the same reason: " +
	"serviceAccountName, configMapRef, secretRef, configMapKeyRef, secretKeyRef and volume names are name references the API server's " +
	"admission and the kubelet resolve in a cluster, and no offline tool evaluates (kubectl create --dry-run=client needs discovery)"

// selService is one core v1 Service with a selector.
type selService struct {
	path, entity, ns string
	sel              klabels.Selector
}

// selObject is one object's pod label set.
type selObject struct {
	path, entity, ns string
	set              klabels.Set
}

// selStats counts what the matcher read.
type selStats struct {
	docs, undecodable, services, noSelector, refused, labelSets, badLabels int
}

// selWalk gathers the Services and the label sets of one subject.
type selWalk struct {
	services []selService
	objects  []selObject
	st       selStats
}

// file reads one manifest file through apimachinery's own decode.
func (w *selWalk) file(rel string, src []byte) {
	r := utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(src)))
	for {
		doc, err := r.Read()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			w.st.undecodable++
			return // the stream cannot be split further
		}
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		var obj map[string]any
		if err := utilyaml.Unmarshal(doc, &obj); err != nil || obj == nil {
			w.st.undecodable++
			continue
		}
		u := unstructured.Unstructured{Object: obj}
		kind, name := u.GetKind(), u.GetName()
		if kind == "" || name == "" {
			continue
		}
		w.st.docs++
		ns := u.GetNamespace()
		entity := kind + "/" + name
		if ns != "" {
			entity = kind + "/" + ns + "/" + name
		}
		scope := ns
		if scope == "" {
			scope = "default"
		}
		addSet := func(fields ...string) {
			m, found, err := unstructured.NestedStringMap(obj, fields...)
			switch {
			case err != nil:
				w.st.badLabels++
			case found && len(m) > 0:
				w.st.labelSets++
				w.objects = append(w.objects, selObject{rel, entity, scope, klabels.Set(m)})
			}
		}
		addSet("spec", "template", "metadata", "labels")
		addSet("spec", "jobTemplate", "spec", "template", "metadata", "labels")
		if kind == "Pod" {
			addSet("metadata", "labels")
		}
		if kind == "Service" && u.GetAPIVersion() == "v1" {
			w.st.services++
			m, found, err := unstructured.NestedStringMap(obj, "spec", "selector")
			switch {
			case err != nil:
				w.st.refused++
			case !found || len(m) == 0:
				w.st.noSelector++
			default:
				sel, err := klabels.ValidatedSelectorFromSet(klabels.Set(m))
				if err != nil {
					w.st.refused++
					continue
				}
				w.services = append(w.services, selService{rel, entity, scope, sel})
			}
		}
	}
}

// pairs joins every Service to the objects its selector selects in its
// namespace, deduplicated and in the walk's order.
func (w *selWalk) pairs() []xref {
	seen := map[xref]bool{}
	var out []xref
	for _, s := range w.services {
		for _, o := range w.objects {
			if o.ns != s.ns || (o.path == s.path && o.entity == s.entity) || !s.sel.Matches(o.set) {
				continue
			}
			x := xref{s.path, s.entity, o.path, o.entity}
			if !seen[x] {
				seen[x] = true
				out = append(out, x)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To
	})
	if out == nil {
		out = []xref{}
	}
	return out
}

// selectorsOut is the tier-A cross_references file, in lang-oracle's
// neutral shape.
type selectorsOut struct {
	Tool            string   `json:"tool"`
	Version         string   `json:"version"`
	Tier            string   `json:"tier"`
	CrossReferences []xref   `json:"cross_references"`
	Disclose        []string `json:"disclose,omitempty"`
}

// writeSelectors writes the apimachinery pairs to p, with the walk's
// agreement and the written ceiling disclosed.
func writeSelectors(p string, w *selWalk, walk []xref, ceiling bool) error {
	pairs := w.pairs()
	in := func(xs []xref) map[xref]bool {
		m := map[xref]bool{}
		for _, x := range xs {
			m[x] = true
		}
		return m
	}
	mine, theirs := in(pairs), in(walk)
	onlyWalk, onlyMine := 0, 0
	for _, x := range walk {
		if !mine[x] {
			onlyWalk++
		}
	}
	for _, x := range pairs {
		if !theirs[x] {
			onlyMine++
		}
	}
	st := w.st
	o := selectorsOut{
		Tool:            "kubernetes-oracle selectors (labels.ValidatedSelectorFromSet, Selector.Matches; apimachinery decode)",
		Version:         apimachineryVersion,
		Tier:            "A",
		CrossReferences: pairs,
		Disclose: []string{fmt.Sprintf("Service selector joins by %s (tier A): %d pairs from %d core v1 Services (%d with no or an empty selector, selecting nothing; %d refused by label validation) over %d pod label sets (%d non-string label maps left out) in %d objects (%d documents apimachinery could not decode); the yaml.v3 walk's own selector pairs agree on %d (%d walk only, %d apimachinery only), disclosed, never scored",
			apimachineryVersion, len(pairs), st.services, st.noSelector, st.refused, st.labelSets, st.badLabels, st.docs, st.undecodable, len(pairs)-onlyMine, onlyWalk, onlyMine)},
	}
	if ceiling {
		o.Disclose = append(o.Disclose, ceilingNote)
	}
	fmt.Fprintf(os.Stderr, "kubernetes-oracle: selectors: %d pairs (%d walk only, %d apimachinery only)\n", len(pairs), onlyWalk, onlyMine)
	b, err := json.MarshalIndent(o, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0o644)
}
