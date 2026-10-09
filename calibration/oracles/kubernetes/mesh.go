package main

// mesh.go — the oracle's side-pass pairs (LB-F315: LB-F307, LB-F312),
// written from the projects' documented semantics, never from fathom's
// resolver:
//
//   - Istio VirtualService (istio.io/latest/docs/reference/config/networking/
//     virtual-service): `gateways` names Gateways as `<namespace>/<name>`, or
//     a name in the rule's namespace; the legacy `<name>.<namespace>.svc.
//     cluster.local` is the name in that namespace (pilot's gateway name
//     resolution); `mesh` is the sidecars, no object. A destination's
//     `subset` names a subset declared by a DestinationRule for the same
//     host (destination-rule: `host` read as the VirtualService's is).
//   - Istio ServiceEntry (service-entry): `hosts` are hosts the mesh learns
//     from the entry: a route to one of them is a route to the entry; each
//     host and each endpoint `address` is also a route target of the entry.
//   - Istio AuthorizationPolicy (authorization-policy): `selector.matchLabels`
//     selects the policy's namespace's workloads by pod labels; a source
//     `principals` entry is the SPIFFE identity `<trust domain>/ns/<ns>/sa/
//     <name>` of a ServiceAccount.
//   - prometheus-operator (api reference, monitoring.coreos.com/v1): a
//     ServiceMonitor's `selector` (a LabelSelector: matchLabels and
//     matchExpressions In, NotIn, Exists, DoesNotExist) selects Services by
//     their own labels, a PodMonitor's selects pods, in the namespaces
//     `namespaceSelector` names (`any: true`, `matchNames`, else the
//     monitor's own); a Probe's `targets.staticConfig.static` and
//     `prober.url` are addresses; a PrometheusRule's expressions are PromQL,
//     read with the Prometheus parser itself (promql/parser
//     ExtractSelectors), and each selector's equality matcher on an
//     identifying label (job, service, app, destination_service,
//     destination_workload, service_name: fathom's identity_labels.json,
//     the rule's data) names a service in the selector's namespace matcher's
//     namespace, else the rule's; a regex or negative matcher on one (or on
//     the namespace scoping it) names none.
//
// A selector joins every match in the nearest scope holding one (the
// referring file, its directory, the subject), an object or a subset the
// nearest declarations of it; a miss is undeclared, a wildcard or a
// non-Kubernetes principal external.

import (
	"strings"

	promlabels "github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"gopkg.in/yaml.v3"
)

// ruleRoutes (-rule-routes) writes a PrometheusRule's identifying matchers
// as route targets. Off by default: the Stage A kill withheld fathom's
// PrometheusRule join (LB-F312, docs/bench/readings/telemetry/
// stage-a-static.md), so the reading's truth is what ships.
var ruleRoutes bool

// identityLabels mirrors internal/promql/identity_labels.json (the rule's
// data); namespace scopes them.
var identityLabels = []string{"job", "service", "app", "destination_service", "destination_workload", "service_name"}

type labelReq struct {
	key, op string
	vals    []string
}

type labelSel struct {
	reqs []labelReq
	ok   bool
}

func (s labelSel) matches(l map[string]string) bool {
	for _, r := range s.reqs {
		v, has := l[r.key]
		switch r.op {
		case "In":
			found := false
			for _, x := range r.vals {
				found = found || (has && x == v)
			}
			if !found {
				return false
			}
		case "NotIn":
			for _, x := range r.vals {
				if has && x == v {
					return false
				}
			}
		case "Exists":
			if !has {
				return false
			}
		case "DoesNotExist":
			if has {
				return false
			}
		}
	}
	return true
}

