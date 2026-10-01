package fusion

// gatewaydispatch_test.go — LB-F104: Envoy, Kong and Traefik files are claimed
// by a manifest basename or glob AND the doc spec's content sniff, in ingest
// and in ReingestFile alike; a Kubernetes manifest under a matching glob, a
// compose file and a file only its content names stay what they were.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const (
	gwEnvoyYAML = "static_resources:\n  listeners:\n  - name: web\n    address:\n      socket_address: {address: 0.0.0.0, port_value: 8080}\n  clusters:\n  - name: app\n    load_assignment:\n      endpoints:\n      - lb_endpoints:\n        - endpoint:\n            address:\n              socket_address: {address: app, port_value: 80}\n"
	gwEnvoyJSON = "{\"static_resources\": {\"clusters\": [{\"name\": \"origin\"}]}}\n"
	gwKong      = "_format_version: \"3.0\"\nservices:\n- name: echo\n  url: http://echo:1027\n"
	gwTraefik   = "entryPoints:\n  web:\n    address: \":80\"\nproviders:\n  file:\n    filename: dynamic.yml\n"
	gwDynamic   = "http:\n  routers:\n    r:\n      rule: Host(`a.example.com`)\n      service: s\n  services:\n    s:\n      loadBalancer:\n        servers:\n        - url: http://s:80\n"
	gwTOML      = "[entryPoints.web]\n  address = \":80\"\n"
	gwK8sEnvoy  = "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: envoy\nspec:\n  selector:\n    matchLabels: {app: envoy}\n  template:\n    metadata:\n      labels: {app: envoy}\n    spec:\n      containers:\n      - name: envoy\n        image: envoyproxy/envoy:v1.39.1\n        args: [\"-c\", \"/etc/envoy/envoy.yaml\"]\n"
	gwCompose   = "services:\n  envoy:\n    image: envoyproxy/envoy:v1.39.1\n"
	// An Istio EnvoyFilter carries Envoy's typed_config (the envoy sniff's
	// "@type": type.googleapis.com/envoy. signal) and its file is named for
	// Envoy, yet it is a Kubernetes object: the apiVersion-and-kind guard
	// keeps it Kubernetes.
	gwEnvoyFilter = "apiVersion: networking.istio.io/v1alpha3\nkind: EnvoyFilter\nmetadata:\n  name: ratelimit\nspec:\n  configPatches:\n  - applyTo: HTTP_FILTER\n    patch:\n      operation: INSERT_BEFORE\n      value:\n        name: envoy.filters.http.ratelimit\n        typed_config:\n          \"@type\": type.googleapis.com/envoy.extensions.filters.http.ratelimit.v3.RateLimit\n"
)

func gatewayRepoCorpus() map[string]string {
	return map[string]string{
		"envoy.yaml":                 gwEnvoyYAML,
		"front/front-envoy.yaml":     gwEnvoyYAML,
		"conf/envoy.json":            gwEnvoyJSON,
		"gw/kong.yml":                gwKong,
		"proxy/traefik.yml":          gwTraefik,
		"proxy/dynamic.yml":          gwDynamic,
		"proxy/traefik.toml":         gwTOML,
		"k8s/envoy-deployment.yaml":  gwK8sEnvoy,
		"compose/docker-compose.yml": gwCompose,
		"misc/config.yaml":           gwEnvoyYAML, // a content signal alone claims nothing
		"misc/dynamic.yml":           "name: not-traefik\nsettings:\n  mode: fast\n",
		"misc/envoy-notes.json":      "{\"note\": \"envoy\"}\n",
	}
}

func TestGatewayDispatchClaims(t *testing.T) {
	cases := []struct {
		path    string
		content string
		want    ToolKind // "" = not claimed
	}{
		{"envoy.yaml", gwEnvoyYAML, ToolEnvoy},
		{"front/front-envoy.yaml", gwEnvoyYAML, ToolEnvoy},
		{"conf/envoy.json", gwEnvoyJSON, ToolEnvoy},
		{"envoy/lds.yaml", "resources:\n- \"@type\": type.googleapis.com/envoy.config.listener.v3.Listener\n  name: l\n", ToolEnvoy},
		{"xds/cds.yaml", "resources:\n- \"@type\": type.googleapis.com/envoy.config.cluster.v3.Cluster\n  name: c\n", ToolEnvoy},
		{"xds/cds.yaml", "name: some-ci\non: push\n", ""},
		{"gw/kong.yml", gwKong, ToolKong},
		{"gw/kong3x.yaml", gwKong, ToolKong},
		{"proxy/traefik.yml", gwTraefik, ToolTraefik},
		{"proxy/dynamic.yml", gwDynamic, ToolTraefik},
		{"proxy/traefik.toml", gwTOML, ToolTraefik},
		{"k8s/envoy-deployment.yaml", gwK8sEnvoy, ""},
		{"istio/envoyfilter-ratelimit.yaml", gwEnvoyFilter, ""},
		{"compose/docker-compose.yml", gwCompose, ""},
		{"misc/config.yaml", gwEnvoyYAML, ""},
		{"misc/dynamic.yml", "name: not-traefik\n", ""},
		{"envoy.yaml", "name: just-a-file\n", ""},
		{"kong.yml", "services:\n  web:\n    image: kong\n", ""},
		{"traefik.yml", "services:\n  web:\n    image: traefik\n", ""},
	}
	for _, c := range cases {
		e, tk, ok := gatewayDispatch(c.path, []byte(c.content))
		if c.want == "" {
			if ok || e != nil {
				t.Errorf("gatewayDispatch(%q) claimed it as %q; want unclaimed", c.path, tk)
			}
			continue
		}
		if !ok || tk != c.want || e == nil {
			t.Errorf("gatewayDispatch(%q) = (%v, %q, %v), want %q", c.path, e, tk, ok, c.want)
		}
	}
}

