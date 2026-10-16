package main

// facts.go — LB-F451's field-for-field parity truth (wave lf8): the
// per-container facts fathom's Kubernetes extractor stores (requests and
// limits of cpu, memory and ephemeral-storage, the three probes' handler,
// port, httpGet path and timings, the container and init-container lists,
// replicas), read by an independent structural decode, k8s.io/api's typed
// structs. Never fathom's code.
//
// Each document is split and decoded by apimachinery (utilyaml.NewYAMLReader,
// utilyaml.Unmarshal), its apiVersion and kind looked up in a scheme holding
// the built-in workload groups (core/v1; apps/v1, v1beta1, v1beta2;
// extensions/v1beta1; batch/v1, v1beta1), and converted to the typed object
// (runtime.DefaultUnstructuredConverter.FromUnstructured). The pod spec is
// the kind's own: a Pod's spec, a workload's spec.template.spec, a CronJob's
// spec.jobTemplate.spec.template.spec (the fact's `class`: spec, template,
// jobTemplate). A field the typed struct holds as a plain int32 (a probe's
// timings) is a fact only when the document writes it (its unstructured
// key); a quantity is compared as Kubernetes reads it (resource.Quantity's
// canonical String). A document of another kind, or one the typed decode
// refuses (a templated quantity, a wrong type), is no truth: fathom's facts
// on it are counted, never scored. No env, args, command or exec command is
// read.
//
// With -facts-fathom (the K8sFact rows of a scratch store of the same
// subject, `sqlite3 -json`), the comparison is written beside the truth.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// apiVersionPin is the typed structs' module (go.mod).
const apiVersionPin = "k8s.io/api v0.37.0"

// tfact is one fact of the truth (or of fathom's rows): the entity, the key
// and the value, and the pod path class of its document.
type tfact struct {
	Path   string `json:"path"`
	Entity string `json:"entity"`
	Key    string `json:"key"`
	Value  string `json:"value"`
	Class  string `json:"class,omitempty"`
}

// factsScheme holds the built-in workload kinds.
var factsScheme = func() *runtime.Scheme {
	s := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, appsv1beta1.AddToScheme,
		appsv1beta2.AddToScheme, extv1beta1.AddToScheme, batchv1.AddToScheme, batchv1beta1.AddToScheme} {
		if err := add(s); err != nil {
			panic(err)
		}
	}
	return s
}()

// podOf is a typed object's pod spec, its class and, for a kind that sizes
// a pod template, its replicas; ok is false for a kind with no pod spec.
func podOf(obj runtime.Object) (class string, pod *corev1.PodSpec, replicas *int32, sized bool, ok bool) {
	switch o := obj.(type) {
	case *corev1.Pod:
		return "spec", &o.Spec, nil, false, true
	case *corev1.ReplicationController:
		if o.Spec.Template == nil {
			return "template", &corev1.PodSpec{}, o.Spec.Replicas, true, true
		}
		return "template", &o.Spec.Template.Spec, o.Spec.Replicas, true, true
	case *appsv1.Deployment:
		return "template", &o.Spec.Template.Spec, o.Spec.Replicas, true, true
	case *appsv1.StatefulSet:
		return "template", &o.Spec.Template.Spec, o.Spec.Replicas, true, true
	case *appsv1.ReplicaSet:
		return "template", &o.Spec.Template.Spec, o.Spec.Replicas, true, true
	case *appsv1.DaemonSet:
		return "template", &o.Spec.Template.Spec, nil, false, true
	case *appsv1beta1.Deployment:
		return "template", &o.Spec.Template.Spec, o.Spec.Replicas, true, true
	case *appsv1beta1.StatefulSet:
		return "template", &o.Spec.Template.Spec, o.Spec.Replicas, true, true
	case *appsv1beta2.Deployment:
		return "template", &o.Spec.Template.Spec, o.Spec.Replicas, true, true
	case *appsv1beta2.StatefulSet:
		return "template", &o.Spec.Template.Spec, o.Spec.Replicas, true, true
	case *appsv1beta2.ReplicaSet:
		return "template", &o.Spec.Template.Spec, o.Spec.Replicas, true, true
	case *appsv1beta2.DaemonSet:
		return "template", &o.Spec.Template.Spec, nil, false, true
	case *extv1beta1.Deployment:
		return "template", &o.Spec.Template.Spec, o.Spec.Replicas, true, true
	case *extv1beta1.ReplicaSet:
		return "template", &o.Spec.Template.Spec, o.Spec.Replicas, true, true
	case *extv1beta1.DaemonSet:
		return "template", &o.Spec.Template.Spec, nil, false, true
	case *batchv1.Job:
		return "template", &o.Spec.Template.Spec, nil, false, true
	case *batchv1.CronJob:
		return "jobTemplate", &o.Spec.JobTemplate.Spec.Template.Spec, nil, false, true
	case *batchv1beta1.CronJob:
		return "jobTemplate", &o.Spec.JobTemplate.Spec.Template.Spec, nil, false, true
	}
	return "", nil, nil, false, false
}

