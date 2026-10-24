package fusion

// routeref.go — LB-F96/LB-F97 (wave px; the runtime topology's first edges,
// docs/design/language-breadth.md §4 "Route references"): a route, an
// upstream or a mesh rule names the host it sends traffic to, and that host
// is a Kubernetes Service or a compose service declared somewhere in the
// repository. ResolveRouteRefs turns each route-host reference into the
// ADR-050 `deploy:` contract key both ends meet on (`deploy:k8s/<ns>/<svc>`,
// `deploy:host/<hostname>`, normalized by syslink.ParseContractKey) and joins
// it to the one provider declaring that key. One resolver serves ingest
// (enrichRouteRefEdges) and the language-breadth harness (indexbench
// lang-routes), so the bench never scores an edge ingest drops.
//
// Interim divergence from ADR-050 (Revisit when: SG-2 lands): the system
// graph's contract_keys/system_links tables are frozen until RT-M1, so an
// intra-repository route edge is stored the doc-xref way (owner decision D4):
// a file-tier typed edge (core.EdgeFileResolved, tier pre-scan, never
// presented as typed) from the route entity to the provider entity, under
// the typed_edges key "route-ref", replaced wholesale by a gated pass. The
// deploy: key each edge was matched on is carried by RouteRef.Key, so SG-21
// and SG-10 can later write the identical pair into system_links. By
// ADR-050's ladder these are `literal` links (a normalized host equal at both
// ends) and never gate (I57): they feed impact (blast radius, the call
// graph), not a verdict.
//
// Unresolved is never guessed: a target that is a variable, an IP literal,
// a unix socket, an external host, an in-cluster name no provider declares,
// or a name whose nearest declarations provide more than one key yields no
// edge and is counted by reason. An intra-format name (an nginx upstream,
// an Envoy cluster) is the dialect's own L2 reference, never route-host.

import (
	"context"
	"fmt"
	"net"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/syslink"
)

// The route contract every route producer emits (docs/design/
// language-breadth.md §4): an extractor.DocCrossFileRef with this Edge and
// Strategy, Raw the target as written, Target = Raw.
const (
	RouteRefEdge      = "route_ref"
	RouteHostStrategy = "route-host"
)

// Why a route-host reference resolved to nothing (RouteResolution.Unresolved).
const (
	// RouteExternal: a host outside the repository (a dotted name no
	// provider declares, an SRV seed list, a wildcard host).
	RouteExternal = "external"
	// RouteIPLiteral: an IP address, or localhost.
	RouteIPLiteral = "ip-literal"
	// RouteVariable: a value the file does not fix ($var, ${X}, $(X), {{ }}).
	RouteVariable = "variable"
	// RouteUnixSocket: a unix socket path.
	RouteUnixSocket = "unix-socket"
	// RouteAmbiguous: the nearest declarations (same file, then same
	// directory, then anywhere) provide more than one of the keys the
	// target may name (a compose service and a Service in namespace
	// default, both called mongo).
	RouteAmbiguous = "ambiguous"
	// RouteUndeclared: an in-cluster or in-compose name no provider
	// declares.
	RouteUndeclared = "undeclared"
)

// RouteReasons is every unresolved reason, in the order reports print them:
// the route reasons, then the PromQL reasons the doc engine's promql mode
// refuses to guess with (a regex or negative identifying matcher, an
// expression that does not scan; LB-F317, LB-F312).
var RouteReasons = []string{RouteExternal, RouteIPLiteral, RouteVariable, RouteUnixSocket, RouteAmbiguous, RouteUndeclared,
	extractor.PromQLRegex, extractor.PromQLNegative, extractor.PromQLUnparsed}

// routeRefPackageKey is the typed_edges key the route-ref pass replaces.
const routeRefPackageKey = "route-ref"

const routeRefStateKey = typedPassStatePrefix + "route-ref"

