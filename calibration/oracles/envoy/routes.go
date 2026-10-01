package main

// routes.go — the route-target truth (the L3 route_ref layer, LB-F96's
// resolver at integration): where a service host, a service url or an upstream
// target is one of the subject's own deployed services. A host naming a
// docker-compose service (the service's key) or a Kubernetes Service
// (metadata.name) of the subject resolves to it; every other host is
// disclosed with its reason. Bench-only; lang-oracle ignores these fields.

import (
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
	// Tier is "C" for an address read from a file the tool did not accept
	// (written text, joined to the subject's services); empty is the file's
	// tool tier.
	Tier string `json:"tier,omitempty"`
}

type routeUnresolved struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	Raw    string `json:"raw"`
	Reason string `json:"reason"`
	Tier   string `json:"tier,omitempty"`
}

// deployed is the subject's deployed service names: name -> files declaring it.
type deployed map[string][]string

var hostRE = regexp.MustCompile(`^(?:[a-zA-Z][a-zA-Z0-9+.-]*://)?(?:[^@/]*@)?(\[[0-9a-fA-F:.]+\]|[^:/?#\s]+)`)

// hostOf is the host token of an address as written: a bare name, host:port,
// scheme://host:port/path or a bracketed IPv6 literal.
func hostOf(raw string) string {
	m := hostRE.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return ""
	}
	return strings.Trim(m[1], "[]")
}

// classify resolves raw against the deployed services: the declaring file and
// service name, or the reason it is not one of them.
func (d deployed) classify(raw string) (toPath, to, reason string) {
	t := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(t, "unix:") || strings.HasPrefix(t, "/") && !strings.Contains(t, "://"):
		return "", "", "unix-socket"
	case strings.Contains(t, "${") || strings.Contains(t, "{{") || strings.HasPrefix(t, "$"):
		return "", "", "variable"
	}
	h := hostOf(t)
	if h == "" {
		return "", "", "undeclared"
	}
	if net.ParseIP(h) != nil {
		return "", "", "ip-literal"
	}
	files := d[h]
	switch {
	case len(files) == 1:
		return files[0], h, ""
	case len(files) > 1:
		return "", "", "ambiguous"
	case strings.Contains(h, "."):
		return "", "", "external"
	}
	return "", "", "undeclared"
}

// scanDeployed reads every compose file and Kubernetes manifest of the subject.
func scanDeployed(subject string) deployed {
	d := deployed{}
	add := func(name, rel string) {
		for _, f := range d[name] {
			if f == rel {
				return
			}
		}
		d[name] = append(d[name], rel)
	}
	if err := filepath.WalkDir(subject, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			if e.Name() == ".git" || e.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !e.Type().IsRegular() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		b, err := readFile(p)
		if err != nil {
			panic(err)
		}
		if len(b) > 2<<20 {
			return nil
		}
		rel, _ := filepath.Rel(subject, p)
		rel = filepath.ToSlash(rel)
		dec := yaml.NewDecoder(strings.NewReader(string(b)))
		for {
			var doc map[string]any
			if err := dec.Decode(&doc); err != nil {
				break
			}
			if kind, _ := doc["kind"].(string); kind == "Service" {
				if md, ok := doc["metadata"].(map[string]any); ok {
					if n, ok := md["name"].(string); ok && n != "" {
						add(n, rel)
					}
				}
				continue
			}
			if svcs, ok := doc["services"].(map[string]any); ok && doc["_format_version"] == nil {
				for n, v := range svcs {
					if m, ok := v.(map[string]any); ok && (m["image"] != nil || m["build"] != nil) {
						add(n, rel)
					}
				}
			}
		}
		return nil
	}); err != nil {
		panic(err)
	}
	for _, fs := range d {
		sort.Strings(fs)
	}
	return d
}
