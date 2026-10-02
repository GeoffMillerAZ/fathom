package main

// routes.go — the oracle's route pairs (LB-F99), written from the
// documented resolution rules, never from fathom's resolver:
//
//   - Kubernetes Service DNS (kubernetes.io/docs/concepts/services-networking/
//     dns-pod-service): a Service is reachable as <svc>.<ns>.svc.<cluster
//     domain> (cluster.local by default), and through a pod's search list
//     (<ns>.svc.cluster.local, svc.cluster.local, cluster.local) as <svc>
//     from its own namespace, <svc>.<ns> and <svc>.<ns>.svc from any.
//   - Istio (istio.io/latest/docs/reference/config/networking/virtual-service,
//     destination-rule): a destination or DestinationRule host written as a
//     short name is interpreted relative to the rule's namespace; a Gateway
//     server's hosts are dnsNames, optionally prefixed `namespace/`.
//   - Gateway API (gateway-api.sigs.k8s.io/reference/spec, BackendObjectReference):
//     a backendRef names a Service (group "" and kind Service, the defaults)
//     in the route's namespace unless it names another.
//   - Ingress: a backend names a Service in the Ingress' own namespace.
//   - Traefik IngressRoute: a service is a Kubernetes Service (kind Service,
//     the default) in the route's namespace unless it names another; a
//     TraefikService is Traefik's own and names none.
//   - compose: a service is reachable from the others by its key.
//   - mongobetween (coinbase/mongobetween README): each positional
//     address=uri argument proxies to the hosts of the URI.
//
// A target resolves to the declarations of that Service (or compose
// service) in the subject: those in the referring file, else in its
// directory, else in the subject — each a declaration of the one object,
// so each is a pair; declarations of two different objects (a compose
// service and a Service both named by a bare name) at the nearest distance
// are ambiguous. A target that names no host is unresolved by reason:
// variable, unix-socket, ip-literal (and localhost), external (a host not of
// the cluster's forms, an SRV seed list, a wildcard), undeclared (a cluster
// form no Service declares).

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type routeRef struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
	To     string `json:"to"`
	Raw    string `json:"raw"`
}

type routeUnresolved struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	Raw    string `json:"raw"`
	Reason string `json:"reason"`
}

type crossCheck struct {
	Tool     string   `json:"tool"`
	Tier     string   `json:"tier"`
	Checked  int      `json:"checked"`
	Agree    int      `json:"agree"`
	Disagree []string `json:"disagree"`
}

// target is one routing reference: a host to read by the DNS rules, or an
// explicit Service (svc, ns) when the field names one structurally.
type target struct {
	path, from, raw string
	ns              string // the referrer's namespace ("" for a compose referrer)
	k8s             bool   // the referrer is a Kubernetes object
	svc, svcNS      string // a structural Service reference (Gateway API, Traefik, Ingress)
	istio           bool   // an Istio VirtualService destination (the cross-check's set)
	subset          string // its subset, when it names one
	kind            string // the referrer's kind
	name            string // the referrer's own name
}

type decl struct{ path, name, key string }

type routeWalk struct {
	targets  []target
	services map[[2]string][]decl // (ns, svc) -> declarations
	compose  map[string][]decl    // compose service key -> declarations
	istio    []string             // files holding an Istio resource
	drs      []destRule           // Istio DestinationRules (the cross-check's other side)
}

// destRule is one DestinationRule's host and subsets.
type destRule struct {
	ns, host string
	subsets  map[string]bool
}

func nsOf(root *yaml.Node) string {
	if ns := scalar(path(root, "metadata", "namespace")); ns != "" {
		return ns
	}
	return "default"
}

var composeBase = regexp.MustCompile(`^(docker-)?compose([.-].+)?\.ya?ml$`)