// RouteRef is one resolved route reference: route entity to provider
// entity, through the deploy: key both ends meet on.
type RouteRef struct {
	FromFile, From, FromKey string
	ToFile, To, ToKey       string
	// Key is the ADR-050 contract key (`deploy:k8s/<ns>/<svc>` or
	// `deploy:host/<hostname>`), normalized by syslink.ParseContractKey: the
	// host's for a route or a subset, the provider's for a side-pass join to
	// a Service, "" for a join to an object that provides none (a Gateway,
	// a ServiceAccount, a workload).
	Key  string
	Raw  string
	Line int
	// Via is the reference's strategy: RouteHostStrategy, or the
	// Kubernetes side pass's RouteObjectStrategy, RouteSubsetStrategy or
	// RouteSelectorStrategy (routeref_mesh.go).
	Via string
}

// RouteUnresolved is one route-host reference that resolved to nothing.
type RouteUnresolved struct {
	FromFile, From, Raw, Reason string
	Line                        int
}

// RouteResolution is ResolveRouteRefs' answer.
type RouteResolution struct {
	Refs       []RouteRef
	Unresolved []RouteUnresolved
	// SelfDropped counts references whose one provider is the referring
	// entity itself.
	SelfDropped int
}

// UnresolvedByReason counts Unresolved by reason (only reasons that occur).
func (r RouteResolution) UnresolvedByReason() map[string]int {
	if len(r.Unresolved) == 0 {
		return nil
	}
	out := map[string]int{}
	for _, u := range r.Unresolved {
		out[u.Reason]++
	}
	return out
}

// routeProvider is one node declaring a deploy: key.
type routeProvider struct {
	file string
	node core.SymbolNode
}

// routeProviderKey is the deploy: key a node provides, "" for none: a
// Kubernetes Service (`Service/<ns>/<name>`, `Service/<name>` in namespace
// default) provides deploy:k8s/<ns>/<name>; a compose service provides
// deploy:host/<service key> (compose's network DNS name).
func routeProviderKey(n core.SymbolNode) string {
	switch n.Kind {
	case core.KindK8sWorkload:
		parts := strings.Split(n.Name, "/")
		switch {
		case len(parts) == 2 && parts[0] == "Service":
			return deployKey("k8s/default/" + parts[1])
		case len(parts) == 3 && parts[0] == "Service":
			return deployKey("k8s/" + parts[1] + "/" + parts[2])
		}
	case core.KindComposeService:
		return deployKey("host/" + n.Name)
	}
	return ""
}

// deployKey is the normalized `deploy:<body>` key, "" when the grammar
// refuses it (syslink.ParseContractKey, ADR-050 §3).
func deployKey(body string) string {
	k, err := syslink.ParseContractKey("deploy:" + body)
	if err != nil {
		return ""
	}
	return k.String()
}

// k8sNamespaceOf is the namespace a Kubernetes entity named
// `Kind/<ns>/<name>` lives in ("default" for `Kind/<name>`), and whether n
// is a Kubernetes object at all.
func k8sNamespaceOf(n core.SymbolNode) (string, bool) {
	if n.Kind != core.KindK8sWorkload {
		return "", false
	}
	if parts := strings.Split(n.Name, "/"); len(parts) == 3 {
		return parts[1], true
	}
	return "default", true
}

