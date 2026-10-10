package main

// routes.go — the route-target truth (bench-only; lang-oracle ignores these
// fields, lang-routes scores them): a scrape job's static address and a
// rule's identifying-label equality matcher name a deployed service of the
// subject, a docker-compose service (its key, read by `docker compose
// config`) or a Kubernetes Service (metadata.name in its namespace). A static
// address joined to a service Docker Compose itself read is Compose's own DNS
// (tier A, the traefik oracle's rule); a join to a Kubernetes Service is this
// oracle's reading of the cluster DNS rules (a bare name in a namespace,
// `name.ns`, `name.ns.svc[.cluster.local]`), and an identifying label's value
// naming a service is a labelling convention, never DNS: both tier C.

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	// Tier is "C" for a pair this oracle's own reading draws (a Kubernetes
	// Service, a compose file Compose refused, a label's value); empty is the
	// file's tier.
	Tier string `json:"tier,omitempty"`
}

type routeUnresolved struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	Raw    string `json:"raw"`
	Reason string `json:"reason"`
}

// provider is one declaration of a deployed service: its file, the entity
// name the dialect reading the file gives it, and the key it provides
// (`k8s/<ns>/<name>` or `host/<name>`).
type provider struct{ file, entity, key, tier string }

var hostRE = regexp.MustCompile(`^(?:[a-zA-Z][a-zA-Z0-9+.-]*://)?(?:[^@/]*@)?(\[[0-9a-fA-F:.]+\]|[^:/?#\s]+)`)

