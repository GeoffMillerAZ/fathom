package main

// routes.go — the exporter route pairs (tier C): an exporter's endpoint host
// joined to the compose service or Kubernetes Service of the subject it
// names, written from the documented resolution rules (the kubernetes
// oracle's routes.go, mirrored for a non-Kubernetes referrer): a bare name
// is a compose service or the Service of that name in namespace default,
// name.ns a Service in ns or a host of that name, name.ns.svc and
// name.ns.svc.cluster.local the Service in ns, anything else a host a compose
// service of that key declares. The nearest declarations win (the referring
// file, its directory, the subject); declarations of two different objects
// at the nearest distance are ambiguous. A target naming no host is
// unresolved by reason: variable, unix-socket, ip-literal (and localhost),
// external, undeclared.

import (
	"io/fs"
	"net"
	"os"
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

type target struct{ path, from, raw string }

type decl struct{ path, name, key string }

type routeWalk struct {
	targets  []target
	services map[[2]string][]decl // (ns, name) -> Kubernetes Service declarations
	compose  map[string][]decl    // compose service key -> declarations
}

var composeBase = regexp.MustCompile(`^(docker-)?compose([.-].+)?\.ya?ml$`)

// providers reads the subject's compose services and Kubernetes Services.
func (w *routeWalk) providers(subject string) error {
	w.services, w.compose = map[[2]string][]decl{}, map[string][]decl{}
	return filepath.WalkDir(subject, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(p); ext != ".yaml" && ext != ".yml" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(subject, p)
		rel = filepath.ToSlash(rel)
		dec := yaml.NewDecoder(strings.NewReader(string(b)))
		for {
			var doc yaml.Node
			if dec.Decode(&doc) != nil {
				break
			}
			if len(doc.Content) == 0 {
				continue
			}
			root := doc.Content[0]
			if composeBase.MatchString(filepath.Base(rel)) {
				svcs := get(root, "services")
				if svcs == nil || svcs.Kind != yaml.MappingNode {
					continue
				}
				for i := 0; i+1 < len(svcs.Content); i += 2 {
					k := svcs.Content[i].Value
					w.compose[k] = append(w.compose[k], decl{rel, k, "host:" + k})
				}
				continue
			}
			if scalar(get(root, "kind")) != "Service" || scalar(get(root, "apiVersion")) != "v1" {
				continue
			}
			name := scalar(path(root, "metadata", "name"))
			if name == "" {
				continue
			}
			ns, entity := "default", "Service/"+name
			if m := scalar(path(root, "metadata", "namespace")); m != "" {
				ns, entity = m, "Service/"+m+"/"+name
			}
			k := [2]string{ns, name}
			w.services[k] = append(w.services[k], decl{rel, entity, "svc:" + ns + "/" + name})
		}
		return nil
	})
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
	}
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if j := strings.IndexAny(s, "/?#"); j >= 0 {
		s = s[:j]
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

// resolve joins every target to its declarations.
func (w *routeWalk) resolve() ([]routeRef, []routeUnresolved) {
	refs, unresolved := []routeRef{}, []routeUnresolved{}
	seen := map[routeRef]bool{}
	for _, t := range w.targets {
		host, reason := classify(t.raw)
		if reason != "" {
			unresolved = append(unresolved, routeUnresolved{t.path, t.from, t.raw, reason})
			continue
		}
		var cands []decl
		missReason := "undeclared"
		labels := strings.Split(host, ".")
		switch {
		case len(labels) == 1:
			cands = append(append([]decl(nil), w.compose[host]...), w.services[[2]string{"default", host}]...)
		case len(labels) == 2:
			cands = append(append([]decl(nil), w.services[[2]string{labels[1], labels[0]}]...), w.compose[host]...)
			missReason = "external"
		case len(labels) == 3 && labels[2] == "svc", len(labels) == 5 && strings.Join(labels[2:], ".") == "svc.cluster.local":
			cands = w.services[[2]string{labels[1], labels[0]}]
		default:
			cands = w.compose[host]
			missReason = "external"
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
			for _, d := range near {
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