// readSelector reads a metav1.LabelSelector: matchLabels are In of one
// value. A present empty selector selects everything.
func readSelector(n *yaml.Node) labelSel {
	if n == nil || n.Kind != yaml.MappingNode {
		return labelSel{}
	}
	var s labelSel
	if ml := get(n, "matchLabels"); ml != nil {
		if ml.Kind != yaml.MappingNode {
			return labelSel{}
		}
		for i := 0; i+1 < len(ml.Content); i += 2 {
			if ml.Content[i+1].Kind != yaml.ScalarNode {
				return labelSel{}
			}
			s.reqs = append(s.reqs, labelReq{ml.Content[i].Value, "In", []string{ml.Content[i+1].Value}})
		}
	}
	for _, e := range seq(get(n, "matchExpressions")) {
		op := scalar(get(e, "operator"))
		switch op {
		case "In", "NotIn", "Exists", "DoesNotExist":
		default:
			return labelSel{}
		}
		var vals []string
		for _, v := range seq(get(e, "values")) {
			vals = append(vals, scalar(v))
		}
		s.reqs = append(s.reqs, labelReq{scalar(get(e, "key")), op, vals})
	}
	s.ok = true
	return s
}

// meshSel is one selector reference: the class it selects (pods or
// services) in the namespaces it names ("*" any).
type meshSel struct {
	path, from, raw string
	class           string
	nss             []string
	sel             labelSel
}

// meshObj is one reference to a named object (`Kind/<ns>/<name>`, "" for
// none).
type meshObj struct{ path, from, raw, target string }

// meshSubset is a destination's subset reference: the host as written, read
// in ns, and the subset.
type meshSubset struct{ path, from, host, ns, subset string }

type labelledObj struct {
	path, entity, class, ns string
	labels                  map[string]string
}

// meshWalk is the oracle's side-pass state.
type meshWalk struct {
	sels    []meshSel
	objs    []meshObj
	subsets []meshSubset
	decls   map[string][]decl // "Kind/<ns>/<name>" -> declarations
	sets    []labelledObj
	// subset providers: DestinationRule declarations by (canonical host,
	// subset)
	drSubsets map[string][]decl
	// gateway references by VirtualService, for the istioctl cross-check
	gws []meshObj
}

func (m *meshWalk) init() {
	if m.decls == nil {
		m.decls, m.drSubsets = map[string][]decl{}, map[string][]decl{}
	}
}