// document reads one YAML document of rel.
func (w *routeWalk) document(rel string, root *yaml.Node) {
	if w.services == nil {
		w.services, w.compose = map[[2]string][]decl{}, map[string][]decl{}
	}
	if composeBase.MatchString(filepath.Base(rel)) {
		w.composeDoc(rel, root)
		return
	}
	kind, name := scalar(get(root, "kind")), scalar(path(root, "metadata", "name"))
	if kind == "" || name == "" || get(root, "metadata") == nil {
		return
	}
	ns := nsOf(root)
	entity := kind + "/" + name
	if m := scalar(path(root, "metadata", "namespace")); m != "" {
		entity = kind + "/" + m + "/" + name
	}
	add := func(t target) {
		t.path, t.from, t.ns, t.k8s, t.kind, t.name = rel, entity, ns, true, kind, name
		w.targets = append(w.targets, t)
	}
	api := scalar(get(root, "apiVersion"))
	switch {
	case kind == "Service" && api == "v1":
		k := [2]string{ns, name}
		w.services[k] = append(w.services[k], decl{rel, entity, "svc:" + ns + "/" + name})
	case kind == "Ingress":
		for _, b := range []*yaml.Node{path(root, "spec", "defaultBackend"), path(root, "spec", "backend")} {
			if s := ingressService(b); s != "" {
				add(target{raw: s, svc: s, svcNS: ns})
			}
		}
		for _, rule := range seq(path(root, "spec", "rules")) {
			for _, p := range seq(path(rule, "http", "paths")) {
				if s := ingressService(get(p, "backend")); s != "" {
					add(target{raw: s, svc: s, svcNS: ns})
				}
			}
		}
	case strings.HasPrefix(api, "gateway.networking.k8s.io/") && strings.HasSuffix(kind, "Route"):
		for _, rule := range seq(path(root, "spec", "rules")) {
			for _, b := range seq(get(rule, "backendRefs")) {
				n := scalar(get(b, "name"))
				if n == "" {
					continue
				}
				group, bk := scalar(get(b, "group")), scalar(get(b, "kind"))
				if group != "" || (bk != "" && bk != "Service") {
					continue // not a Service: names no route-host target
				}
				bns := scalar(get(b, "namespace"))
				if bns == "" {
					bns = ns
				}
				add(target{raw: n, svc: n, svcNS: bns})
			}
		}
	case strings.HasPrefix(api, "networking.istio.io/") && kind == "VirtualService":
		w.istio = append(w.istio, rel)
		for _, proto := range []string{"http", "tcp", "tls"} {
			for _, r := range seq(path(root, "spec", proto)) {
				for _, d := range seq(get(r, "route")) {
					if h := scalar(path(d, "destination", "host")); h != "" {
						add(target{raw: h, istio: true, subset: scalar(path(d, "destination", "subset"))})
					}
				}
			}
		}
	case strings.HasPrefix(api, "networking.istio.io/") && kind == "DestinationRule":
		w.istio = append(w.istio, rel)
		if h := scalar(path(root, "spec", "host")); h != "" {
			add(target{raw: h})
			dr := destRule{ns: ns, host: h, subsets: map[string]bool{}}
			for _, sub := range seq(path(root, "spec", "subsets")) {
				if n := scalar(get(sub, "name")); n != "" {
					dr.subsets[n] = true
				}
			}
			w.drs = append(w.drs, dr)
		}
	case strings.HasPrefix(api, "networking.istio.io/") && kind == "Gateway":
		w.istio = append(w.istio, rel)
		for _, srv := range seq(path(root, "spec", "servers")) {
			for _, h := range seq(get(srv, "hosts")) {
				if v := scalar(h); v != "" {
					add(target{raw: v})
				}
			}
		}
	case strings.HasPrefix(api, "networking.istio.io/"):
		w.istio = append(w.istio, rel)
	case strings.HasPrefix(api, "traefik.") && (kind == "IngressRoute" || kind == "IngressRouteTCP"):
		for _, r := range seq(path(root, "spec", "routes")) {
			for _, s := range seq(get(r, "services")) {
				n := scalar(get(s, "name"))
				if n == "" || (scalar(get(s, "kind")) != "" && scalar(get(s, "kind")) != "Service") {
					continue
				}
				sns := scalar(get(s, "namespace"))
				if sns == "" {
					sns = ns
				}
				add(target{raw: n, svc: n, svcNS: sns})
			}
		}
	}
	// mongobetween launched by a container of this object.
	for _, pod := range []*yaml.Node{get(root, "spec"), path(root, "spec", "template", "spec"), path(root, "spec", "jobTemplate", "spec", "template", "spec")} {
		for _, list := range []string{"containers", "initContainers"} {
			for _, c := range seq(get(pod, list)) {
				cmd, args := strs(get(c, "command")), strs(get(c, "args"))
				if !mongobetween(scalar(get(c, "image")), cmd) {
					continue
				}
				for _, h := range mongoHosts(append(trimBin(cmd), args...)) {
					add(target{raw: h})
				}
			}
		}
	}
}