// classPath is where a class's pod spec lives in the unstructured object.
var classPath = map[string][]string{
	"spec":        {"spec"},
	"template":    {"spec", "template", "spec"},
	"jobTemplate": {"spec", "jobTemplate", "spec", "template", "spec"},
}

// factsStats counts what the typed decode read.
type factsStats struct {
	Docs        int            `json:"docs"`
	Typed       int            `json:"typed"`
	TypedByPath map[string]int `json:"typed_by_class"`
	Refused     int            `json:"refused"`
	RefusedDocs []string       `json:"refused_docs,omitempty"`
	OtherKinds  int            `json:"other_kinds"`
	Unsplit     int            `json:"unsplit_files"`
}

// factsWalk gathers one subject's truth.
type factsWalk struct {
	facts    []tfact
	entities map[[2]string]string // (path, entity) -> class
	st       factsStats
}

func (w *factsWalk) file(rel string, src []byte) {
	if w.entities == nil {
		w.entities, w.st.TypedByPath = map[[2]string]string{}, map[string]int{}
	}
	r := utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(src)))
	for {
		doc, err := r.Read()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			w.st.Unsplit++
			return
		}
		var m map[string]any
		if err := utilyaml.Unmarshal(doc, &m); err != nil || m == nil {
			continue
		}
		u := unstructured.Unstructured{Object: m}
		kind, name := u.GetKind(), u.GetName()
		if kind == "" || name == "" {
			continue
		}
		w.st.Docs++
		gv, err := schema.ParseGroupVersion(u.GetAPIVersion())
		if err != nil {
			w.st.OtherKinds++
			continue
		}
		obj, err := factsScheme.New(gv.WithKind(kind))
		if err != nil {
			w.st.OtherKinds++
			continue
		}
		if _, _, _, _, ok := podOf(obj); !ok {
			w.st.OtherKinds++
			continue
		}
		entity := kind + "/" + name
		if ns := u.GetNamespace(); ns != "" {
			entity = kind + "/" + ns + "/" + name
		}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(m, obj); err != nil {
			w.st.Refused++
			if len(w.st.RefusedDocs) < 10 {
				w.st.RefusedDocs = append(w.st.RefusedDocs, rel+" "+entity+": "+firstLine(err.Error()))
			}
			continue
		}
		class, pod, replicas, sized, _ := podOf(obj)
		w.st.Typed++
		w.st.TypedByPath[class]++
		w.entities[[2]string{rel, entity}] = class
		add := func(key, value string) {
			w.facts = append(w.facts, tfact{rel, entity, key, value, class})
		}
		if sized && replicas != nil {
			add("replicas", strconv.Itoa(int(*replicas)))
		}
		raw, _, _ := unstructured.NestedMap(m, classPath[class]...)
		for _, list := range []struct {
			key string
			cs  []corev1.Container
		}{{"initContainers", pod.InitContainers}, {"containers", pod.Containers}} {
			if len(list.cs) == 0 {
				continue
			}
			names := make([]string, len(list.cs))
			for i, c := range list.cs {
				names[i] = c.Name
			}
			add(list.key, strings.Join(names, ","))
			rawList, _, _ := unstructured.NestedSlice(raw, list.key)
			for i, c := range list.cs {
				if c.Name == "" {
					continue
				}
				prefix := list.key + "." + c.Name + "."
				for _, b := range []struct {
					key string
					rl  corev1.ResourceList
				}{{"requests", c.Resources.Requests}, {"limits", c.Resources.Limits}} {
					for _, res := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory, corev1.ResourceEphemeralStorage} {
						if q, ok := b.rl[res]; ok {
							add(prefix+"resources."+b.key+"."+string(res), q.String())
						}
					}
				}
				var rawC map[string]any
				if i < len(rawList) {
					rawC, _ = rawList[i].(map[string]any)
				}
				for _, p := range []struct {
					key   string
					probe *corev1.Probe
				}{{"livenessProbe", c.LivenessProbe}, {"readinessProbe", c.ReadinessProbe}, {"startupProbe", c.StartupProbe}} {
					if p.probe == nil {
						continue
					}
					h := p.probe.ProbeHandler
					handler := "none"
					switch {
					case h.Exec != nil:
						handler = "exec"
					case h.HTTPGet != nil:
						handler = "httpGet"
						if h.HTTPGet.Path != "" {
							add(prefix+p.key+".path", h.HTTPGet.Path)
						}
						if has(rawC, p.key, "httpGet", "port") {
							add(prefix+p.key+".port", h.HTTPGet.Port.String())
						}
					case h.TCPSocket != nil:
						handler = "tcpSocket"
						if has(rawC, p.key, "tcpSocket", "port") {
							add(prefix+p.key+".port", h.TCPSocket.Port.String())
						}
					case h.GRPC != nil:
						handler = "grpc"
						if has(rawC, p.key, "grpc", "port") {
							add(prefix+p.key+".port", strconv.Itoa(int(h.GRPC.Port)))
						}
					}
					add(prefix+p.key, handler)
					for _, t := range []struct {
						key string
						v   int32
					}{{"initialDelaySeconds", p.probe.InitialDelaySeconds}, {"periodSeconds", p.probe.PeriodSeconds},
						{"timeoutSeconds", p.probe.TimeoutSeconds}, {"successThreshold", p.probe.SuccessThreshold},
						{"failureThreshold", p.probe.FailureThreshold}} {
						if has(rawC, p.key, t.key) {
							add(prefix+p.key+"."+t.key, strconv.Itoa(int(t.v)))
						}
					}
				}
			}
		}
	}
}