// keysOf is the deploy keys a target may name, the reason it names none, or
// the reason to give when no provider declares them.
func keysOf(t target) (keys []string, miss, reason string) {
	host := t.value
	if !t.label {
		m := hostRE.FindStringSubmatch(strings.TrimSpace(t.raw))
		if m == nil {
			return nil, "", "undeclared"
		}
		host = strings.Trim(m[1], "[]")
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	switch {
	case strings.Contains(t.raw, "$") || strings.Contains(t.raw, "{{"):
		return nil, "", "variable"
	case host == "localhost" || net.ParseIP(host) != nil:
		return nil, "", "ip-literal"
	}
	parts := strings.Split(host, ".")
	switch {
	case len(parts) == 1 && t.ns != "":
		return []string{"k8s/" + t.ns + "/" + host}, "undeclared", ""
	case len(parts) == 1:
		return []string{"host/" + host, "k8s/default/" + host}, "undeclared", ""
	case len(parts) == 2:
		return []string{"k8s/" + parts[1] + "/" + parts[0], "host/" + host}, "external", ""
	case len(parts) == 3 && parts[2] == "svc", len(parts) == 5 && strings.Join(parts[2:], ".") == "svc.cluster.local":
		return []string{"k8s/" + parts[1] + "/" + parts[0]}, "undeclared", ""
	}
	return []string{"host/" + host}, "external", ""
}

// resolveRoutes joins every pending target to the nearest providers of its
// keys (its own file, its directory, then the subject), one key only.
func (r *result) resolveRoutes(subject string) {
	provs := scanProviders(subject)
	byKey := map[string][]provider{}
	for _, p := range provs {
		byKey[p.key] = append(byKey[p.key], p)
	}
	seen := map[routeRef]bool{}
	for _, t := range r.pending {
		keys, miss, reason := keysOf(t)
		if reason != "" {
			r.unresolved = append(r.unresolved, routeUnresolved{Path: t.path, From: t.from, Raw: t.raw, Reason: reason})
			continue
		}
		var cands []provider
		for _, k := range keys {
			cands = append(cands, byKey[k]...)
		}
		var near []provider
		for _, scope := range []func(string) bool{
			func(f string) bool { return f == t.path },
			func(f string) bool { return path.Dir(f) == path.Dir(t.path) },
			func(string) bool { return true },
		} {
			for _, c := range cands {
				if scope(c.file) {
					near = append(near, c)
				}
			}
			if len(near) > 0 {
				break
			}
		}
		ks := map[string]bool{}
		for _, c := range near {
			ks[c.key] = true
		}
		switch {
		case len(near) == 0:
			r.unresolved = append(r.unresolved, routeUnresolved{Path: t.path, From: t.from, Raw: t.raw, Reason: miss})
		case len(ks) > 1:
			r.unresolved = append(r.unresolved, routeUnresolved{Path: t.path, From: t.from, Raw: t.raw, Reason: "ambiguous"})
		default:
			for _, c := range near {
				ref := routeRef{Path: t.path, From: t.from, ToPath: c.file, To: c.entity, Raw: t.raw}
				if c.tier == "C" || t.label {
					ref.Tier = "C"
				}
				if !seen[ref] {
					seen[ref] = true
					r.routes = append(r.routes, ref)
				}
			}
		}
	}
}

// routesTier is a routes file's tier: A when every pair is Compose's own
// reading, C when any is this oracle's.
func routesTier(refs []routeRef) string {
	for _, r := range refs {
		if r.Tier == "C" {
			return "C"
		}
	}
	return "A"
}

func composeNamed(base string) bool {
	b := strings.ToLower(base)
	if !strings.HasSuffix(b, ".yml") && !strings.HasSuffix(b, ".yaml") {
		return false
	}
	return strings.HasPrefix(b, "compose.") || strings.HasPrefix(b, "compose-") || strings.HasPrefix(b, "docker-compose.") || strings.HasPrefix(b, "docker-compose-")
}

// composeKeys is the service names `docker compose config` reads from one
// compose file, offline, with no consistency check; ok is false when Compose
// refuses the file or is absent.
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

// scanProviders reads every compose file and Kubernetes Service of the subject.
func scanProviders(subject string) []provider {
	var out []provider
	_ = filepath.WalkDir(subject, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			if p != subject && skipDirs[e.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".yml" && ext != ".yaml" {
			return nil
		}
		rel, _ := filepath.Rel(subject, p)
		rel = filepath.ToSlash(rel)
		if composeNamed(filepath.Base(p)) {
			if keys, ok := composeKeys(p); ok {
				for _, k := range keys {
					out = append(out, provider{rel, k, "host/" + k, "A"})
				}
				return nil
			}
		}
		b, err := os.ReadFile(p)
		if err != nil || len(b) > 2<<20 {
			return nil
		}
		dec := yaml.NewDecoder(bytes.NewReader(b))
		for {
			var doc map[string]any
			if dec.Decode(&doc) != nil {
				break
			}
			if kind, _ := doc["kind"].(string); kind == "Service" {
				md, _ := doc["metadata"].(map[string]any)
				name, _ := md["name"].(string)
				if name == "" {
					continue
				}
				ns, _ := md["namespace"].(string)
				entity := "Service/" + name
				if ns != "" {
					entity = "Service/" + ns + "/" + name
				} else {
					ns = "default"
				}
				out = append(out, provider{rel, entity, "k8s/" + ns + "/" + name, "C"})
				continue
			}
			// a compose file by another name (a swarm stack file): Compose's
			// own reading when it accepts the file, else its services with an
			// image or a build, read by yaml.v3
			if svcs, ok := doc["services"].(map[string]any); ok {
				if keys, ok := composeKeys(p); ok {
					for _, k := range keys {
						out = append(out, provider{rel, k, "host/" + k, "A"})
					}
					continue
				}
				for n, v := range svcs {
					if m, ok := v.(map[string]any); ok && (m["image"] != nil || m["build"] != nil) {
						out = append(out, provider{rel, n, "host/" + n, "C"})
					}
				}
			}
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		return fmt.Sprint(out[i]) < fmt.Sprint(out[j])
	})
	return out
}

// ── derive ────────────────────────────────────────────────────────────────

// deriveRules writes each PrometheusRule custom resource's spec.groups as a
// plain rule file at the same relative path under out.
func deriveRules(subject, out string) (int, error) {
	n := 0
	err := filepath.WalkDir(subject, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			if p != subject && skipDirs[e.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".yml" && ext != ".yaml" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var groups []*yaml.Node
		dec := yaml.NewDecoder(bytes.NewReader(b))
		for {
			var doc yaml.Node
			if dec.Decode(&doc) != nil {
				break
			}
			if len(doc.Content) == 0 {
				continue
			}
			root := doc.Content[0]
			if k := get(root, "kind"); k == nil || k.Value != "PrometheusRule" {
				continue
			}
			if g := get(get(root, "spec"), "groups"); g != nil && g.Kind == yaml.SequenceNode {
				groups = append(groups, g.Content...)
			}
		}
		if len(groups) == 0 {
			return nil
		}
		rel, _ := filepath.Rel(subject, p)
		doc := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "groups"},
			{Kind: yaml.SequenceNode, Content: groups},
		}}
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(doc); err != nil {
			return err
		}
		dst := filepath.Join(out, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		n++
		return os.WriteFile(dst, buf.Bytes(), 0o644)
	})
	return n, err
}