// routeTarget classifies a route-host value: the candidate deploy: keys it
// may name (in no particular preference — ResolveRouteRefs keeps the
// nearest provider of any of them), the reason to give when none is
// declared, or, for a value that names no host at all, the reason it never
// resolves. ns is the referring Kubernetes object's namespace; inK8s is
// false for any other referrer (an nginx config, a compose service).
//
// Normalization strips a scheme, userinfo, port and path, and Istio's
// `namespace/dnsName` prefix (a Gateway server's hosts). Then, by the
// Kubernetes and Istio DNS rules: `name` is the Service in the referrer's
// namespace (a non-Kubernetes referrer: a compose service or host `name`,
// or the Service in namespace default); `name.ns` is the Service in ns (or a
// host of that name); `name.ns.svc` and `name.ns.svc.cluster.local` are the
// Service in ns; anything else is a host.
func routeTarget(raw, ns string, inK8s bool) (keys []string, missReason, reason string) {
	s := strings.TrimSpace(raw)
	switch {
	case s == "":
		return nil, "", RouteUndeclared
	case strings.ContainsAny(s, "$") || strings.Contains(s, "{{") || strings.Contains(s, "}}"):
		return nil, "", RouteVariable
	case strings.HasPrefix(s, "unix:") || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "@"):
		return nil, "", RouteUnixSocket
	case strings.HasPrefix(strings.ToLower(s), "mongodb+srv://"):
		// A DNS seed list: the hosts are SRV records, never declared here.
		return nil, "", RouteExternal
	}
	scheme := false
	if i := strings.Index(s, "://"); i >= 0 {
		s, scheme = s[i+3:], true
	}
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "/"); i >= 0 {
		head, rest := s[:i], s[i+1:]
		if j := strings.Index(rest, "/"); j >= 0 {
			rest = rest[:j]
		}
		if !scheme && (head == "." || head == "*" || (head != "" && !strings.ContainsAny(head, ".:@"))) &&
			(rest == "*" || strings.Contains(rest, ".")) {
			s = rest // Istio's namespace/dnsName
		} else {
			s = head
		}
	}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	if strings.Contains(strings.ToLower(s), "%2f") || strings.HasSuffix(s, ".sock") {
		return nil, "", RouteUnixSocket // a mongodb:// socket path, URL-encoded
	}
	host := s
	switch {
	case strings.HasPrefix(host, "["):
		return nil, "", RouteIPLiteral
	case strings.Count(host, ":") > 1:
		return nil, "", RouteIPLiteral
	case strings.Contains(host, ":"):
		host = host[:strings.Index(host, ":")]
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	switch {
	case host == "":
		return nil, "", RouteUndeclared
	case host == "localhost" || net.ParseIP(host) != nil:
		return nil, "", RouteIPLiteral
	case strings.Contains(host, "*"):
		return nil, "", RouteExternal
	}
	add := func(body string) {
		if k := deployKey(body); k != "" {
			keys = append(keys, k)
		}
	}
	labels := strings.Split(host, ".")
	switch {
	case len(labels) == 1:
		if inK8s {
			add("k8s/" + ns + "/" + host)
		} else {
			add("host/" + host)
			add("k8s/default/" + host)
		}
		missReason = RouteUndeclared
	case len(labels) == 2:
		add("k8s/" + labels[1] + "/" + labels[0])
		add("host/" + host)
		missReason = RouteExternal
	case len(labels) == 3 && labels[2] == "svc",
		len(labels) == 5 && labels[2] == "svc" && labels[3] == "cluster" && labels[4] == "local":
		add("k8s/" + labels[1] + "/" + labels[0])
		missReason = RouteUndeclared
	default:
		add("host/" + host)
		missReason = RouteExternal
	}
	if len(keys) == 0 {
		return nil, "", RouteUndeclared
	}
	return keys, missReason, ""
}

