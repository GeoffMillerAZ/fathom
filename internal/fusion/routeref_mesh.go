package fusion

// routeref_mesh.go — LB-F307, LB-F312 (wave c1, item mesh): the route pass's
// Kubernetes side pass beyond route hosts. ingest still dispatches Kubernetes
// manifests to k8sextract.go (the kubernetes dialect is measured, not
// dispatched; the switch is its own row), so what the dialect's route-host
// entries cannot say is read here, from the manifest itself, and joined by
// ResolveRouteRefs to the objects the incumbent stores (`Kind/<ns>/<name>`):
//
//   - Istio VirtualService `spec.gateways` (`<ns>/<name>`, `<name>` in the
//     rule's namespace, the legacy `<name>.<ns>.svc.cluster.local`; `mesh`
//     names no object) to the Gateway; a destination's `subset` to the
//     DestinationRule declaring that subset for the same host (both hosts
//     read by the Kubernetes and Istio DNS rules, routeTarget).
//   - Istio ServiceEntry: each non-wildcard host is a provider of its
//     `deploy:` key (a cluster form names the Service's key, anything else
//     deploy:host/<host>), so a route to an external host joins the entry
//     that declares it; the entry's own hosts and endpoint addresses are
//     the dialect's route-host references.
//   - Istio AuthorizationPolicy: `spec.selector.matchLabels` to every
//     workload in the policy's namespace whose pod template carries those
//     labels; `rules[].from[].source.principals`
//     (`<trust domain>/ns/<ns>/sa/<name>`) to the ServiceAccount.
//   - Prometheus-operator ServiceMonitor and PodMonitor: `spec.selector`
//     (matchLabels and matchExpressions) over the Services' own labels, or
//     the workloads' pod-template labels, in the namespaces
//     `spec.namespaceSelector` names (`any: true`, `matchNames`, else the
//     monitor's own).
//
// A selector joins every match in the nearest scope holding one (the
// referring file, else its directory, else the repository), the route-host
// rule's nearness; an object or subset reference joins the nearest
// declarations of its key. Unresolved is counted by the route reasons: an
// object or a selection no manifest declares is undeclared, a wildcard or
// non-Kubernetes principal external. Out of scope (K15): the Istio
// Telemetry CRD, mirrors and delegates, VMRule and VMServiceScrape.
//
// Storage is the route-ref pass's (typed_edges "route-ref", file tier,
// never a gate, I57): no table, cache or job is added.

import (
	"bytes"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// The side pass's own reference strategies: the route pass alone reads
// them (never a doc-engine strategy).
const (
	// RouteObjectStrategy: Target names one Kubernetes object,
	// `Kind/<namespace>/<name>`; "" when the value names none (a wildcard
	// or non-Kubernetes principal: external).
	RouteObjectStrategy = "route-object"
	// RouteSubsetStrategy: Raw is a destination host as written, Target the
	// subset it names; it joins the DestinationRule declaring that subset
	// for the same host.
	RouteSubsetStrategy = "route-subset"
	// RouteSelectorStrategy: Raw (= Target) is a label selector in kubectl's
	// string form ("{}" selects every object), Scope `<class>/<namespace>`
	// (`*` for any namespace): class pods matches a workload's pod-template
	// labels, services a Service's own labels.
	RouteSelectorStrategy = "route-selector"
	// routeProvidesStrategy declares, never references: From provides the
	// key Raw names (a ServiceEntry host, Target ""), or the subset Target
	// of the DestinationRule host Raw.
	routeProvidesStrategy = "route-provides"
)

// Selector classes (RouteSelectorStrategy scopes, label-set scopes).
const (
	selectPods     = "pods"
	selectServices = "services"
)

// The side pass's edge names (DocCrossFileRef.Edge; disclosure only, the
// stored edge is the file-tier pair).
const (
	edgeMeshGateway     = "mesh_gateway"
	edgeMeshSubset      = "mesh_subset"
	edgeAuthzSelector   = "authz_selector"
	edgeAuthzPrincipal  = "authz_principal"
	edgeMonitorSelector = "monitor_selector"
	edgeProvidesHost    = "provides_host"
	edgeProvidesSubset  = "provides_subset"
)

// k8sObject is one Kubernetes object of a manifest: a document with kind and
// metadata.name, as k8sextract.go names it.
type k8sObject struct {
	kind, api, name string
	ns              string // "" when metadata.namespace is absent
	root            *yaml.Node
	offset          int // a node's absolute line is offset + node.Line
}

// entity is the incumbent's name for the object: `Kind/<ns>/<name>`, or
// `Kind/<name>` with no namespace written.
func (o k8sObject) entity() string {
	if o.ns != "" {
		return o.kind + "/" + o.ns + "/" + o.name
	}
	return o.kind + "/" + o.name
}

// namespace is the namespace the object lives in ("default" when none is
// written, the route rule's reading).
func (o k8sObject) namespace() string {
	if o.ns != "" {
		return o.ns
	}
	return "default"
}

func (o k8sObject) line(n *yaml.Node) int {
	if n == nil {
		return 0
	}
	return o.offset + n.Line
}

// group is the API group of the object's apiVersion ("" for the core group).
func (o k8sObject) group() string {
	if i := strings.Index(o.api, "/"); i >= 0 {
		return o.api[:i]
	}
	return ""
}

// k8sObjects splits a manifest the way k8sextract.go does (a line that is
// `---` once trimmed ends a document; a document that does not decode is
// skipped, best-effort) and returns its objects in document order.
func k8sObjects(content []byte) []k8sObject {
	var out []k8sObject
	var cur bytes.Buffer
	start, line := 1, 1
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(cur.Bytes(), &doc); err == nil && len(doc.Content) > 0 {
			root := doc.Content[0]
			kind, name := yScalar(yGet(root, "kind")), yScalar(yPath(root, "metadata", "name"))
			if kind != "" && name != "" {
				out = append(out, k8sObject{kind: kind, api: yScalar(yGet(root, "apiVersion")), name: name,
					ns: yScalar(yPath(root, "metadata", "namespace")), root: root, offset: start - 1})
			}
		}
		cur.Reset()
	}
	for _, raw := range bytes.Split(content, []byte("\n")) {
		if string(bytes.TrimSpace(raw)) == "---" {
			flush()
			line++
			start = line
			continue
		}
		cur.Write(raw)
		cur.WriteByte('\n')
		line++
	}
	flush()
	return out
}