// has reports whether the unstructured map writes the nested key.
func has(m map[string]any, keys ...string) bool {
	_, found, err := unstructured.NestedFieldNoCopy(m, keys...)
	return found && err == nil
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(s, "\n")
	return l
}

// fathomRow is one K8sFact row of a scratch store (sqlite3 -json).
type fathomRow struct {
	Path   string `json:"path"`
	Symbol string `json:"symbol"`
	Claim  string `json:"claim"`
}

// parityClass is one pod path class's comparison.
type parityClass struct {
	Truth   int `json:"truth"`
	Fathom  int `json:"fathom"`
	Matched int `json:"matched"`
}

// parity is the comparison of fathom's rows with the truth on the entities
// the typed decode read.
type parity struct {
	ByClass       map[string]*parityClass `json:"by_class"`
	Truth         int                     `json:"truth"`
	Fathom        int                     `json:"fathom"`
	Matched       int                     `json:"matched"`
	FathomUntyped int                     `json:"fathom_on_untyped_entities"`
	// TruthOutOfScope counts the truth's facts on files fathom does not read.
	TruthOutOfScope int `json:"truth_on_files_fathom_does_not_read"`
	// UntypedEntities names the entities fathom's rows fall on that the
	// typed decode did not read (another kind, or a document it refused).
	UntypedEntities []string `json:"untyped_entities,omitempty"`
	TruthOnly       []tfact  `json:"truth_only,omitempty"`
	FathomOnly      []tfact  `json:"fathom_only,omitempty"`
	TruthOnlyCount  int      `json:"truth_only_count"`
	FathomOnlyCnt   int      `json:"fathom_only_count"`
}

// parseQuantity is a value as Kubernetes reads a quantity, canonical.
func parseQuantity(v string) (string, error) {
	q, err := resource.ParseQuantity(v)
	if err != nil {
		return "", err
	}
	return q.String(), nil
}

// isQuantityKey reports whether a fact's value is a resource.Quantity.
func isQuantityKey(k string) bool { return strings.Contains(k, ".resources.") }