// ResolveRouteRefs resolves every file's route-host references (Strategy
// RouteHostStrategy) against the providers the same files declare
// (deterministic, sorted). A reference's referring entity is the node of
// its From name in its own file; a reference whose entity no node carries
// yields nothing. Among the declarations of the reference's candidate keys
// the nearest are kept — in the referring file, else its directory, else
// the repository — and when they provide one key the reference joins each
// of them: a Service declared in several files of one directory (an
// all-in-one manifest beside per-service ones) is one runtime object, and a
// change to any of its declarations reaches the route (ADR-050: a link is a
// key match, one per provider node). Nearest declarations of more than one
// key are ambiguous. A join to the referring entity itself is dropped
// (SelfDropped).
//
// The Kubernetes side pass's references (routeref_mesh.go, LB-F307,
// LB-F312) resolve the same way: an object reference (RouteObjectStrategy)
// to the nearest declarations of that object, a subset reference
// (RouteSubsetStrategy) to the nearest DestinationRules declaring that
// subset for the same host, and a selector (RouteSelectorStrategy) to every
// label set in its scope it matches, in the nearest file, directory or
// repository holding one. A ServiceEntry host and a DestinationRule subset
// are providers (routeProvidesStrategy), never references.
func ResolveRouteRefs(files []DocFile) RouteResolution {
	type fk struct{ file, name string }
	nodeAt := map[fk]core.SymbolNode{}
	byKey := map[string][]routeProvider{}
	for _, f := range files {
		for _, n := range f.Result.Nodes {
			k := fk{f.Path, n.Name}
			if _, dup := nodeAt[k]; !dup {
				nodeAt[k] = n
				if key := routeProviderKey(n); key != "" {
					byKey[key] = append(byKey[key], routeProvider{f.Path, n})
				}
				if key := objectKey(n); key != "" {
					byKey[key] = append(byKey[key], routeProvider{f.Path, n})
				}
			}
		}
	}
	type labelled struct {
		p      routeProvider
		labels map[string]string
	}
	labelsBy := map[string][]labelled{} // "<class>/<ns>" -> label sets
	for _, f := range files {
		for _, c := range f.Result.CrossFile {
			if c.Strategy != routeProvidesStrategy {
				continue
			}
			n, ok := nodeAt[fk{f.Path, c.From}]
			if !ok {
				continue
			}
			ns, _ := k8sNamespaceOf(n)
			for _, key := range providedKeys(c, ns) {
				byKey[key] = append(byKey[key], routeProvider{f.Path, n})
			}
		}
		for _, ls := range f.Result.Labels {
			if n, ok := nodeAt[fk{f.Path, ls.Entity}]; ok {
				labelsBy[ls.Scope] = append(labelsBy[ls.Scope], labelled{routeProvider{f.Path, n}, ls.Labels})
			}
		}
	}
	var res RouteResolution
	seen := map[string]bool{}
	type hit struct {
		p   routeProvider
		key string
	}
	// nearest keeps the hits in the referring file, else its directory,
	// else the nearest ancestor directory holding one (a compose project is
	// a directory, O435), else anywhere.
	nearest := func(file string, cands []hit) []hit {
		var near []hit
		scopes := []func(string) bool{
			func(p string) bool { return p == file },
			func(p string) bool { return path.Dir(p) == path.Dir(file) },
		}
		for d := path.Dir(path.Dir(file)); ; d = path.Dir(d) {
			scopes = append(scopes, func(p string) bool { return path.Dir(p) == d })
			if d == "." || d == "/" {
				break
			}
		}
		scopes = append(scopes, func(string) bool { return true })
		for _, scope := range scopes {
			for _, h := range cands {
				if scope(h.p.file) {
					near = append(near, h)
				}
			}
			if len(near) > 0 {
				break
			}
		}
		return near
	}
	for _, f := range files {
		for _, c := range f.Result.CrossFile {
			switch c.Strategy {
			case RouteHostStrategy, RouteObjectStrategy, RouteSubsetStrategy, RouteSelectorStrategy, routeRefusedStrategy:
			default:
				continue
			}
			from, ok := nodeAt[fk{f.Path, c.From}]
			if !ok {
				continue
			}
			ns, inK8s := k8sNamespaceOf(from)
			miss := func(reason string) {
				res.Unresolved = append(res.Unresolved, RouteUnresolved{FromFile: f.Path, From: c.From, Raw: c.Raw, Reason: reason, Line: c.Line})
			}
			if c.Strategy == routeRefusedStrategy {
				miss(c.Target)
				continue
			}
			// join records a reference to each provider of near (the
			// referring entity itself dropped); key is the deploy: key the
			// pair is matched on ("" for none).
			join := func(near []hit, keyOf func(hit) string) {
				for _, to := range near {
					if to.p.node.Key() == from.Key() {
						res.SelfDropped++
						continue
					}
					r := RouteRef{FromFile: f.Path, From: c.From, FromKey: from.Key(), ToFile: to.p.file, To: to.p.node.Name,
						ToKey: to.p.node.Key(), Key: keyOf(to), Raw: c.Raw, Line: c.Line, Via: c.Strategy}
					// one reference per line: a rule file's expressions, each
					// on its own line, are scored apiece (Stage A)
					id := r.FromFile + "\x00" + r.From + "\x00" + r.ToFile + "\x00" + r.To + "\x00" + r.Raw + "\x00" + strconv.Itoa(r.Line)
					if !seen[id] {
						seen[id] = true
						res.Refs = append(res.Refs, r)
					}
				}
			}
			if c.Strategy == RouteSelectorStrategy {
				sel, ok := parseLabelSelector(c.Target)
				class, sns, _ := strings.Cut(c.Scope, "/")
				if !ok || class == "" || sns == "" {
					continue
				}
				var cands []hit
				for scope, sets := range labelsBy {
					sc, sn, _ := strings.Cut(scope, "/")
					if sc != class || (sns != "*" && sn != sns) {
						continue
					}
					for _, ls := range sets {
						if sel.matches(ls.labels) {
							cands = append(cands, hit{ls.p, routeProviderKey(ls.p.node)})
						}
					}
				}
				if len(cands) == 0 {
					miss(RouteUndeclared)
					continue
				}
				join(nearest(f.Path, cands), func(h hit) string { return h.key })
				continue
			}
			var keys []string
			missReason := RouteUndeclared
			deployOf := map[string]string{} // candidate key -> the deploy: key it is matched on
			switch c.Strategy {
			case RouteObjectStrategy:
				k := objectKeyOf(c.Target)
				if k == "" {
					miss(RouteExternal)
					continue
				}
				keys = []string{k}
			case RouteSubsetStrategy:
				hostKeys, _, reason := routeTarget(c.Raw, ns, inK8s)
				if reason != "" {
					miss(reason)
					continue
				}
				for _, hk := range hostKeys {
					sk := subsetKey(hk, c.Target)
					keys = append(keys, sk)
					deployOf[sk] = hk
				}
			default: // RouteHostStrategy
				var reason string
				keys, missReason, reason = routeTarget(c.Raw, ns, inK8s)
				if reason != "" {
					miss(reason)
					continue
				}
				for _, k := range keys {
					deployOf[k] = k
				}
			}
			var cands []hit
			dup := map[fk]bool{}
			for _, k := range keys {
				for _, p := range byKey[k] {
					if id := (fk{p.file, p.node.Name}); !dup[id] {
						dup[id] = true
						cands = append(cands, hit{p, k})
					}
				}
			}
			if len(cands) == 0 {
				miss(missReason)
				continue
			}
			near := nearest(f.Path, cands)
			keyset := map[string]bool{}
			for _, h := range near {
				keyset[h.key] = true
			}
			if len(keyset) > 1 {
				miss(RouteAmbiguous)
				continue
			}
			join(near, func(h hit) string {
				if d, ok := deployOf[h.key]; ok {
					return d
				}
				return routeProviderKey(h.p.node) // an object: its deploy: key when it is a Service
			})
		}
	}
	sort.Slice(res.Refs, func(i, j int) bool {
		a, b := res.Refs[i], res.Refs[j]
		for _, p := range [][2]string{{a.FromFile, b.FromFile}, {a.From, b.From}, {a.ToFile, b.ToFile}, {a.To, b.To}, {a.Raw, b.Raw}} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		return a.Line < b.Line
	})
	sort.Slice(res.Unresolved, func(i, j int) bool {
		a, b := res.Unresolved[i], res.Unresolved[j]
		for _, p := range [][2]string{{a.FromFile, b.FromFile}, {a.From, b.From}, {a.Raw, b.Raw}, {a.Reason, b.Reason}} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		return a.Line < b.Line
	})
	return res
}