// isK8sTool: the tool kinds ingest dispatches Kubernetes manifests as.
func isK8sTool(tk ToolKind) bool { return tk == ToolK8sManifest || tk == ToolK8sCRD }

// meshScan reads one Kubernetes manifest's side-pass facts: the object,
// subset and selector references and provider declarations its mesh and
// monitoring objects make, and the label sets its workloads (pod
// templates) and Services carry. Sorted by line, deterministic.
func meshScan(content []byte) (refs []extractor.DocCrossFileRef, labels []extractor.DocLabelSet) {
	for _, o := range k8sObjects(content) {
		refs = append(refs, meshObjectRefs(o)...)
		labels = append(labels, objectLabelSets(o)...)
	}
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].Line != refs[j].Line {
			return refs[i].Line < refs[j].Line
		}
		return refs[i].Raw < refs[j].Raw
	})
	return refs, labels
}

// objectLabelSets: a workload's pod-template labels (the kubernetes
// dialect's labels entries: spec.template, a CronJob's job template, a bare
// Pod's own) and a core Service's own labels.
func objectLabelSets(o k8sObject) []extractor.DocLabelSet {
	var out []extractor.DocLabelSet
	add := func(class string, n *yaml.Node) {
		if m, ok := yLabelMap(n); ok {
			out = append(out, extractor.DocLabelSet{Entity: o.entity(), Scope: class + "/" + o.namespace(), Labels: m, Line: o.line(n)})
		}
	}
	add(selectPods, yPath(o.root, "spec", "template", "metadata", "labels"))
	add(selectPods, yPath(o.root, "spec", "jobTemplate", "spec", "template", "metadata", "labels"))
	if o.kind == "Pod" {
		add(selectPods, yPath(o.root, "metadata", "labels"))
	}
	if o.kind == "Service" && o.group() == "" {
		add(selectServices, yPath(o.root, "metadata", "labels"))
	}
	return out
}

// yLabelMap is a non-empty mapping of scalars (a label map); anything else
// is none.
func yLabelMap(n *yaml.Node) (map[string]string, bool) {
	if n == nil || n.Kind != yaml.MappingNode || len(n.Content) == 0 {
		return nil, false
	}
	m := map[string]string{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i+1].Kind != yaml.ScalarNode {
			return nil, false
		}
		m[n.Content[i].Value] = n.Content[i+1].Value
	}
	return m, true
}

