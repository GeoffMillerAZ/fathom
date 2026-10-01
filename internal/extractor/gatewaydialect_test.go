package extractor

// gatewaydialect_test.go — goldens for the envoy, kong and traefik doc-engine
// dialects (LB-Q-envoy, LB-Q-kong, LB-Q-traefik; LB-F105..LB-F107): entities,
// the intra-format references an entity's Calls carry, the cross-file
// references (an xDS path, a service name) and the route targets (strategy
// route-host, edge route_ref) a cluster, service or upstream emits. A fixture
// is read as if it sat in infra/<name>. Goldens are hand-written from reading
// the fixtures (design §13.2 step 10).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var gatewayDialects = []string{"envoy", "kong", "traefik"}

func gatewayFixtures(t *testing.T, id string) []string {
	t.Helper()
	dir := filepath.Join("dialects", id, "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".golden.json") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	if len(out) < 3 {
		t.Fatalf("%s: %d fixtures, want at least 3", dir, len(out))
	}
	return out
}

// gatewayExtract reads one fixture as if it sat in infra/<name> and returns
// its entities, references and cross-file references in the golden's shape.
func gatewayExtract(t *testing.T, x *DocExtractor, fx string) cfnGolden {
	t.Helper()
	rel := "infra/" + filepath.Base(fx)
	src, err := os.ReadFile(fx)
	if err != nil {
		t.Fatal(err)
	}
	res, err := x.ExtractDoc("svc", rel, src)
	if err != nil {
		t.Fatalf("ExtractDoc: %v", err)
	}
	got := cfnGolden{Symbols: []cfnGoldenSymbol{}, CrossFile: []cfnGoldenCross{}}
	for _, n := range res.Nodes {
		if n.Service != "svc" || n.FilePath != rel {
			t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
		}
		got.Symbols = append(got.Symbols, cfnGoldenSymbol{
			Name: n.Name, Kind: string(n.Kind), Package: n.Package,
			Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
			Calls: n.Calls, Tier: tierName(n.Tier),
		})
	}
	for _, c := range res.CrossFile {
		got.CrossFile = append(got.CrossFile, cfnGoldenCross{
			From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
		})
	}
	return got
}

// TestGatewayDialectGoldens: every fixture's entities, references and
// cross-file references (route targets included) against its hand-written
// golden. The fixtures are looped in the test's own body, not subtests, so a
// plan row's reachable_from can name this test (the call graph does not enter
// a t.Run closure).
func TestGatewayDialectGoldens(t *testing.T) {
	for _, id := range gatewayDialects {
		x, err := NewDocDialectExtractor(id)
		if err != nil {
			t.Fatalf("NewDocDialectExtractor(%s): %v", id, err)
		}
		for _, fx := range gatewayFixtures(t, id) {
			got := gatewayExtract(t, x, fx)
			gp := fx + ".golden.json"
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want cfnGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []cfnGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []cfnGoldenCross{}
			}
			if !reflect.DeepEqual(got, want) {
				gj, _ := json.MarshalIndent(got, "", "  ")
				t.Errorf("%s/%s: extraction differs from %s\ngot:\n%s", id, filepath.Base(fx), gp, gj)
			}
		}
	}
}

// TestGatewayDetectsByContent: each dialect's detect.content_sniff claims its
// own documents and none of the neighbouring formats' or a Kubernetes
// manifest's (fusion's dispatch pairs it with the manifest's basenames and
// globs, never alone: LB-F104).
func TestGatewayDetectsByContent(t *testing.T) {
	yes := map[string][]string{
		"envoy": {
			"static_resources:\n  listeners: []\n",
			"dynamic_resources:\n  cds_config: {}\n",
			"{\"static_resources\": {}}",
			"version_info: '1'\nresources:\n- \"@type\": type.googleapis.com/envoy.config.cluster.v3.Cluster\n  name: a\n",
		},
		"kong": {
			"_format_version: \"3.0\"\nservices: []\n",
			"{\"_format_version\": \"1.1\"}",
		},
		"traefik": {
			"http:\n  routers: {}\n",
			"tcp:\n  routers: {}\n",
			"udp:\n  services: {}\n",
			"entryPoints:\n  web:\n    address: \":80\"\n",
			"providers:\n  file:\n    directory: /etc/traefik\n",
			"[http.routers.r]\n  rule = \"Host(`a`)\"\n",
			"[entryPoints.web]\n  address = \":80\"\n",
			"tls:\n  options: {}\n",
		},
	}
	no := []string{
		"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: envoy\nspec:\n  template:\n    spec:\n      containers:\n      - name: envoy\n        args: [\"--config-path\", \"/etc/envoy/envoy.yaml\"]\n",
		"services:\n  web:\n    image: envoyproxy/envoy:v1.39\n",
		"name: ci\non: push\njobs:\n  b:\n    runs-on: ubuntu-latest\n",
		"# static_resources are described in the docs\nkey: value\n",
	}
	for _, id := range gatewayDialects {
		x, err := NewDocDialectExtractor(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range yes[id] {
			if !x.DetectsContent([]byte(s)) {
				t.Errorf("%s: sniff missed %q", id, s)
			}
		}
		for _, s := range no {
			if x.DetectsContent([]byte(s)) {
				t.Errorf("%s: sniff claimed %q", id, s)
			}
		}
	}
	// A kong file never reads as envoy or traefik, and the reverse.
	for id, docs := range yes {
		for _, other := range gatewayDialects {
			if other == id {
				continue
			}
			x, _ := NewDocDialectExtractor(other)
			for _, s := range docs {
				if x.DetectsContent([]byte(s)) {
					t.Errorf("%s sniff claimed a %s document %q", other, id, s)
				}
			}
		}
	}
}

// TestLooksTOML: a doc spec that decodes yaml reads a file that opens as a
// TOML document with the TOML decoder (Traefik's static and dynamic files come
// in both), and leaves every YAML document alone.
func TestLooksTOML(t *testing.T) {
	for src, want := range map[string]bool{
		"[entryPoints.web]\n  address = \":80\"\n": true,
		"# comment\n\n[[http.routers]]\n":          true,
		"defaultEntryPoints = [\"http\"]\n[web]\n": true,
		"\"quoted.key\" = 1\n":                     true,
		"http:\n  routers: {}\n":                   false,
		"- a\n- b\n":                               false,
		"key: value\n":                             false,
		"# only a comment\n":                       false,
		"services:\n  web:\n    command: a = b\n":  false,
		"[unclosed flow sequence\n":                false,
		"":                                         false,
	} {
		if got := looksTOML([]byte(src)); got != want {
			t.Errorf("looksTOML(%q) = %v, want %v", src, got, want)
		}
	}
	x, err := NewDocDialectExtractor("traefik")
	if err != nil {
		t.Fatal(err)
	}
	// a TOML-looking file that is not valid TOML falls through to the YAML decoder, which refuses it
	if res, _ := x.ExtractDoc("s", "bad.toml", []byte("[http.routers.r\n  rule = 1\n")); len(res.Nodes) != 0 {
		t.Errorf("a broken TOML file yielded %d nodes", len(res.Nodes))
	}
}