// RouteRefEdges is the store form of resolved routes: one file-tier typed
// edge per (route entity, provider entity) symbol pair, sorted.
func RouteRefEdges(refs []RouteRef) []core.Edge {
	seen := map[[2]string]bool{}
	var edges []core.Edge
	for _, r := range refs {
		k := [2]string{r.FromKey, r.ToKey}
		if seen[k] {
			continue
		}
		seen[k] = true
		edges = append(edges, core.Edge{FromFile: r.FromFile, FromSymbol: r.FromKey, ToSymbol: r.ToKey, Kind: core.EdgeFileResolved})
	}
	sortEdges(edges)
	return edges
}

// routeCandidateRules are the match rules of every manifest whose dialect
// is not a pure tags-engine one (doc, tags+doc, native): the files a route
// producer or a provider can live in. Their sniffs are dropped (a sniff
// only narrows an extension the rules already claim), so a path is a
// candidate when a rule claims it, alone or ambiguously. A dialect added
// later (nginx, Envoy, Kong) joins with no edit here.
var routeCandidateRules = sync.OnceValue(func() []connector.DialectRule {
	ds, err := extractor.LoadDialects()
	if err != nil {
		return nil
	}
	var rules []connector.DialectRule
	for _, d := range ds {
		if d.Engine == "tags" || d.ID == "markdown" { // a Markdown file carries no routes (LB-Q-markdown)
			continue
		}
		r := connector.DialectRule{ID: d.ID, Basenames: d.Match.Basenames, Globs: d.Match.Globs}
		r.Extensions = append(r.Extensions, d.Match.Extensions...)
		for _, v := range d.Variants {
			r.Extensions = append(r.Extensions, v.Extensions...)
		}
		rules = append(rules, r)
	}
	return rules
})