// meshObjectRefs is one object's side-pass references and declarations.
func meshObjectRefs(o k8sObject) []extractor.DocCrossFileRef {
	var out []extractor.DocCrossFileRef
	ref := func(edge, strategy, raw, target string, n *yaml.Node) {
		out = append(out, extractor.DocCrossFileRef{From: o.entity(), Edge: edge, Strategy: strategy, Raw: raw, Target: target, Line: o.line(n)})
	}
	switch g := o.group(); {
	case g == "networking.istio.io" && o.kind == "VirtualService":
		for _, gw := range ySeq(yPath(o.root, "spec", "gateways")) {
			if v := yScalar(gw); v != "" && v != "mesh" {
				ref(edgeMeshGateway, RouteObjectStrategy, v, gatewayObject(v, o.namespace()), gw)
			}
		}
		for _, proto := range []string{"http", "tcp", "tls"} {
			for _, r := range ySeq(yPath(o.root, "spec", proto)) {
				for _, d := range ySeq(yGet(r, "route")) {
					host, sub := yPath(d, "destination", "host"), yPath(d, "destination", "subset")
					if h, s := yScalar(host), yScalar(sub); h != "" && s != "" {
						ref(edgeMeshSubset, RouteSubsetStrategy, h, s, sub)
					}
				}
			}
		}
	case g == "networking.istio.io" && o.kind == "DestinationRule":
		host := yPath(o.root, "spec", "host")
		if h := yScalar(host); h != "" {
			for _, s := range ySeq(yPath(o.root, "spec", "subsets")) {
				if n := yGet(s, "name"); yScalar(n) != "" {
					ref(edgeProvidesSubset, routeProvidesStrategy, h, yScalar(n), n)
				}
			}
		}
	case g == "networking.istio.io" && o.kind == "ServiceEntry":
		for _, h := range ySeq(yPath(o.root, "spec", "hosts")) {
			if v := yScalar(h); v != "" {
				ref(edgeProvidesHost, routeProvidesStrategy, v, "", h)
			}
		}
	case g == "security.istio.io" && o.kind == "AuthorizationPolicy":
		sel := yPath(o.root, "spec", "selector", "matchLabels")
		if m, ok := yLabelMap(sel); ok {
			s := labelSelectorOf(m, nil).String()
			out = append(out, extractor.DocCrossFileRef{From: o.entity(), Edge: edgeAuthzSelector, Strategy: RouteSelectorStrategy,
				Raw: s, Target: s, Scope: selectPods + "/" + o.namespace(), Labels: m, Line: o.line(sel)})
		}
		for _, r := range ySeq(yPath(o.root, "spec", "rules")) {
			for _, f := range ySeq(yGet(r, "from")) {
				for _, p := range ySeq(yPath(f, "source", "principals")) {
					if v := yScalar(p); v != "" {
						ref(edgeAuthzPrincipal, RouteObjectStrategy, v, principalObject(v), p)
					}
				}
			}
		}
	case g == "monitoring.coreos.com" && (o.kind == "ServiceMonitor" || o.kind == "PodMonitor"):
		class := selectServices
		if o.kind == "PodMonitor" {
			class = selectPods
		}
		selNode := yPath(o.root, "spec", "selector")
		sel, ok := yLabelSelector(selNode)
		if !ok {
			break
		}
		s := sel.String()
		for _, ns := range monitorNamespaces(o) {
			out = append(out, extractor.DocCrossFileRef{From: o.entity(), Edge: edgeMonitorSelector, Strategy: RouteSelectorStrategy,
				Raw: s, Target: s, Scope: class + "/" + ns, Labels: sel.equalities(), Line: o.line(selNode)})
		}
	}
	return out
}

// monitorNamespaces: the namespaces a ServiceMonitor or PodMonitor selects
// in (prometheus-operator's NamespaceSelector): `any: true` is every one
// ("*"), `matchNames` those, else the monitor's own.
func monitorNamespaces(o k8sObject) []string {
	ns := yPath(o.root, "spec", "namespaceSelector")
	if yScalar(yGet(ns, "any")) == "true" {
		return []string{"*"}
	}
	var out []string
	seen := map[string]bool{}
	for _, n := range ySeq(yGet(ns, "matchNames")) {
		if v := yScalar(n); v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return []string{o.namespace()}
	}
	sort.Strings(out)
	return out
}