// compare reads fathom's rows and scores them against the truth. With
// filesPath (the store's file list, one path per line), a truth fact on a
// file fathom does not read (discovery's exclusions: testdata, vendor) is
// out of scope, counted, never a miss (§1.4 Scope).
func (w *factsWalk) compare(rowsPath, filesPath string) (*parity, error) {
	b, err := os.ReadFile(rowsPath)
	if err != nil {
		return nil, err
	}
	var read map[string]bool
	if filesPath != "" {
		fb, err := os.ReadFile(filesPath)
		if err != nil {
			return nil, err
		}
		read = map[string]bool{}
		for _, l := range strings.Split(string(fb), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				read[l] = true
			}
		}
	}
	var rows []fathomRow
	if len(bytes.TrimSpace(b)) > 0 {
		if err := json.Unmarshal(b, &rows); err != nil {
			return nil, fmt.Errorf("%s: %w", rowsPath, err)
		}
	}
	key := func(f tfact) tfact { f.Class = ""; return f }
	truth := map[tfact]string{}
	outOfScope := map[tfact]bool{}
	for _, f := range w.facts {
		if read != nil && !read[f.Path] {
			outOfScope[key(f)] = true
			continue
		}
		truth[key(f)] = f.Class
	}
	p := &parity{ByClass: map[string]*parityClass{}, TruthOutOfScope: len(outOfScope)}
	for _, c := range []string{"spec", "template", "jobTemplate"} {
		p.ByClass[c] = &parityClass{}
	}
	mine := map[tfact]bool{}
	for _, r := range rows {
		entity, k, ok := strings.Cut(r.Symbol, "#")
		if !ok {
			continue
		}
		class, typed := w.entities[[2]string{r.Path, entity}]
		if !typed {
			p.FathomUntyped++
			if e := r.Path + " " + entity; !slices.Contains(p.UntypedEntities, e) {
				p.UntypedEntities = append(p.UntypedEntities, e)
			}
			continue
		}
		v := r.Claim
		if isQuantityKey(k) {
			if q, err := parseQuantity(v); err == nil {
				v = q
			}
		}
		f := tfact{Path: r.Path, Entity: entity, Key: k, Value: v}
		if mine[f] {
			continue
		}
		mine[f] = true
		p.Fathom++
		p.ByClass[class].Fathom++
		if _, ok := truth[f]; ok {
			p.Matched++
			p.ByClass[class].Matched++
		} else {
			p.FathomOnlyCnt++
			if len(p.FathomOnly) < 20 {
				f.Class = class
				p.FathomOnly = append(p.FathomOnly, f)
			}
		}
	}
	keys := make([]tfact, 0, len(truth))
	for f := range truth {
		keys = append(keys, f)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		return a.Path+"\x00"+a.Entity+"\x00"+a.Key < b.Path+"\x00"+b.Entity+"\x00"+b.Key
	})
	for _, f := range keys {
		class := truth[f]
		p.Truth++
		p.ByClass[class].Truth++
		if !mine[f] {
			p.TruthOnlyCount++
			if len(p.TruthOnly) < 20 {
				f.Class = class
				p.TruthOnly = append(p.TruthOnly, f)
			}
		}
	}
	return p, nil
}

// factsOut is the truth file.
type factsOut struct {
	Tool    string     `json:"tool"`
	Version string     `json:"version"`
	Tier    string     `json:"tier"`
	Stats   factsStats `json:"stats"`
	Facts   []tfact    `json:"facts"`
	Parity  *parity    `json:"parity,omitempty"`
}

// writeFacts writes the truth (and, with rowsPath, the comparison) to p.
func writeFacts(p string, w *factsWalk, rowsPath, filesPath string) error {
	sort.SliceStable(w.facts, func(i, j int) bool {
		a, b := w.facts[i], w.facts[j]
		return a.Path+"\x00"+a.Entity+"\x00"+a.Key < b.Path+"\x00"+b.Entity+"\x00"+b.Key
	})
	if w.facts == nil {
		w.facts = []tfact{}
	}
	o := factsOut{Tool: "kubernetes-oracle facts (k8s.io/api typed structs; apimachinery decode)", Version: apiVersionPin + ", " + apimachineryVersion, Tier: "A", Stats: w.st, Facts: w.facts}
	if rowsPath != "" {
		par, err := w.compare(rowsPath, filesPath)
		if err != nil {
			return err
		}
		o.Parity = par
		fmt.Fprintf(os.Stderr, "kubernetes-oracle: facts: truth %d, fathom %d, matched %d (truth only %d, fathom only %d, fathom on untyped entities %d, truth on files fathom does not read %d)\n",
			par.Truth, par.Fathom, par.Matched, par.TruthOnlyCount, par.FathomOnlyCnt, par.FathomUntyped, par.TruthOutOfScope)
	}
	b, err := json.MarshalIndent(o, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0o644)
}