func ingressService(b *yaml.Node) string {
	if s := scalar(path(b, "service", "name")); s != "" {
		return s
	}
	return scalar(get(b, "serviceName"))
}

func (w *routeWalk) composeDoc(rel string, root *yaml.Node) {
	svcs := get(root, "services")
	if svcs == nil || svcs.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(svcs.Content); i += 2 {
		key, svc := svcs.Content[i].Value, svcs.Content[i+1]
		w.compose[key] = append(w.compose[key], decl{rel, key, "host:" + key})
		entry, cmd := strs(get(svc, "entrypoint")), strs(get(svc, "command"))
		if !mongobetween(scalar(get(svc, "image")), append(append([]string(nil), entry...), cmd...)) {
			continue
		}
		for _, h := range mongoHosts(append(trimBin(entry), trimBin(cmd)...)) {
			w.targets = append(w.targets, target{path: rel, from: key, raw: h})
		}
	}
}

func strs(n *yaml.Node) []string {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.ScalarNode {
		return strings.Fields(strings.NewReplacer(`"`, " ", `'`, " ").Replace(n.Value))
	}
	var out []string
	for _, it := range seq(n) {
		if it.Kind == yaml.ScalarNode {
			out = append(out, it.Value)
		}
	}
	return out
}

func mongobetween(image string, cmd []string) bool {
	repo := image[strings.LastIndex(image, "/")+1:]
	if i := strings.IndexAny(repo, ":@"); i >= 0 {
		repo = repo[:i]
	}
	return strings.Contains(repo, "mongobetween") || (len(cmd) > 0 && filepath.Base(cmd[0]) == "mongobetween")
}

func trimBin(cmd []string) []string {
	if len(cmd) > 0 && filepath.Base(cmd[0]) == "mongobetween" {
		return cmd[1:]
	}
	return cmd
}

// mongoHosts: every host of each positional address=uri; flags with a value
// (-loglevel -network -password -statsd -username -dynamic) skip theirs.
func mongoHosts(args []string) []string {
	valued := map[string]bool{"loglevel": true, "network": true, "password": true, "statsd": true, "username": true, "dynamic": true}
	var out []string
	pos := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !pos && strings.HasPrefix(a, "-") {
			f := strings.TrimLeft(a, "-")
			if a == "--" {
				pos = true
			} else if !strings.Contains(f, "=") && valued[f] {
				i++
			}
			continue
		}
		pos = true
		_, uri, ok := strings.Cut(a, "=")
		if !ok {
			continue
		}
		low := strings.ToLower(uri)
		switch {
		case strings.HasPrefix(low, "mongodb://"):
			hosts := uri[len("mongodb://"):]
			if i := strings.IndexAny(hosts, "/?"); i >= 0 {
				hosts = hosts[:i]
			}
			if i := strings.LastIndex(hosts, "@"); i >= 0 {
				hosts = hosts[i+1:]
			}
			for _, h := range strings.Split(hosts, ",") {
				if h != "" {
					out = append(out, h)
				}
			}
		case strings.HasPrefix(low, "mongodb+srv://") || strings.Contains(uri, "$"):
			out = append(out, uri)
		}
	}
	return out
}