// isRouteCandidate: a path a doc or native dialect's manifest claims by
// extension, basename or glob — the route pass's input set — and every YAML
// file (LB-F313): the late-YAML dispatch (LB-F309) claims a YAML file by
// its content (an OpenTelemetry Collector configuration, a Prometheus rule
// file under any name), which no path rule can see, so the pass reads, and
// passNeeded re-runs on, any `.yaml` or `.yml` path, the hub's own input
// set.
func isRouteCandidate(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".yaml", ".yml":
		return true
	}
	v := connector.ClassifyDialect(routeCandidateRules(), p, nil)
	return v.Dialect != "" || v.By == connector.ClassifiedAmbiguous
}

// routePort is the doc-engine dialect a tool kind's routes are read with
// while ingest still dispatches that kind to a bespoke incumbent that is no
// DocResulter: Kubernetes manifests (k8sextract.go; the kubernetes dialect
// is measured, LB-Q-kubernetes, but not dispatched yet). Its nodes are never
// stored: only its route-host references are read, and each joins only a
// node the dispatched extractor stored (the dialect qualifies a Kubernetes
// object `Kind/<ns>/<name>`, the incumbent's own naming). nil for any other
// kind; once dispatch moves to the dialect, its DocResult serves directly.
func routePort(tk ToolKind) *extractor.DocExtractor {
	switch tk {
	case ToolK8sManifest, ToolK8sCRD:
		x, err := k8sRoutePort()
		if err != nil {
			return nil
		}
		return x
	}
	return nil
}

var k8sRoutePort = sync.OnceValues(func() (*extractor.DocExtractor, error) {
	return extractor.NewDocDialectExtractor("kubernetes")
})

// routeProviderTool: the tool kinds whose dispatched nodes can be route
// providers or carry launch routes (Kubernetes Services, compose services).
func routeProviderTool(tk ToolKind) bool {
	return tk == ToolK8sManifest || tk == ToolK8sCRD || tk == ToolCompose
}

// routeFile reads one candidate file for the route pass: the nodes
// ingest's dispatch stores for it and the route-host references it makes —
// a DocResulter's own, a routePort's for an incumbent-dispatched kind, and
// the launch arguments a container or compose service passes a proxy
// (launchroutes.go). ok is false for a file with neither a provider nor a
// route reference; its nodes are trimmed to the providers and the
// referring entities.
func (p *Pipeline) routeFile(service string, idx *siblingIndex, relPath string, content []byte) (DocFile, bool) {
	e, tk, ok, _ := p.resolveExtractor(relPath, content, idx)
	if !ok || e == nil {
		return DocFile{}, false
	}
	var nodes []core.SymbolNode
	var cross []extractor.DocCrossFileRef
	var refused []extractor.DocUnresolved
	if doc, isDoc := e.(extractor.DocResulter); isDoc {
		res, err := doc.ExtractDoc(service, relPath, content)
		if err != nil {
			return DocFile{}, false
		}
		nodes, cross, refused = res.Nodes, res.CrossFile, res.Unresolved
	} else {
		if !routeProviderTool(tk) {
			return DocFile{}, false
		}
		ns, err := e.Extract(service, relPath, content)
		if err != nil {
			return DocFile{}, false
		}
		nodes = ns
		if port := routePort(tk); port != nil {
			if res, err := port.ExtractDoc(service, relPath, content); err == nil {
				cross, refused = res.CrossFile, res.Unresolved
			}
		}
	}
	var routes []extractor.DocCrossFileRef
	for _, c := range cross {
		if c.Strategy == RouteHostStrategy {
			routes = append(routes, c)
		}
	}
	// a route value the engine refused to guess (a PromQL regex or negative
	// identifying matcher, LB-F317) is counted unresolved by its reason
	for _, u := range refused {
		if u.Edge == RouteRefEdge {
			routes = append(routes, extractor.DocCrossFileRef{From: u.From, Edge: u.Edge, Strategy: routeRefusedStrategy, Raw: u.Raw, Target: u.Reason, Line: u.Line})
		}
	}
	if routeProviderTool(tk) {
		routes = append(routes, launchRouteRefs(tk, content)...)
	}
	var labels []extractor.DocLabelSet
	if isK8sTool(tk) {
		// the side pass (routeref_mesh.go): mesh and monitoring objects'
		// object, subset and selector references, and the label sets
		mesh, ls := meshScan(content)
		routes, labels = append(routes, mesh...), ls
	}
	from := map[string]bool{}
	for _, c := range routes {
		from[c.From] = true
	}
	var keep []core.SymbolNode
	for _, n := range nodes {
		if from[n.Name] || routeProviderKey(n) != "" || objectKey(n) != "" {
			keep = append(keep, n)
		}
	}
	if len(keep) == 0 {
		return DocFile{}, false
	}
	return DocFile{Path: relPath, Result: extractor.DocResult{Nodes: keep, CrossFile: routes, Labels: labels}}, true
}