// document reads one object's side-pass facts; w gets its route targets
// and host providers.
func (m *meshWalk) document(w *routeWalk, rel string, root *yaml.Node, kind, api, name, entity, ns string) {
	m.init()
	m.decls[kind+"/"+ns+"/"+name] = append(m.decls[kind+"/"+ns+"/"+name], decl{rel, entity, "obj:" + kind + "/" + ns + "/" + name})
	addSet := func(class string, n *yaml.Node) {
		if l := labels(n); l != nil {
			m.sets = append(m.sets, labelledObj{rel, entity, class, ns, l})
		}
	}
	addSet("pods", path(root, "spec", "template", "metadata", "labels"))
	addSet("pods", path(root, "spec", "jobTemplate", "spec", "template", "metadata", "labels"))
	if kind == "Pod" {
		addSet("pods", path(root, "metadata", "labels"))
	}
	group := api
	if i := strings.Index(api, "/"); i >= 0 {
		group = api[:i]
	} else {
		group = ""
	}
	target := func(raw string) {
		w.targets = append(w.targets, target{path: rel, from: entity, raw: raw, ns: ns, k8s: true, kind: kind, name: name})
	}
	switch {
	case kind == "Service" && api == "v1":
		addSet("services", path(root, "metadata", "labels"))
	case group == "networking.istio.io" && kind == "VirtualService":
		for _, g := range seq(path(root, "spec", "gateways")) {
			v := scalar(g)
			if v == "" || v == "mesh" {
				continue
			}
			o := meshObj{rel, entity, v, gatewayTarget(v, ns)}
			m.objs = append(m.objs, o)
			m.gws = append(m.gws, o)
		}
		for _, proto := range []string{"http", "tcp", "tls"} {
			for _, r := range seq(path(root, "spec", proto)) {
				for _, d := range seq(get(r, "route")) {
					h, s := scalar(path(d, "destination", "host")), scalar(path(d, "destination", "subset"))
					if h != "" && s != "" {
						m.subsets = append(m.subsets, meshSubset{rel, entity, h, ns, s})
					}
				}
			}
		}
	case group == "networking.istio.io" && kind == "DestinationRule":
		if h := scalar(path(root, "spec", "host")); h != "" {
			for _, s := range seq(path(root, "spec", "subsets")) {
				if n := scalar(get(s, "name")); n != "" {
					k := canonHost(h, ns) + "#" + n
					m.drSubsets[k] = append(m.drSubsets[k], decl{rel, entity, "dr:" + k})
				}
			}
		}
	case group == "networking.istio.io" && kind == "ServiceEntry":
		for _, h := range seq(path(root, "spec", "hosts")) {
			if v := scalar(h); v != "" {
				target(v)
				if key := entryHost(v); key != "" {
					w.entries[key] = append(w.entries[key], decl{rel, entity, key})
				}
			}
		}
		for _, e := range seq(path(root, "spec", "endpoints")) {
			if v := scalar(get(e, "address")); v != "" {
				target(v)
			}
		}
	case group == "security.istio.io" && kind == "AuthorizationPolicy":
		if ml := path(root, "spec", "selector", "matchLabels"); labels(ml) != nil {
			m.sels = append(m.sels, meshSel{rel, entity, "matchLabels", "pods", []string{ns}, readSelector(path(root, "spec", "selector"))})
		}
		for _, r := range seq(path(root, "spec", "rules")) {
			for _, f := range seq(get(r, "from")) {
				for _, p := range seq(path(f, "source", "principals")) {
					if v := scalar(p); v != "" {
						m.objs = append(m.objs, meshObj{rel, entity, v, principalTarget(v)})
					}
				}
			}
		}
	case group == "monitoring.coreos.com" && (kind == "ServiceMonitor" || kind == "PodMonitor"):
		class := "services"
		if kind == "PodMonitor" {
			class = "pods"
		}
		s := readSelector(path(root, "spec", "selector"))
		if !s.ok {
			break
		}
		nss := []string{ns}
		nsSel := path(root, "spec", "namespaceSelector")
		if scalar(get(nsSel, "any")) == "true" {
			nss = []string{"*"}
		} else if names := seq(get(nsSel, "matchNames")); len(names) > 0 {
			nss = nil
			for _, n := range names {
				if v := scalar(n); v != "" {
					nss = append(nss, v)
				}
			}
		}
		m.sels = append(m.sels, meshSel{rel, entity, "selector", class, nss, s})
	case group == "monitoring.coreos.com" && kind == "Probe":
		for _, t := range seq(path(root, "spec", "targets", "staticConfig", "static")) {
			if v := scalar(t); v != "" {
				target(v)
			}
		}
		if v := scalar(path(root, "spec", "prober", "url")); v != "" {
			target(v)
		}
	case group == "monitoring.coreos.com" && kind == "PrometheusRule" && ruleRoutes:
		for _, g := range seq(path(root, "spec", "groups")) {
			for _, r := range seq(get(g, "rules")) {
				expr := scalar(get(r, "expr"))
				if expr == "" {
					continue
				}
				for _, v := range ruleTargets(expr, ns) {
					target(v)
				}
			}
		}
	}
}

// ruleTargets are an expression's identifying matchers, each written as the
// route value `<value>.<namespace>` (a dotted value as written): the
// Prometheus parser's selectors.
func ruleTargets(expr, ns string) []string {
	e, err := promParser.ParseExpr(expr)
	if err != nil {
		return nil
	}
	var out []string
	for _, sel := range parser.ExtractSelectors(e) {
		scope, fixed := ns, true
		for _, mt := range sel {
			if mt.Name == "namespace" {
				if mt.Type == promlabels.MatchEqual {
					scope = mt.Value
				} else {
					fixed = false
				}
			}
		}
		for _, mt := range sel {
			if !isIdentity(mt.Name) || mt.Type != promlabels.MatchEqual || mt.Value == "" || !fixed {
				continue
			}
			v := mt.Value
			if scope != "" && !strings.Contains(v, ".") {
				v += "." + scope
			}
			out = append(out, v)
		}
	}
	return out
}

