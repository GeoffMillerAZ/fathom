package main

// routes.go — the route-target truth (the L3 route_ref layer, LB-F96's
// resolver at integration): where a service host, a service url or an upstream
// target is one of the subject's own deployed services. A host naming a
// docker-compose service (the service's key) or a Kubernetes Service
// (metadata.name) of the subject resolves to it; every other host is
// disclosed with its reason. Bench-only; lang-oracle ignores these fields.

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path"
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

// decl is one declaration of a deployed service: its file, tier and the entity name
// the dialect that reads the file gives it (a compose service is its key, a
// Kubernetes Service `Service/<namespace>/<name>`, `Service/<name>` with none).
type decl struct {
	File, Entity string
	// Tier is A when `docker compose config` read the declaring compose file,
	// C when the oracle read it with yaml.v3 (a Kubernetes Service, a file
	// Compose refuses).
	Tier string
}

// deployed is the subject's deployed service names: name -> declarations.
type deployed map[string][]decl

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

// classify resolves raw, written in the file from, against the deployed
// services: the declaring files and service name, or the reason it is not one
// of them. A compose project is a directory (compose resolves a stack from the
// directory of its files), so the declarations that count are the nearest
// project's: the route's own directory, then each ancestor, every file of that
// directory that declares the service (an override file is the same service).
// A name no ancestor directory declares joins only when one file of the
// subject declares it; several files elsewhere in the subject are ambiguous.
func (d deployed) classify(raw, from string) (decls []decl, reason string) {
	t := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(t, "unix:") || strings.HasPrefix(t, "/") && !strings.Contains(t, "://"):
		return nil, "unix-socket"
	case strings.Contains(t, "${") || strings.Contains(t, "{{") || strings.HasPrefix(t, "$"):
		return nil, "variable"
	}
	h := hostOf(t)
	if h == "" {
		return nil, "undeclared"
	}
	if net.ParseIP(h) != nil {
		return nil, "ip-literal"
	}
	files := d[h]
	for dir := path.Dir(from); ; dir = path.Dir(dir) {
		var near []decl
		for _, f := range files {
			if path.Dir(f.File) == dir {
				near = append(near, f)
			}
		}
		if len(near) > 0 {
			return near, ""
		}
		if dir == "." || dir == "/" {
			break
		}
	}
	switch {
	case len(files) == 1:
		return files, ""
	case len(files) > 1:
		return nil, "ambiguous"
	case strings.Contains(h, "."):
		return nil, "external"
	}
	return nil, "undeclared"
}

// composeNamed: a file Compose itself names (compose.y[a]ml, docker-compose.y[a]ml
// and their override or environment variants): every service key of it is a
// declaration, an override file's partial service included.
func composeNamed(base string) bool {
	b := strings.ToLower(base)
	if !strings.HasSuffix(b, ".yml") && !strings.HasSuffix(b, ".yaml") {
		return false
	}
	return strings.HasPrefix(b, "compose.") || strings.HasPrefix(b, "compose-") || strings.HasPrefix(b, "docker-compose.") || strings.HasPrefix(b, "docker-compose-")
}

// composeKeys is the service names `docker compose config` reads from one
// compose file, offline and without the consistency check (an override file
// holds services with no image or build); ok is false when Compose refuses the
// file (a version-1 file) or is not installed.
func composeKeys(file string) (keys []string, ok bool) {
	out, err := exec.Command("docker", "compose", "-f", file, "config", "--format", "json",
		"--no-interpolate", "--no-path-resolution", "--no-consistency", "--no-normalize").Output()
	if err != nil {
		return nil, false
	}
	var cfg struct {
		Services map[string]json.RawMessage `json:"services"`
	}
	if json.Unmarshal(out, &cfg) != nil {
		return nil, false
	}
	for n := range cfg.Services {
		keys = append(keys, n)
	}
	sort.Strings(keys)
	return keys, true
}

// tierC is a decl's tier as a routeRef's Tier: empty (the file's tool tier) for A.
func tierC(tier string) string {
	if tier == "C" {
		return "C"
	}
	return ""
}

// routesTier is the tier a routes file is scored at: A when every pair's
// compose side was read by Docker Compose, C when any was read by yaml.v3.
func routesTier(refs []routeRef) string {
	for _, r := range refs {
		if r.Tier == "C" {
			return "C"
		}
	}
	return "A"
}

// scanDeployed reads every compose file and Kubernetes manifest of the subject.
func scanDeployed(subject string) deployed {
	d := deployed{}
	add := func(name, rel, entity, tier string) {
		for _, f := range d[name] {
			if f.File == rel && f.Entity == entity {
				return
			}
		}
		d[name] = append(d[name], decl{rel, entity, tier})
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
		if composeNamed(filepath.Base(p)) {
			// Docker Compose's own reading of the file: the services it declares
			// (an override file's partial services included), tier A
			if keys, ok := composeKeys(p); ok {
				for _, n := range keys {
					add(n, rel, n, "A")
				}
				return nil
			}
		}
		dec := yaml.NewDecoder(strings.NewReader(string(b)))
		for {
			var doc map[string]any
			if err := dec.Decode(&doc); err != nil {
				break
			}
			if kind, _ := doc["kind"].(string); kind == "Service" {
				if md, ok := doc["metadata"].(map[string]any); ok {
					if n, ok := md["name"].(string); ok && n != "" {
						entity := "Service/" + n
						if ns, _ := md["namespace"].(string); ns != "" {
							entity = "Service/" + ns + "/" + n
						}
						add(n, rel, entity, "C")
					}
				}
				continue
			}
			if svcs, ok := doc["services"].(map[string]any); ok && doc["_format_version"] == nil {
				for n, v := range svcs {
					if m, ok := v.(map[string]any); ok && (composeNamed(filepath.Base(p)) || m["image"] != nil || m["build"] != nil) {
						add(n, rel, n, "C")
					}
				}
			}
		}
		return nil
	}); err != nil {
		panic(err)
	}
	for _, fs := range d {
		sort.Slice(fs, func(i, j int) bool {
			return fs[i].File < fs[j].File || fs[i].File == fs[j].File && fs[i].Entity < fs[j].Entity
		})
	}
	return d
}