// gatewayObject is the Gateway a VirtualService's gateways entry names
// (Istio's gateway name resolution): `<ns>/<name>`; the legacy
// `<name>.<ns>[.svc.cluster.local]`; else `<name>` in the rule's own
// namespace. A wildcard names no one object ("").
func gatewayObject(v, ns string) string {
	if strings.Contains(v, "*") {
		return ""
	}
	if a, b, ok := strings.Cut(v, "/"); ok {
		if a == "" || b == "" {
			return ""
		}
		return "Gateway/" + a + "/" + b
	}
	if parts := strings.Split(v, "."); len(parts) >= 2 {
		return "Gateway/" + parts[1] + "/" + parts[0]
	}
	return "Gateway/" + ns + "/" + v
}

// principalObject is the ServiceAccount an Istio principal names
// (`[spiffe://]<trust domain>/ns/<ns>/sa/<name>`), "" for a wildcard or any
// other form.
func principalObject(p string) string {
	if strings.Contains(p, "*") {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(p, "spiffe://"), "/")
	if len(parts) != 5 || parts[0] == "" || parts[1] != "ns" || parts[2] == "" || parts[3] != "sa" || parts[4] == "" {
		return ""
	}
	return "ServiceAccount/" + parts[2] + "/" + parts[4]
}

// objectKey is the provider key of a stored Kubernetes object node
// (`obj:Kind/<ns>/<name>`, namespace default when none is written), "" for
// any other node.
func objectKey(n core.SymbolNode) string {
	if n.Kind != core.KindK8sWorkload {
		return ""
	}
	return objectKeyOf(n.Name)
}

// objectKeyOf normalizes `Kind/<ns>/<name>` or `Kind/<name>` to its key.
func objectKeyOf(name string) string {
	switch parts := strings.Split(name, "/"); len(parts) {
	case 2:
		if parts[0] != "" && parts[1] != "" {
			return "obj:" + parts[0] + "/default/" + parts[1]
		}
	case 3:
		if parts[0] != "" && parts[1] != "" && parts[2] != "" {
			return "obj:" + name
		}
	}
	return ""
}

// subsetKey is a DestinationRule subset's provider key for one of its
// host's deploy: keys.
func subsetKey(deploy, subset string) string { return "subset:" + deploy + "#" + subset }

// providedKeys are the keys a route-provides declaration gives its node,
// whose namespace is ns: a ServiceEntry host's deploy: key (a cluster form
// names the Service's, anything else with a dot deploy:host/<host>; a
// short name or a wildcard none), or a DestinationRule subset's key under
// each deploy: key its host may name.
func providedKeys(c extractor.DocCrossFileRef, ns string) []string {
	if c.Edge == edgeProvidesSubset {
		keys, _, reason := routeTarget(c.Raw, ns, true)
		if reason != "" {
			return nil
		}
		out := make([]string, 0, len(keys))
		for _, k := range keys {
			out = append(out, subsetKey(k, c.Target))
		}
		return out
	}
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(c.Raw)), ".")
	if host == "" || strings.ContainsAny(host, "*$/:{}") || !strings.Contains(host, ".") {
		return nil
	}
	labels := strings.Split(host, ".")
	if len(labels) == 3 && labels[2] == "svc" || len(labels) == 5 && labels[2] == "svc" && labels[3] == "cluster" && labels[4] == "local" {
		if k := deployKey("k8s/" + labels[1] + "/" + labels[0]); k != "" {
			return []string{k}
		}
		return nil
	}
	if k := deployKey("host/" + host); k != "" {
		return []string{k}
	}
	return nil
}

// labelReq is one requirement of a label selector.
type labelReq struct {
	key, op string // op: "=", "!=", "in", "notin", "exists", "!"
	vals    []string
}

// labelSelector is a Kubernetes label selector: every requirement holds.
// No requirement selects every object.
type labelSelector []labelReq

// labelSelectorOf is matchLabels and the requirements of matchExpressions.
func labelSelectorOf(match map[string]string, exprs []labelReq) labelSelector {
	var s labelSelector
	for k, v := range match {
		s = append(s, labelReq{key: k, op: "=", vals: []string{v}})
	}
	s = append(s, exprs...)
	sort.SliceStable(s, func(i, j int) bool {
		if s[i].key != s[j].key {
			return s[i].key < s[j].key
		}
		return s[i].op < s[j].op
	})
	return s
}