// classify reads a raw target: the reason it names no host, or the host.
func classify(raw string) (host, reason string) {
	s := strings.TrimSpace(raw)
	low := strings.ToLower(s)
	switch {
	case strings.Contains(s, "$") || strings.Contains(s, "{{"):
		return "", "variable"
	case strings.HasPrefix(s, "unix:") || strings.HasPrefix(s, "/") || strings.HasSuffix(s, ".sock") || strings.Contains(low, "%2f"):
		return "", "unix-socket"
	case strings.HasPrefix(low, "mongodb+srv://"):
		return "", "external"
	}
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
		if j := strings.IndexAny(s, "/?#"); j >= 0 {
			s = s[:j]
		}
	} else if ns, h, ok := strings.Cut(s, "/"); ok && !strings.Contains(ns, ".") {
		s = h // Istio's namespace/dnsName
	}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	if h, _, err := net.SplitHostPort(s); err == nil {
		s = h
	} else if strings.HasPrefix(s, "[") {
		return "", "ip-literal"
	}
	s = strings.TrimSuffix(strings.ToLower(s), ".")
	switch {
	case s == "":
		return "", "undeclared"
	case s == "localhost" || net.ParseIP(s) != nil:
		return "", "ip-literal"
	case strings.Contains(s, "*"):
		return "", "external"
	}
	return s, ""
}