// promParser is the reference parser with every optional syntax on (a rule
// the collector or a newer Prometheus accepts is read, not refused).
var promParser = parser.NewParser(parser.Options{EnableExperimentalFunctions: true, ExperimentalDurationExpr: true, EnableExtendedRangeSelectors: true, EnableBinopFillModifiers: true})

func isIdentity(l string) bool {
	for _, x := range identityLabels {
		if x == l {
			return true
		}
	}
	return false
}

// gatewayTarget: the Gateway a gateways entry names, "" for none.
func gatewayTarget(v, ns string) string {
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

// principalTarget: the ServiceAccount a SPIFFE principal names, "" for none.
func principalTarget(p string) string {
	if strings.Contains(p, "*") {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(p, "spiffe://"), "/")
	if len(parts) != 5 || parts[0] == "" || parts[1] != "ns" || parts[2] == "" || parts[3] != "sa" || parts[4] == "" {
		return ""
	}
	return "ServiceAccount/" + parts[2] + "/" + parts[4]
}

// entryHost is the key a ServiceEntry host provides: a cluster form is the
// Service's ("svc:<ns>/<name>"), a dotted host "host:<host>", a short name
// or a wildcard none.
func entryHost(h string) string {
	h = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
	if h == "" || strings.ContainsAny(h, "*$/:{}") || !strings.Contains(h, ".") {
		return ""
	}
	l := strings.Split(h, ".")
	if len(l) == 3 && l[2] == "svc" || len(l) == 5 && strings.Join(l[2:], ".") == "svc.cluster.local" {
		return "svc:" + l[1] + "/" + l[0]
	}
	return "host:" + h
}

// resolve joins the side-pass references.
func (m *meshWalk) resolve(near func(from string, cands []decl) []decl) ([]routeRef, []routeUnresolved) {
	m.init()
	var refs []routeRef
	var un []routeUnresolved
	join := func(p, from, raw string, cands []decl, ambiguousByKey bool) {
		ns := near(p, cands)
		if len(ns) == 0 {
			un = append(un, routeUnresolved{p, from, raw, "undeclared"})
			return
		}
		if ambiguousByKey {
			keys := map[string]bool{}
			for _, d := range ns {
				keys[d.key] = true
			}
			if len(keys) > 1 {
				un = append(un, routeUnresolved{p, from, raw, "ambiguous"})
				return
			}
		}
		for _, d := range ns {
			if d.path == p && d.name == from {
				continue
			}
			refs = append(refs, routeRef{p, from, d.path, d.name, raw})
		}
	}
	for _, o := range m.objs {
		if o.target == "" {
			un = append(un, routeUnresolved{o.path, o.from, o.raw, "external"})
			continue
		}
		join(o.path, o.from, o.raw, m.decls[o.target], false)
	}
	for _, s := range m.subsets {
		host, reason := classify(s.host)
		if reason != "" {
			un = append(un, routeUnresolved{s.path, s.from, s.host, reason})
			continue
		}
		_ = host
		join(s.path, s.from, s.host, m.drSubsets[canonHost(s.host, s.ns)+"#"+s.subset], true)
	}
	for _, s := range m.sels {
		var cands []decl
		for _, l := range m.sets {
			if l.class != s.class {
				continue
			}
			inNS := false
			for _, n := range s.nss {
				inNS = inNS || n == "*" || n == l.ns
			}
			if inNS && s.sel.matches(l.labels) {
				cands = append(cands, decl{l.path, l.entity, ""})
			}
		}
		raw := s.raw
		if len(cands) == 0 {
			un = append(un, routeUnresolved{s.path, s.from, raw, "undeclared"})
			continue
		}
		for _, d := range near(s.path, cands) {
			if d.path == s.path && d.name == s.from {
				continue
			}
			refs = append(refs, routeRef{s.path, s.from, d.path, d.name, raw})
		}
	}
	return refs, un
}