// yLabelSelector reads a metav1.LabelSelector node (matchLabels,
// matchExpressions with In, NotIn, Exists, DoesNotExist). A present but
// empty selector selects everything; an absent one, or an operator outside
// the four, is none.
func yLabelSelector(n *yaml.Node) (labelSelector, bool) {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil, false
	}
	match := map[string]string{}
	if ml := yGet(n, "matchLabels"); ml != nil {
		m, ok := yLabelMap(ml)
		if !ok && !(ml.Kind == yaml.MappingNode && len(ml.Content) == 0) {
			return nil, false
		}
		for k, v := range m {
			match[k] = v
		}
	}
	var exprs []labelReq
	for _, e := range ySeq(yGet(n, "matchExpressions")) {
		key := yScalar(yGet(e, "key"))
		if key == "" {
			return nil, false
		}
		var vals []string
		for _, v := range ySeq(yGet(e, "values")) {
			vals = append(vals, yScalar(v))
		}
		sort.Strings(vals)
		switch yScalar(yGet(e, "operator")) {
		case "In":
			exprs = append(exprs, labelReq{key: key, op: "in", vals: vals})
		case "NotIn":
			exprs = append(exprs, labelReq{key: key, op: "notin", vals: vals})
		case "Exists":
			exprs = append(exprs, labelReq{key: key, op: "exists"})
		case "DoesNotExist":
			exprs = append(exprs, labelReq{key: key, op: "!"})
		default:
			return nil, false
		}
	}
	return labelSelectorOf(match, exprs), true
}

// String is the selector in kubectl's form (`k=v,k in (a,b),k,!k`); "{}"
// for the selector of everything.
func (s labelSelector) String() string {
	if len(s) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(s))
	for _, r := range s {
		switch r.op {
		case "=", "!=":
			parts = append(parts, r.key+r.op+r.vals[0])
		case "in", "notin":
			parts = append(parts, r.key+" "+r.op+" ("+strings.Join(r.vals, ",")+")")
		case "exists":
			parts = append(parts, r.key)
		case "!":
			parts = append(parts, "!"+r.key)
		}
	}
	return strings.Join(parts, ",")
}

// equalities is the selector's equality requirements as a map (the
// DocCrossFileRef.Labels disclosure; nil when it has none).
func (s labelSelector) equalities() map[string]string {
	var m map[string]string
	for _, r := range s {
		if r.op == "=" {
			if m == nil {
				m = map[string]string{}
			}
			m[r.key] = r.vals[0]
		}
	}
	return m
}

// matches: every requirement holds for labels.
func (s labelSelector) matches(labels map[string]string) bool {
	for _, r := range s {
		v, has := labels[r.key]
		switch r.op {
		case "=":
			if !has || v != r.vals[0] {
				return false
			}
		case "!=":
			if has && v == r.vals[0] {
				return false
			}
		case "in":
			if !has || !slices.Contains(r.vals, v) {
				return false
			}
		case "notin":
			if has && slices.Contains(r.vals, v) {
				return false
			}
		case "exists":
			if !has {
				return false
			}
		case "!":
			if has {
				return false
			}
		}
	}
	return true
}

// parseLabelSelector reads the String form back ("{}" is everything).
func parseLabelSelector(s string) (labelSelector, bool) {
	s = strings.TrimSpace(s)
	if s == "{}" {
		return labelSelector{}, true
	}
	if s == "" {
		return nil, false
	}
	var out labelSelector
	depth, start := 0, 0
	var terms []string
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				terms = append(terms, s[start:i])
				start = i + 1
			}
		}
	}
	terms = append(terms, s[start:])
	for _, t := range terms {
		t = strings.TrimSpace(t)
		switch {
		case t == "":
			return nil, false
		case strings.Contains(t, " notin ("), strings.Contains(t, " in ("):
			op := "in"
			if strings.Contains(t, " notin (") {
				op = "notin"
			}
			key, rest, _ := strings.Cut(t, " "+op+" (")
			rest = strings.TrimSuffix(rest, ")")
			var vals []string
			if rest != "" {
				vals = strings.Split(rest, ",")
			}
			out = append(out, labelReq{key: key, op: op, vals: vals})
		case strings.Contains(t, "!="):
			k, v, _ := strings.Cut(t, "!=")
			out = append(out, labelReq{key: k, op: "!=", vals: []string{v}})
		case strings.Contains(t, "="):
			k, v, _ := strings.Cut(t, "=")
			out = append(out, labelReq{key: k, op: "=", vals: []string{v}})
		case strings.HasPrefix(t, "!"):
			out = append(out, labelReq{key: t[1:], op: "!"})
		default:
			out = append(out, labelReq{key: t, op: "exists"})
		}
	}
	return out, true
}