// RouteFileForBench is routeFile for the language-breadth harness
// (indexbench lang-routes): ingest's gates, dispatch and route reading on
// one file of a subject, so the bench scores exactly what ingest stores.
func (p *Pipeline) RouteFileForBench(serviceID string, refs []core.SourceRef, relPath string, content []byte) (DocFile, bool) {
	if len(content) > MaxExtractSize || connector.IsBinary(content) || gateDialectFile(relPath, content).excluded != "" {
		return DocFile{}, false
	}
	p.mu.Lock()
	idx, ok := p.siblingIdx[serviceID]
	if !ok {
		idx = buildSiblingIndex(refs)
		p.siblingIdx[serviceID] = idx
	}
	p.mu.Unlock()
	return p.routeFile(serviceID, idx, relPath, content)
}

// IsRouteCandidate reports whether the route pass reads relPath (the
// harness's file set, the same predicate ingest gates on).
func IsRouteCandidate(relPath string) bool { return isRouteCandidate(relPath) }

// routeRefReport is the route-ref pass's disclosure on the ingest Report.
type routeRefReport struct {
	edges      int
	unresolved map[string]int
	skipped    string
}

// enrichRouteRefEdges is the Ingest hook for LB-F97: when a route candidate
// changed (or the last pass did not complete), every candidate is read
// through ingest's gates and dispatch (routeFile), the route-host
// references are resolved (ResolveRouteRefs) and the edges replace the
// "route-ref" typed edges wholesale. Best-effort per file; a halt is
// returned. No table, cache or background job: typed_edges and the
// pass-state row are existing rows, and the provider index lives for one
// pass.
func (p *Pipeline) enrichRouteRefEdges(ctx context.Context, svc core.ServiceRef, refs []core.SourceRef, changed []string) (routeRefReport, error) {
	if p.cfg.DisableTypedEdges {
		return routeRefReport{skipped: "typed edges disabled"}, nil
	}
	if !passNeeded(changed, p.passStateOK(ctx, svc.ID, routeRefStateKey), isRouteCandidate) {
		return routeRefReport{skipped: SkippedPassReason}, nil
	}
	p.beginPassState(ctx, svc.ID, routeRefStateKey)
	idx := buildSiblingIndex(refs)
	var files []DocFile
	for _, r := range refs {
		if !isRouteCandidate(r.Path) {
			continue
		}
		content, err := p.conn.Fetch(ctx, r)
		if err != nil {
			if h := haltError(ctx, err); h != nil {
				return routeRefReport{}, h
			}
			continue
		}
		if len(content) > MaxExtractSize || connector.IsBinary(content) || gateDialectFile(r.Path, content).excluded != "" {
			continue
		}
		if f, ok := p.routeFile(svc.ID, idx, r.Path, content); ok {
			files = append(files, f)
		}
	}
	res := ResolveRouteRefs(files)
	edges := RouteRefEdges(res.Refs)
	if err := p.st.PutTypedEdges(ctx, svc.ID, routeRefPackageKey, edges); err != nil {
		return routeRefReport{}, fmt.Errorf("route-ref edges: store: %w", err)
	}
	p.commitPassState(ctx, svc.ID, routeRefStateKey)
	return routeRefReport{edges: len(edges), unresolved: res.UnresolvedByReason()}, nil
}