func gatewayKindsBySource(t *testing.T, st store.Store, svc string) map[string]string {
	t.Helper()
	obs, err := st.QueryObservations(context.Background(), svc, "")
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]map[string]bool{}
	for _, o := range obs {
		if by[o.SourceRef] == nil {
			by[o.SourceRef] = map[string]bool{}
		}
		by[o.SourceRef][string(o.SourceKind)] = true
	}
	out := map[string]string{}
	for ref, ks := range by {
		var l []string
		for k := range ks {
			l = append(l, k)
		}
		sort.Strings(l)
		out[ref] = l[0]
	}
	return out
}

// TestIngest_GatewayRepo: ingest reads each claimed file through its dialect
// and leaves the neighbours alone; the Traefik static file's providers.file
// filename becomes a doc-xref edge to the dynamic file.
func TestIngest_GatewayRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, gatewayRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "gw", Path: dir}); err != nil {
		t.Fatal(err)
	}
	got := gatewayKindsBySource(t, st, "gw")
	for rel, want := range map[string]string{
		"envoy.yaml":                "EnvoyCluster",
		"front/front-envoy.yaml":    "EnvoyCluster",
		"conf/envoy.json":           "EnvoyCluster",
		"gw/kong.yml":               "KongService",
		"proxy/traefik.yml":         "TraefikConfig",
		"proxy/dynamic.yml":         "TraefikConfig",
		"proxy/traefik.toml":        "TraefikConfig",
		"k8s/envoy-deployment.yaml": "K8s",
	} {
		if got[rel] != want && !(want == "K8s" && strings.HasPrefix(got[rel], "K8s")) {
			t.Errorf("%s read as %q, want %q", rel, got[rel], want)
		}
	}
	for _, rel := range []string{"misc/config.yaml", "misc/dynamic.yml", "compose/docker-compose.yml", "misc/envoy-notes.json"} {
		switch got[rel] {
		case "EnvoyCluster", "EnvoyConfig", "TraefikConfig", "KongService":
			t.Errorf("%s was claimed by a gateway dialect (%s)", rel, got[rel])
		}
	}
	all, _ := st.TypedEdges(ctx, "gw")
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved && e.FromFile == "proxy/traefik.yml" {
			edges = append(edges, e.FromFile+"->"+e.ToSymbol)
		}
	}
	if len(edges) != 1 {
		t.Errorf("doc-xref edges from proxy/traefik.yml = %q, want one to the dynamic file's package", edges)
	}
}

// TestReingestFile_GatewayRepo: a one-file re-ingest decides what ingest did.
func TestReingestFile_GatewayRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, gatewayRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "gw", Path: dir}); err != nil {
		t.Fatal(err)
	}
	before := gatewayKindsBySource(t, st, "gw")
	edits := map[string]string{
		"envoy.yaml":                gwEnvoyYAML + "  - name: extra\n    type: STATIC\n",
		"conf/envoy.json":           "{\"static_resources\": {\"clusters\": [{\"name\": \"origin\"}, {\"name\": \"second\"}]}}\n",
		"gw/kong.yml":               gwKong + "- name: two\n  host: two.example.com\n",
		"proxy/traefik.toml":        gwTOML + "[entryPoints.websecure]\n  address = \":443\"\n",
		"k8s/envoy-deployment.yaml": gwK8sEnvoy + "# edited\n",
	}
	for rel, body := range edits {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(ctx, st, "gw", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	after := gatewayKindsBySource(t, st, "gw")
	if !reflect.DeepEqual(before, after) {
		t.Errorf("ReingestFile changed what a file reads as\nbefore: %v\n after: %v", before, after)
	}
	obs, _ := st.QueryObservations(ctx, "gw", "")
	names := map[string]bool{}
	for _, o := range obs {
		if o.SourceRef == "conf/envoy.json" || o.SourceRef == "proxy/traefik.toml" || o.SourceRef == "gw/kong.yml" || o.SourceRef == "envoy.yaml" {
			names[o.SourceRef+"|"+o.Symbol] = true
		}
	}
	for _, w := range []string{"conf/envoy.json|second", "proxy/traefik.toml|websecure", "gw/kong.yml|two", "envoy.yaml|extra"} {
		if !names[w] {
			t.Errorf("after ReingestFile %s is missing", w)
		}
	}
}
