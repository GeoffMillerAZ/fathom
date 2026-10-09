// Command otelcol-oracle is the language-breadth harness's oracle for the
// otel-collector doc-engine dialect (LB-Q-otel-collector, design §1.3,
// calibration/oracles/otelcol/README.md). A separate Go module, never
// imported by fathom, never a product dependency.
//
//   - Acceptance (tier A): the collector itself. `otelcol-contrib validate
//     --config=file:<f>` loads, decodes and validates a configuration the way
//     the collector starts: unknown component types and fields, a pipeline
//     naming an undeclared component, an invalid component config are
//     refused. A file that does not validate alone is tried as a layer over
//     each sibling that does (the opentelemetry-demo layering, `--config base
//     --config layer`). Every referenced environment variable is given a
//     placeholder (a port-named one a port number), and an error line that is
//     about the host rather than the file (a Linux-only root_path, a storage
//     directory that must exist, the profiles feature gate) is disclosed and
//     not counted against the file; any other error rejects it.
//   - Definitions and references (tier A): the collector's own decoding of
//     what the dialect reads. Each accepted file is loaded with
//     go.opentelemetry.io/collector/confmap and decoded with confmap's own
//     Unmarshal into component.ID and pipeline.ID keys (the collector's ID
//     grammar, `type[/name]`) and the service's pipelines and extensions:
//     receivers, processors, exporters, connectors, extensions, pipelines,
//     and a pipeline to each component it lists (a reference when the
//     component is declared in the same file: the dialect's intra-file L2).
//   - With -rules-out (tier C, never scored: the dialect leaves these kinds
//     out of oracle_kinds): a resource or attributes processor's service.name
//     is a TelemetryIdentity (TL-5; a value with `${`, `{{` or `$(` is not a
//     literal and yields none), and a span_metrics/spanmetrics or
//     service_graph/servicegraph connector derives the metric families its
//     source names (connector/spanmetricsconnector connector.go
//     metricNameCalls, metricNameDuration, metricNameEvents under
//     factory.go DefaultNamespace or the connector's namespace;
//     connector/servicegraphconnector connector.go's SetName literals, at
//     collector-contrib v0.162.0): rules read off the source and the
//     contract, not the collector's decode.
//   - Route pairs (tier C, routes.go): an exporter's endpoint fields to the
//     compose service or Kubernetes Service of the subject its host names.
//
// Lines come from gopkg.in/yaml.v3 by key path: a map entry at its key, a
// list item at its first line (a decoder detail, never an entity decision).
//
//	go build -o <scratch>/otelcol-oracle . && <scratch>/otelcol-oracle -subject <checkout> -otelcol <otelcol-contrib> \
//	    -refs-out <refs.json> -routes-out <routes.json> -files-out <files.txt> > <defs.json>
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
	"gopkg.in/yaml.v3"
)

type def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	Kind string `json:"kind"`
}

type ref struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

type out struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	References      []ref  `json:"references"`
	CrossReferences []any  `json:"cross_references"`
}

// sections are the configuration's component sections and the canonical
// kind each maps to (the dialect's kinds).
var sections = []struct{ key, kind string }{
	{"receivers", "data_source"}, {"processors", "step"}, {"exporters", "output"},
	{"connectors", "route"}, {"extensions", "mixin"},
}

// collectorConfig is the part of the collector's configuration the oracle
// decodes with confmap: every component section keyed by component.ID and
// the service's pipelines keyed by pipeline.ID (otelcol.Config's shape).
type collectorConfig struct {
	Receivers  map[component.ID]any `mapstructure:"receivers"`
	Processors map[component.ID]any `mapstructure:"processors"`
	Exporters  map[component.ID]any `mapstructure:"exporters"`
	Connectors map[component.ID]any `mapstructure:"connectors"`
	Extensions map[component.ID]any `mapstructure:"extensions"`
	Service    struct {
		Extensions []component.ID                  `mapstructure:"extensions"`
		Pipelines  map[pipeline.ID]*pipelineConfig `mapstructure:"pipelines"`
	} `mapstructure:"service"`
}

type pipelineConfig struct {
	Receivers  []component.ID `mapstructure:"receivers"`
	Processors []component.ID `mapstructure:"processors"`
	Exporters  []component.ID `mapstructure:"exporters"`
}