// resolve joins every target to its declaration.
func (w *routeWalk) resolve() ([]routeRef, []routeUnresolved) {
	refs, unresolved := []routeRef{}, []routeUnresolved{}
	seen := map[routeRef]bool{}
	for _, t := range w.targets {
		var cands []decl
		missReason := "undeclared"
		if t.svc != "" {
			cands = w.services[[2]string{t.svcNS, t.svc}]
		} else {
			host, reason := classify(t.raw)
			if reason != "" {
				unresolved = append(unresolved, routeUnresolved{t.path, t.from, t.raw, reason})
				continue
			}
			labels := strings.Split(host, ".")
			switch {
			case len(labels) == 1:
				if t.k8s {
					cands = w.services[[2]string{t.ns, host}]
				} else {
					cands = append(append([]decl(nil), w.compose[host]...), w.services[[2]string{"default", host}]...)
				}
			case len(labels) == 2:
				cands = append(append([]decl(nil), w.services[[2]string{labels[1], labels[0]}]...), w.compose[host]...)
				missReason = "external"
			case len(labels) == 3 && labels[2] == "svc", len(labels) == 5 && strings.Join(labels[2:], ".") == "svc.cluster.local":
				cands = w.services[[2]string{labels[1], labels[0]}]
			default:
				cands = w.compose[host]
				missReason = "external"
			}
		}
		var near []decl
		for _, in := range []func(d decl) bool{
			func(d decl) bool { return d.path == t.path },
			func(d decl) bool { return filepath.Dir(d.path) == filepath.Dir(t.path) },
			func(decl) bool { return true },
		} {
			for _, d := range cands {
				if in(d) {
					near = append(near, d)
				}
			}
			if len(near) > 0 {
				break
			}
		}
		keys := map[string]bool{}
		for _, d := range near {
			keys[d.key] = true
		}
		switch {
		case len(near) == 0:
			unresolved = append(unresolved, routeUnresolved{t.path, t.from, t.raw, missReason})
		case len(keys) > 1:
			unresolved = append(unresolved, routeUnresolved{t.path, t.from, t.raw, "ambiguous"})
		default:
			// every declaration of the one object, the referrer itself aside
			for _, d := range near {
				if d.path == t.path && d.name == t.from {
					continue
				}
				r := routeRef{t.path, t.from, d.path, d.name, t.raw}
				if !seen[r] {
					seen[r] = true
					refs = append(refs, r)
				}
			}
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		a, b := refs[i], refs[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To+"\x00"+a.Raw < b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To+"\x00"+b.Raw
	})
	sort.Slice(unresolved, func(i, j int) bool {
		a, b := unresolved[i], unresolved[j]
		return a.Path+"\x00"+a.From+"\x00"+a.Raw+"\x00"+a.Reason < b.Path+"\x00"+b.From+"\x00"+b.Raw+"\x00"+b.Reason
	})
	return refs, unresolved
}

// istioCrossCheck runs `istioctl analyze --use-kube=false -A` over every
// file holding an Istio resource. Offline, istioctl skips its destination
// host analyzer (virtualservice.DestinationHostAnalyzer needs the cluster's
// Services; `--verbose` lists it under "Skipped analyzers" in 1.31.1), but
// it runs virtualservice.DestinationRuleAnalyzer, whose IST0101 "Referenced
// host+subset in destinationrule not found" applies Istio's own host
// normalization to a VirtualService destination and to every
// DestinationRule host. So each destination naming a subset is checked: the
// oracle's reading (the destination and a DestinationRule declaring that
// subset name the same Service or host, by the DNS rules above) agrees when
// istioctl flags it exactly when the oracle finds no such rule.
func istioCrossCheck(bin, subject string, w *routeWalk, _ []routeRef, _ []routeUnresolved) (*crossCheck, error) {
	ver, _ := exec.Command(bin, "version", "--remote=false", "--short").Output()
	v := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(ver)), "client version:"))
	c := &crossCheck{Tool: "istioctl analyze " + v + " (IST0101 host+subset)", Tier: "A", Disagree: []string{}}
	files := map[string]bool{}
	for _, f := range w.istio {
		files[f] = true
	}
	if len(files) == 0 {
		return c, nil
	}
	var list []string
	for f := range files {
		list = append(list, filepath.Join(subject, filepath.FromSlash(f)))
	}
	sort.Strings(list)
	out, err := exec.Command(bin, append([]string{"analyze", "--use-kube=false", "-A", "-o", "json"}, list...)...).Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("%s: %w", bin, err)
	}
	var msgs []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Origin  string `json:"origin"`
	}
	if err := json.Unmarshal(out, &msgs); err != nil {
		return nil, fmt.Errorf("istioctl output: %w", err)
	}
	flagged := map[string]bool{} // "<origin>|<host>+<subset>"
	re := regexp.MustCompile(`Referenced host\+subset in destinationrule not found: "([^"]+)"`)
	for _, m := range msgs {
		if m.Code != "IST0101" {
			continue
		}
		if h := re.FindStringSubmatch(m.Message); h != nil {
			flagged[m.Origin+"|"+h[1]] = true
		}
	}
	rules := map[string]map[string]bool{}
	for _, dr := range w.drs {
		k := canonHost(dr.host, dr.ns)
		if rules[k] == nil {
			rules[k] = map[string]bool{}
		}
		for sub := range dr.subsets {
			rules[k][sub] = true
		}
	}
	for _, t := range w.targets {
		if !t.istio || t.subset == "" {
			continue
		}
		c.Checked++
		found := rules[canonHost(t.raw, t.ns)][t.subset]
		isFlagged := flagged["VirtualService "+t.ns+"/"+t.name+"|"+t.raw+"+"+t.subset]
		if isFlagged == !found {
			c.Agree++
			continue
		}
		c.Disagree = append(c.Disagree, fmt.Sprintf("%s %s %s+%s (istioctl not found %v, oracle rule found %v)", t.path, t.from, t.raw, t.subset, isFlagged, found))
	}
	sort.Strings(c.Disagree)
	return c, nil
}

// canonHost is the oracle's identity for a host written in namespace ns:
// a cluster form (name, name.ns, name.ns.svc, name.ns.svc.cluster.local)
// is the Service ns/name it names, anything else the host itself.
func canonHost(raw, ns string) string {
	host, reason := classify(raw)
	if reason != "" {
		return raw
	}
	labels := strings.Split(host, ".")
	switch {
	case len(labels) == 1:
		return "svc:" + ns + "/" + host
	case len(labels) == 2, len(labels) == 3 && labels[2] == "svc",
		len(labels) == 5 && strings.Join(labels[2:], ".") == "svc.cluster.local":
		return "svc:" + labels[1] + "/" + labels[0]
	}
	return host
}