func main() {
	subject := flag.String("subject", "", "subject checkout")
	otelcol := flag.String("otelcol", "", "otelcol-contrib binary (acceptance)")
	refsOut := flag.String("refs-out", "", "where the references go")
	routesOut := flag.String("routes-out", "", "where route_references and route_unresolved go")
	filesOut := flag.String("files-out", "", "the accepted files, one per line")
	rulesOut := flag.String("rules-out", "", "where the tier-C TelemetryIdentity and derived-metric definitions go")
	flag.Parse()
	if *subject == "" || *otelcol == "" || *refsOut == "" {
		fmt.Fprintln(os.Stderr, "usage: otelcol-oracle -subject <dir> -otelcol <bin> -refs-out <json> [-routes-out <json>] [-files-out <txt>]")
		os.Exit(2)
	}
	ver, _ := exec.Command(*otelcol, "--version").Output()
	version := strings.TrimSpace(string(ver))
	var candidates []string
	err := filepath.WalkDir(*subject, func(p string, d fs.DirEntry, err error) error {
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
		var root yaml.Node
		if yaml.Unmarshal(b, &root) != nil || len(root.Content) == 0 || get(root.Content[0], "service") == nil {
			return nil
		}
		rel, _ := filepath.Rel(*subject, p)
		candidates = append(candidates, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sort.Strings(candidates)
	accepted := accept(*otelcol, *subject, candidates)
	defs := out{Tool: "otelcol-contrib validate + collector confmap decode (component.ID, pipeline.ID)", Version: version + "; go.opentelemetry.io/collector v1.68.0", Tier: "A",
		Definitions: []def{}, References: []ref{}, CrossReferences: []any{}}
	refs := out{Tool: defs.Tool, Version: defs.Version, Tier: "A", Definitions: []def{}, References: []ref{}, CrossReferences: []any{}}
	rules := out{Tool: "otelcol-oracle rules (TL-5 identity, collector-contrib v0.162.0 derived metric names)", Version: "gopkg.in/yaml.v3 v3.0.1", Tier: "C",
		Definitions: []def{}, References: []ref{}, CrossReferences: []any{}}
	rw := &routeWalk{}
	var files []string
	for _, rel := range candidates {
		if !accepted[rel] {
			continue
		}
		b, err := os.ReadFile(filepath.Join(*subject, filepath.FromSlash(rel)))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		d, rd, r, routes, err := readConfig(rel, b)
		if err != nil {
			fmt.Fprintf(os.Stderr, "otelcol-oracle: %s: %v (left out)\n", rel, err)
			continue
		}
		files = append(files, rel)
		defs.Definitions = append(defs.Definitions, d...)
		rules.Definitions = append(rules.Definitions, rd...)
		refs.References = append(refs.References, r...)
		rw.targets = append(rw.targets, routes...)
	}
	if *routesOut != "" {
		if err := rw.providers(*subject); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		rr, ru := rw.resolve()
		o := map[string]any{"tool": "otelcol-oracle routes (yaml.v3 walk, the Kubernetes and compose DNS rules)", "version": "gopkg.in/yaml.v3 v3.0.1", "tier": "C",
			"definitions": []def{}, "references": []ref{}, "cross_references": []any{}, "route_references": rr, "route_unresolved": ru}
		if err := writeJSON(*routesOut, o); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err := writeJSON(*refsOut, refs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *rulesOut != "" {
		if err := writeJSON(*rulesOut, rules); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if *filesOut != "" {
		if err := os.WriteFile(*filesOut, []byte(strings.Join(files, "\n")+"\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Fprintf(os.Stderr, "otelcol-oracle: %d candidate files, %d accepted, %d definitions, %d rule definitions, %d references, %d route targets\n",
		len(candidates), len(files), len(defs.Definitions), len(rules.Definitions), len(refs.References), len(rw.targets))
	b, _ := json.MarshalIndent(defs, "", " ")
	os.Stdout.Write(append(b, '\n'))
}

func writeJSON(p string, v any) error {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0o644)
}

var envRef = regexp.MustCompile(`\$\{(?:env:)?([A-Za-z_][A-Za-z0-9_]*)(?::-[^}]*)?\}`)

// hostErrors are validate errors about the measuring host, not the file:
// disclosed, never a rejection.
var hostErrors = []*regexp.Regexp{
	regexp.MustCompile(`root_path is supported on linux only`),
	regexp.MustCompile(`directory must exist`),
	regexp.MustCompile(`gated under the "service\.profilesSupport" feature gate`),
}

// accept runs the collector's validate on every candidate, alone, then as a
// layer over each sibling that validates alone.
func accept(bin, subject string, candidates []string) map[string]bool {
	env := os.Environ()
	seen := map[string]bool{}
	for _, rel := range candidates {
		b, _ := os.ReadFile(filepath.Join(subject, filepath.FromSlash(rel)))
		for _, m := range envRef.FindAllStringSubmatch(string(b), -1) {
			if seen[m[1]] {
				continue
			}
			seen[m[1]] = true
			v := "placeholder"
			if strings.Contains(strings.ToUpper(m[1]), "PORT") {
				v = "4317"
			}
			env = append(env, m[1]+"="+v)
		}
	}
	run := func(files ...string) (bool, []string) {
		args := []string{"validate"}
		for _, f := range files {
			args = append(args, "--config=file:"+filepath.Join(subject, filepath.FromSlash(f)))
		}
		cmd := exec.Command(bin, args...)
		cmd.Env = env
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if cmd.Run() == nil {
			return true, nil
		}
		var hostOnly []string
		for _, line := range strings.Split(strings.TrimSpace(stderr.String()), "\n") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "Error:"))
			if line == "" {
				continue
			}
			host := false
			for _, re := range hostErrors {
				host = host || re.MatchString(line)
			}
			if !host {
				return false, []string{line}
			}
			hostOnly = append(hostOnly, line)
		}
		return true, hostOnly
	}
	ok := map[string]bool{}
	alone := map[string]bool{}
	for _, rel := range candidates {
		if pass, notes := run(rel); pass {
			ok[rel], alone[rel] = true, true
			for _, n := range notes {
				fmt.Fprintf(os.Stderr, "otelcol-oracle: %s: accepted; a host-only error disclosed: %s\n", rel, n)
			}
		}
	}
	for _, rel := range candidates {
		if ok[rel] {
			continue
		}
		var why []string
		for _, base := range candidates {
			if !alone[base] || filepath.Dir(base) != filepath.Dir(rel) {
				continue
			}
			pass, notes := run(base, rel)
			if pass {
				ok[rel] = true
				fmt.Fprintf(os.Stderr, "otelcol-oracle: %s: accepted as a layer over %s\n", rel, base)
				for _, n := range notes {
					fmt.Fprintf(os.Stderr, "otelcol-oracle: %s: a host-only error disclosed: %s\n", rel, n)
				}
				break
			}
			why = notes
		}
		if !ok[rel] {
			_, first := run(rel)
			if len(first) == 0 {
				first = why
			}
			fmt.Fprintf(os.Stderr, "otelcol-oracle: %s: refused by validate: %s\n", rel, strings.Join(first, "; "))
		}
	}
	return ok
}

// readConfig is one accepted file's definitions (the collector's decode),
// rule definitions (identities, derived metrics), references and route
// targets.
func readConfig(rel string, b []byte) ([]def, []def, []ref, []target, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(b, &root); err != nil || len(root.Content) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("yaml: %v", err)
	}
	doc := root.Content[0]
	retrieved, err := confmap.NewRetrievedFromYAML(b)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	conf, err := retrieved.AsConf()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var cfg collectorConfig
	if err := conf.Unmarshal(&cfg, confmap.WithIgnoreUnused()); err != nil {
		return nil, nil, nil, nil, err
	}
	var defs, ruleDefs []def
	at := func(n *yaml.Node) int {
		if n == nil {
			return 0
		}
		return n.Line
	}
	add := func(name, kind string, n *yaml.Node) {
		defs = append(defs, def{Path: rel, Name: name, Line: at(n), Kind: kind})
	}
	addRule := func(name, kind string, n *yaml.Node) {
		ruleDefs = append(ruleDefs, def{Path: rel, Name: name, Line: at(n), Kind: kind})
	}
	if len(doc.Content) > 0 {
		add(filepath.Base(rel), "package", doc.Content[0])
	}
	declared := map[string]bool{}
	bySection := map[string]map[component.ID]any{
		"receivers": cfg.Receivers, "processors": cfg.Processors, "exporters": cfg.Exporters,
		"connectors": cfg.Connectors, "extensions": cfg.Extensions,
	}
	var routes []target
	for _, s := range sections {
		ids := sortedIDs(bySection[s.key])
		for _, id := range ids {
			name := s.key + "/" + id.String()
			declared[name] = true
			key := keyNode(get(doc, s.key), id.String())
			add(name, s.kind, key)
			body := valueNode(get(doc, s.key), id.String())
			switch {
			case s.key == "processors" && (id.Type().String() == "resource" || id.Type().String() == "attributes"):
				list := "attributes"
				if id.Type().String() == "attributes" {
					list = "actions"
				}
				for _, it := range seq(get(body, list)) {
					if scalar(get(it, "key")) != "service.name" || get(it, "value") == nil || scalar(get(it, "action")) == "delete" {
						continue
					}
					if v := scalar(get(it, "value")); literal(v) {
						addRule(v, "key", it)
					}
				}
			case s.key == "connectors":
				for _, fam := range derivedFamilies(id, body) {
					addRule(fam, "view", key)
				}
			case s.key == "exporters":
				for _, f := range [][]string{{"endpoint"}, {"http", "endpoint"}, {"traces_endpoint"}, {"metrics_endpoint"}, {"logs_endpoint"}} {
					if v := scalar(path(body, f...)); v != "" {
						routes = append(routes, target{path: rel, from: name, raw: v})
					}
				}
				for _, e := range seq(get(body, "endpoints")) {
					if v := scalar(e); v != "" {
						routes = append(routes, target{path: rel, from: name, raw: v})
					}
				}
			}
		}
	}
	pipes := get(get(doc, "service"), "pipelines")
	var refs []ref
	seenRef := map[ref]bool{}
	addRef := func(from, to string) {
		r := ref{Path: rel, From: from, To: to}
		if !seenRef[r] {
			seenRef[r] = true
			refs = append(refs, r)
		}
	}
	var pids []pipeline.ID
	for id := range cfg.Service.Pipelines {
		pids = append(pids, id)
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i].String() < pids[j].String() })
	for _, pid := range pids {
		name := "pipelines/" + pid.String()
		add(name, "job", keyNode(pipes, pid.String()))
		p := cfg.Service.Pipelines[pid]
		if p == nil {
			continue
		}
		for _, l := range []struct {
			ids      []component.ID
			sections []string
		}{{p.Receivers, []string{"receivers", "connectors"}}, {p.Processors, []string{"processors"}}, {p.Exporters, []string{"exporters", "connectors"}}} {
			for _, id := range l.ids {
				for _, s := range l.sections {
					if to := s + "/" + id.String(); declared[to] {
						addRef(name, to)
					}
				}
			}
		}
	}
	for _, id := range cfg.Service.Extensions {
		if to := "extensions/" + id.String(); declared[to] {
			addRef(filepath.Base(rel), to)
		}
	}
	return defs, ruleDefs, refs, routes, nil
}

// derivedFamilies are the metric families a span-metrics or service-graph
// connector emits, as its source names them (collector-contrib v0.162.0).
func derivedFamilies(id component.ID, body *yaml.Node) []string {
	switch id.Type().String() {
	case "span_metrics", "spanmetrics":
		ns := "traces.span.metrics" // factory.go DefaultNamespace
		if v := scalar(get(body, "namespace")); v != "" {
			ns = v
		}
		fams := []string{ns + ".calls", ns + ".duration"}
		if scalar(path(body, "events", "enabled")) == "true" {
			fams = append(fams, ns+".events")
		}
		return fams
	case "service_graph", "servicegraph":
		return []string{"traces_service_graph_request_total", "traces_service_graph_request_failed_total",
			"traces_service_graph_request_server", "traces_service_graph_request_client"}
	}
	return nil
}

// literal: a value with no interpolation (the TL-5 contract).
func literal(v string) bool {
	if strings.TrimSpace(v) == "" {
		return false
	}
	for _, m := range []string{"${", "{{", "$("} {
		if strings.Contains(v, m) {
			return false
		}
	}
	return true
}

func sortedIDs(m map[component.ID]any) []component.ID {
	ids := make([]component.ID, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	return ids
}

func get(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func path(n *yaml.Node, keys ...string) *yaml.Node {
	for _, k := range keys {
		n = get(n, k)
	}
	return n
}

// keyNode is the key node of a mapping entry (its line is the entity's).
func keyNode(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i]
		}
	}
	return nil
}

func valueNode(n *yaml.Node, key string) *yaml.Node { return get(n, key) }

func scalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

func seq(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}
